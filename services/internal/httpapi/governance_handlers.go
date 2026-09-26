package httpapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
)

// Access governance: custom roles, groups, role bindings (workspace or folder scope,
// users or groups), the access inspector and the access review export.

func (s *Server) governanceOrgRoutes(r chi.Router) {
	r.Get("/roles", s.handleListRoles)
	r.Get("/roles/{id}", s.handleGetRole)
	r.With(requireOrg(authz.OrgManage)).Post("/roles", s.handleCreateRole)
	r.With(requireOrg(authz.OrgManage)).Patch("/roles/{id}", s.handleUpdateRole)
	r.With(requireOrg(authz.OrgManage)).Delete("/roles/{id}", s.handleDeleteRole)
	r.With(requireOrg(authz.OrgMembers)).Get("/groups", s.handleListGroups)
	r.With(requireOrg(authz.OrgMembers)).Get("/groups/{id}", s.handleGetGroup)
	r.With(requireOrg(authz.OrgMembers)).Post("/groups", s.handleCreateGroup)
	r.With(requireOrg(authz.OrgMembers)).Patch("/groups/{id}", s.handleUpdateGroup)
	r.With(requireOrg(authz.OrgMembers)).Delete("/groups/{id}", s.handleDeleteGroup)
	r.With(requireOrg(authz.OrgMembers)).Post("/groups/{id}/members", s.handleAddGroupMembers)
	r.With(requireOrg(authz.OrgMembers)).Delete("/groups/{id}/members/{userId}", s.handleRemoveGroupMember)
	r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Get("/access-review", s.handleAccessReview)
	r.With(requireOrg(authz.OrgAudit)).Post("/access-review/complete", s.handleCompleteAccessReview)
}

func (s *Server) governanceWSRoutes(r chi.Router) {
	r.With(authz.RequireWorkspaceWide(authz.WorkspaceRead)).Get("/roles", s.handleWorkspaceRoles)
	r.With(authz.Require(authz.RoleManage)).Get("/role-bindings", s.handleListBindings)
	r.With(authz.Require(authz.RoleManage)).Post("/role-bindings", s.handleCreateBinding)
	r.With(authz.Require(authz.RoleManage)).Delete("/role-bindings/{id}", s.handleDeleteBinding)
	r.With(authz.Require(authz.RoleManage)).Get("/access/explain", s.handleExplainAccess)
}

// ---- roles -----------------------------------------------------------------------------

type roleDTO struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       *uuid.UUID `json:"org_id"`
	Key         string     `json:"key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Permissions []string   `json:"permissions"`
	System      bool       `json:"system"`
	Bindings    int        `json:"bindings"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const roleCols = `r.id, r.org_id, r.key, r.name, r.description, r.permissions, r.created_at, r.updated_at,
	(SELECT count(*) FROM role_bindings b WHERE b.role_id = r.id AND ($1::uuid IS NULL OR b.org_id = $1))::int`

func scanRole(row pgx.Row) (roleDTO, error) {
	var d roleDTO
	err := row.Scan(&d.ID, &d.OrgID, &d.Key, &d.Name, &d.Description, &d.Permissions, &d.CreatedAt, &d.UpdatedAt, &d.Bindings)
	d.System = d.OrgID == nil
	return d, err
}

func (s *Server) listRoles(ctx context.Context, orgID uuid.UUID) ([]roleDTO, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+roleCols+` FROM roles r WHERE r.org_id IS NULL OR r.org_id = $1
		ORDER BY r.org_id NULLS FIRST, CASE r.key WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'editor' THEN 2 WHEN 'reviewer' THEN 3
		WHEN 'analyst' THEN 4 ELSE 5 END, r.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []roleDTO{}
	for rows.Next() {
		d, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	list, err := s.listRoles(r.Context(), orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list roles"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (s *Server) handleWorkspaceRoles(w http.ResponseWriter, r *http.Request) {
	list, err := s.listRoles(r.Context(), workspaceRow(r).OrgID)
	if err != nil {
		fail(w, apierr.Internal("failed to list roles"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": list})
}

func (s *Server) loadRole(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (roleDTO, error) {
	d, err := scanRole(s.pool.QueryRow(ctx, `SELECT `+roleCols+` FROM roles r WHERE r.id = $2 AND (r.org_id IS NULL OR r.org_id = $1)`, orgID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, apierr.NotFound("role not found")
	}
	return d, err
}

func (s *Server) handleGetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("role not found"))
		return
	}
	d, err := s.loadRole(r.Context(), orgRow(r).ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load role"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)

type roleReq struct {
	Key         string    `json:"key"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Permissions *[]string `json:"permissions"`
}

