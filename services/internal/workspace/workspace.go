package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/rbac"
)

const (
	InviteDuration = 7 * 24 * time.Hour
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

	ErrInvalidSlug       = errors.New("slug must be 3-48 characters, lowercase alphanumeric with optional single hyphens")
	ErrReservedSlug      = errors.New("this workspace slug is reserved")
	ErrInvalidRole       = errors.New("invalid role for member invite")
	ErrCannotInviteOwner = errors.New("cannot invite member as owner")
)

var reservedSlugs = map[string]struct{}{
	"admin": {}, "api": {}, "app": {}, "auth": {}, "billing": {},
	"dash": {}, "dashboard": {}, "docs": {}, "help": {}, "login": {},
	"register": {}, "root": {}, "settings": {}, "static": {}, "status": {},
	"system": {}, "terms": {}, "privacy": {}, "webhooks": {}, "support": {},
}

// Workspace represents tenant metadata.
type Workspace struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	PlanID    string     `json:"plan_id"`
	Timezone  string     `json:"timezone"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type workspaceContextKey struct{}

// WithWorkspace attaches the active Workspace to the context.
func WithWorkspace(ctx context.Context, ws *Workspace) context.Context {
	return context.WithValue(ctx, workspaceContextKey{}, ws)
}

// GetWorkspace retrieves the active Workspace from the context.
func GetWorkspace(ctx context.Context) (*Workspace, bool) {
	ws, ok := ctx.Value(workspaceContextKey{}).(*Workspace)
	return ws, ok
}

// ValidateSlug validates that a proposed slug is URL-safe and unreserved.
func ValidateSlug(slug string) error {
	s := strings.ToLower(strings.TrimSpace(slug))
	if len(s) < 3 || len(s) > 48 {
		return ErrInvalidSlug
	}
	if !slugRegex.MatchString(s) {
		return ErrInvalidSlug
	}
	if _, reserved := reservedSlugs[s]; reserved {
		return ErrReservedSlug
	}
	return nil
}

// Slugify converts an arbitrary name into a clean candidate slug.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	// Replace non-alphanumeric with hyphen
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	s = reg.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 48 {
		s = s[:48]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// GenerateInviteToken creates an invite secret token and its SHA-256 hash.
func GenerateInviteToken() (plain string, hash []byte, err error) {
	b, err := crypto.RandomBytes(32)
	if err != nil {
		return "", nil, err
	}
	plain = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(plain))
	return plain, h[:], nil
}

// ValidateInviteRole ensures the invited role is valid (admin, editor, analyst).
func ValidateInviteRole(role rbac.Role) error {
	switch role {
	case rbac.RoleAdmin, rbac.RoleEditor, rbac.RoleAnalyst:
		return nil
	case rbac.RoleOwner:
		return ErrCannotInviteOwner
	default:
		return ErrInvalidRole
	}
}
