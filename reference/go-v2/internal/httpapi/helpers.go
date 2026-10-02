package httpapi

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/plans"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

func itoa(i int) string { return strconv.Itoa(i) }

func hexString(b []byte) string { return hex.EncodeToString(b) }

// ipKey is the rate-limit key for the client: the full IPv4 address or the IPv6 /64.
func ipKey(r *http.Request) string {
	ip := clientIP(r)
	if ip == nil {
		return "unknown"
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// IdentityHooks lets the enterprise identity layer enforce organisation policy
// (SSO-only domains, password policy, MFA, session attributes) without the core
// handlers knowing about organisations.
type IdentityHooks interface {
	// PasswordPolicy validates a new password for an email (org password policy, SSO-only domains).
	PasswordPolicy(ctx context.Context, email, password string) error
	// LoginAllowed is called after primary authentication succeeds (method: password|oidc|saml|magic).
	LoginAllowed(ctx context.Context, u dbgen.User, method string) error
	// MFAPending reports whether the user must complete a second factor before full access.
	MFAPending(ctx context.Context, userID uuid.UUID) bool
	// SessionCreated records session attributes (auth method, SSO connection) after a login.
	SessionCreated(ctx context.Context, sess dbgen.Session, method string, ssoConnectionID *uuid.UUID) error
	// SessionRotated copies session attributes when a refresh token rotates.
	SessionRotated(ctx context.Context, tx pgx.Tx, oldID, newID uuid.UUID) error
	// MeExtras adds fields (organisations, MFA state) to GET /me.
	MeExtras(ctx context.Context, u dbgen.User) map[string]any
	// VerifySecondFactor checks a TOTP/recovery code for step-up when the user has factors.
	VerifySecondFactor(ctx context.Context, userID uuid.UUID, code string) error
}

type noIdentity struct{}

func (noIdentity) PasswordPolicy(context.Context, string, string) error               { return nil }
func (noIdentity) LoginAllowed(context.Context, dbgen.User, string) error             { return nil }
func (noIdentity) MFAPending(context.Context, uuid.UUID) bool                         { return false }
func (noIdentity) SessionRotated(context.Context, pgx.Tx, uuid.UUID, uuid.UUID) error { return nil }
func (noIdentity) MeExtras(context.Context, dbgen.User) map[string]any                { return nil }
func (noIdentity) VerifySecondFactor(context.Context, uuid.UUID, string) error        { return nil }
func (noIdentity) SessionCreated(context.Context, dbgen.Session, string, *uuid.UUID) error {
	return nil
}

// SetIdentityHooks installs the enterprise identity layer.
func (s *Server) SetIdentityHooks(h IdentityHooks) { s.identity = h }

func (s *Server) idh() IdentityHooks {
	if s.identity == nil {
		return noIdentity{}
	}
	return s.identity
}

func (s *Server) checkPasswordPolicyForEmail(ctx context.Context, email, password string) error {
	return s.idh().PasswordPolicy(ctx, email, password)
}

func (s *Server) checkLoginAllowed(ctx context.Context, u dbgen.User, method string) error {
	return s.idh().LoginAllowed(ctx, u, method)
}

func (s *Server) mfaPending(ctx context.Context, userID uuid.UUID) bool {
	return s.idh().MFAPending(ctx, userID)
}

// Entitlements answers plan questions for a workspace. The enterprise layer replaces the
// default (static plan table) with contract-aware limits and per-org feature overrides.
type Entitlements interface {
	Limits(ctx context.Context, ws dbgen.Workspace) entitlements.Limits
	CheckFeature(ctx context.Context, ws dbgen.Workspace, feature string) error
	Features(ctx context.Context, ws dbgen.Workspace) []string
}

type planEntitlements struct{}

func (planEntitlements) Limits(_ context.Context, ws dbgen.Workspace) entitlements.Limits {
	return entitlements.GetLimits(ws.PlanID)
}

func (planEntitlements) CheckFeature(_ context.Context, ws dbgen.Workspace, feature string) error {
	return entitlements.CheckFeature(ws.PlanID, feature)
}

func (planEntitlements) Features(_ context.Context, ws dbgen.Workspace) []string {
	var out []string
	for _, f := range entitlements.AllFeatures {
		if entitlements.HasFeature(ws.PlanID, f) {
			out = append(out, f)
		}
	}
	return out
}

// SetEntitlements installs a replacement entitlement source.
func (s *Server) SetEntitlements(e Entitlements) { s.entitle = e }

func (s *Server) ent() Entitlements {
	if s.entitle == nil {
		return planEntitlements{}
	}
	return s.entitle
}

// featureErr converts an entitlement failure into a 402 problem.
func featureErr(err error) error {
	if err == nil {
		return nil
	}
	if ee, ok := err.(*entitlements.EntitlementError); ok {
		p := paymentRequired(ee.Code, ee.Message)
		p.RequiredPlan = string(ee.RequiredPlan)
		return p
	}
	return err
}

// issueVerificationEmail creates a single-use verify_email token and mails it.
// Failures are logged: the user can request another from the dashboard.
func (s *Server) issueVerificationEmail(ctx context.Context, u dbgen.User) {
	if u.EmailVerifiedAt.Valid || s.mail == nil {
		return
	}
	plain, hash, err := auth.GenerateRandomToken(32)
	if err != nil {
		slog.Error("verification token", "error", err)
		return
	}
	_ = s.q.InvalidateEmailTokens(ctx, dbgen.InvalidateEmailTokensParams{UserID: u.ID, Purpose: "verify_email"})
	if _, err := s.q.CreateEmailToken(ctx, dbgen.CreateEmailTokenParams{
		ID: idgen.New(), UserID: u.ID, Purpose: "verify_email", TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}); err != nil {
		slog.Error("store verification token", "error", err)
		return
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.mail.SendVerification(sendCtx, u.Email, plain, s.cfg.AppBaseURL); err != nil {
		slog.Error("send verification email", "error", err)
	}
}

// maskEmail turns "alice@example.com" into "a***e@example.com" for unauthenticated previews.
func maskEmail(e string) string {
	at := strings.LastIndex(e, "@")
	if at <= 0 {
		return "***"
	}
	local := e[:at]
	switch {
	case len(local) <= 2:
		local = local[:1] + "***"
	default:
		local = local[:1] + "***" + local[len(local)-1:]
	}
	return local + e[at:]
}

// planEntitlementsV2 resolves org plan ⊕ contract ⊖ workspace-disabled features.
type planEntitlementsV2 struct{ p *plans.Service }

func (e planEntitlementsV2) Limits(ctx context.Context, ws dbgen.Workspace) entitlements.Limits {
	eff, err := e.p.ForWorkspace(ctx, ws)
	if err != nil {
		return entitlements.GetLimits(ws.PlanID)
	}
	return eff.Limits
}

func (e planEntitlementsV2) CheckFeature(ctx context.Context, ws dbgen.Workspace, feature string) error {
	return e.p.CheckFeature(ctx, ws, feature)
}

func (e planEntitlementsV2) Features(ctx context.Context, ws dbgen.Workspace) []string {
	eff, err := e.p.ForWorkspace(ctx, ws)
	if err != nil {
		return nil
	}
	return eff.Features
}
