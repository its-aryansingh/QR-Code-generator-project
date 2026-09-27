// Package authz is the permission-based authorization model.
//
// Roles are named permission sets (5 system roles + org custom roles). Principals receive
// roles through bindings at workspace scope or folder scope. A handler asks one question:
// "does this principal hold permission P for this resource?" — never "what is the role?".
package authz

import (
	"context"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
)

type Permission string

// Workspace-scoped permissions (catalogue is stable; UI groups them).
const (
	WorkspaceRead        Permission = "workspace.read"
	WorkspaceUpdate      Permission = "workspace.update"
	MemberManage         Permission = "member.manage"
	RoleManage           Permission = "role.manage"
	QRRead               Permission = "qr.read"
	QRCreate             Permission = "qr.create"
	QRUpdate             Permission = "qr.update"
	QRDelete             Permission = "qr.delete"
	QRDestinationUpdate  Permission = "qr.destination.update"
	QRDestinationApprove Permission = "qr.destination.approve"
	QRDesignBypassLock   Permission = "qr.design.bypass_lock"
	FolderManage         Permission = "folder.manage"
	CampaignManage       Permission = "campaign.manage"
	TemplateManage       Permission = "template.manage"
	AnalyticsRead        Permission = "analytics.read"
	AnalyticsExport      Permission = "analytics.export"
	AnalyticsRaw         Permission = "analytics.raw"
	DomainManage         Permission = "domain.manage"
	APIKeyManage         Permission = "apikey.manage"
	WebhookManage        Permission = "webhook.manage"
	IntegrationManage    Permission = "integration.manage"
	PolicyManage         Permission = "policy.manage"
	AuditRead            Permission = "audit.read"
	AuditExport          Permission = "audit.export"
	FormManage           Permission = "form.manage"
	LeadRead             Permission = "lead.read"
	LeadExport           Permission = "lead.export"
	PixelManage          Permission = "pixel.manage"
	AlertManage          Permission = "alert.manage"
	ReportManage         Permission = "report.manage"
	SerialManage         Permission = "serial.manage"
	GS1Manage            Permission = "gs1.manage"
	BulkRun              Permission = "bulk.run"
	BillingManage        Permission = "billing.manage" // owner only (via "*")
	WorkspaceDelete      Permission = "workspace.delete"
	All                  Permission = "*"
)

// Org-level permissions are held through org_members.org_role, not bindings.
const (
	OrgManage   Permission = "org.manage"
	OrgBilling  Permission = "org.billing"
	OrgSSO      Permission = "org.sso"
	OrgSCIM     Permission = "org.scim"
	OrgPolicy   Permission = "org.policy"
	OrgAudit    Permission = "org.audit"
	OrgMembers  Permission = "org.members"
	OrgBranding Permission = "org.branding"
	OrgClients  Permission = "org.clients"
)

// Catalogue lists every assignable workspace permission (used by the roles editor and validation).
var Catalogue = []Permission{
	WorkspaceRead, WorkspaceUpdate, MemberManage, RoleManage, QRRead, QRCreate, QRUpdate, QRDelete,
	QRDestinationUpdate, QRDestinationApprove, QRDesignBypassLock, FolderManage, CampaignManage,
	TemplateManage, AnalyticsRead, AnalyticsExport, AnalyticsRaw, DomainManage, APIKeyManage,
	WebhookManage, IntegrationManage, PolicyManage, AuditRead, AuditExport, FormManage, LeadRead,
	LeadExport, PixelManage, AlertManage, ReportManage, SerialManage, GS1Manage, BulkRun,
}

// IsAssignable reports whether p can appear in a custom role.
func IsAssignable(p Permission) bool {
	for _, c := range Catalogue {
		if c == p {
			return true
		}
	}
	return false
}

// SystemRoles mirrors the seed rows in migration 00003 (roles with org_id NULL).
var SystemRoles = map[string][]Permission{
	"owner": {All},
	"admin": {WorkspaceRead, WorkspaceUpdate, MemberManage, RoleManage, QRRead, QRCreate, QRUpdate, QRDelete,
		QRDestinationUpdate, QRDestinationApprove, QRDesignBypassLock, FolderManage, CampaignManage,
		TemplateManage, AnalyticsRead, AnalyticsExport, AnalyticsRaw, DomainManage, APIKeyManage,
		WebhookManage, IntegrationManage, PolicyManage, AuditRead, AuditExport, FormManage,
		LeadRead, LeadExport, PixelManage, AlertManage, ReportManage, SerialManage, GS1Manage, BulkRun},
	"editor": {WorkspaceRead, QRRead, QRCreate, QRUpdate, QRDestinationUpdate, FolderManage, CampaignManage,
		TemplateManage, AnalyticsRead, AnalyticsExport, FormManage, LeadRead, AlertManage, ReportManage, BulkRun},
	"reviewer": {WorkspaceRead, QRRead, QRDestinationApprove, AnalyticsRead, AuditRead, LeadRead},
	"analyst":  {WorkspaceRead, QRRead, AnalyticsRead, AnalyticsExport},
}

// OrgRolePermissions maps org_members.org_role to org-level permissions.
var OrgRolePermissions = map[string][]Permission{
	"org_owner":     {OrgManage, OrgBilling, OrgSSO, OrgSCIM, OrgPolicy, OrgAudit, OrgMembers, OrgBranding, OrgClients},
	"org_admin":     {OrgManage, OrgSSO, OrgSCIM, OrgPolicy, OrgAudit, OrgMembers, OrgBranding, OrgClients},
	"billing_admin": {OrgBilling},
	"member":        {},
}

