package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

const (
	loginIPLimit       = 10
	loginIPWindow      = 15 * time.Minute
	loginAccountLimit  = 5
	loginAccountWindow = 15 * time.Minute
	registerIPLimit    = 5
	registerIPWindow   = time.Hour
)

// dummyHash is verified when an account doesn't exist so response timing doesn't reveal it.
var dummyHash, _ = crypto.HashPassword("qrit-timing-equaliser-password", crypto.UserPasswordConfig)

func validEmail(s string) (string, bool) {
	s = trimLower(s)
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || !strings.Contains(s[strings.LastIndex(s, "@"):], ".") {
		return "", false
	}
	return s, true
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if !decode(w, r, &req) {
		return
	}
	if ok, _, _, _ := s.limiter.Allow(r.Context(), "rl:register:"+ipKey(r), registerIPLimit, registerIPWindow); !ok {
		fail(w, tooMany("too many sign-ups from this network; try again later", registerIPWindow))
		return
	}
	emailAddr, ok := validEmail(req.Email)
	if !ok {
		fail(w, unprocessable("invalid_email", "a valid email address is required"))
		return
	}
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		fail(w, unprocessable("weak_password", err.Error()))
		return
	}
	if err := s.checkPasswordPolicyForEmail(r.Context(), emailAddr, req.Password); err != nil {
		fail(w, err)
		return
	}
	hash, err := crypto.HashPassword(req.Password, crypto.UserPasswordConfig)
	if err != nil {
		fail(w, apierr.Internal("failed to hash password"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.Split(emailAddr, "@")[0]
	}

	var user dbgen.User
	var ws dbgen.Workspace
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		user, err = q.CreateUser(r.Context(), dbgen.CreateUserParams{
			ID: idgen.New(), Email: emailAddr, PasswordHash: &hash, Name: name, Locale: "en", Timezone: "UTC",
		})
		if err != nil {
			return err
		}
		ws, err = s.createWorkspace(r.Context(), q, tx, user, name+"'s Workspace", "", nil)
		return err
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation && strings.Contains(pgConstraint(err), "email") {
			fail(w, apierr.Conflict("email_exists", "an account with this email already exists"))
			return
		}
		slog.Error("register failed", "error", err)
		fail(w, apierr.Internal("failed to create account"))
		return
	}
	s.issueVerificationEmail(r.Context(), user)
	token, err := s.startSession(w, r, user.ID, "password", nil)
	if err != nil {
		fail(w, apierr.Internal("failed to create session"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"user": toUserDTO(user), "workspace": toWorkspaceDTO(ws, "owner"), "token": token,
	})
}

// createWorkspace inserts a workspace with a unique slug and runs the provisioner.
func (s *Server) createWorkspace(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, owner dbgen.User, name, slug string, orgID *uuid.UUID) (dbgen.Workspace, error) {
	base := slug
	if base == "" {
		base = workspace.Slugify(name)
	}
	if err := workspace.ValidateSlug(base); err != nil {
		base = "workspace"
	}
	candidate := base
	for i := 0; i < 8; i++ {
		exists, err := q.SlugExists(ctx, candidate)
		if err != nil {
			return dbgen.Workspace{}, err
		}
		if !exists {
			break
		}
		if slug != "" {
			return dbgen.Workspace{}, apierr.Conflict("slug_exists", "a workspace with this slug already exists")
		}
		suffix, _ := crypto.RandomBytes(3)
		candidate = base + "-" + strings.ToLower(hexString(suffix))
	}
	ws, err := q.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID: idgen.New(), Name: strings.TrimSpace(name), Slug: candidate, OwnerID: owner.ID, PlanID: "free",
		Timezone: "UTC", Brand: []byte("{}"), Settings: []byte("{}"),
	})
	if err != nil {
		return ws, err
	}
	if err := s.prov().ProvisionWorkspace(ctx, q, tx, owner, ws, orgID); err != nil {
		return ws, err
	}
	return ws, nil
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !decode(w, r, &req) {
		return
	}
	emailAddr := trimLower(req.Email)
	if ok, _, retry, _ := s.limiter.Allow(r.Context(), "rl:login:ip:"+ipKey(r), loginIPLimit, loginIPWindow); !ok {
		fail(w, tooMany("too many sign-in attempts; try again later", retry))
		return
	}
	lockKey := "lock:login:" + emailAddr
	if s.rdb != nil {
		if n, _ := s.rdb.Get(r.Context(), lockKey).Int(); n >= loginAccountLimit {
			fail(w, tooMany("this account is temporarily locked after repeated failures; try again in 15 minutes", loginAccountWindow))
			return
		}
	}
	u, err := s.q.GetUserByEmail(r.Context(), emailAddr)
	hash := dummyHash
	if err == nil && u.PasswordHash != nil {
		hash = *u.PasswordHash
	}
	ok, verr := crypto.VerifyPassword(req.Password, hash)
	if err != nil || u.PasswordHash == nil || verr != nil || !ok {
		if s.rdb != nil && emailAddr != "" {
			pipe := s.rdb.TxPipeline()
			pipe.Incr(r.Context(), lockKey)
			pipe.Expire(r.Context(), lockKey, loginAccountWindow)
			_, _ = pipe.Exec(r.Context())
		}
		fail(w, apierr.Unauthorized("invalid email or password"))
		return
	}
	if s.rdb != nil {
		_ = s.rdb.Del(r.Context(), lockKey).Err()
	}
	if err := s.checkLoginAllowed(r.Context(), u, "password"); err != nil {
		fail(w, err)
		return
	}
	token, err := s.startSession(w, r, u.ID, "password", nil)
	if err != nil {
		fail(w, apierr.Internal("failed to create session"))
		return
	}
	_ = s.q.SetLastLogin(r.Context(), u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserDTO(u), "token": token, "mfa_required": s.mfaPending(r.Context(), u.ID)})
}