// validPermissions checks a custom role's permission set: catalogue only, never '*'.
func validPermissions(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == string(authz.All) {
			return nil, unprocessable("wildcard_not_allowed", "custom roles can't contain '*'")
		}
		if !authz.IsAssignable(authz.Permission(p)) {
			return nil, unprocessable("unknown_permission", "unknown permission "+p+"; see GET /v1/permissions")
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, unprocessable("permissions_required", "a role needs at least one permission")
	}
	if !seen[string(authz.WorkspaceRead)] {
		out = append(out, string(authz.WorkspaceRead)) // every role can open the workspace it's bound in
	}
	sort.Strings(out)
	return out, nil
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if err := s.requireFeatureOrg(r, entitlements.FeatureRoles); err != nil {
		fail(w, err)
		return
	}
	var req roleReq
	if !decode(w, r, &req) {
		return
	}
	key := strings.ToLower(strings.TrimSpace(req.Key))
	if !roleKeyPattern.MatchString(key) {
		fail(w, unprocessable("invalid_key", "key must be 2–41 lowercase letters, digits or underscores, starting with a letter"))
		return
	}
	if _, reserved := authz.SystemRoles[key]; reserved {
		fail(w, apierr.Conflict("reserved_key", key+" is a built-in role"))
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 80 {
		fail(w, unprocessable("invalid_name", "name is required (at most 80 characters)"))
		return
	}
	if req.Permissions == nil {
		fail(w, unprocessable("permissions_required", "a role needs at least one permission"))
		return
	}
	perms, err := validPermissions(*req.Permissions)
	if err != nil {
		fail(w, err)
		return
	}
	desc := ""
	if req.Description != nil {
		desc = strings.TrimSpace(*req.Description)
	}
	var d roleDTO
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		id := idgen.New()
		var err error
		d, err = scanRole(tx.QueryRow(r.Context(), `WITH ins AS (INSERT INTO roles (id, org_id, key, name, description, permissions)
			VALUES ($2, $1, $3, $4, $5, $6) RETURNING *) SELECT `+strings.ReplaceAll(roleCols, "r.", "ins.")+` FROM ins`,
			o.ID, id, key, strings.TrimSpace(*req.Name), desc, perms))
		if err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return apierr.Conflict("role_exists", "a role with this key already exists")
			}
			return err
		}
		return s.auditOrg(r, q, o.ID, "role.created", "role", &id, map[string]any{"key": key, "permissions": perms})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create role"))
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("role not found"))
		return
	}
	var req roleReq
	if !decode(w, r, &req) {
		return
	}
	cur, err := s.loadRole(r.Context(), o.ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load role"))
		return
	}
	if cur.System {
		fail(w, forbidden("system_role", "built-in roles can't be changed; create a custom role instead"))
		return
	}
	name, desc, perms := cur.Name, cur.Description, cur.Permissions
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 80 {
			fail(w, unprocessable("invalid_name", "name is required (at most 80 characters)"))
			return
		}
		name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		desc = strings.TrimSpace(*req.Description)
	}
	if req.Permissions != nil {
		if perms, err = validPermissions(*req.Permissions); err != nil {
			fail(w, err)
			return
		}
	}
	var d roleDTO
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE roles SET name = $3, description = $4, permissions = $5, updated_at = now()
			WHERE id = $2 AND org_id = $1`, o.ID, id, name, desc, perms); err != nil {
			return err
		}
		var err error
		if d, err = scanRole(tx.QueryRow(r.Context(), `SELECT `+roleCols+` FROM roles r WHERE r.id = $2`, o.ID, id)); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "role.updated", "role", &id, map[string]any{
			"before": map[string]any{"name": cur.Name, "permissions": cur.Permissions},
			"after":  map[string]any{"name": name, "permissions": perms}})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update role"))
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("role not found"))
		return
	}
	cur, err := s.loadRole(r.Context(), o.ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load role"))
		return
	}
	if cur.System {
		fail(w, forbidden("system_role", "built-in roles can't be deleted"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM roles WHERE id = $1 AND org_id = $2`, id, o.ID); err != nil {
			if pgCode(err) == sqlFKViolation {
				return apierr.Conflict("role_in_use", "remove this role's bindings first ("+itoa(cur.Bindings)+" in use)")
			}
			return err
		}
		return s.auditOrg(r, q, o.ID, "role.deleted", "role", &id, map[string]any{"key": cur.Key})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete role"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- groups ----------------------------------------------------------------------------

type groupMemberDTO struct {
	UserID  uuid.UUID `json:"user_id"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	AddedAt time.Time `json:"added_at"`
}

type groupDTO struct {
	ID          uuid.UUID        `json:"id"`
	DisplayName string           `json:"display_name"`
	Source      string           `json:"source"` // manual | scim | sso
	ReadOnly    bool             `json:"read_only"`
	Members     int              `json:"member_count"`
	Bindings    int              `json:"binding_count"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	MemberList  *[]groupMemberDTO `json:"members,omitempty"` // detail only
}