// APIKeyScopePermissions maps API key scopes to permissions. Keys never approve.
var APIKeyScopePermissions = map[string][]Permission{
	"qr:read":        {WorkspaceRead, QRRead},
	"qr:write":       {WorkspaceRead, QRRead, QRCreate, QRUpdate, QRDestinationUpdate},
	"analytics:read": {WorkspaceRead, AnalyticsRead},
	"webhooks:write": {WorkspaceRead, WebhookManage},
	"leads:read":     {WorkspaceRead, LeadRead},
}

// Grants is the resolved permission set of a principal in one workspace.
type Grants struct {
	all       bool
	workspace map[Permission]bool
	folders   map[uuid.UUID]map[Permission]bool
	// Sources explain where permissions came from (for the access inspector).
	Sources []Source
}

type Source struct {
	Kind      string     `json:"kind"` // role_binding | group_binding | org_role | api_key | membership
	RoleKey   string     `json:"role_key,omitempty"`
	GroupID   *uuid.UUID `json:"group_id,omitempty"`
	FolderID  *uuid.UUID `json:"folder_id,omitempty"`
	OrgRole   string     `json:"org_role,omitempty"`
	BindingID *uuid.UUID `json:"binding_id,omitempty"`
}

func NewGrants() *Grants {
	return &Grants{workspace: map[Permission]bool{}, folders: map[uuid.UUID]map[Permission]bool{}}
}

// Add grants permissions at workspace scope (folder nil) or folder scope.
func (g *Grants) Add(perms []Permission, folder *uuid.UUID) {
	for _, p := range perms {
		if p == All {
			if folder == nil {
				g.all = true
			} else {
				// '*' at folder scope is never produced by system data; treat as full folder access
				for _, c := range Catalogue {
					g.addOne(c, folder)
				}
			}
			continue
		}
		g.addOne(p, folder)
	}
}

func (g *Grants) addOne(p Permission, folder *uuid.UUID) {
	if folder == nil {
		g.workspace[p] = true
		return
	}
	m, ok := g.folders[*folder]
	if !ok {
		m = map[Permission]bool{}
		g.folders[*folder] = m
	}
	m[p] = true
}

// IsOwner reports '*' at workspace scope.
func (g *Grants) IsOwner() bool { return g != nil && g.all }

// HasWorkspace reports p granted workspace-wide.
func (g *Grants) HasWorkspace(p Permission) bool {
	if g == nil {
		return false
	}
	return g.all || g.workspace[p]
}

// HasAnywhere reports p granted workspace-wide or on at least one folder.
func (g *Grants) HasAnywhere(p Permission) bool {
	if g.HasWorkspace(p) {
		return true
	}
	for _, m := range g.folders {
		if m[p] {
			return true
		}
	}
	return false
}

// HasInFolderChain reports p granted workspace-wide or on any folder in the chain
// (the resource's folder followed by its ancestors).
func (g *Grants) HasInFolderChain(p Permission, chain []uuid.UUID) bool {
	if g.HasWorkspace(p) {
		return true
	}
	for _, f := range chain {
		if g.folders[f][p] {
			return true
		}
	}
	return false
}

// FolderGrantsFor returns folders on which p is granted (not workspace-wide).
func (g *Grants) FolderGrantsFor(p Permission) []uuid.UUID {
	var out []uuid.UUID
	for f, m := range g.folders {
		if m[p] {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// HasFolderGrants reports whether any folder-scoped grants exist.
func (g *Grants) HasFolderGrants() bool { return g != nil && len(g.folders) > 0 }

// List returns workspace-wide permissions (sorted); owners get the whole catalogue plus '*'.
func (g *Grants) List() []Permission {
	if g == nil {
		return nil
	}
	set := map[Permission]bool{}
	if g.all {
		set[All] = true
		for _, c := range Catalogue {
			set[c] = true
		}
		set[BillingManage] = true
		set[WorkspaceDelete] = true
	}
	for p := range g.workspace {
		set[p] = true
	}
	out := make([]Permission, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Can is a convenience for permissions that do not depend on a folder.
func (g *Grants) Can(p Permission) bool { return g.HasWorkspace(p) }

type ctxKey struct{}

func WithGrants(ctx context.Context, g *Grants) context.Context {
	return context.WithValue(ctx, ctxKey{}, g)
}

func FromContext(ctx context.Context) *Grants {
	g, _ := ctx.Value(ctxKey{}).(*Grants)
	return g
}

// Require enforces a workspace-level permission. For folder-scoped users it allows the
// request through when they hold the permission on at least one folder; handlers that act
// on a specific resource must then check HasInFolderChain for that resource.
func Require(p Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			g := FromContext(r.Context())
			if g == nil || !g.HasAnywhere(p) {
				apierr.Render(w, apierr.New(http.StatusForbidden, "forbidden", "Forbidden",
					"you do not have the "+string(p)+" permission in this workspace"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireWorkspaceWide enforces a permission that has no folder meaning (settings, keys, audit).
func RequireWorkspaceWide(p Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !FromContext(r.Context()).HasWorkspace(p) {
				apierr.Render(w, apierr.New(http.StatusForbidden, "forbidden", "Forbidden",
					"you do not have the "+string(p)+" permission in this workspace"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
