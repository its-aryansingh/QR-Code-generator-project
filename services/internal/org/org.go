// Package org is the organisation layer: the enterprise tenant above workspaces that owns
// the plan, identity policy, audit chain and billing.
package org

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

// DBTX is satisfied by a pool, a connection or a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	ErrNotFound  = errors.New("organization not found")
	ErrNotMember = errors.New("not a member of this organization")
	ErrLastOwner = errors.New("an organization must keep its owner")
)

// Roles an org member can hold.
const (
	RoleOwner        = "org_owner"
	RoleAdmin        = "org_admin"
	RoleBillingAdmin = "billing_admin"
	RoleMember       = "member"
)

func ValidRole(r string) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleBillingAdmin, RoleMember:
		return true
	}
	return false
}

type Org struct {
	ID             uuid.UUID       `json:"id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Kind           string          `json:"kind"`
	ParentOrgID    *uuid.UUID      `json:"parent_org_id"`
	PlanID         string          `json:"plan_id"`
	DataRegion     string          `json:"data_region"`
	LegalName      *string         `json:"legal_name"`
	BillingEmail   *string         `json:"billing_email"`
	GSTIN          *string         `json:"gstin"`
	TaxCountry     string          `json:"tax_country"`
	BillingAddress json.RawMessage `json:"billing_address"`
	Settings       json.RawMessage `json:"settings"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

const orgCols = `id, name, slug::text, kind, parent_org_id, plan_id, data_region, legal_name, billing_email::text, gstin,
	tax_country, billing_address, settings, created_at, updated_at`

func scanOrg(row pgx.Row) (Org, error) {
	var o Org
	err := row.Scan(&o.ID, &o.Name, &o.Slug, &o.Kind, &o.ParentOrgID, &o.PlanID, &o.DataRegion, &o.LegalName,
		&o.BillingEmail, &o.GSTIN, &o.TaxCountry, &o.BillingAddress, &o.Settings, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

var slugClean = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a name into a slug candidate.
func Slugify(name string) string {
	s := strings.Trim(slugClean.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if len(s) < 3 {
		s = "org-" + s
	}
	return s
}

// CreateParams describes a new organisation.
type CreateParams struct {
	Name        string
	Slug        string // optional; derived from Name
	Kind        string // personal | standard | enterprise | agency
	PlanID      string
	DataRegion  string
	OwnerID     uuid.UUID
	ParentOrgID *uuid.UUID
	Source      string // creator | invite | sso_jit | scim
}

// Create inserts an organisation with a unique slug, its owner membership and a default
// security policy.
func Create(ctx context.Context, db DBTX, p CreateParams) (Org, error) {
	if p.Kind == "" {
		p.Kind = "standard"
	}
	if p.PlanID == "" {
		p.PlanID = "free"
	}
	if p.DataRegion == "" {
		p.DataRegion = "in"
	}
	if p.Source == "" {
		p.Source = "creator"
	}
	base := p.Slug
	if base == "" {
		base = Slugify(p.Name)
	}
	slug := base
	for i := 0; i < 8; i++ {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM organizations WHERE slug = $1)`, slug).Scan(&exists); err != nil {
			return Org{}, err
		}
		if !exists {
			break
		}
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		slug = base + "-" + hex.EncodeToString(b)
	}
	o, err := scanOrg(db.QueryRow(ctx, `INSERT INTO organizations (id, name, slug, kind, parent_org_id, plan_id, data_region)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+orgCols,
		idgen.New(), strings.TrimSpace(p.Name), slug, p.Kind, p.ParentOrgID, p.PlanID, p.DataRegion))
	if err != nil {
		return o, err
	}
	if _, err := db.Exec(ctx, `INSERT INTO org_members (org_id, user_id, org_role, source) VALUES ($1, $2, 'org_owner', $3)`,
		o.ID, p.OwnerID, p.Source); err != nil {
		return o, err
	}
	if _, err := db.Exec(ctx, `INSERT INTO org_security_policies (org_id) VALUES ($1) ON CONFLICT DO NOTHING`, o.ID); err != nil {
		return o, err
	}
	return o, nil
}

func Get(ctx context.Context, db DBTX, id uuid.UUID) (Org, error) {
	return scanOrg(db.QueryRow(ctx, `SELECT `+orgCols+` FROM organizations WHERE id = $1 AND deleted_at IS NULL`, id))
}

func GetBySlug(ctx context.Context, db DBTX, slug string) (Org, error) {
	return scanOrg(db.QueryRow(ctx, `SELECT `+orgCols+` FROM organizations WHERE slug = $1 AND deleted_at IS NULL`, slug))
}

// Membership is a user's standing in an organisation.
type Membership struct {
	OrgID     uuid.UUID `json:"org_id"`
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"org_role"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

func GetMember(ctx context.Context, db DBTX, orgID, userID uuid.UUID) (Membership, error) {
	var m Membership
	err := db.QueryRow(ctx, `SELECT org_id, user_id, org_role, status, source, created_at FROM org_members
		WHERE org_id = $1 AND user_id = $2`, orgID, userID).Scan(&m.OrgID, &m.UserID, &m.Role, &m.Status, &m.Source, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, ErrNotMember
	}
	return m, err
}

// EnsureMember adds the user to the org (role member) if absent; an existing membership is
// left untouched (never downgraded, never silently reactivated).
func EnsureMember(ctx context.Context, db DBTX, orgID, userID uuid.UUID, source string) error {
	_, err := db.Exec(ctx, `INSERT INTO org_members (org_id, user_id, org_role, source) VALUES ($1, $2, 'member', $3)
		ON CONFLICT (org_id, user_id) DO NOTHING`, orgID, userID, source)
	return err
}

// OrgWithRole is an organisation as seen by one of its members.
type OrgWithRole struct {
	Org
	Role           string `json:"org_role"`
	Status         string `json:"status"`
	WorkspaceCount int    `json:"workspace_count"`
}

func ListForUser(ctx context.Context, db DBTX, userID uuid.UUID) ([]OrgWithRole, error) {
	rows, err := db.Query(ctx, `SELECT o.id, o.name, o.slug::text, o.kind, o.parent_org_id, o.plan_id, o.data_region,
		o.legal_name, o.billing_email::text, o.gstin, o.tax_country, o.billing_address, o.settings, o.created_at, o.updated_at,
		m.org_role, m.status, (SELECT count(*) FROM workspaces w WHERE w.org_id = o.id AND w.deleted_at IS NULL)::int
		FROM organizations o JOIN org_members m ON m.org_id = o.id
		WHERE m.user_id = $1 AND o.deleted_at IS NULL ORDER BY o.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrgWithRole{}
	for rows.Next() {
		var o OrgWithRole
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Kind, &o.ParentOrgID, &o.PlanID, &o.DataRegion, &o.LegalName,
			&o.BillingEmail, &o.GSTIN, &o.TaxCountry, &o.BillingAddress, &o.Settings, &o.CreatedAt, &o.UpdatedAt,
			&o.Role, &o.Status, &o.WorkspaceCount); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// MemberRow is an org member with profile fields for listings.
type MemberRow struct {
	Membership
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	WorkspaceCount int        `json:"workspace_count"`
	MFAEnabled     bool       `json:"mfa_enabled"`
}

func ListMembers(ctx context.Context, db DBTX, orgID uuid.UUID) ([]MemberRow, error) {
	rows, err := db.Query(ctx, `SELECT m.org_id, m.user_id, m.org_role, m.status, m.source, m.created_at, u.email::text, u.name,
		u.last_login_at,
		(SELECT count(*) FROM workspace_members wm JOIN workspaces w ON w.id = wm.workspace_id
		  WHERE wm.user_id = m.user_id AND w.org_id = m.org_id AND w.deleted_at IS NULL)::int,
		EXISTS (SELECT 1 FROM user_mfa_factors f WHERE f.user_id = m.user_id)
		FROM org_members m JOIN users u ON u.id = m.user_id
		WHERE m.org_id = $1 ORDER BY m.created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MemberRow{}
	for rows.Next() {
		var r MemberRow
		if err := rows.Scan(&r.OrgID, &r.UserID, &r.Role, &r.Status, &r.Source, &r.CreatedAt, &r.Email, &r.Name,
			&r.LastLoginAt, &r.WorkspaceCount, &r.MFAEnabled); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetRole changes an org role. Ownership moves only through TransferOwnership.
func SetRole(ctx context.Context, db DBTX, orgID, userID uuid.UUID, role string) error {
	if role == RoleOwner {
		return ErrLastOwner
	}
	tag, err := db.Exec(ctx, `UPDATE org_members SET org_role = $3, updated_at = now()
		WHERE org_id = $1 AND user_id = $2 AND org_role <> 'org_owner'`, orgID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLastOwner
	}
	return nil
}

// SetStatus suspends, reactivates or deprovisions a member (never the owner).
func SetStatus(ctx context.Context, db DBTX, orgID, userID uuid.UUID, status string) error {
	tag, err := db.Exec(ctx, `UPDATE org_members SET status = $3, updated_at = now()
		WHERE org_id = $1 AND user_id = $2 AND (org_role <> 'org_owner' OR $3 = 'active')`, orgID, userID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLastOwner
	}
	return nil
}

// Remove deletes a member and every grant they hold inside the organisation.
func Remove(ctx context.Context, db DBTX, orgID, userID uuid.UUID) error {
	var role string
	if err := db.QueryRow(ctx, `SELECT org_role FROM org_members WHERE org_id = $1 AND user_id = $2`, orgID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		return err
	}
	if role == RoleOwner {
		return ErrLastOwner
	}
	for _, q := range []string{
		`DELETE FROM role_bindings WHERE org_id = $1 AND principal_type = 'user' AND principal_id = $2`,
		`DELETE FROM workspace_members wm USING workspaces w WHERE w.id = wm.workspace_id AND w.org_id = $1 AND wm.user_id = $2`,
		`DELETE FROM group_members gm USING groups g WHERE g.id = gm.group_id AND g.org_id = $1 AND gm.user_id = $2`,
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`,
	} {
		if _, err := db.Exec(ctx, q, orgID, userID); err != nil {
			return err
		}
	}
	return nil
}

// TransferOwnership makes another active member the owner; the previous owner becomes admin.
func TransferOwnership(ctx context.Context, db DBTX, orgID, from, to uuid.UUID) error {
	m, err := GetMember(ctx, db, orgID, to)
	if err != nil {
		return err
	}
	if m.Status != "active" {
		return ErrNotMember
	}
	if _, err := db.Exec(ctx, `UPDATE org_members SET org_role = 'org_admin', updated_at = now() WHERE org_id = $1 AND user_id = $2`, orgID, from); err != nil {
		return err
	}
	_, err = db.Exec(ctx, `UPDATE org_members SET org_role = 'org_owner', updated_at = now() WHERE org_id = $1 AND user_id = $2`, orgID, to)
	return err
}

// SystemRoleID returns the id of a built-in role (owner, admin, editor, reviewer, analyst).
func SystemRoleID(ctx context.Context, db DBTX, key string) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRow(ctx, `SELECT id FROM roles WHERE org_id IS NULL AND key = $1`, key).Scan(&id)
	return id, err
}

// BindMemberRole mirrors a workspace membership role into role_bindings: the member's
// workspace-wide system-role binding is replaced by one for role. Folder-scoped and custom
// bindings are left alone.
func BindMemberRole(ctx context.Context, db DBTX, orgID, workspaceID, userID uuid.UUID, role string, by *uuid.UUID) error {
	if _, err := db.Exec(ctx, `DELETE FROM role_bindings b USING roles r
		WHERE b.role_id = r.id AND r.org_id IS NULL AND b.workspace_id = $1 AND b.principal_type = 'user'
		  AND b.principal_id = $2 AND b.folder_id IS NULL`, workspaceID, userID); err != nil {
		return err
	}
	if role == "" || role == "custom" {
		return nil
	}
	rid, err := SystemRoleID(ctx, db, role)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO role_bindings (id, org_id, workspace_id, principal_type, principal_id, role_id, created_by)
		VALUES ($1, $2, $3, 'user', $4, $5, $6) ON CONFLICT DO NOTHING`, idgen.New(), orgID, workspaceID, userID, rid, by)
	return err
}

// UnbindMember removes every binding a user holds directly in a workspace.
func UnbindMember(ctx context.Context, db DBTX, workspaceID, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `DELETE FROM role_bindings WHERE workspace_id = $1 AND principal_type = 'user' AND principal_id = $2`,
		workspaceID, userID)
	return err
}