const groupCols = `g.id, g.display_name, g.source, g.created_at, g.updated_at,
	(SELECT count(*) FROM group_members gm WHERE gm.group_id = g.id)::int,
	(SELECT count(*) FROM role_bindings b WHERE b.principal_type = 'group' AND b.principal_id = g.id)::int`

func scanGroup(row pgx.Row) (groupDTO, error) {
	var g groupDTO
	err := row.Scan(&g.ID, &g.DisplayName, &g.Source, &g.CreatedAt, &g.UpdatedAt, &g.Members, &g.Bindings)
	g.ReadOnly = g.Source != "manual"
	return g, err
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+groupCols+` FROM groups g WHERE g.org_id = $1 ORDER BY lower(g.display_name)`, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list groups"))
		return
	}
	defer rows.Close()
	out := []groupDTO{}
	for rows.Next() {
		if g, err := scanGroup(rows); err == nil {
			out = append(out, g)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) loadGroup(ctx context.Context, orgID, id uuid.UUID) (groupDTO, error) {
	g, err := scanGroup(s.pool.QueryRow(ctx, `SELECT `+groupCols+` FROM groups g WHERE g.id = $1 AND g.org_id = $2`, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return g, apierr.NotFound("group not found")
	}
	return g, err
}

func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("group not found"))
		return
	}
	g, err := s.loadGroup(r.Context(), orgRow(r).ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load group"))
		return
	}
	rows, err := s.pool.Query(r.Context(), `SELECT u.id, u.email::text, u.name, gm.added_at FROM group_members gm JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1 ORDER BY lower(u.name)`, id)
	if err != nil {
		fail(w, apierr.Internal("failed to load members"))
		return
	}
	defer rows.Close()
	members := []groupMemberDTO{}
	for rows.Next() {
		var m groupMemberDTO
		if rows.Scan(&m.UserID, &m.Email, &m.Name, &m.AddedAt) == nil {
			members = append(members, m)
		}
	}
	g.MemberList = &members
	writeJSON(w, http.StatusOK, g)
}

type groupReq struct {
	DisplayName string      `json:"display_name"`
	UserIDs     []uuid.UUID `json:"user_ids"`
}

// addGroupMembers adds active organisation members; anyone else is refused.
func addGroupMembers(ctx context.Context, tx pgx.Tx, orgID, groupID uuid.UUID, users []uuid.UUID) (int, error) {
	added := 0
	for _, u := range users {
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2 AND status = 'active')`,
			orgID, u).Scan(&ok); err != nil {
			return added, err
		}
		if !ok {
			return added, unprocessable("not_org_member", "user "+u.String()+" isn't an active member of this organisation")
		}
		tag, err := tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, groupID, u)
		if err != nil {
			return added, err
		}
		added += int(tag.RowsAffected())
	}
	return added, nil
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if err := s.requireFeatureOrg(r, entitlements.FeatureRoles); err != nil {
		fail(w, err)
		return
	}
	var req groupReq
	if !decode(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" || len(name) > 120 {
		fail(w, unprocessable("invalid_name", "display_name is required (at most 120 characters)"))
		return
	}
	id := idgen.New()
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO groups (id, org_id, display_name, source) VALUES ($1, $2, $3, 'manual')`, id, o.ID, name); err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return apierr.Conflict("group_exists", "a group with this name already exists")
			}
			return err
		}
		if _, err := addGroupMembers(r.Context(), tx, o.ID, id, req.UserIDs); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "group.created", "group", &id, map[string]any{"display_name": name, "members": len(req.UserIDs)})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create group"))
		return
	}
	g, _ := s.loadGroup(r.Context(), o.ID, id)
	writeJSON(w, http.StatusCreated, g)
}

// manualGroup loads a group and refuses changes to directory- or IdP-managed ones.
func (s *Server) manualGroup(r *http.Request, id uuid.UUID) (groupDTO, error) {
	g, err := s.loadGroup(r.Context(), orgRow(r).ID, id)
	if err != nil {
		return g, err
	}
	if g.ReadOnly {
		src := "your identity provider"
		if g.Source == "scim" {
			src = "your SCIM directory"
		}
		return g, apierr.Conflict("group_managed_externally", "this group is managed by "+src+"; change it there")
	}
	return g, nil
}