// startSession creates a session row and sets cookies; returns the access token.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID, method string, ssoConnectionID *uuid.UUID) (string, error) {
	plainRefresh, refreshHash, err := auth.GenerateRandomToken(32)
	if err != nil {
		return "", err
	}
	ua := truncate(r.UserAgent(), 300)
	sess, err := s.q.CreateSession(r.Context(), dbgen.CreateSessionParams{
		ID: idgen.New(), UserID: userID, FamilyID: idgen.New(), RefreshTokenHash: refreshHash,
		UserAgent: &ua, IpPrefix: ipPrefix(r), ExpiresAt: time.Now().UTC().Add(auth.RefreshTokenDuration),
	})
	if err != nil {
		return "", err
	}
	if err := s.idh().SessionCreated(r.Context(), sess, method, ssoConnectionID); err != nil {
		return "", err
	}
	access, err := s.tm.CreateAccessToken(userID, sess.ID)
	if err != nil {
		return "", err
	}
	csrf, _, _ := auth.GenerateRandomToken(16)
	auth.SetAuthCookies(w, access, plainRefresh, csrf, s.cfg.CookieSecure)
	return access, nil
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if p, ok := auth.GetPrincipal(r.Context()); ok && p.SessionID != uuid.Nil {
		_ = s.q.RevokeSession(r.Context(), p.SessionID)
		s.sessions.evict(p.SessionID)
	}
	auth.ClearAuthCookies(w, s.cfg.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var tokenStr string
	if c, err := r.Cookie(auth.RefreshCookieName); err == nil {
		tokenStr = c.Value
	}
	if tokenStr == "" {
		tokenStr = r.Header.Get("X-Refresh-Token")
	}
	if tokenStr == "" {
		fail(w, apierr.Unauthorized("refresh token required"))
		return
	}
	sess, err := s.q.GetSessionByHash(r.Context(), auth.HashToken(tokenStr))
	if err != nil {
		fail(w, apierr.Unauthorized("invalid refresh token"))
		return
	}
	if sess.ReplacedBy.Valid || sess.RevokedAt.Valid {
		_ = s.q.RevokeSessionFamily(r.Context(), sess.FamilyID)
		auth.ClearAuthCookies(w, s.cfg.CookieSecure)
		fail(w, apierr.Unauthorized("refresh token reuse detected; all sessions in this family were revoked"))
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		fail(w, apierr.New(http.StatusUnauthorized, "session_expired", "Unauthorized", "session expired"))
		return
	}
	if err := s.sessPolicy().CheckRefresh(r.Context(), sess); err != nil {
		_ = s.q.RevokeSession(r.Context(), sess.ID)
		auth.ClearAuthCookies(w, s.cfg.CookieSecure)
		fail(w, err)
		return
	}
	plainRefresh, newHash, err := auth.GenerateRandomToken(32)
	if err != nil {
		fail(w, apierr.Internal("failed to rotate session"))
		return
	}
	var newSess dbgen.Session
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		// Row lock prevents two concurrent refreshes both succeeding.
		var replaced *uuid.UUID
		if err := tx.QueryRow(r.Context(), `SELECT replaced_by FROM sessions WHERE id = $1 FOR UPDATE`, sess.ID).Scan(&replaced); err != nil {
			return err
		}
		if replaced != nil {
			return errReuse
		}
		ua := truncate(r.UserAgent(), 300)
		newSess, err = q.CreateSession(r.Context(), dbgen.CreateSessionParams{
			ID: idgen.New(), UserID: sess.UserID, FamilyID: sess.FamilyID, RefreshTokenHash: newHash,
			UserAgent: &ua, IpPrefix: ipPrefix(r), ExpiresAt: sess.ExpiresAt,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `UPDATE sessions SET replaced_by = $2, last_used_at = now() WHERE id = $1`, sess.ID, newSess.ID); err != nil {
			return err
		}
		// Carry enterprise session attributes (auth method, MFA, step-up) to the rotated session.
		_, err = tx.Exec(r.Context(), `
			UPDATE sessions n SET created_at = o.created_at
			FROM sessions o WHERE n.id = $1 AND o.id = $2`, newSess.ID, sess.ID)
		if err != nil {
			return err
		}
		return s.idh().SessionRotated(r.Context(), tx, sess.ID, newSess.ID)
	})
	if errors.Is(err, errReuse) {
		_ = s.q.RevokeSessionFamily(r.Context(), sess.FamilyID)
		auth.ClearAuthCookies(w, s.cfg.CookieSecure)
		fail(w, apierr.Unauthorized("refresh token reuse detected; all sessions in this family were revoked"))
		return
	}
	if err != nil {
		slog.Error("refresh failed", "error", err)
		fail(w, apierr.Internal("failed to rotate session"))
		return
	}
	access, err := s.tm.CreateAccessToken(sess.UserID, newSess.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to sign token"))
		return
	}
	csrf, _, _ := auth.GenerateRandomToken(16)
	auth.SetAuthCookies(w, access, plainRefresh, csrf, s.cfg.CookieSecure)
	writeJSON(w, http.StatusOK, map[string]any{"token": access})
}

