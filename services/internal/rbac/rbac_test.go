package rbac_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/its-aryansingh/qrit/services/internal/rbac"
)

func TestHasPermission(t *testing.T) {
	matrix := []struct {
		role rbac.Role
		perm rbac.Permission
		want bool
	}{
		// Owner has all permissions
		{rbac.RoleOwner, rbac.PermView, true},
		{rbac.RoleOwner, rbac.PermCreateEdit, true},
		{rbac.RoleOwner, rbac.PermManage, true},
		{rbac.RoleOwner, rbac.PermManageMembers, true},
		{rbac.RoleOwner, rbac.PermBillingAndWorkspace, true},

		// Admin has all except billing and ownership
		{rbac.RoleAdmin, rbac.PermView, true},
		{rbac.RoleAdmin, rbac.PermCreateEdit, true},
		{rbac.RoleAdmin, rbac.PermManage, true},
		{rbac.RoleAdmin, rbac.PermManageMembers, true},
		{rbac.RoleAdmin, rbac.PermBillingAndWorkspace, false},

		// Editor has view and create_edit
		{rbac.RoleEditor, rbac.PermView, true},
		{rbac.RoleEditor, rbac.PermCreateEdit, true},
		{rbac.RoleEditor, rbac.PermManage, false},
		{rbac.RoleEditor, rbac.PermManageMembers, false},
		{rbac.RoleEditor, rbac.PermBillingAndWorkspace, false},

		// Analyst has view only
		{rbac.RoleAnalyst, rbac.PermView, true},
		{rbac.RoleAnalyst, rbac.PermCreateEdit, false},
		{rbac.RoleAnalyst, rbac.PermManage, false},
		{rbac.RoleAnalyst, rbac.PermManageMembers, false},
		{rbac.RoleAnalyst, rbac.PermBillingAndWorkspace, false},
	}

	for _, tt := range matrix {
		got := rbac.HasPermission(tt.role, tt.perm)
		if got != tt.want {
			t.Errorf("HasPermission(%s, %s) = %v, want %v", tt.role, tt.perm, got, tt.want)
		}
	}
}

func TestRequireMiddleware(t *testing.T) {
	handler := rbac.Require(rbac.PermCreateEdit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Case 1: allowed role (Editor)
	req := httptest.NewRequest("POST", "/test", nil)
	ctx := rbac.WithRole(req.Context(), rbac.RoleEditor)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for editor on PermCreateEdit, got %d", w.Code)
	}

	// Case 2: forbidden role (Analyst)
	req2 := httptest.NewRequest("POST", "/test", nil)
	ctx2 := rbac.WithRole(req2.Context(), rbac.RoleAnalyst)
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Errorf("expected 403 for analyst on PermCreateEdit, got %d", w2.Code)
	}

	// Case 3: no role in context
	req3 := httptest.NewRequest("POST", "/test", nil)
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Errorf("expected 403 when no role in context, got %d", w3.Code)
	}
}
