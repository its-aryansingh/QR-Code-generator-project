// Package access resolves who may do what: effective permissions from organisation roles,
// role bindings (direct and via groups, workspace- or folder-scoped) and API key scopes,
// plus the organisation security policy used by the identity gate.
package access

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// InvalidateChannel tells every API instance to drop cached grants and policies.
const InvalidateChannel = "authz:invalidate"

// Engine caches grants for 30 s and policies for 60 s; any change to bindings, groups,
// roles, org members or policies calls Invalidate, which also notifies other instances.
type Engine struct {
	pool     *pgxpool.Pool
	rdb      *redis.Client
	grants   *expirable.LRU[string, *authz.Grants]
	policies *expirable.LRU[uuid.UUID, *Policy]
	mu       sync.Mutex
}

func NewEngine(pool *pgxpool.Pool, rdb *redis.Client) *Engine {
	return &Engine{pool: pool, rdb: rdb,
		grants:   expirable.NewLRU[string, *authz.Grants](20000, nil, 30*time.Second),
		policies: expirable.NewLRU[uuid.UUID, *Policy](5000, nil, 60*time.Second)}
}

// Invalidate drops local caches and asks other instances to do the same.
func (e *Engine) Invalidate(ctx context.Context) {
	e.grants.Purge()
	e.policies.Purge()
	if e.rdb != nil {
		_ = e.rdb.Publish(ctx, InvalidateChannel, "*").Err()
	}
}

// Subscribe purges caches on remote invalidations until ctx ends.
func (e *Engine) Subscribe(ctx context.Context) {
	if e.rdb == nil {
		return
	}
	sub := e.rdb.Subscribe(ctx, InvalidateChannel)
	defer sub.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sub.Channel():
			if !ok {
				return
			}
			e.grants.Purge()
			e.policies.Purge()
		}
	}
}

// Grants returns the principal's permissions in the workspace, or nil when it has none.
func (e *Engine) Grants(ctx context.Context, p *auth.Principal, ws dbgen.Workspace) (*authz.Grants, error) {
	if p == nil {
		return nil, nil
	}
	if p.IsAPIKey() {
		return apiKeyGrants(p, ws), nil
	}
	key := ws.ID.String() + ":" + p.UserID.String()
	if p.StaffGrantID != uuid.Nil {
		key += ":staff:" + p.StaffGrantID.String()
	}
	if g, ok := e.grants.Get(key); ok {
		return g, nil
	}
	g, err := e.load(ctx, p, ws)
	if err != nil {
		return nil, err
	}
	e.grants.Add(key, g)
	return g, nil
}

// apiKeyGrants maps scopes to permissions. A key is bound to one workspace and never
// exceeds an editor; it can never approve changes.
func apiKeyGrants(p *auth.Principal, ws dbgen.Workspace) *authz.Grants {
	if p.KeyWorkspaceID != ws.ID {
		return nil
	}
	editor := map[authz.Permission]bool{}
	for _, perm := range authz.SystemRoles["editor"] {
		editor[perm] = true
	}
	editor[authz.WebhookManage] = true
	editor[authz.LeadRead] = true
	g := authz.NewGrants()
	for _, sc := range p.Scopes {
		var ok []authz.Permission
		for _, perm := range authz.APIKeyScopePermissions[sc] {
			if editor[perm] {
				ok = append(ok, perm)
			}
		}
		g.Add(ok, nil)
	}
	g.Sources = append(g.Sources, authz.Source{Kind: "api_key"})
	return g
}

