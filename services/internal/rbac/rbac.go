package rbac

import (
	"context"
	"net/http"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
)

type Role string

const (
	RoleOwner   Role = "owner"
	RoleAdmin   Role = "admin"
	RoleEditor  Role = "editor"
	RoleAnalyst Role = "analyst"
)

type Permission string

const (
	// PermView allows viewing QR codes, analytics, and campaigns (owner, admin, editor, analyst)
	PermView Permission = "view"
	// PermCreateEdit allows creating and editing QR codes, versions, rules, campaigns, templates (owner, admin, editor)
	PermCreateEdit Permission = "create_edit"
	// PermManage allows deleting/restoring QR codes, managing domains, API keys, webhooks, audit logs (owner, admin)
	PermManage Permission = "manage"
	// PermManageMembers allows inviting, removing members and modifying member roles (owner, admin)
	PermManageMembers Permission = "manage_members"
	// PermBillingAndWorkspace allows managing billing, workspace deletion, and ownership transfer (owner only)
	PermBillingAndWorkspace Permission = "billing_and_workspace"
)

type roleContextKey struct{}

// WithRole attaches the workspace role to the context.
func WithRole(ctx context.Context, role Role) context.Context {
	return context.WithValue(ctx, roleContextKey{}, role)
}

// GetRole retrieves the workspace role from the context.
func GetRole(ctx context.Context) (Role, bool) {
	role, ok := ctx.Value(roleContextKey{}).(Role)
	return role, ok
}

// HasPermission checks if a role has the required permission according to the RBAC matrix.
func HasPermission(role Role, perm Permission) bool {
	switch perm {
	case PermView:
		return role == RoleOwner || role == RoleAdmin || role == RoleEditor || role == RoleAnalyst
	case PermCreateEdit:
		return role == RoleOwner || role == RoleAdmin || role == RoleEditor
	case PermManage:
		return role == RoleOwner || role == RoleAdmin
	case PermManageMembers:
		return role == RoleOwner || role == RoleAdmin
	case PermBillingAndWorkspace:
		return role == RoleOwner
	default:
		return false
	}
}

// Require returns a Chi-compatible HTTP middleware enforcing that the caller has the required permission.
func Require(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := GetRole(r.Context())
			if !ok || !HasPermission(role, perm) {
				apierr.Render(w, apierr.Forbidden("insufficient workspace permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
