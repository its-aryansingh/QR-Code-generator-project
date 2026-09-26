package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/audit"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/org"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

type orgCtxKey struct{}

type orgContextValue struct {
	org   org.Org
	role  string
	perms []authz.Permission
}

func orgRow(r *http.Request) org.Org {
	v, _ := r.Context().Value(orgCtxKey{}).(orgContextValue)
	return v.org
}

func orgRole(r *http.Request) string {
	v, _ := r.Context().Value(orgCtxKey{}).(orgContextValue)
	return v.role
}

func orgCan(r *http.Request, p authz.Permission) bool {
	v, _ := r.Context().Value(orgCtxKey{}).(orgContextValue)
	return hasPerm(v.perms, p)
}

// orgContext resolves {org} (UUID or slug) for an active member and applies the org's
// identity gate. Non-members get 404.
func (s *Server) orgContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principal(r)
		if p == nil || p.IsAPIKey() {
			fail(w, apierr.NotFound("organization not found"))
			return
		}
		param := chi.URLParam(r, "org")
		var o org.Org
		var err error
		if id, perr := uuid.Parse(param); perr == nil {
			o, err = org.Get(r.Context(), s.pool, id)
		} else {
			o, err = org.GetBySlug(r.Context(), s.pool, param)
		}
		if err != nil {
			fail(w, apierr.NotFound("organization not found"))
			return
		}
		role, perms, err := s.access.OrgPermissions(r.Context(), o.ID, p.UserID)
		if err != nil {
			fail(w, apierr.Internal("failed to resolve organisation access"))
			return
		}
		if role == "" || (perms == nil && role != org.RoleMember) {
			fail(w, apierr.NotFound("organization not found"))
			return
		}
		if m, err := org.GetMember(r.Context(), s.pool, o.ID, p.UserID); err == nil && m.Status != "active" {
			fail(w, apierr.NotFound("organization not found"))
			return
		}
		if err := s.checkOrgAccess(r, p, o.ID); err != nil {
			fail(w, err)
			return
		}
		ctx := context.WithValue(r.Context(), orgCtxKey{}, orgContextValue{org: o, role: role, perms: perms})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireOrg(p authz.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !orgCan(r, p) {
				fail(w, forbidden("forbidden", "you need the "+string(p)+" permission in this organisation"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireStepUp demands a recent re-authentication (POST /auth/step-up) for sensitive actions.
func (s *Server) requireStepUp(maxAge time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := currentSession(r)
			if !ok || !sess.StepUpAt.Valid || time.Since(sess.StepUpAt.Time) > maxAge {
				pd := apierr.New(http.StatusUnauthorized, "step_up_required", "Unauthorized",
					"confirm your identity to continue (POST /v1/auth/step-up)")
				pd.Instance = "/v1/auth/step-up"
				fail(w, pd)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// auditOrg writes an org-level entry (no workspace).
func (s *Server) auditOrg(r *http.Request, q *dbgen.Queries, orgID uuid.UUID, action, targetType string, targetID *uuid.UUID, changes map[string]any) error {
	e := audit.Entry{OrgID: &orgID, Action: action, TargetType: targetType, TargetID: targetID, Changes: changes,
		UserAgent: truncate(r.UserAgent(), 300), RequestID: middleware.GetReqID(r.Context()), ActorType: audit.ActorSystem}
	if ip := clientIP(r); ip != nil {
		e.IP = ip.String()
	}
	if p, ok := auth.GetPrincipal(r.Context()); ok {
		id := p.UserID
		e.ActorID = &id
		e.ActorType = audit.ActorUser
		if p.StaffGrantID != uuid.Nil {
			e.ActorType = audit.ActorStaff
		}
	}
	return s.aud().Record(r.Context(), q, e)
}

func (s *Server) orgRoutes(r chi.Router) {
	r.Get("/permissions", s.handlePermissionCatalogue)
	r.Post("/auth/step-up", s.handleStepUp)
	r.Route("/orgs", func(r chi.Router) {
		r.Get("/", s.handleListOrgs)
		r.Post("/", s.handleCreateOrg)
		r.Route("/{org}", func(r chi.Router) {
			r.Use(s.orgContext)
			r.Get("/", s.handleGetOrg)
			r.With(requireOrg(authz.OrgManage)).Patch("/", s.handleUpdateOrg)
			r.Get("/entitlements", s.handleOrgEntitlements)
			r.Get("/features", s.handleOrgFeatures)
			r.Get("/workspaces", s.handleOrgWorkspaces)
			r.With(requireOrg(authz.OrgManage)).Post("/workspaces", s.handleOrgCreateWorkspace)
			r.With(requireOrg(authz.OrgMembers)).Get("/members", s.handleOrgMembers)
			r.With(requireOrg(authz.OrgMembers)).Patch("/members/{userId}", s.handleOrgUpdateMember)
			r.With(requireOrg(authz.OrgMembers)).Delete("/members/{userId}", s.handleOrgRemoveMember)
			r.With(s.requireStepUp(10*time.Minute)).Post("/transfer-ownership", s.handleOrgTransferOwnership)
			r.With(requireOrg(authz.OrgPolicy)).Get("/security-policy", s.handleGetSecurityPolicy)
			r.With(requireOrg(authz.OrgPolicy), s.requireStepUp(10*time.Minute)).Put("/security-policy", s.handlePutSecurityPolicy)
			r.With(requireOrg(authz.OrgAudit)).Get("/audit-logs", s.handleOrgAuditLogs)
			r.With(requireOrg(authz.OrgAudit)).Get("/audit-logs/verify", s.handleAuditVerify)
			r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Get("/audit-logs/export", s.handleAuditExport)
			s.ssoOrgRoutes(r)
			s.scimDirectoryRoutes(r)
			s.governanceOrgRoutes(r)
			s.auditStreamRoutes(r)
			for _, m := range s.orgMounts {
				m(r)
			}
		})
	})
}

// MountOrg registers routes under /v1/orgs/{org} (org context + identity gate applied).
func (s *Server) MountOrg(fn func(r chi.Router)) { s.orgMounts = append(s.orgMounts, fn) }

// OrgFromRequest exposes the resolved organisation to enterprise modules.
func OrgFromRequest(r *http.Request) (org.Org, string, bool) {
	v, ok := r.Context().Value(orgCtxKey{}).(orgContextValue)
	return v.org, v.role, ok
}

// OrgCan reports an org-level permission for enterprise modules.
func OrgCan(r *http.Request, p authz.Permission) bool { return orgCan(r, p) }

// ---- handlers ---------------------------------------------------------------

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if p.IsAPIKey() {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	orgs, err := org.ListForUser(r.Context(), s.pool, p.UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list organisations"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": orgs})
}

type createOrgReq struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	WorkspaceName string `json:"workspace_name"`
	Kind          string `json:"kind"`
}

var orgSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,46}[a-z0-9]$`)

// POST /orgs: a new organisation with its first workspace.
func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if p.IsAPIKey() {
		fail(w, forbidden("api_key_not_allowed", "API keys cannot create organisations"))
		return
	}
	var req createOrgReq
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 80 {
		fail(w, unprocessable("invalid_name", "name must be 1–80 characters"))
		return
	}
	if req.Slug = trimLower(req.Slug); req.Slug != "" && !orgSlugPattern.MatchString(req.Slug) {
		fail(w, unprocessable("invalid_slug", "slug must be 3–48 lowercase letters, digits and hyphens"))
		return
	}
	if req.Kind == "" {
		req.Kind = "standard"
	}
	if req.Kind != "standard" && req.Kind != "agency" {
		fail(w, unprocessable("invalid_kind", "kind must be standard or agency"))
		return
	}
	var owned, limit int
	err := s.pool.QueryRow(r.Context(), `SELECT count(*), COALESCE(max(CASE o.plan_id WHEN 'enterprise' THEN 100 WHEN 'business' THEN 10
		WHEN 'pro' THEN 3 ELSE 1 END), 1) FROM org_members m JOIN organizations o ON o.id = m.org_id
		WHERE m.user_id = $1 AND m.org_role = 'org_owner' AND o.deleted_at IS NULL`, p.UserID).Scan(&owned, &limit)
	if err != nil {
		fail(w, apierr.Internal("failed to check quota"))
		return
	}
	if owned >= limit+1 { // one extra: a personal org plus the plan's allowance
		pd := paymentRequired("limit_reached", "you already own the maximum number of organisations for your plan")
		pd.RequiredPlan = "pro"
		fail(w, pd)
		return
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Unauthorized("user not found"))
		return
	}
	wsName := strings.TrimSpace(req.WorkspaceName)
	if wsName == "" {
		wsName = req.Name
	}
	var o org.Org
	var ws dbgen.Workspace
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		if req.Slug != "" {
			var exists bool
			_ = tx.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM organizations WHERE slug = $1)`, req.Slug).Scan(&exists)
			if exists {
				return apierr.Conflict("slug_exists", "an organisation with this slug already exists")
			}
		}
		if o, err = org.Create(r.Context(), tx, org.CreateParams{Name: req.Name, Slug: req.Slug, Kind: req.Kind, OwnerID: u.ID}); err != nil {
			return err
		}
		if ws, err = s.createWorkspace(r.Context(), q, tx, u, wsName, "", &o.ID); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "org.created", "organization", &o.ID, map[string]any{"name": o.Name, "slug": o.Slug, "kind": o.Kind})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create organisation"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"organization": o, "workspace": toWorkspaceDTO(ws, "owner")})
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	v, _ := r.Context().Value(orgCtxKey{}).(orgContextValue)
	perms := v.perms
	if perms == nil {
		perms = []authz.Permission{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"organization": v.org, "org_role": v.role, "permissions": perms})
}