func (e *Engine) load(ctx context.Context, p *auth.Principal, ws dbgen.Workspace) (*authz.Grants, error) {
	g := authz.NewGrants()
	// Customer-granted staff support access (the only way staff see customer data).
	if p.StaffGrantID != uuid.Nil {
		var scope string
		err := e.pool.QueryRow(ctx, `SELECT scope FROM support_access_grants
			WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL AND expires_at > now()`, p.StaffGrantID, ws.OrgID).Scan(&scope)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		g.Add(StaffPermissions(scope), nil)
		g.Sources = append(g.Sources, authz.Source{Kind: "support_grant", RoleKey: scope})
		return g, nil
	}

	var role, status string
	err := e.pool.QueryRow(ctx, `SELECT org_role, status FROM org_members WHERE org_id = $1 AND user_id = $2`,
		ws.OrgID, p.UserID).Scan(&role, &status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Agency administrators manage their client organisations as admins.
		var parentRole string
		err := e.pool.QueryRow(ctx, `SELECT m.org_role FROM organizations o
			JOIN org_members m ON m.org_id = o.parent_org_id AND m.user_id = $2 AND m.status = 'active'
			WHERE o.id = $1 AND m.org_role IN ('org_owner','org_admin')`, ws.OrgID, p.UserID).Scan(&parentRole)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		g.Add(authz.SystemRoles["admin"], nil)
		g.Sources = append(g.Sources, authz.Source{Kind: "agency", OrgRole: parentRole})
		return g, nil
	case err != nil:
		return nil, err
	}
	if status != "active" {
		return nil, nil // suspended and deprovisioned members lose all access at once
	}
	switch role {
	case "org_owner":
		g.Add([]authz.Permission{authz.All}, nil)
		g.Sources = append(g.Sources, authz.Source{Kind: "org_role", OrgRole: role})
	case "org_admin":
		g.Add(authz.SystemRoles["admin"], nil)
		g.Sources = append(g.Sources, authz.Source{Kind: "org_role", OrgRole: role})
	}
	rows, err := e.pool.Query(ctx, `
		WITH principal AS (
		    SELECT 'user'::text AS principal_type, $1::uuid AS principal_id
		    UNION ALL
		    SELECT 'group', gm.group_id FROM group_members gm JOIN groups gr ON gr.id = gm.group_id
		    WHERE gm.user_id = $1 AND gr.org_id = $3
		)
		SELECT b.id, b.principal_type, b.principal_id, r.key, r.permissions, b.folder_id
		FROM role_bindings b
		JOIN principal p ON p.principal_type = b.principal_type AND p.principal_id = b.principal_id
		JOIN roles r ON r.id = b.role_id
		WHERE b.workspace_id = $2`, p.UserID, ws.ID, ws.OrgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			bindingID, principalID uuid.UUID
			principalType, roleKey string
			perms                  []string
			folder                 *uuid.UUID
		)
		if err := rows.Scan(&bindingID, &principalType, &principalID, &roleKey, &perms, &folder); err != nil {
			return nil, err
		}
		ps := make([]authz.Permission, len(perms))
		for i, s := range perms {
			ps[i] = authz.Permission(s)
		}
		g.Add(ps, folder)
		src := authz.Source{Kind: "role_binding", RoleKey: roleKey, FolderID: folder, BindingID: &bindingID}
		if principalType == "group" {
			src.Kind = "group_binding"
			gid := principalID
			src.GroupID = &gid
		}
		g.Sources = append(g.Sources, src)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(g.Sources) == 0 {
		return nil, nil // an org member without any grant in this workspace
	}
	return g, nil
}

// StaffPermissions maps a support grant scope to permissions: read = analyst + audit.read;
// read_write = admin without billing, identity, policy and key management.
func StaffPermissions(scope string) []authz.Permission {
	if scope == "read_write" {
		var out []authz.Permission
		for _, p := range authz.SystemRoles["admin"] {
			switch p {
			case authz.PolicyManage, authz.APIKeyManage, authz.RoleManage, authz.MemberManage:
				continue
			}
			out = append(out, p)
		}
		return out
	}
	return append(append([]authz.Permission{}, authz.SystemRoles["analyst"]...), authz.AuditRead)
}

