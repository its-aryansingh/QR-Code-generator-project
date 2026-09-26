package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

func (s *Server) mfaRoutes(r chi.Router) {
	r.Get("/me/mfa", s.handleMFAStatus)
	r.Post("/me/mfa/totp", s.handleTOTPBegin)
	r.Post("/me/mfa/totp/confirm", s.handleTOTPConfirm)
	r.With(s.requireStepUp(10*time.Minute)).Delete("/me/mfa/{id}", s.handleDeleteFactor)
	r.With(s.requireStepUp(10*time.Minute)).Post("/me/mfa/recovery-codes", s.handleRegenerateRecoveryCodes)
	r.Post("/auth/mfa/verify", s.handleMFAVerify)
}

type factorDTO struct {
	ID         uuid.UUID  `json:"id"`
	Kind       string     `json:"kind"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	rows, err := s.pool.Query(r.Context(), `SELECT id, kind, name, created_at, last_used_at FROM user_mfa_factors
		WHERE user_id = $1 ORDER BY created_at`, p.UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list factors"))
		return
	}
	defer rows.Close()
	out := []factorDTO{}
	for rows.Next() {
		var f factorDTO
		if rows.Scan(&f.ID, &f.Kind, &f.Name, &f.CreatedAt, &f.LastUsedAt) == nil {
			out = append(out, f)
		}
	}
	var codes int
	_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, p.UserID).Scan(&codes)
	var required bool
	_ = s.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM org_members m JOIN org_security_policies sp ON sp.org_id = m.org_id
		WHERE m.user_id = $1 AND m.status = 'active' AND sp.require_mfa)`, p.UserID).Scan(&required)
	sess, _ := currentSession(r)
	writeJSON(w, http.StatusOK, map[string]any{"factors": out, "recovery_codes_remaining": codes, "required_by_org": required,
		"session_verified": sess.MfaVerifiedAt.Valid, "webauthn_supported": false})
}

type totpBeginReq struct {
	Name string `json:"name"`
}

type totpEnrollment struct {
	Secret string `json:"secret"`
	Name   string `json:"name"`
}

// POST /me/mfa/totp: start enrolment; the secret is held for 10 minutes until confirmed.
func (s *Server) handleTOTPBegin(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	var req totpBeginReq
	if !decodeOptional(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Authenticator app"
	}
	if len(name) > 60 {
		fail(w, unprocessable("invalid_name", "name must be at most 60 characters"))
		return
	}
	if s.rdb == nil {
		fail(w, apierr.Internal("enrolment store unavailable"))
		return
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Unauthorized("user not found"))
		return
	}
	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		fail(w, apierr.Internal("failed to create secret"))
		return
	}
	b, _ := json.Marshal(totpEnrollment{Secret: secret, Name: name})
	if err := s.rdb.Set(r.Context(), "mfa:enroll:"+p.UserID.String(), b, 10*time.Minute).Err(); err != nil {
		fail(w, apierr.Internal("failed to start enrolment"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secret": secret, "otpauth_uri": auth.GenerateKeyURI("QRit", u.Email, secret),
		"expires_in": 600})
}

type codeReq struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// POST /me/mfa/totp/confirm: prove the app works; the first factor also issues recovery codes.
func (s *Server) handleTOTPConfirm(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	var req codeReq
	if !decode(w, r, &req) {
		return
	}
	if s.rdb == nil {
		fail(w, apierr.Internal("enrolment store unavailable"))
		return
	}
	raw, err := s.rdb.Get(r.Context(), "mfa:enroll:"+p.UserID.String()).Bytes()
	if err != nil {
		fail(w, apierr.New(http.StatusGone, "enrolment_expired", "Gone", "start the set-up again"))
		return
	}
	var en totpEnrollment
	_ = json.Unmarshal(raw, &en)
	if !auth.VerifyTOTPCode(en.Secret, strings.TrimSpace(req.Code), time.Now()) {
		fail(w, unprocessable("invalid_mfa_code", "that code doesn't match; check the time on your phone and try again"))
		return
	}
	ct, err := s.keyring.SealPlatform([]byte(en.Secret), "totp:"+p.UserID.String())
	if err != nil {
		fail(w, apierr.Internal("failed to store factor"))
		return
	}
	var codes []string
	id := idgen.New()
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var existing int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM user_mfa_factors WHERE user_id = $1`, p.UserID).Scan(&existing); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO user_mfa_factors (id, user_id, kind, name, totp_secret_ct) VALUES ($1, $2, 'totp', $3, $4)`,
			id, p.UserID, en.Name, ct); err != nil {
			return err
		}
		if existing == 0 {
			var err error
			if codes, err = replaceRecoveryCodes(r, tx, p.UserID); err != nil {
				return err
			}
		}
		if sess, ok := currentSession(r); ok {
			if _, err := tx.Exec(r.Context(), `UPDATE sessions SET mfa_verified_at = now(), mfa_pending = false WHERE id = $1`, sess.ID); err != nil {
				return err
			}
		}
		return s.aud().Record(r.Context(), q, auditUserEntry(r, p.UserID, "auth.mfa.enrolled"))
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to enrol factor"))
		return
	}
	_ = s.rdb.Del(r.Context(), "mfa:enroll:"+p.UserID.String()).Err()
	if sess, ok := currentSession(r); ok {
		s.sessions.evict(sess.ID)
	}
	resp := map[string]any{"factor": factorDTO{ID: id, Kind: "totp", Name: en.Name, CreatedAt: time.Now().UTC()}}
	if codes != nil {
		resp["recovery_codes"] = codes
	}
	writeJSON(w, http.StatusCreated, resp)
}