type updateOrgReq struct {
	Name           opt[string]         `json:"name"`
	LegalName      opt[string]         `json:"legal_name"`
	BillingEmail   opt[string]         `json:"billing_email"`
	GSTIN          opt[string]         `json:"gstin"`
	TaxCountry     opt[string]         `json:"tax_country"`
	BillingAddress opt[map[string]any] `json:"billing_address"`
}

var gstinPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)

func (s *Server) handleUpdateOrg(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	var req updateOrgReq
	if !decode(w, r, &req) {
		return
	}
	sets := []string{}
	args := []any{o.ID}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+itoa(len(args)))
	}
	if req.Name.Set {
		n := strings.TrimSpace(req.Name.Value)
		if n == "" || len(n) > 80 {
			fail(w, unprocessable("invalid_name", "name must be 1–80 characters"))
			return
		}
		add("name", n)
	}
	if req.LegalName.Set {
		add("legal_name", req.LegalName.ptr())
	}
	if req.BillingEmail.Set {
		if v := req.BillingEmail.ptr(); v != nil {
			e, ok := validEmail(*v)
			if !ok {
				fail(w, unprocessable("invalid_email", "billing_email must be an email address"))
				return
			}
			add("billing_email", e)
		} else {
			add("billing_email", nil)
		}
	}
	if req.GSTIN.Set {
		if v := req.GSTIN.ptr(); v != nil {
			g := strings.ToUpper(strings.TrimSpace(*v))
			if !gstinPattern.MatchString(g) || !validGSTINChecksum(g) {
				fail(w, unprocessable("invalid_gstin", "gstin is not a valid 15-character GSTIN"))
				return
			}
			add("gstin", g)
		} else {
			add("gstin", nil)
		}
	}
	if req.TaxCountry.Set {
		c := strings.ToUpper(strings.TrimSpace(req.TaxCountry.Value))
		if len(c) != 2 {
			fail(w, unprocessable("invalid_country", "tax_country must be an ISO 3166-1 alpha-2 code"))
			return
		}
		add("tax_country", c)
	}
	if req.BillingAddress.Set {
		v := req.BillingAddress.Value
		if v == nil {
			v = map[string]any{}
		}
		add("billing_address", v)
	}
	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, o)
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE organizations SET `+strings.Join(sets, ", ")+`, updated_at = now() WHERE id = $1`, args...); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "org.updated", "organization", &o.ID, map[string]any{"fields": len(sets)})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update organisation"))
		return
	}
	updated, _ := org.Get(r.Context(), s.pool, o.ID)
	writeJSON(w, http.StatusOK, updated)
}

// validGSTINChecksum verifies the 15th character (base-36 weighted checksum).
func validGSTINChecksum(g string) bool {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sum := 0
	for i := 0; i < 14; i++ {
		v := strings.IndexByte(chars, g[i])
		if v < 0 {
			return false
		}
		f := 1
		if i%2 == 1 {
			f = 2
		}
		p := v * f
		sum += p/36 + p%36
	}
	check := (36 - sum%36) % 36
	return chars[check] == g[14]
}

func (s *Server) handleOrgEntitlements(w http.ResponseWriter, r *http.Request) {
	eff, err := s.plans.ForOrg(r.Context(), orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to resolve entitlements"))
		return
	}
	writeJSON(w, http.StatusOK, eff)
}

func (s *Server) handleOrgFeatures(w http.ResponseWriter, r *http.Request) {
	m, err := s.flags.All(r.Context(), orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to load feature flags"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"flags": m})
}

// GET /orgs/{org}/workspaces: admins see all; members see the ones they can access.
func (s *Server) handleOrgWorkspaces(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	p := principal(r)
	all := orgCan(r, authz.OrgManage)
	rows, err := s.pool.Query(r.Context(), `SELECT w.id, w.name, w.slug::text, w.plan_id, w.timezone, w.is_sandbox, w.created_at,
		m.role, (SELECT count(*) FROM qr_codes q WHERE q.workspace_id = w.id AND q.deleted_at IS NULL)::int,
		(SELECT count(*) FROM workspace_members x WHERE x.workspace_id = w.id)::int
		FROM workspaces w LEFT JOIN workspace_members m ON m.workspace_id = w.id AND m.user_id = $2
		WHERE w.org_id = $1 AND w.deleted_at IS NULL
		  AND ($3 OR m.user_id IS NOT NULL OR EXISTS (SELECT 1 FROM role_bindings b JOIN group_members gm
		       ON b.principal_type = 'group' AND b.principal_id = gm.group_id AND gm.user_id = $2 WHERE b.workspace_id = w.id))
		ORDER BY w.name`, o.ID, p.UserID, all)
	if err != nil {
		fail(w, apierr.Internal("failed to list workspaces"))
		return
	}
	defer rows.Close()
	type row struct {
		ID        uuid.UUID `json:"id"`
		Name      string    `json:"name"`
		Slug      string    `json:"slug"`
		PlanID    string    `json:"plan_id"`
		Timezone  string    `json:"timezone"`
		IsSandbox bool      `json:"is_sandbox"`
		CreatedAt time.Time `json:"created_at"`
		Role      *string   `json:"role"`
		QRCount   int       `json:"qr_count"`
		Members   int       `json:"member_count"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.Name, &x.Slug, &x.PlanID, &x.Timezone, &x.IsSandbox, &x.CreatedAt, &x.Role, &x.QRCount, &x.Members); err != nil {
			fail(w, apierr.Internal("failed to list workspaces"))
			return
		}
		out = append(out, x)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) handleOrgCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceReq
	if !decode(w, r, &req) {
		return
	}
	id := orgRow(r).ID
	req.OrgID = &id
	r2 := r.Clone(r.Context())
	body, _ := jsonBody(req)
	r2.Body = body
	r2.ContentLength = -1
	s.handleCreateWorkspace(w, r2)
}