// OrgPermissions returns a member's org-level permissions (nil when not an active member).
func (e *Engine) OrgPermissions(ctx context.Context, orgID, userID uuid.UUID) (string, []authz.Permission, error) {
	var role, status string
	err := e.pool.QueryRow(ctx, `SELECT org_role, status FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID).
		Scan(&role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		var parentRole string
		err := e.pool.QueryRow(ctx, `SELECT m.org_role FROM organizations o
			JOIN org_members m ON m.org_id = o.parent_org_id AND m.user_id = $2 AND m.status = 'active'
			WHERE o.id = $1 AND m.org_role IN ('org_owner','org_admin')`, orgID, userID).Scan(&parentRole)
		if err != nil {
			return "", nil, nil
		}
		// Agency admins manage clients as org admins (never as owner/billing).
		return "agency_admin", authz.OrgRolePermissions["org_admin"], nil
	}
	if err != nil || status != "active" {
		return role, nil, err
	}
	return role, authz.OrgRolePermissions[role], nil
}

// Policy is an organisation's security policy.
type Policy struct {
	OrgID                uuid.UUID      `json:"org_id"`
	EnforceSSO           bool           `json:"enforce_sso"`
	BreakGlassUserIDs    []uuid.UUID    `json:"sso_break_glass_user_ids"`
	RequireMFA           bool           `json:"require_mfa"`
	AllowedMFAKinds      []string       `json:"allowed_mfa_kinds"`
	SessionIdleMinutes   int            `json:"session_idle_minutes"`
	SessionMaxHours      int            `json:"session_max_hours"`
	DashboardIPAllowlist []netip.Prefix `json:"dashboard_ip_allowlist"`
	APIIPAllowlist       []netip.Prefix `json:"api_ip_allowlist"`
	PasswordMinLength    int            `json:"password_min_length"`
	InviteEmailDomains   []string       `json:"invite_email_domains"`
	APIKeyMaxDays        int            `json:"api_key_max_days"`
	ExportPermission     string         `json:"export_permission"`
	UpdatedBy            *uuid.UUID     `json:"updated_by"`
	UpdatedAt            time.Time      `json:"updated_at"`
	// BillingHold freezes dashboard edits (unpaid invoice > 30 days overdue); redirects never stop.
	BillingHold bool `json:"-"`
}

// Policy loads (and caches) an organisation's security policy.
func (e *Engine) Policy(ctx context.Context, orgID uuid.UUID) (*Policy, error) {
	if p, ok := e.policies.Get(orgID); ok {
		return p, nil
	}
	p, err := LoadPolicy(ctx, e.pool, orgID)
	if err != nil {
		return nil, err
	}
	e.policies.Add(orgID, p)
	return p, nil
}

// LoadPolicy reads the policy row (defaults when absent).
func LoadPolicy(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, orgID uuid.UUID) (*Policy, error) {
	p := &Policy{OrgID: orgID, AllowedMFAKinds: []string{"totp", "webauthn"}, PasswordMinLength: 10, ExportPermission: "role"}
	var domains []string
	err := db.QueryRow(ctx, `SELECT enforce_sso, sso_break_glass_user_ids, require_mfa, allowed_mfa_kinds, session_idle_minutes,
		session_max_hours, dashboard_ip_allowlist, api_ip_allowlist, password_min_length, invite_email_domains::text[],
		api_key_max_days, export_permission, updated_by, updated_at
		FROM org_security_policies WHERE org_id = $1`, orgID).Scan(&p.EnforceSSO, &p.BreakGlassUserIDs, &p.RequireMFA,
		&p.AllowedMFAKinds, &p.SessionIdleMinutes, &p.SessionMaxHours, &p.DashboardIPAllowlist, &p.APIIPAllowlist,
		&p.PasswordMinLength, &domains, &p.APIKeyMaxDays, &p.ExportPermission, &p.UpdatedBy, &p.UpdatedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	p.InviteEmailDomains = domains
	if err := db.QueryRow(ctx, `SELECT billing_hold_since IS NOT NULL FROM organizations WHERE id = $1`, orgID).Scan(&p.BillingHold); err != nil &&
		!errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	return p, nil
}

// IPAllowed reports whether ip is inside the list (an empty list allows everything).
func IPAllowed(list []netip.Prefix, ip netip.Addr) bool {
	if len(list) == 0 {
		return true
	}
	if !ip.IsValid() {
		return false
	}
	ip = ip.Unmap()
	for _, p := range list {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// IsBreakGlass reports whether the user is one of the policy's break-glass owners.
func (p *Policy) IsBreakGlass(userID uuid.UUID) bool {
	for _, id := range p.BreakGlassUserIDs {
		if id == userID {
			return true
		}
	}
	return false
}