func (s *Server) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("group not found"))
		return
	}
	var req groupReq
	if !decode(w, r, &req) {
		return
	}
	cur, err := s.manualGroup(r, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load group"))
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if name == "" || len(name) > 120 {
		fail(w, unprocessable("invalid_name", "display_name is required (at most 120 characters)"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE groups SET display_name = $3, updated_at = now(), version = version + 1
			WHERE id = $1 AND org_id = $2`, id, o.ID, name); err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return apierr.Conflict("group_exists", "a group with this name already exists")
			}
			return err
		}
		return s.auditOrg(r, q, o.ID, "group.renamed", "group", &id, map[string]any{"before": cur.DisplayName, "after": name})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update group"))
		return
	}
	g, _ := s.loadGroup(r.Context(), o.ID, id)
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("group not found"))
		return
	}
	cur, err := s.manualGroup(r, id)
	if err != nil && !(cur.Source == "sso" && cur.ID != uuid.Nil) { // stale SSO groups may be cleaned up
		fail(w, problemOr500(err, "failed to load group"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM role_bindings WHERE principal_type = 'group' AND principal_id = $1`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(r.Context(), `DELETE FROM groups WHERE id = $1 AND org_id = $2`, id, o.ID); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "group.deleted", "group", &id, map[string]any{"display_name": cur.DisplayName, "bindings_removed": cur.Bindings})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete group"))
		return
	}
	s.access.Invalidate(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAddGroupMembers(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("group not found"))
		return
	}
	var req groupReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.UserIDs) == 0 || len(req.UserIDs) > 500 {
		fail(w, unprocessable("invalid_user_ids", "user_ids needs 1–500 users"))
		return
	}
	if _, err := s.manualGroup(r, id); err != nil {
		fail(w, problemOr500(err, "failed to load group"))
		return
	}
	var added int
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		if added, err = addGroupMembers(r.Context(), tx, o.ID, id, req.UserIDs); err != nil {
			return err
		}
		ids := make([]string, len(req.UserIDs))
		for i, u := range req.UserIDs {
			ids[i] = u.String()
		}
		return s.auditOrg(r, q, o.ID, "group.members_added", "group", &id, map[string]any{"user_ids": ids})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to add members"))
		return
	}
	s.access.Invalidate(r.Context())
	g, _ := s.loadGroup(r.Context(), o.ID, id)
	writeJSON(w, http.StatusOK, map[string]any{"group": g, "added": added})
}

