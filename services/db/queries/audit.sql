-- name: CreateAuditLog :one
-- org_id defaults to the workspace's organisation so every entry joins its org's hash chain.
INSERT INTO audit_logs (
    org_id, workspace_id, actor_type, actor_id, action, target_type, target_id,
    changes, ip_prefix, user_agent, request_id
)
VALUES (
    COALESCE(sqlc.narg(org_id)::uuid, (SELECT w.org_id FROM workspaces w WHERE w.id = sqlc.narg(workspace_id)::uuid)),
    sqlc.narg(workspace_id), sqlc.arg(actor_type), sqlc.narg(actor_id), sqlc.arg(action), sqlc.arg(target_type),
    sqlc.narg(target_id), sqlc.arg(changes), sqlc.narg(ip_prefix), sqlc.narg(user_agent), sqlc.narg(request_id)
)
RETURNING id;

-- name: ListAuditLogs :many
SELECT a.*, u.email as actor_email, u.name as actor_name
FROM audit_logs a
LEFT JOIN users u ON u.id = a.actor_id AND a.actor_type = 'user'
WHERE a.workspace_id = $1
ORDER BY a.created_at DESC
LIMIT $2;