func (s *Server) handleOrgMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := org.ListMembers(r.Context(), s.pool, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list members"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}

type orgMemberReq struct {
	OrgRole *string `json:"org_role"`
	Status  *string `json:"status"`
}

func (s *Server) handleOrgUpdateMember(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	target, ok := uuidParam(r, "userId")
	if !ok {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	var req orgMemberReq
	if !decode(w, r, &req) {
		return
	}
	cur, err := org.GetMember(r.Context(), s.pool, o.ID, target)
	if err != nil {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	actor := principal(r)
	if target == actor.UserID {
		fail(w, forbidden("self_change", "you cannot change your own organisation role or status"))
		return
	}
	isOwner := orgRole(r) == org.RoleOwner
	if cur.Role == org.RoleOwner {
		fail(w, forbidden("owner_immutable", "the owner's role and status can only change through ownership transfer"))
		return
	}
	if cur.Role == org.RoleAdmin && !isOwner {
		fail(w, forbidden("owner_required", "only the organisation owner can change an admin"))
		return
	}
	changes := map[string]any{}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if req.OrgRole != nil {
			role := *req.OrgRole
			if !org.ValidRole(role) || role == org.RoleOwner {
				return unprocessable("invalid_org_role", "org_role must be org_admin, billing_admin or member")
			}
			if role == org.RoleAdmin && !isOwner {
				return forbidden("owner_required", "only the organisation owner can make admins")
			}
			if err := org.SetRole(r.Context(), tx, o.ID, target, role); err != nil {
				return err
			}
			changes["org_role"] = map[string]any{"before": cur.Role, "after": role}
		}
		if req.Status != nil {
			st := *req.Status
			if st != "active" && st != "suspended" {
				return unprocessable("invalid_status", "status must be active or suspended")
			}
			if err := org.SetStatus(r.Context(), tx, o.ID, target, st); err != nil {
				return err
			}
			changes["status"] = map[string]any{"before": cur.Status, "after": st}
			if st == "suspended" && (cur.Source == "scim" || cur.Source == "sso_jit") {
				// Org-managed identities: end every session now, not at token expiry.
				if err := q.RevokeUserSessions(r.Context(), target); err != nil {
					return err
				}
			}
		}
		action := "member.org.updated"
		if req.Status != nil && *req.Status == "suspended" {
			action = "member.suspended"
		}
		return s.auditOrg(r, q, o.ID, action, "user", &target, changes)
	})
	if err != nil {
		if errors.Is(err, org.ErrLastOwner) {
			fail(w, forbidden("owner_immutable", "the owner cannot be changed this way"))
			return
		}
		fail(w, problemOr500(err, "failed to update member"))
		return
	}
	s.access.Invalidate(r.Context())
	s.sessions.evictAll()
	m, _ := org.GetMember(r.Context(), s.pool, o.ID, target)
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleOrgRemoveMember(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	target, ok := uuidParam(r, "userId")
	if !ok {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	cur, err := org.GetMember(r.Context(), s.pool, o.ID, target)
	if err != nil {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	if cur.Role == org.RoleAdmin && orgRole(r) != org.RoleOwner {
		fail(w, forbidden("owner_required", "only the organisation owner can remove an admin"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if err := org.Remove(r.Context(), tx, o.ID, target); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "member.removed", "user", &target, map[string]any{"org_role": cur.Role})
	})
	if err != nil {
		if errors.Is(err, org.ErrLastOwner) {
			fail(w, forbidden("owner_immutable", "transfer ownership before removing the owner"))
			return
		}
		fail(w, problemOr500(err, "failed to remove member"))
		return
	}
	s.access.Invalidate(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOrgTransferOwnership(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if orgRole(r) != org.RoleOwner {
		fail(w, forbidden("owner_required", "only the owner can transfer the organisation"))
		return
	}
	var req transferReq
	if !decode(w, r, &req) {
		return
	}
	from := principal(r).UserID
	if req.UserID == from {
		fail(w, unprocessable("already_owner", "you already own this organisation"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if err := org.TransferOwnership(r.Context(), tx, o.ID, from, req.UserID); err != nil {
			if errors.Is(err, org.ErrNotMember) {
				return unprocessable("not_a_member", "the new owner must be an active member of the organisation")
			}
			return err
		}
		return s.auditOrg(r, q, o.ID, "org.ownership.transferred", "organization", &o.ID, map[string]any{"from": from, "to": req.UserID})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to transfer ownership"))
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"owner_id": req.UserID})
}

// ---- step-up ----------------------------------------------------------------

type stepUpReq struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

// POST /auth/step-up: re-confirm identity (password, plus a second factor when enrolled).
func (s *Server) handleStepUp(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	sess, ok := currentSession(r)
	if p.IsAPIKey() || !ok {
		fail(w, forbidden("session_required", "step-up needs a signed-in session"))
		return
	}
	if allowed, _, retry, _ := s.limiter.Allow(r.Context(), "rl:stepup:"+p.UserID.String(), 5, 15*time.Minute); !allowed {
		fail(w, tooMany("too many attempts; try again later", retry))
		return
	}
	var req stepUpReq
	if !decode(w, r, &req) {
		return
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Unauthorized("user not found"))
		return
	}
	if u.PasswordHash == nil {
		pd := forbidden("sso_reauth_required", "this account signs in with SSO; re-authenticate with your identity provider")
		pd.Instance = "/v1/auth/sso/start?prompt=login"
		fail(w, pd)
		return
	}
	okPw, _ := crypto.VerifyPassword(req.Password, *u.PasswordHash)
	if !okPw {
		fail(w, apierr.Unauthorized("incorrect password"))
		return
	}
	if err := s.idh().VerifySecondFactor(r.Context(), u.ID, req.Code); err != nil {
		fail(w, err)
		return
	}
	if _, err := s.pool.Exec(r.Context(), `UPDATE sessions SET step_up_at = now() WHERE id = $1`, sess.ID); err != nil {
		fail(w, apierr.Internal("failed to record step-up"))
		return
	}
	s.sessions.evict(sess.ID)
	_ = s.aud().Record(r.Context(), s.q, auditUserEntry(r, u.ID, "auth.step_up"))
	writeJSON(w, http.StatusOK, map[string]any{"step_up_until": time.Now().Add(10 * time.Minute).UTC()})
}

// ---- permissions catalogue --------------------------------------------------

var permissionDescriptions = map[authz.Permission]string{
	authz.WorkspaceRead: "View the workspace", authz.WorkspaceUpdate: "Rename and configure the workspace",
	authz.MemberManage: "Invite and manage members", authz.RoleManage: "Assign roles and bindings",
	authz.QRRead: "View QR codes", authz.QRCreate: "Create QR codes", authz.QRUpdate: "Edit QR code settings",
	authz.QRDelete: "Delete and restore QR codes", authz.QRDestinationUpdate: "Change where codes point",
	authz.QRDestinationApprove: "Approve destination changes", authz.QRDesignBypassLock: "Override locked brand templates",
	authz.FolderManage: "Create and organise folders", authz.CampaignManage: "Manage campaigns",
	authz.TemplateManage: "Manage design templates", authz.AnalyticsRead: "View analytics",
	authz.AnalyticsExport: "Export analytics", authz.AnalyticsRaw: "View the raw scan log",
	authz.DomainManage: "Manage custom domains", authz.APIKeyManage: "Manage API keys",
	authz.WebhookManage: "Manage webhooks", authz.IntegrationManage: "Manage integrations",
	authz.PolicyManage: "Change workspace policies", authz.AuditRead: "View the audit log",
	authz.AuditExport: "Export the audit log", authz.FormManage: "Manage lead forms", authz.LeadRead: "View leads",
	authz.LeadExport: "Export leads", authz.PixelManage: "Manage retargeting pixels", authz.AlertManage: "Manage alerts",
	authz.ReportManage: "Manage scheduled reports", authz.SerialManage: "Manage serialised batches",
	authz.GS1Manage: "Manage GS1 items", authz.BulkRun: "Run bulk jobs",
}

func (s *Server) handlePermissionCatalogue(w http.ResponseWriter, r *http.Request) {
	type perm struct {
		Key         authz.Permission `json:"key"`
		Group       string           `json:"group"`
		Description string           `json:"description"`
	}
	out := make([]perm, 0, len(authz.Catalogue))
	for _, p := range authz.Catalogue {
		out = append(out, perm{Key: p, Group: strings.SplitN(string(p), ".", 2)[0], Description: permissionDescriptions[p]})
	}
	roles := map[string][]authz.Permission{}
	for k, v := range authz.SystemRoles {
		roles[k] = v
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": out, "system_roles": roles, "org_roles": authz.OrgRolePermissions})
}

func (s *Server) logErr(msg string, err error) {
	if err != nil {
		slog.Error(msg, "error", err)
	}
}

// requireFeatureOrg gates an org-level route on an entitlement.
func (s *Server) requireFeatureOrg(r *http.Request, feature string) error {
	eff, err := s.plans.ForOrg(r.Context(), orgRow(r).ID)
	if err != nil {
		return apierr.Internal("failed to resolve entitlements")
	}
	if eff.Has(feature) {
		return nil
	}
	pd := paymentRequired("upgrade_required", "this needs the "+string(entitlements.RequiredPlan(feature))+" plan")
	pd.RequiredPlan = string(entitlements.RequiredPlan(feature))
	return pd
}
