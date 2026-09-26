package httpapi

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/access"
	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// identityGate applies an organisation's security policy to every request for its
// resources, after authentication and before any handler: IP allowlist → SSO enforcement
// → MFA → session idle/max lifetime. (Membership status is enforced by the grant source.)
type identityGate struct{ s *Server }

func (g identityGate) Check(r *http.Request, p *auth.Principal, ws dbgen.Workspace) error {
	return g.s.checkOrgAccess(r, p, ws.OrgID)
}

// checkOrgAccess is shared by workspace and organisation routes.
func (s *Server) checkOrgAccess(r *http.Request, p *auth.Principal, orgID uuid.UUID) error {
	if p == nil || orgID == uuid.Nil {
		return nil
	}
	pol, err := s.access.Policy(r.Context(), orgID)
	if err != nil {
		return apierr.Internal("failed to load security policy")
	}
	ip := clientAddr(r)
	if p.IsAPIKey() {
		if !access.IPAllowed(pol.APIIPAllowlist, ip) {
			return forbidden("ip_not_allowed", "this API key is not allowed from your IP address by the organisation's policy")
		}
		return nil
	}
	if p.StaffGrantID != uuid.Nil {
		return nil // staff access is governed by the customer's support grant and staff controls
	}
	sess, ok := currentSession(r)
	if !ok {
		return apierr.Unauthorized("session required")
	}
	breakGlass := pol.IsBreakGlass(p.UserID) && sess.MfaVerifiedAt.Valid
	if !breakGlass && !access.IPAllowed(pol.DashboardIPAllowlist, ip) {
		return forbidden("ip_not_allowed", "your organisation only allows access from approved networks")
	}
	if pol.EnforceSSO && !breakGlass {
		viaOrgSSO := false
		if sess.AuthMethod == "sso" && sess.SsoConnectionID.Valid {
			var connOrg uuid.UUID
			if err := s.pool.QueryRow(r.Context(), `SELECT org_id FROM sso_connections WHERE id = $1`,
				uuid.UUID(sess.SsoConnectionID.Bytes)).Scan(&connOrg); err == nil && connOrg == orgID {
				viaOrgSSO = true
			}
		}
		if !viaOrgSSO {
			pd := forbidden("sso_required", "this organisation requires single sign-on; sign in with your company account")
			pd.Instance = "/v1/auth/sso/start"
			return pd
		}
	}
	if pol.RequireMFA && sess.AuthMethod != "sso" && !sess.MfaVerifiedAt.Valid {
		pd := forbidden("mfa_required", "this organisation requires two-factor authentication; set it up or verify your second factor")
		pd.Instance = "/v1/me/mfa"
		return pd
	}
	now := time.Now()
	if pol.SessionIdleMinutes > 0 && now.Sub(sess.LastUsedAt) > time.Duration(pol.SessionIdleMinutes)*time.Minute {
		s.expireSession(r, sess.ID)
		return apierr.New(http.StatusUnauthorized, "session_expired", "Unauthorized", "signed out after inactivity (organisation policy)")
	}
	if pol.SessionMaxHours > 0 && now.Sub(sess.CreatedAt) > time.Duration(pol.SessionMaxHours)*time.Hour {
		s.expireSession(r, sess.ID)
		return apierr.New(http.StatusUnauthorized, "session_expired", "Unauthorized", "session reached the organisation's maximum lifetime; sign in again")
	}
	return nil
}

func (s *Server) expireSession(r *http.Request, id uuid.UUID) {
	_ = s.q.RevokeSession(r.Context(), id)
	s.sessions.evict(id)
}

func clientAddr(r *http.Request) netip.Addr {
	ip := clientIP(r)
	if ip == nil {
		return netip.Addr{}
	}
	a, _ := netip.AddrFromSlice(ip)
	return a.Unmap()
}