var errReuse = errors.New("refresh token reuse")

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.NotFound("user not found"))
		return
	}
	rows, err := s.q.ListWorkspacesForUser(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list workspaces"))
		return
	}
	list := make([]workspaceDTO, 0, len(rows))
	for _, row := range rows {
		list = append(list, toWorkspaceDTO(dbgen.Workspace{ID: row.ID, Name: row.Name, Slug: row.Slug, OwnerID: row.OwnerID,
			PlanID: row.PlanID, Timezone: row.Timezone, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, row.Role))
	}
	resp := map[string]any{"user": toUserDTO(u), "workspaces": list}
	for k, v := range s.idh().MeExtras(r.Context(), u) {
		resp[k] = v
	}
	writeJSON(w, http.StatusOK, resp)
}

type updateMeReq struct {
	Name     *string `json:"name"`
	Locale   *string `json:"locale"`
	Timezone *string `json:"timezone"`
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var req updateMeReq
	if !decode(w, r, &req) {
		return
	}
	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil {
			fail(w, unprocessable("invalid_timezone", "timezone must be an IANA zone name"))
			return
		}
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 120 {
			fail(w, unprocessable("invalid_name", "name must be 1–120 characters"))
			return
		}
		req.Name = &n
	}
	u, err := s.q.UpdateUser(r.Context(), dbgen.UpdateUserParams{
		ID: principal(r).UserID, Name: derefOr(req.Name, ""), AvatarUrl: nil,
		Locale: derefOr(req.Locale, ""), Timezone: derefOr(req.Timezone, ""),
	})
	if err != nil {
		fail(w, apierr.Internal("failed to update profile"))
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	rows, err := s.q.ListUserSessions(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list sessions"))
		return
	}
	type sessDTO struct {
		ID         uuid.UUID `json:"id"`
		UserAgent  *string   `json:"user_agent"`
		IPPrefix   *string   `json:"ip_prefix"`
		CreatedAt  time.Time `json:"created_at"`
		LastUsedAt time.Time `json:"last_used_at"`
		Current    bool      `json:"current"`
	}
	out := make([]sessDTO, 0, len(rows))
	for _, s := range rows {
		out = append(out, sessDTO{ID: s.ID, UserAgent: s.UserAgent, IPPrefix: s.IpPrefix, CreatedAt: s.CreatedAt,
			LastUsedAt: s.LastUsedAt, Current: s.ID == p.SessionID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("session not found"))
		return
	}
	n, err := s.q.RevokeUserSession(r.Context(), dbgen.RevokeUserSessionParams{ID: id, UserID: principal(r).UserID})
	if err != nil || n == 0 {
		fail(w, apierr.NotFound("session not found"))
		return
	}
	s.sessions.evict(id)
	w.WriteHeader(http.StatusNoContent)
}

func derefOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}
