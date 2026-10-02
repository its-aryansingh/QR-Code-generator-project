package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

const (
	resetTokenTTL     = time.Hour
	forgotIPLimit     = 10
	forgotEmailLimit  = 3
	forgotWindow      = time.Hour
	resendVerifyLimit = 5
)

type tokenReq struct {
	Token string `json:"token"`
}

// POST /auth/verify-email {token}
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req tokenReq
	if !decode(w, r, &req) {
		return
	}
	tok, err := s.q.GetValidEmailToken(r.Context(), dbgen.GetValidEmailTokenParams{
		TokenHash: auth.HashToken(req.Token), Purpose: "verify_email",
	})
	if err != nil {
		fail(w, apierr.New(http.StatusGone, "token_invalid", "Gone", "this verification link is invalid or has expired"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		n, err := q.ConsumeEmailToken(r.Context(), tok.ID)
		if err != nil {
			return err
		}
		if n == 0 {
			return apierr.New(http.StatusGone, "token_invalid", "Gone", "this verification link was already used")
		}
		return q.MarkEmailVerified(r.Context(), tok.UserID)
	})
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true})
}

// POST /auth/resend-verification (signed in)
func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if ok, _, retry, _ := s.limiter.Allow(r.Context(), "rl:verify:"+p.UserID.String(), resendVerifyLimit, time.Hour); !ok {
		fail(w, tooMany("too many verification emails; try again later", retry))
		return
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.NotFound("user not found"))
		return
	}
	if u.EmailVerifiedAt.Valid {
		writeJSON(w, http.StatusOK, map[string]any{"verified": true})
		return
	}
	s.issueVerificationEmail(r.Context(), u)
	writeJSON(w, http.StatusAccepted, map[string]any{"sent": true})
}

type forgotReq struct {
	Email string `json:"email"`
}

// POST /auth/password/forgot {email}: always 202 so account existence is not revealed.
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotReq
	if !decode(w, r, &req) {
		return
	}
	if ok, _, retry, _ := s.limiter.Allow(r.Context(), "rl:forgot:ip:"+ipKey(r), forgotIPLimit, forgotWindow); !ok {
		fail(w, tooMany("too many password reset requests; try again later", retry))
		return
	}
	emailAddr, valid := validEmail(req.Email)
	accepted := map[string]any{"accepted": true}
	if !valid {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	if ok, _, _, _ := s.limiter.Allow(r.Context(), "rl:forgot:email:"+emailAddr, forgotEmailLimit, forgotWindow); !ok {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	u, err := s.q.GetUserByEmail(r.Context(), emailAddr)
	if err != nil {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	// SSO-enforced accounts cannot reset a password they are not allowed to use.
	if err := s.checkLoginAllowed(r.Context(), u, "password"); err != nil {
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	plain, hash, err := auth.GenerateRandomToken(32)
	if err == nil {
		_ = s.q.InvalidateEmailTokens(r.Context(), dbgen.InvalidateEmailTokensParams{UserID: u.ID, Purpose: "reset_password"})
		_, err = s.q.CreateEmailToken(r.Context(), dbgen.CreateEmailTokenParams{
			ID: idgen.New(), UserID: u.ID, Purpose: "reset_password", TokenHash: hash,
			ExpiresAt: time.Now().UTC().Add(resetTokenTTL),
		})
	}
	if err != nil {
		slog.Error("reset token", "error", err)
		writeJSON(w, http.StatusAccepted, accepted)
		return
	}
	go func(to, token string) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.mail.SendPasswordReset(ctx, to, token, s.cfg.AppBaseURL); err != nil {
			slog.Error("send reset email", "error", err)
		}
	}(u.Email, plain)
	writeJSON(w, http.StatusAccepted, accepted)
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// POST /auth/password/reset {token, password}: sets the password and signs out every session.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if !decode(w, r, &req) {
		return
	}
	tok, err := s.q.GetValidEmailToken(r.Context(), dbgen.GetValidEmailTokenParams{
		TokenHash: auth.HashToken(req.Token), Purpose: "reset_password",
	})
	if err != nil {
		fail(w, apierr.New(http.StatusGone, "token_invalid", "Gone", "this reset link is invalid or has expired"))
		return
	}
	u, err := s.q.GetUserByID(r.Context(), tok.UserID)
	if err != nil {
		fail(w, apierr.New(http.StatusGone, "token_invalid", "Gone", "this reset link is invalid or has expired"))
		return
	}
	if err := auth.ValidatePasswordStrength(req.Password); err != nil {
		fail(w, unprocessable("weak_password", err.Error()))
		return
	}
	if err := s.checkPasswordPolicyForEmail(r.Context(), u.Email, req.Password); err != nil {
		fail(w, err)
		return
	}
	hash, err := crypto.HashPassword(req.Password, crypto.UserPasswordConfig)
	if err != nil {
		fail(w, apierr.Internal("failed to hash password"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		n, err := q.ConsumeEmailToken(r.Context(), tok.ID)
		if err != nil {
			return err
		}
		if n == 0 {
			return apierr.New(http.StatusGone, "token_invalid", "Gone", "this reset link was already used")
		}
		if err := q.SetUserPassword(r.Context(), dbgen.SetUserPasswordParams{ID: u.ID, PasswordHash: &hash}); err != nil {
			return err
		}
		// Possession of the mailbox proves the address.
		if !u.EmailVerifiedAt.Valid {
			if err := q.MarkEmailVerified(r.Context(), u.ID); err != nil {
				return err
			}
		}
		return q.RevokeUserSessions(r.Context(), u.ID)
	})
	if err != nil {
		fail(w, err)
		return
	}
	if s.rdb != nil {
		_ = s.rdb.Del(r.Context(), "lock:login:"+u.Email).Err()
	}
	s.sessions.evictAll()
	_ = s.aud().Record(r.Context(), s.q, auditUserEntry(r, u.ID, "user.password.reset"))
	w.WriteHeader(http.StatusNoContent)
}

type changePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// POST /me/password: requires the current password; other sessions are revoked.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordReq
	if !decode(w, r, &req) {
		return
	}
	p := principal(r)
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.NotFound("user not found"))
		return
	}
	if u.PasswordHash != nil {
		ok, verr := crypto.VerifyPassword(req.CurrentPassword, *u.PasswordHash)
		if verr != nil || !ok {
			fail(w, forbidden("invalid_password", "current password is incorrect"))
			return
		}
	}
	if err := auth.ValidatePasswordStrength(req.NewPassword); err != nil {
		fail(w, unprocessable("weak_password", err.Error()))
		return
	}
	if err := s.checkPasswordPolicyForEmail(r.Context(), u.Email, req.NewPassword); err != nil {
		fail(w, err)
		return
	}
	hash, err := crypto.HashPassword(req.NewPassword, crypto.UserPasswordConfig)
	if err != nil {
		fail(w, apierr.Internal("failed to hash password"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if err := q.SetUserPassword(r.Context(), dbgen.SetUserPasswordParams{ID: u.ID, PasswordHash: &hash}); err != nil {
			return err
		}
		_, err := tx.Exec(r.Context(), `UPDATE sessions SET revoked_at = now()
			WHERE user_id = $1 AND revoked_at IS NULL AND id <> $2`, u.ID, p.SessionID)
		return err
	})
	if err != nil {
		fail(w, apierr.Internal("failed to change password"))
		return
	}
	s.sessions.evictAll()
	_ = s.aud().Record(r.Context(), s.q, auditUserEntry(r, u.ID, "user.password.changed"))
	w.WriteHeader(http.StatusNoContent)
}
