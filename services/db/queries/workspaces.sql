-- name: CreateWorkspace :one
INSERT INTO workspaces (id, org_id, name, slug, owner_id, plan_id, timezone, brand, settings)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces
WHERE slug = $1 AND deleted_at IS NULL;

-- name: ListWorkspacesForUser :many
-- Workspaces the user is a member of, plus every workspace of organisations they administer
-- (org owners/admins have implicit admin on all of their org's workspaces).
SELECT w.*, COALESCE(m.role, CASE om.org_role WHEN 'org_owner' THEN 'org_owner' ELSE 'org_admin' END)::text AS role
FROM workspaces w
JOIN org_members om ON om.org_id = w.org_id AND om.user_id = $1 AND om.status = 'active'
LEFT JOIN workspace_members m ON m.workspace_id = w.id AND m.user_id = $1
WHERE w.deleted_at IS NULL AND (m.user_id IS NOT NULL OR om.org_role IN ('org_owner','org_admin'))
ORDER BY w.name;

-- name: AddWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = EXCLUDED.role;

-- name: GetWorkspaceMember :one
SELECT * FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: ListWorkspaceMembers :many
SELECT m.*, u.email, u.name, u.avatar_url
FROM workspace_members m
JOIN users u ON u.id = m.user_id
WHERE m.workspace_id = $1
ORDER BY m.created_at;

-- name: UpdateWorkspaceMemberRole :exec
UPDATE workspace_members
SET role = $3
WHERE workspace_id = $1 AND user_id = $2;

-- name: RemoveWorkspaceMember :exec
DELETE FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: TransferWorkspaceOwnership :exec
UPDATE workspaces
SET owner_id = $2, updated_at = now()
WHERE id = $1;

-- name: CreateInvite :one
INSERT INTO invites (id, workspace_id, email, role, token_hash, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetInviteByHash :one
SELECT * FROM invites
WHERE token_hash = $1 AND expires_at > now() AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: AcceptInvite :exec
UPDATE invites
SET accepted_at = now()
WHERE id = $1;

-- name: RevokeInvite :exec
UPDATE invites
SET revoked_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: UpdateWorkspaceSettings :one
UPDATE workspaces SET
    name = COALESCE(sqlc.narg(name), name),
    timezone = COALESCE(sqlc.narg(timezone), timezone),
    updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: SlugExists :one
SELECT EXISTS (SELECT 1 FROM workspaces WHERE slug = $1)::bool;

-- name: ListInvites :many
SELECT * FROM invites
WHERE workspace_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: GetInviteByHashAny :one
SELECT i.*, w.name AS workspace_name, w.slug AS workspace_slug
FROM invites i JOIN workspaces w ON w.id = i.workspace_id
WHERE i.token_hash = $1;

-- name: CountWorkspaceMembers :one
SELECT count(*)::int FROM workspace_members WHERE workspace_id = $1;

-- name: CountOwners :one
SELECT count(*)::int FROM workspace_members WHERE workspace_id = $1 AND role = 'owner';

-- name: GetInviteScoped :one
SELECT * FROM invites WHERE id = $1 AND workspace_id = $2;

-- name: CountOwnedWorkspaces :one
SELECT count(*)::int FROM workspaces WHERE owner_id = $1 AND deleted_at IS NULL;

-- name: CountPendingInvites :one
SELECT count(*)::int FROM invites
WHERE workspace_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now();

-- name: SoftDeleteWorkspace :exec
UPDATE workspaces SET deleted_at = now(), updated_at = now() WHERE id = $1;
