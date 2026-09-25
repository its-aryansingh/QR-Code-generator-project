-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    workspace_id, actor_type, actor_id, action, target_type, target_id,
    changes, ip_prefix, user_agent, request_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: ListAuditLogs :many
SELECT a.*, u.email as actor_email, u.name as actor_name
FROM audit_logs a
LEFT JOIN users u ON u.id = a.actor_id AND a.actor_type = 'user'
WHERE a.workspace_id = $1
ORDER BY a.created_at DESC
LIMIT $2;
