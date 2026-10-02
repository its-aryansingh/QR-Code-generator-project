-- Folders, tags and campaigns: how a workspace organises its codes.

-- name: ListFolders :many
SELECT f.*, (SELECT count(*) FROM qr_codes q WHERE q.folder_id = f.id AND q.deleted_at IS NULL)::int AS qr_count
FROM folders f WHERE f.workspace_id = $1
ORDER BY f.parent_id NULLS FIRST, f.position, f.name;

-- name: GetFolder :one
SELECT * FROM folders WHERE id = $1 AND workspace_id = $2;

-- name: CreateFolder :one
INSERT INTO folders (id, workspace_id, parent_id, name, position)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.narg(parent_id), sqlc.arg(name), sqlc.arg(position))
RETURNING *;

-- name: UpdateFolder :one
UPDATE folders SET
    name      = COALESCE(sqlc.narg(name), name),
    parent_id = CASE WHEN sqlc.arg(set_parent)::bool THEN sqlc.narg(parent_id) ELSE parent_id END,
    position  = COALESCE(sqlc.narg(position), position),
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;

-- name: DeleteFolder :execrows
DELETE FROM folders WHERE id = $1 AND workspace_id = $2;

-- name: FolderAncestors :many
-- The folder itself and every ancestor, nearest first (used for folder-scoped permissions and cycle checks).
WITH RECURSIVE chain AS (
    SELECT f.id, f.parent_id, 0 AS depth FROM folders f WHERE f.id = sqlc.arg(id) AND f.workspace_id = sqlc.arg(workspace_id)
    UNION ALL
    SELECT p.id, p.parent_id, c.depth + 1 FROM folders p JOIN chain c ON p.id = c.parent_id WHERE c.depth < 32
)
SELECT id FROM chain ORDER BY depth;

-- name: FolderDescendants :many
WITH RECURSIVE tree AS (
    SELECT f.id, 0 AS depth FROM folders f WHERE f.id = sqlc.arg(id) AND f.workspace_id = sqlc.arg(workspace_id)
    UNION ALL
    SELECT c.id, t.depth + 1 FROM folders c JOIN tree t ON c.parent_id = t.id WHERE t.depth < 32
)
SELECT id FROM tree;

-- name: ListTags :many
SELECT t.*, (SELECT count(*) FROM qr_code_tags x JOIN qr_codes q ON q.id = x.qr_code_id
             WHERE x.tag_id = t.id AND q.deleted_at IS NULL)::int AS qr_count
FROM tags t WHERE t.workspace_id = $1 ORDER BY t.name;

-- name: CreateTag :one
INSERT INTO tags (id, workspace_id, name, color) VALUES ($1, $2, $3, $4) RETURNING *;

-- name: UpdateTag :one
UPDATE tags SET name = COALESCE(sqlc.narg(name), name), color = COALESCE(sqlc.narg(color), color)
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) RETURNING *;

-- name: DeleteTag :execrows
DELETE FROM tags WHERE id = $1 AND workspace_id = $2;

-- name: ClearQRTags :exec
DELETE FROM qr_code_tags WHERE qr_code_id = $1;

-- name: AddQRTags :exec
-- Only tags belonging to the code's workspace are attached.
INSERT INTO qr_code_tags (qr_code_id, tag_id)
SELECT sqlc.arg(qr_code_id), t.id FROM tags t
WHERE t.workspace_id = sqlc.arg(workspace_id) AND t.id = ANY(sqlc.arg(tag_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: ListQRTags :many
SELECT t.* FROM tags t JOIN qr_code_tags x ON x.tag_id = t.id WHERE x.qr_code_id = $1 ORDER BY t.name;

-- name: ListCampaigns :many
SELECT c.*,
       (SELECT count(*) FROM qr_codes q WHERE q.campaign_id = c.id AND q.deleted_at IS NULL)::int AS qr_count,
       (SELECT COALESCE(sum(q.total_scans), 0) FROM qr_codes q WHERE q.campaign_id = c.id AND q.deleted_at IS NULL)::bigint AS total_scans
FROM campaigns c WHERE c.workspace_id = $1
ORDER BY c.created_at DESC;

-- name: GetCampaign :one
SELECT * FROM campaigns WHERE id = $1 AND workspace_id = $2;

-- name: CreateCampaign :one
INSERT INTO campaigns (id, workspace_id, name, status, starts_at, ends_at, goal_scans, utm, created_by)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), sqlc.arg(status), sqlc.narg(starts_at), sqlc.narg(ends_at),
        sqlc.narg(goal_scans), sqlc.arg(utm), sqlc.narg(created_by))
RETURNING *;

-- name: UpdateCampaign :one
UPDATE campaigns SET
    name       = COALESCE(sqlc.narg(name), name),
    status     = COALESCE(sqlc.narg(status), status),
    starts_at  = CASE WHEN sqlc.arg(set_starts_at)::bool THEN sqlc.narg(starts_at) ELSE starts_at END,
    ends_at    = CASE WHEN sqlc.arg(set_ends_at)::bool THEN sqlc.narg(ends_at) ELSE ends_at END,
    goal_scans = CASE WHEN sqlc.arg(set_goal)::bool THEN sqlc.narg(goal_scans) ELSE goal_scans END,
    utm        = COALESCE(sqlc.narg(utm), utm),
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;

-- name: DeleteCampaign :execrows
DELETE FROM campaigns WHERE id = $1 AND workspace_id = $2;

-- name: GetTemplate :one
SELECT * FROM templates WHERE id = $1 AND workspace_id = $2;

-- name: GetDefaultTemplate :one
SELECT * FROM templates WHERE workspace_id = $1 AND is_default;

-- name: ListTemplates :many
SELECT * FROM templates WHERE workspace_id = $1 ORDER BY is_default DESC, name;

-- name: CreateTemplate :one
INSERT INTO templates (id, workspace_id, name, design, is_locked, is_default, created_by)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), sqlc.arg(design), sqlc.arg(is_locked), sqlc.arg(is_default), sqlc.narg(created_by))
RETURNING *;

-- name: UpdateTemplate :one
UPDATE templates SET
    name = COALESCE(sqlc.narg(name), name),
    design = COALESCE(sqlc.narg(design), design),
    is_locked = COALESCE(sqlc.narg(is_locked), is_locked),
    is_default = COALESCE(sqlc.narg(is_default), is_default),
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id)
RETURNING *;

-- name: ClearDefaultTemplate :exec
UPDATE templates SET is_default = false WHERE workspace_id = $1 AND is_default;

-- name: DeleteTemplate :execrows
DELETE FROM templates WHERE id = $1 AND workspace_id = $2;

-- name: CountTemplates :one
SELECT count(*)::int FROM templates WHERE workspace_id = $1;