func replaceRecoveryCodes(r *http.Request, tx pgx.Tx, userID uuid.UUID) ([]string, error) {
	plain, hashed, err := auth.GenerateRecoveryCodes(10)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM user_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}
	for _, h := range hashed {
		if _, err := tx.Exec(r.Context(), `INSERT INTO user_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, userID, hashHex(h)); err != nil {
			return nil, err
		}
	}
	return plain, nil
}

func (s *Server) handleRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if !s.idh().MFAPending(r.Context(), p.UserID) {
		fail(w, apierr.Conflict("mfa_not_enabled", "set up an authenticator app first"))
		return
	}
	var codes []string
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		if codes, err = replaceRecoveryCodes(r, tx, p.UserID); err != nil {
			return err
		}
		return s.aud().Record(r.Context(), q, auditUserEntry(r, p.UserID, "auth.mfa.recovery_codes_regenerated"))
	})
	if err != nil {
		fail(w, apierr.Internal("failed to regenerate codes"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (s *Server) handleDeleteFactor(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("factor not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var total int
		if err := tx.QueryRow(r.Context(), `SELECT count(*) FROM user_mfa_factors WHERE user_id = $1`, p.UserID).Scan(&total); err != nil {
			return err
		}
		if total == 1 {
			var required bool
			_ = tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM org_members m JOIN org_security_policies sp ON sp.org_id = m.org_id
				WHERE m.user_id = $1 AND m.status = 'active' AND sp.require_mfa)`, p.UserID).Scan(&required)
			if required {
				return apierr.Conflict("mfa_required_by_org", "an organisation you belong to requires two-factor authentication; add another factor first")
			}
		}
		tag, err := tx.Exec(r.Context(), `DELETE FROM user_mfa_factors WHERE id = $1 AND user_id = $2`, id, p.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("factor not found")
		}
		if total == 1 {
			if _, err := tx.Exec(r.Context(), `DELETE FROM user_recovery_codes WHERE user_id = $1`, p.UserID); err != nil {
				return err
			}
		}
		return s.aud().Record(r.Context(), q, auditUserEntry(r, p.UserID, "auth.mfa.removed"))
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to remove factor"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /auth/mfa/verify: second step of sign-in (TOTP or recovery code).
func (s *Server) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	sess, ok := currentSession(r)
	if p == nil || !ok {
		fail(w, apierr.Unauthorized("sign in first"))
		return
	}
	if okS, _, retry, _ := s.limiter.Allow(r.Context(), "rl:mfa:s:"+sess.ID.String(), 5, 15*time.Minute); !okS {
		fail(w, tooMany("too many attempts; try again in 15 minutes", retry))
		return
	}
	if okU, _, retry, _ := s.limiter.Allow(r.Context(), "rl:mfa:u:"+p.UserID.String(), 20, 24*time.Hour); !okU {
		fail(w, tooMany("too many attempts today; use a recovery code later or contact your administrator", retry))
		return
	}
	var req codeReq
	if !decode(w, r, &req) {
		return
	}
	code := req.Code
	method := "totp"
	if req.RecoveryCode != "" {
		code, method = req.RecoveryCode, "recovery_code"
	}
	good, err := s.checkSecondFactor(r.Context(), p.UserID, code)
	if err != nil {
		fail(w, apierr.Internal("failed to verify code"))
		return
	}
	if !good {
		_ = s.aud().Record(r.Context(), s.q, auditUserEntry(r, p.UserID, "auth.mfa.failed"))
		fail(w, apierr.New(http.StatusUnauthorized, "invalid_mfa_code", "Unauthorized", "that code is not valid"))
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE sessions SET mfa_verified_at = now(), mfa_pending = false WHERE id = $1`, sess.ID); err != nil {
		fail(w, apierr.Internal("failed to record verification"))
		return
	}
	s.sessions.evict(sess.ID)
	e := auditUserEntry(r, p.UserID, "auth.mfa.verified")
	e.Changes = map[string]any{"method": method}
	_ = s.aud().Record(r.Context(), s.q, e)
	var remaining int
	_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL`, p.UserID).Scan(&remaining)
	writeJSON(w, http.StatusOK, map[string]any{"verified": true, "recovery_codes_remaining": remaining})
}
