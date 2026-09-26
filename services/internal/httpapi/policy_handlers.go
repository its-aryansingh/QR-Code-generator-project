package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/access"
	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// WorkspacePolicy is the governance policy of one workspace.
type WorkspacePolicy struct {
	WorkspaceID             uuid.UUID  `json:"workspace_id"`
	AllowedDestinationHosts []string   `json:"allowed_destination_hosts"`
	BlockedDestinationHosts []string   `json:"blocked_destination_hosts"`
	RequireHTTPS            bool       `json:"require_https"`
	ApprovalMode            string     `json:"approval_mode"`
	ApprovalsRequired       int        `json:"approvals_required"`
	ApprovalExpiryHours     int        `json:"approval_expiry_hours"`
	RequireTemplate         bool       `json:"require_template"`
	PixelConsentMode        string     `json:"pixel_consent_mode"`
	DisabledFeatures        []string   `json:"disabled_features"`
	UpdatedBy               *uuid.UUID `json:"updated_by"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

var wsPolicyCache = expirable.NewLRU[uuid.UUID, *WorkspacePolicy](10000, nil, 30*time.Second)

// wsPolicy loads (cached 30 s) a workspace policy, defaulting when the row is missing.
func (s *Server) wsPolicy(ctx context.Context, wsID uuid.UUID) (*WorkspacePolicy, error) {
	if p, ok := wsPolicyCache.Get(wsID); ok {
		return p, nil
	}
	p := &WorkspacePolicy{WorkspaceID: wsID, RequireHTTPS: true, ApprovalMode: "off", ApprovalsRequired: 1,
		ApprovalExpiryHours: 168, PixelConsentMode: "opt_in_all", AllowedDestinationHosts: []string{},
		BlockedDestinationHosts: []string{}, DisabledFeatures: []string{}}
	err := s.pool.QueryRow(ctx, `SELECT allowed_destination_hosts, blocked_destination_hosts, require_https, approval_mode,
		approvals_required, approval_expiry_hours, require_template, pixel_consent_mode, disabled_features, updated_by, updated_at
		FROM workspace_policies WHERE workspace_id = $1`, wsID).Scan(&p.AllowedDestinationHosts, &p.BlockedDestinationHosts,
		&p.RequireHTTPS, &p.ApprovalMode, &p.ApprovalsRequired, &p.ApprovalExpiryHours, &p.RequireTemplate, &p.PixelConsentMode,
		&p.DisabledFeatures, &p.UpdatedBy, &p.UpdatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	wsPolicyCache.Add(wsID, p)
	return p, nil
}

func (s *Server) handleGetWorkspacePolicy(w http.ResponseWriter, r *http.Request) {
	p, err := s.wsPolicy(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to load policy"))
		return
	}
	writeJSON(w, http.StatusOK, p)
}

var hostPattern = regexp.MustCompile(`^(\*\.)?([a-z0-9-]+\.)+[a-z]{2,}$`)

func normaliseHosts(field string, in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, h := range in {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" || seen[h] {
			continue
		}
		if !hostPattern.MatchString(h) {
			return nil, unprocessable("invalid_host", field+": "+h+" is not a host name or *.suffix pattern")
		}
		seen[h] = true
		out = append(out, h)
	}
	if len(out) > 500 {
		return nil, unprocessable("too_many_hosts", field+": at most 500 entries")
	}
	return out, nil
}

// PUT /workspaces/{ws}/policies (policy.manage): full replace; audited with before/after.
func (s *Server) handlePutWorkspacePolicy(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	before, err := s.wsPolicy(r.Context(), ws.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to load policy"))
		return
	}
	req := *before
	if !decode(w, r, &req) {
		return
	}
	if req.AllowedDestinationHosts, err = normaliseHosts("allowed_destination_hosts", req.AllowedDestinationHosts); err != nil {
		fail(w, err)
		return
	}
	if req.BlockedDestinationHosts, err = normaliseHosts("blocked_destination_hosts", req.BlockedDestinationHosts); err != nil {
		fail(w, err)
		return
	}
	switch req.ApprovalMode {
	case "off", "outside_allowlist", "all_destination_changes", "all_changes":
	default:
		fail(w, unprocessable("invalid_approval_mode", "approval_mode must be off, outside_allowlist, all_destination_changes or all_changes"))
		return
	}
	if req.ApprovalMode != "off" {
		if err := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureRoles); err != nil {
			fail(w, featureErr(err))
			return
		}
	}
	if req.ApprovalMode == "outside_allowlist" && len(req.AllowedDestinationHosts) == 0 {
		fail(w, unprocessable("allowlist_required", "approval mode outside_allowlist needs allowed_destination_hosts"))
		return
	}
	if req.ApprovalsRequired < 1 || req.ApprovalsRequired > 3 {
		fail(w, unprocessable("invalid_approvals_required", "approvals_required must be 1–3"))
		return
	}
	if req.ApprovalExpiryHours < 1 || req.ApprovalExpiryHours > 720 {
		fail(w, unprocessable("invalid_approval_expiry", "approval_expiry_hours must be 1–720"))
		return
	}
	if req.PixelConsentMode != "opt_in_all" && req.PixelConsentMode != "opt_in_where_required" {
		fail(w, unprocessable("invalid_pixel_consent_mode", "pixel_consent_mode must be opt_in_all or opt_in_where_required"))
		return
	}
	known := map[string]bool{}
	for _, f := range entitlements.AllFeatures {
		known[f] = true
	}
	for _, f := range req.DisabledFeatures {
		if !known[f] {
			fail(w, unprocessable("invalid_feature", "unknown feature "+f))
			return
		}
	}
	if req.DisabledFeatures == nil {
		req.DisabledFeatures = []string{}
	}
	userID, _ := actorIDs(r)
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `INSERT INTO workspace_policies (workspace_id, allowed_destination_hosts, blocked_destination_hosts,
			require_https, approval_mode, approvals_required, approval_expiry_hours, require_template, pixel_consent_mode,
			disabled_features, updated_by, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11, now())
			ON CONFLICT (workspace_id) DO UPDATE SET allowed_destination_hosts = EXCLUDED.allowed_destination_hosts,
			  blocked_destination_hosts = EXCLUDED.blocked_destination_hosts, require_https = EXCLUDED.require_https,
			  approval_mode = EXCLUDED.approval_mode, approvals_required = EXCLUDED.approvals_required,
			  approval_expiry_hours = EXCLUDED.approval_expiry_hours, require_template = EXCLUDED.require_template,
			  pixel_consent_mode = EXCLUDED.pixel_consent_mode, disabled_features = EXCLUDED.disabled_features,
			  updated_by = EXCLUDED.updated_by, updated_at = now()`,
			ws.ID, req.AllowedDestinationHosts, req.BlockedDestinationHosts, req.RequireHTTPS, req.ApprovalMode,
			req.ApprovalsRequired, req.ApprovalExpiryHours, req.RequireTemplate, req.PixelConsentMode, req.DisabledFeatures, userID)
		if err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "policy.workspace.updated", "workspace", &id, map[string]any{"before": before, "after": req})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to save policy"))
		return
	}
	wsPolicyCache.Remove(ws.ID)
	s.plans.Invalidate()
	p, _ := s.wsPolicy(r.Context(), ws.ID)
	writeJSON(w, http.StatusOK, p)
}

// ---- organisation security policy -------------------------------------------

func (s *Server) handleGetSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	p, err := access.LoadPolicy(r.Context(), s.pool, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to load policy"))
		return
	}
	writeJSON(w, http.StatusOK, policyDTO(p))
}

type securityPolicyDTO struct {
	EnforceSSO           bool        `json:"enforce_sso"`
	BreakGlassUserIDs    []uuid.UUID `json:"sso_break_glass_user_ids"`
	RequireMFA           bool        `json:"require_mfa"`
	AllowedMFAKinds      []string    `json:"allowed_mfa_kinds"`
	SessionIdleMinutes   int         `json:"session_idle_minutes"`
	SessionMaxHours      int         `json:"session_max_hours"`
	DashboardIPAllowlist []string    `json:"dashboard_ip_allowlist"`
	APIIPAllowlist       []string    `json:"api_ip_allowlist"`
	PasswordMinLength    int         `json:"password_min_length"`
	InviteEmailDomains   []string    `json:"invite_email_domains"`
	APIKeyMaxDays        int         `json:"api_key_max_days"`
	ExportPermission     string      `json:"export_permission"`
	UpdatedBy            *uuid.UUID  `json:"updated_by,omitempty"`
	UpdatedAt            *time.Time  `json:"updated_at,omitempty"`
}

func prefixes(ps []netip.Prefix) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.String())
	}
	return out
}

func policyDTO(p *access.Policy) securityPolicyDTO {
	d := securityPolicyDTO{EnforceSSO: p.EnforceSSO, BreakGlassUserIDs: p.BreakGlassUserIDs, RequireMFA: p.RequireMFA,
		AllowedMFAKinds: p.AllowedMFAKinds, SessionIdleMinutes: p.SessionIdleMinutes, SessionMaxHours: p.SessionMaxHours,
		DashboardIPAllowlist: prefixes(p.DashboardIPAllowlist), APIIPAllowlist: prefixes(p.APIIPAllowlist),
		PasswordMinLength: p.PasswordMinLength, InviteEmailDomains: p.InviteEmailDomains, APIKeyMaxDays: p.APIKeyMaxDays,
		ExportPermission: p.ExportPermission, UpdatedBy: p.UpdatedBy}
	if !p.UpdatedAt.IsZero() {
		t := p.UpdatedAt
		d.UpdatedAt = &t
	}
	if d.BreakGlassUserIDs == nil {
		d.BreakGlassUserIDs = []uuid.UUID{}
	}
	if d.InviteEmailDomains == nil {
		d.InviteEmailDomains = []string{}
	}
	return d
}

func parsePrefixes(field string, in []string) ([]netip.Prefix, error) {
	out := []netip.Prefix{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			if a, err := netip.ParseAddr(s); err == nil {
				out = append(out, netip.PrefixFrom(a, a.BitLen()))
				continue
			}
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, unprocessable("invalid_cidr", field+": "+s+" is not an IP address or CIDR range")
		}
		out = append(out, p.Masked())
	}
	if len(out) > 200 {
		return nil, unprocessable("too_many_ranges", field+": at most 200 ranges")
	}
	return out, nil
}

// PUT /orgs/{org}/security-policy (org.policy + step-up). Refuses changes that would lock
// the caller out, and SSO enforcement without an active connection and an MFA break-glass owner.
func (s *Server) handlePutSecurityPolicy(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	cur, err := access.LoadPolicy(r.Context(), s.pool, o.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to load policy"))
		return
	}
	req := policyDTO(cur)
	if !decode(w, r, &req) {
		return
	}
	dash, err := parsePrefixes("dashboard_ip_allowlist", req.DashboardIPAllowlist)
	if err != nil {
		fail(w, err)
		return
	}
	api, err := parsePrefixes("api_ip_allowlist", req.APIIPAllowlist)
	if err != nil {
		fail(w, err)
		return
	}
	if (len(dash) > 0 || len(api) > 0 || req.EnforceSSO) && o.PlanID != "enterprise" {
		if eff, err := s.plans.ForOrg(r.Context(), o.ID); err != nil || !eff.Has(entitlements.FeatureSSO) {
			pd := paymentRequired("upgrade_required", "IP allowlists and SSO enforcement need the Enterprise plan")
			pd.RequiredPlan = string(entitlements.PlanEnterprise)
			fail(w, pd)
			return
		}
	}
	if len(dash) > 0 && !access.IPAllowed(dash, clientAddr(r)) {
		fail(w, unprocessable("would_lock_out", "your current IP address is outside dashboard_ip_allowlist; add it first"))
		return
	}
	if req.SessionIdleMinutes != 0 && (req.SessionIdleMinutes < 5 || req.SessionIdleMinutes > 10080) {
		fail(w, unprocessable("invalid_session_idle", "session_idle_minutes must be 0 or 5–10080"))
		return
	}
	if req.SessionMaxHours != 0 && (req.SessionMaxHours < 1 || req.SessionMaxHours > 2160) {
		fail(w, unprocessable("invalid_session_max", "session_max_hours must be 0 or 1–2160"))
		return
	}
	if req.PasswordMinLength < 10 || req.PasswordMinLength > 128 {
		fail(w, unprocessable("invalid_password_min_length", "password_min_length must be 10–128"))
		return
	}
	if req.ExportPermission != "role" && req.ExportPermission != "admins_only" && req.ExportPermission != "disabled" {
		fail(w, unprocessable("invalid_export_permission", "export_permission must be role, admins_only or disabled"))
		return
	}
	for _, k := range req.AllowedMFAKinds {
		if k != "totp" && k != "webauthn" {
			fail(w, unprocessable("invalid_mfa_kind", "allowed_mfa_kinds accepts totp and webauthn"))
			return
		}
	}
	if len(req.BreakGlassUserIDs) > 2 {
		fail(w, unprocessable("too_many_break_glass", "at most 2 break-glass owners"))
		return
	}
	for _, id := range req.BreakGlassUserIDs {
		var role string
		if err := s.pool.QueryRow(r.Context(), `SELECT org_role FROM org_members WHERE org_id = $1 AND user_id = $2 AND status = 'active'`,
			o.ID, id).Scan(&role); err != nil || (role != "org_owner" && role != "org_admin") {
			fail(w, unprocessable("invalid_break_glass", "break-glass users must be active org owners or admins"))
			return
		}
	}
	if req.EnforceSSO && !cur.EnforceSSO {
		var active int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM sso_connections WHERE org_id = $1 AND status = 'active'`, o.ID).Scan(&active)
		if active == 0 {
			fail(w, apierr.Conflict("sso_not_ready", "activate and test an SSO connection before enforcing SSO"))
			return
		}
		var withMFA int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM unnest($1::uuid[]) u(id) WHERE EXISTS
			(SELECT 1 FROM user_mfa_factors f WHERE f.user_id = u.id)`, req.BreakGlassUserIDs).Scan(&withMFA)
		if withMFA == 0 {
			fail(w, apierr.Conflict("break_glass_required", "name at least one break-glass owner who has two-factor authentication"))
			return
		}
	}
	domains := make([]string, 0, len(req.InviteEmailDomains))
	for _, d := range req.InviteEmailDomains {
		if d = trimLower(d); d != "" {
			domains = append(domains, d)
		}
	}
	userID, _ := actorIDs(r)
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `INSERT INTO org_security_policies (org_id, enforce_sso, sso_break_glass_user_ids, require_mfa,
			allowed_mfa_kinds, session_idle_minutes, session_max_hours, dashboard_ip_allowlist, api_ip_allowlist,
			password_min_length, invite_email_domains, api_key_max_days, export_permission, updated_by, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::citext[],$12,$13,$14, now())
			ON CONFLICT (org_id) DO UPDATE SET enforce_sso = EXCLUDED.enforce_sso,
			  sso_break_glass_user_ids = EXCLUDED.sso_break_glass_user_ids, require_mfa = EXCLUDED.require_mfa,
			  allowed_mfa_kinds = EXCLUDED.allowed_mfa_kinds, session_idle_minutes = EXCLUDED.session_idle_minutes,
			  session_max_hours = EXCLUDED.session_max_hours, dashboard_ip_allowlist = EXCLUDED.dashboard_ip_allowlist,
			  api_ip_allowlist = EXCLUDED.api_ip_allowlist, password_min_length = EXCLUDED.password_min_length,
			  invite_email_domains = EXCLUDED.invite_email_domains, api_key_max_days = EXCLUDED.api_key_max_days,
			  export_permission = EXCLUDED.export_permission, updated_by = EXCLUDED.updated_by, updated_at = now()`,
			o.ID, req.EnforceSSO, req.BreakGlassUserIDs, req.RequireMFA, req.AllowedMFAKinds, req.SessionIdleMinutes,
			req.SessionMaxHours, dash, api, req.PasswordMinLength, domains, req.APIKeyMaxDays, req.ExportPermission, userID)
		if err != nil {
			return err
		}
		before, _ := json.Marshal(policyDTO(cur))
		var b map[string]any
		_ = json.Unmarshal(before, &b)
		return s.auditOrg(r, q, o.ID, "policy.security.updated", "organization", &o.ID, map[string]any{"before": b, "after": req})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to save policy"))
		return
	}
	s.access.Invalidate(r.Context())
	p, _ := access.LoadPolicy(r.Context(), s.pool, o.ID)
	writeJSON(w, http.StatusOK, policyDTO(p))
}