func (s *Server) handleRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok1 := uuidParam(r, "id")
	uid, ok2 := uuidParam(r, "userId")
	if !ok1 || !ok2 {
		fail(w, apierr.NotFound("group member not found"))
		return
	}
	if _, err := s.manualGroup(r, id); err != nil {
		fail(w, problemOr500(err, "failed to load group"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, id, uid)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("group member not found")
		}
		return s.auditOrg(r, q, o.ID, "group.member_removed", "group", &id, map[string]any{"user_id": uid})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to remove member"))
		return
	}
	s.access.Invalidate(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// ---- role bindings -------------------------------------------------------------------

type bindingDTO struct {
	ID            uuid.UUID  `json:"id"`
	PrincipalType string     `json:"principal_type"`
	PrincipalID   uuid.UUID  `json:"principal_id"`
	PrincipalName string     `json:"principal_name"`
	PrincipalMail *string    `json:"principal_email,omitempty"`
	RoleID        uuid.UUID  `json:"role_id"`
	RoleKey       string     `json:"role_key"`
	RoleName      string     `json:"role_name"`
	ScopeType     string     `json:"scope_type"`
	FolderID      *uuid.UUID `json:"folder_id"`
	FolderName    *string    `json:"folder_name"`
	ManagedBy     string     `json:"managed_by"` // membership | binding
	CreatedAt     time.Time  `json:"created_at"`
	permissions   []string
	systemRole    bool
}

const bindingCols = `b.id, b.principal_type, b.principal_id,
	COALESCE(u.name, g.display_name, '(deleted)'), u.email::text, r.id, r.key, r.name, r.permissions, r.org_id IS NULL,
	b.scope_type, b.folder_id, f.name, b.created_at`

const bindingFrom = ` FROM role_bindings b JOIN roles r ON r.id = b.role_id
	LEFT JOIN users u ON b.principal_type = 'user' AND u.id = b.principal_id
	LEFT JOIN groups g ON b.principal_type = 'group' AND g.id = b.principal_id
	LEFT JOIN folders f ON f.id = b.folder_id`

func scanBinding(row pgx.Row) (bindingDTO, error) {
	var b bindingDTO
	err := row.Scan(&b.ID, &b.PrincipalType, &b.PrincipalID, &b.PrincipalName, &b.PrincipalMail, &b.RoleID, &b.RoleKey, &b.RoleName,
		&b.permissions, &b.systemRole, &b.ScopeType, &b.FolderID, &b.FolderName, &b.CreatedAt)
	b.ManagedBy = "binding"
	if b.PrincipalType == "user" && b.systemRole && b.FolderID == nil {
		b.ManagedBy = "membership" // change it with PATCH /members/{userId}
	}
	return b, err
}

func (s *Server) handleListBindings(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	args := []any{ws.ID}
	where := ` WHERE b.workspace_id = $1`
	if v := r.URL.Query().Get("principal_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fail(w, apierr.BadRequest("invalid_principal_id", "principal_id must be a UUID"))
			return
		}
		args = append(args, id)
		where += ` AND b.principal_id = $2`
	}
	rows, err := s.pool.Query(r.Context(), `SELECT `+bindingCols+bindingFrom+where+` ORDER BY b.principal_type, 4, b.created_at`, args...)
	if err != nil {
		fail(w, apierr.Internal("failed to list bindings"))
		return
	}
	defer rows.Close()
	out := []bindingDTO{}
	for rows.Next() {
		if b, err := scanBinding(rows); err == nil {
			out = append(out, b)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type bindingReq struct {
	PrincipalType string     `json:"principal_type"`
	PrincipalID   uuid.UUID  `json:"principal_id"`
	RoleID        *uuid.UUID `json:"role_id"`
	RoleKey       string     `json:"role_key"`
	FolderID      *uuid.UUID `json:"folder_id"`
}

// canGrant enforces "no privilege escalation": the caller must hold every permission of
// the role at the binding's scope ('*' only for workspace owners).
func (s *Server) canGrant(r *http.Request, perms []string, folder *uuid.UUID) bool {
	g := authz.FromContext(r.Context())
	if g.IsOwner() {
		return true
	}
	var chain []uuid.UUID
	if folder != nil {
		var err error
		if chain, err = s.folderChain(r.Context(), workspaceRow(r).ID, folder); err != nil {
			return false
		}
	}
	for _, p := range perms {
		if p == string(authz.All) {
			return false
		}
		if !g.HasInFolderChain(authz.Permission(p), chain) {
			return false
		}
	}
	return true
}

func (s *Server) handleCreateBinding(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req bindingReq
	if !decode(w, r, &req) {
		return
	}
	if req.PrincipalType != "user" && req.PrincipalType != "group" {
		fail(w, unprocessable("invalid_principal_type", "principal_type must be user or group"))
		return
	}
	// Role: by id (custom or system) or key (system first, then the org's custom role).
	var role roleDTO
	var err error
	switch {
	case req.RoleID != nil:
		role, err = s.loadRole(r.Context(), ws.OrgID, *req.RoleID)
	case req.RoleKey != "":
		role, err = scanRole(s.pool.QueryRow(r.Context(), `SELECT `+roleCols+` FROM roles r WHERE r.key = $2 AND (r.org_id IS NULL OR r.org_id = $1)
			ORDER BY r.org_id NULLS FIRST LIMIT 1`, ws.OrgID, req.RoleKey))
		if errors.Is(err, pgx.ErrNoRows) {
			err = apierr.NotFound("role not found")
		}
	default:
		err = unprocessable("role_required", "role_id or role_key is required")
	}
	if err != nil {
		fail(w, problemOr500(err, "failed to load role"))
		return
	}
	if !role.System {
		if e := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureRoles); e != nil {
			fail(w, featureErr(e))
			return
		}
	}
	if req.PrincipalType == "user" && role.System && req.FolderID == nil {
		fail(w, apierr.Conflict("use_membership", "a member's workspace-wide built-in role is their membership role; change it with PATCH /members/{userId}"))
		return
	}
	if req.FolderID != nil {
		var n int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM folders WHERE id = $1 AND workspace_id = $2`, *req.FolderID, ws.ID).Scan(&n)
		if n == 0 {
			fail(w, unprocessable("invalid_folder", "folder_id must be a folder in this workspace"))
			return
		}
		if role.Key == "owner" && role.System {
			fail(w, unprocessable("invalid_scope", "the owner role can't be folder-scoped"))
			return
		}
	}
	if !s.canGrant(r, role.Permissions, req.FolderID) {
		fail(w, forbidden("privilege_escalation", "you can only grant permissions you hold yourself at that scope"))
		return
	}
	switch req.PrincipalType {
	case "user":
		var ok bool
		_ = s.pool.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM org_members WHERE org_id = $1 AND user_id = $2 AND status = 'active')`,
			ws.OrgID, req.PrincipalID).Scan(&ok)
		if !ok {
			fail(w, unprocessable("not_org_member", "the user must be an active member of this organisation; invite them first"))
			return
		}
	case "group":
		var n int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM groups WHERE id = $1 AND org_id = $2`, req.PrincipalID, ws.OrgID).Scan(&n)
		if n == 0 {
			fail(w, unprocessable("invalid_group", "the group must belong to this organisation"))
			return
		}
	}
	id := idgen.New()
	scope := "workspace"
	if req.FolderID != nil {
		scope = "folder"
	}
	uid, _ := actorIDs(r)
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO role_bindings (id, org_id, workspace_id, principal_type, principal_id, role_id,
			scope_type, folder_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			id, ws.OrgID, ws.ID, req.PrincipalType, req.PrincipalID, role.ID, scope, req.FolderID, uid); err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return apierr.Conflict("binding_exists", "this principal already has that role at that scope")
			}
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "binding.created", "role_binding", &id, map[string]any{
			"principal_type": req.PrincipalType, "principal_id": req.PrincipalID, "role": role.Key, "folder_id": req.FolderID})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create binding"))
		return
	}
	s.access.Invalidate(r.Context())
	b, err := scanBinding(s.pool.QueryRow(r.Context(), `SELECT `+bindingCols+bindingFrom+` WHERE b.id = $1`, id))
	if err != nil {
		fail(w, apierr.Internal("created, but failed to load binding"))
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleDeleteBinding(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("binding not found"))
		return
	}
	b, err := scanBinding(s.pool.QueryRow(r.Context(), `SELECT `+bindingCols+bindingFrom+` WHERE b.id = $1 AND b.workspace_id = $2`, id, ws.ID))
	if err != nil {
		fail(w, apierr.NotFound("binding not found"))
		return
	}
	if b.ManagedBy == "membership" {
		fail(w, apierr.Conflict("use_membership", "this is the member's workspace role; change or remove it on the Members page"))
		return
	}
	if !s.canGrant(r, b.permissions, b.FolderID) {
		fail(w, forbidden("privilege_escalation", "you can't remove a role that grants more than you hold"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM role_bindings WHERE id = $1`, id); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "binding.deleted", "role_binding", &id, map[string]any{
			"principal_type": b.PrincipalType, "principal_id": b.PrincipalID, "role": b.RoleKey, "folder_id": b.FolderID})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete binding"))
		return
	}
	s.access.Invalidate(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

// ---- access inspector ----------------------------------------------------------------

type accessVia struct {
	Kind       string     `json:"kind"` // org_role | agency | role_binding | group_binding
	OrgRole    string     `json:"org_role,omitempty"`
	BindingID  *uuid.UUID `json:"binding_id,omitempty"`
	RoleKey    string     `json:"role_key,omitempty"`
	RoleName   string     `json:"role_name,omitempty"`
	ScopeType  string     `json:"scope_type,omitempty"`
	FolderID   *uuid.UUID `json:"folder_id,omitempty"`
	FolderName *string    `json:"folder_name,omitempty"`
	GroupID    *uuid.UUID `json:"group_id,omitempty"`
	GroupName  *string    `json:"group_name,omitempty"`
	Grants     bool       `json:"grants"` // grants the permission for the asked resource
}

// GET /workspaces/{ws}/access/explain?user_id=&permission=&qr_id=|folder_id=
func (s *Server) handleExplainAccess(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	qs := r.URL.Query()
	uid, err := uuid.Parse(qs.Get("user_id"))
	if err != nil {
		fail(w, apierr.BadRequest("invalid_user_id", "user_id must be a UUID"))
		return
	}
	perm := authz.Permission(qs.Get("permission"))
	if perm == "" {
		perm = authz.QRRead
	}
	if !authz.IsAssignable(perm) && perm != authz.BillingManage && perm != authz.WorkspaceDelete {
		fail(w, apierr.BadRequest("invalid_permission", "unknown permission"))
		return
	}
	var folder *uuid.UUID
	var resource map[string]any
	if v := qs.Get("qr_id"); v != "" {
		qid, err := uuid.Parse(v)
		if err != nil {
			fail(w, apierr.BadRequest("invalid_qr_id", "qr_id must be a UUID"))
			return
		}
		code, err := s.q.GetQRCode(r.Context(), dbgen.GetQRCodeParams{ID: qid, WorkspaceID: ws.ID})
		if err != nil {
			fail(w, apierr.NotFound("QR code not found"))
			return
		}
		folder = uuidPtr(code.FolderID)
		resource = map[string]any{"type": "qr_code", "id": code.ID, "name": code.Name, "folder_id": folder}
	} else if v := qs.Get("folder_id"); v != "" {
		fid, err := uuid.Parse(v)
		if err != nil {
			fail(w, apierr.BadRequest("invalid_folder_id", "folder_id must be a UUID"))
			return
		}
		folder = &fid
		resource = map[string]any{"type": "folder", "id": fid}
	}
	var chain []uuid.UUID
	if folder != nil {
		if chain, err = s.folderChain(r.Context(), ws.ID, folder); err != nil {
			fail(w, apierr.Internal("failed to resolve folder"))
			return
		}
	}
	inChain := func(f *uuid.UUID) bool {
		if f == nil {
			return true
		}
		for _, c := range chain {
			if c == *f {
				return true
			}
		}
		return false
	}
	has := func(perms []string) bool {
		for _, p := range perms {
			if p == string(authz.All) || p == string(perm) {
				return true
			}
		}
		return false
	}
	via := []accessVia{}
	notes := []string{}
	var role, status string
	err = s.pool.QueryRow(r.Context(), `SELECT org_role, status FROM org_members WHERE org_id = $1 AND user_id = $2`, ws.OrgID, uid).Scan(&role, &status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		var parent string
		if s.pool.QueryRow(r.Context(), `SELECT m.org_role FROM organizations o JOIN org_members m ON m.org_id = o.parent_org_id
			AND m.user_id = $2 AND m.status = 'active' WHERE o.id = $1 AND m.org_role IN ('org_owner','org_admin')`, ws.OrgID, uid).Scan(&parent) == nil {
			via = append(via, accessVia{Kind: "agency", OrgRole: parent, Grants: has(stringsOf(authz.SystemRoles["admin"]))})
		} else {
			notes = append(notes, "not a member of this organisation")
		}
	case err != nil:
		fail(w, apierr.Internal("failed to load membership"))
		return
	default:
		if status != "active" {
			notes = append(notes, "organisation membership is "+status+": all access is suspended")
		}
		switch role {
		case "org_owner":
			via = append(via, accessVia{Kind: "org_role", OrgRole: role, Grants: true})
		case "org_admin":
			via = append(via, accessVia{Kind: "org_role", OrgRole: role, Grants: has(stringsOf(authz.SystemRoles["admin"]))})
		}
	}
	rows, err := s.pool.Query(r.Context(), `SELECT b.id, b.principal_type, b.principal_id, g.display_name, r.key, r.name, r.permissions,
			b.scope_type, b.folder_id, f.name
		FROM role_bindings b JOIN roles r ON r.id = b.role_id
		LEFT JOIN groups g ON b.principal_type = 'group' AND g.id = b.principal_id
		LEFT JOIN folders f ON f.id = b.folder_id
		WHERE b.workspace_id = $1 AND ((b.principal_type = 'user' AND b.principal_id = $2)
		   OR (b.principal_type = 'group' AND b.principal_id IN (SELECT gm.group_id FROM group_members gm JOIN groups gg ON gg.id = gm.group_id
		       WHERE gm.user_id = $2 AND gg.org_id = $3)))
		ORDER BY b.created_at`, ws.ID, uid, ws.OrgID)
	if err != nil {
		fail(w, apierr.Internal("failed to load bindings"))
		return
	}
	for rows.Next() {
		var (
			v     accessVia
			ptype string
			pid   uuid.UUID
			perms []string
		)
		if err := rows.Scan(&v.BindingID, &ptype, &pid, &v.GroupName, &v.RoleKey, &v.RoleName, &perms, &v.ScopeType, &v.FolderID, &v.FolderName); err != nil {
			continue
		}
		v.Kind = "role_binding"
		if ptype == "group" {
			v.Kind, v.GroupID = "group_binding", &pid
		}
		// A folder binding grants the permission for resources inside that folder's subtree.
		v.Grants = has(perms) && (v.FolderID == nil || (folder != nil && inChain(v.FolderID)))
		via = append(via, v)
	}
	rows.Close()
	allowed := status == "active" || (role == "" && len(via) > 0)
	if allowed {
		allowed = false
		for _, v := range via {
			if v.Grants {
				allowed = true
			}
		}
	}
	if !allowed && folder == nil {
		for _, v := range via {
			if v.FolderID != nil && v.Kind != "org_role" {
				notes = append(notes, "folder-scoped bindings apply only inside their folders; pass qr_id or folder_id to check a resource")
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed, "user_id": uid, "permission": perm, "resource": resource,
		"via": via, "notes": notes})
}

func stringsOf(ps []authz.Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}

// ---- access review -------------------------------------------------------------------

type reviewRow struct {
	UserID        uuid.UUID  `json:"user_id"`
	Email         string     `json:"email"`
	Name          string     `json:"name"`
	OrgRole       string     `json:"org_role"`
	MemberStatus  string     `json:"member_status"`
	MemberSource  string     `json:"member_source"`
	MFA           bool       `json:"mfa_enabled"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	WorkspaceID   *uuid.UUID `json:"workspace_id"`
	WorkspaceName string     `json:"workspace_name"`
	Role          string     `json:"role"`
	Scope         string     `json:"scope"`
	Via           string     `json:"via"`
}

// GET /orgs/{org}/access-review?format=json|csv: every user × workspace × role × source.
func (s *Server) handleAccessReview(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	rows, err := s.pool.Query(r.Context(), `
		WITH members AS (
		    SELECT m.user_id, u.email::text AS email, u.name, m.org_role, m.status, m.source, u.last_login_at,
		           EXISTS (SELECT 1 FROM user_mfa_factors f WHERE f.user_id = u.id) AS mfa
		    FROM org_members m JOIN users u ON u.id = m.user_id WHERE m.org_id = $1
		)
		SELECT m.user_id, m.email, m.name, m.org_role, m.status, m.source, m.mfa, m.last_login_at,
		       NULL::uuid, 'All workspaces', CASE m.org_role WHEN 'org_owner' THEN 'owner' ELSE 'admin' END, 'organisation', 'org role ' || m.org_role
		FROM members m WHERE m.org_role IN ('org_owner','org_admin')
		UNION ALL
		SELECT m.user_id, m.email, m.name, m.org_role, m.status, m.source, m.mfa, m.last_login_at,
		       w.id, w.name, r.key,
		       CASE WHEN b.folder_id IS NULL THEN 'workspace' ELSE 'folder: ' || COALESCE(f.name, b.folder_id::text) END,
		       CASE WHEN b.principal_type = 'user' THEN 'direct' ELSE 'group: ' || g.display_name END
		FROM members m
		JOIN role_bindings b ON b.org_id = $1 AND ((b.principal_type = 'user' AND b.principal_id = m.user_id)
		     OR (b.principal_type = 'group' AND b.principal_id IN (SELECT gm.group_id FROM group_members gm WHERE gm.user_id = m.user_id)))
		JOIN workspaces w ON w.id = b.workspace_id AND w.deleted_at IS NULL
		JOIN roles r ON r.id = b.role_id
		LEFT JOIN folders f ON f.id = b.folder_id
		LEFT JOIN groups g ON b.principal_type = 'group' AND g.id = b.principal_id
		ORDER BY 3, 10 NULLS FIRST, 11`, o.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to build the review"))
		return
	}
	out := []reviewRow{}
	for rows.Next() {
		var x reviewRow
		if err := rows.Scan(&x.UserID, &x.Email, &x.Name, &x.OrgRole, &x.MemberStatus, &x.MemberSource, &x.MFA, &x.LastLoginAt,
			&x.WorkspaceID, &x.WorkspaceName, &x.Role, &x.Scope, &x.Via); err == nil {
			out = append(out, x)
		}
	}
	rows.Close()
	_ = s.auditOrg(r, s.q, o.ID, "access_review.exported", "organization", &o.ID, map[string]any{"rows": len(out), "format": r.URL.Query().Get("format")})
	var last map[string]any
	var settings []byte
	_ = s.pool.QueryRow(r.Context(), `SELECT settings FROM organizations WHERE id = $1`, o.ID).Scan(&settings)
	var st map[string]any
	if json.Unmarshal(settings, &st) == nil {
		last, _ = st["access_review"].(map[string]any)
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="access-review-`+o.Slug+`-`+time.Now().UTC().Format("2006-01-02")+`.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"user_id", "email", "name", "org_role", "member_status", "member_source", "mfa_enabled", "last_login_at",
			"workspace_id", "workspace", "role", "scope", "via"})
		for _, x := range out {
			ll, wid := "", ""
			if x.LastLoginAt != nil {
				ll = x.LastLoginAt.UTC().Format(time.RFC3339)
			}
			if x.WorkspaceID != nil {
				wid = x.WorkspaceID.String()
			}
			_ = cw.Write([]string{x.UserID.String(), csvCell(x.Email), csvCell(x.Name), x.OrgRole, x.MemberStatus, x.MemberSource,
				boolStr(x.MFA), ll, wid, csvCell(x.WorkspaceName), x.Role, csvCell(x.Scope), csvCell(x.Via)})
		}
		cw.Flush()
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"generated_at": time.Now().UTC(), "last_review": last, "rows": out})
}

// csvCell neutralises spreadsheet formula injection.
func csvCell(v string) string { return csvSafe(&v) }

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

type reviewCompleteReq struct {
	Note string `json:"note"`
}

// POST /orgs/{org}/access-review/complete: record who reviewed access and when (SOC 2 evidence).
func (s *Server) handleCompleteAccessReview(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	var req reviewCompleteReq
	if !decodeOptional(w, r, &req) {
		return
	}
	if len(req.Note) > 2000 {
		fail(w, unprocessable("invalid_note", "note must be at most 2000 characters"))
		return
	}
	p := principal(r)
	rec := map[string]any{"reviewed_at": time.Now().UTC(), "reviewed_by": p.UserID, "note": strings.TrimSpace(req.Note)}
	b, _ := json.Marshal(rec)
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `UPDATE organizations SET settings = jsonb_set(settings, '{access_review}', $2::jsonb), updated_at = now()
			WHERE id = $1`, o.ID, b); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "access_review.completed", "organization", &o.ID, rec)
	})
	if err != nil {
		fail(w, apierr.Internal("failed to record the review"))
		return
	}
	writeJSON(w, http.StatusOK, rec)
}
