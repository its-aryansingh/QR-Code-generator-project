package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// coreIdentity is the built-in identity layer: org password policy, MFA state on sessions,
// second-factor checks for step-up.
type coreIdentity struct{ s *Server }

func (c coreIdentity) PasswordPolicy(ctx context.Context, email, password string) error {
	var min int
	err := c.s.pool.QueryRow(ctx, `SELECT COALESCE(max(sp.password_min_length), 10) FROM users u
		JOIN org_members m ON m.user_id = u.id AND m.status = 'active'
		JOIN org_security_policies sp ON sp.org_id = m.org_id
		WHERE u.email = $1`, email).Scan(&min)
	if err == nil && len(password) < min {
		return unprocessable("weak_password", "your organisation requires passwords of at least "+itoa(min)+" characters")
	}
	return nil
}

func (c coreIdentity) LoginAllowed(ctx context.Context, u dbgen.User, method string) error {
	// Accounts created by SCIM or SSO have no password; SSO enforcement is applied per
	// organisation at the access gate so users keep access to their other organisations.
	return nil
}

func (c coreIdentity) MFAPending(ctx context.Context, userID uuid.UUID) bool {
	var n int
	_ = c.s.pool.QueryRow(ctx, `SELECT count(*) FROM user_mfa_factors WHERE user_id = $1`, userID).Scan(&n)
	return n > 0
}

func (c coreIdentity) SessionCreated(ctx context.Context, sess dbgen.Session, method string, ssoConnectionID *uuid.UUID) error {
	am := "password"
	switch method {
	case "sso", "oidc", "saml":
		am = "sso"
	case "google", "magic_link":
		am = method
	}
	pending := am != "sso" && c.MFAPending(ctx, sess.UserID)
	_, err := c.s.pool.Exec(ctx, `UPDATE sessions SET auth_method = $2, sso_connection_id = $3, mfa_pending = $4 WHERE id = $1`,
		sess.ID, am, ssoConnectionID, pending)
	return err
}

func (c coreIdentity) SessionRotated(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error { return nil }

func (c coreIdentity) MeExtras(ctx context.Context, u dbgen.User) map[string]any {
	var factors, codes int
	_ = c.s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM user_mfa_factors WHERE user_id = $1),
		(SELECT count(*) FROM user_recovery_codes WHERE user_id = $1 AND used_at IS NULL)`, u.ID).Scan(&factors, &codes)
	return map[string]any{"mfa": map[string]any{"enabled": factors > 0, "factors": factors, "recovery_codes_remaining": codes}}
}

func (c coreIdentity) VerifySecondFactor(ctx context.Context, userID uuid.UUID, code string) error {
	if !c.MFAPending(ctx, userID) {
		return nil
	}
	if strings.TrimSpace(code) == "" {
		return apierr.New(http.StatusUnauthorized, "mfa_code_required", "Unauthorized", "enter a code from your authenticator app")
	}
	ok, err := c.s.checkSecondFactor(ctx, userID, code)
	if err != nil {
		return apierr.Internal("failed to verify code")
	}
	if !ok {
		return apierr.New(http.StatusUnauthorized, "invalid_mfa_code", "Unauthorized", "that code is not valid")
	}
	return nil
}

// checkSecondFactor verifies a TOTP code (each code usable once) or consumes a recovery code.
func (s *Server) checkSecondFactor(ctx context.Context, userID uuid.UUID, code string) (bool, error) {
	code = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), " ", ""))
	if len(code) == 6 && strings.Trim(code, "0123456789") == "" {
		rows, err := s.pool.Query(ctx, `SELECT id, totp_secret_ct FROM user_mfa_factors WHERE user_id = $1 AND kind = 'totp'`, userID)
		if err != nil {
			return false, err
		}
		type factor struct {
			id uuid.UUID
			ct []byte
		}
		var fs []factor
		for rows.Next() {
			var f factor
			if rows.Scan(&f.id, &f.ct) == nil {
				fs = append(fs, f)
			}
		}
		rows.Close()
		for _, f := range fs {
			secret, err := s.keyring.OpenPlatform(f.ct, "totp:"+userID.String())
			if err != nil {
				continue
			}
			if auth.VerifyTOTPCode(string(secret), code, time.Now()) {
				// Replay protection: a code is good once within its validity window.
				if s.rdb != nil {
					ok, err := s.rdb.SetNX(ctx, "totp:used:"+f.id.String()+":"+code, 1, 2*time.Minute).Result()
					if err == nil && !ok {
						return false, nil
					}
				}
				_, _ = s.pool.Exec(ctx, `UPDATE user_mfa_factors SET last_used_at = now() WHERE id = $1`, f.id)
				return true, nil
			}
		}
		return false, nil
	}
	sum := sha256.Sum256([]byte(code))
	tag, err := s.pool.Exec(ctx, `UPDATE user_recovery_codes SET used_at = now() WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL`,
		userID, sum[:])
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// mfaGate confines a session that still owes its second factor to the MFA endpoints.
func (s *Server) mfaGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, ok := currentSession(r)
		if !ok || !sess.MfaPending || sess.MfaVerifiedAt.Valid {
			next.ServeHTTP(w, r)
			return
		}
		p := strings.TrimPrefix(r.URL.Path, "/api")
		if strings.HasPrefix(p, "/v1/auth/mfa/") || strings.HasPrefix(p, "/v1/auth/logout") || strings.HasPrefix(p, "/v1/auth/refresh") ||
			(p == "/v1/me" && r.Method == http.MethodGet) || p == "/healthz" || p == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		pd := forbidden("mfa_verification_required", "enter the code from your authenticator app to finish signing in")
		pd.Instance = "/v1/auth/mfa/verify"
		fail(w, pd)
	})
}

var errNoFactor = errors.New("no factor")

func hashHex(h string) []byte {
	b, _ := hex.DecodeString(h)
	return b
}
