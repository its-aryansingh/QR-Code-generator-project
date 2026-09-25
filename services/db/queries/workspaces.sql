-- name: CreateWorkspace :one
INSERT INTO workspaces (id, name, slug, owner_id, plan_id, timezone, brand, settings)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetWorkspaceBySlug :one
SELECT * FROM workspaces
WHERE slug = $1 AND deleted_at IS NULL;

-- name: ListWorkspacesForUser :many
SELECT w.*, m.role
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id
WHERE m.user_id = $1 AND w.deleted_at IS NULL
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
