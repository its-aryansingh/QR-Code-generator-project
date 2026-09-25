package workspace_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/rbac"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		slug    string
		wantErr bool
	}{
		{"my-team", false},
		{"team123", false},
		{"qrit-design", false},
		{"ab", true},                     // too short (< 3)
		{"this-slug-is-way-too-long-and-exceeds-the-maximum-allowed-length-for-workspace-slugs", true},
		{"-leading-hyphen", true},
		{"trailing-hyphen-", true},
		{"double--hyphen", true},
		{"admin", true},                  // reserved
		{"api", true},                    // reserved
		{"settings", true},               // reserved
	}

	for _, tt := range tests {
		err := workspace.ValidateSlug(tt.slug)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateSlug(%q) err = %v; wantErr %v", tt.slug, err, tt.wantErr)
		}
	}
}

func TestSlugify(t *testing.T) {
	got := workspace.Slugify("My Awesome Team!")
	want := "my-awesome-team"
	if got != want {
		t.Errorf("Slugify() = %q; want %q", got, want)
	}
}

func TestInviteRole(t *testing.T) {
	if err := workspace.ValidateInviteRole(rbac.RoleAdmin); err != nil {
		t.Errorf("expected admin to be valid invite role")
	}
	if err := workspace.ValidateInviteRole(rbac.RoleEditor); err != nil {
		t.Errorf("expected editor to be valid invite role")
	}
	if err := workspace.ValidateInviteRole(rbac.RoleAnalyst); err != nil {
		t.Errorf("expected analyst to be valid invite role")
	}
	if err := workspace.ValidateInviteRole(rbac.RoleOwner); err != workspace.ErrCannotInviteOwner {
		t.Errorf("expected ErrCannotInviteOwner for owner invite")
	}
}

func TestContext(t *testing.T) {
	ws := &workspace.Workspace{
		ID:   uuid.New(),
		Name: "Acme Corp",
		Slug: "acme-corp",
	}

	ctx := workspace.WithWorkspace(context.Background(), ws)
	retrieved, ok := workspace.GetWorkspace(ctx)
	if !ok || retrieved.ID != ws.ID {
		t.Errorf("failed to retrieve workspace from context")
	}
}
