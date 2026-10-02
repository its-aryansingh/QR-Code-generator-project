package authz

import (
	"testing"

	"github.com/google/uuid"
)

func TestOwnerHasEverything(t *testing.T) {
	g := NewGrants()
	g.Add(SystemRoles["owner"], nil)
	for _, p := range Catalogue {
		if !g.HasWorkspace(p) {
			t.Fatalf("owner missing %s", p)
		}
	}
	if !g.IsOwner() {
		t.Fatal("owner flag")
	}
}

func TestSystemRolesMatrix(t *testing.T) {
	cases := []struct {
		role string
		p    Permission
		want bool
	}{
		{"admin", DomainManage, true}, {"admin", BillingManage, false},
		{"editor", QRCreate, true}, {"editor", QRDelete, false}, {"editor", QRDestinationApprove, false},
		{"reviewer", QRDestinationApprove, true}, {"reviewer", QRCreate, false},
		{"analyst", AnalyticsRead, true}, {"analyst", QRUpdate, false},
	}
	for _, c := range cases {
		g := NewGrants()
		g.Add(SystemRoles[c.role], nil)
		if got := g.HasWorkspace(c.p); got != c.want {
			t.Errorf("%s/%s: got %v want %v", c.role, c.p, got, c.want)
		}
	}
}

func TestFolderScopedGrants(t *testing.T) {
	parent, child, other := uuid.New(), uuid.New(), uuid.New()
	g := NewGrants()
	g.Add(SystemRoles["analyst"], nil)
	g.Add(SystemRoles["editor"], &parent)

	if g.HasWorkspace(QRCreate) {
		t.Fatal("folder grant leaked to workspace scope")
	}
	if !g.HasAnywhere(QRCreate) {
		t.Fatal("HasAnywhere should see folder grant")
	}
	if !g.HasInFolderChain(QRCreate, []uuid.UUID{child, parent}) {
		t.Fatal("child of granted folder should inherit")
	}
	if g.HasInFolderChain(QRCreate, []uuid.UUID{other}) {
		t.Fatal("unrelated folder must not be granted")
	}
	if got := g.FolderGrantsFor(QRCreate); len(got) != 1 || got[0] != parent {
		t.Fatalf("folder grants: %v", got)
	}
}

func TestSystemRolesOnlyUseCatalogue(t *testing.T) {
	for role, perms := range SystemRoles {
		for _, p := range perms {
			if p != All && !IsAssignable(p) {
				t.Errorf("role %s uses non-catalogue permission %s", role, p)
			}
		}
	}
}
