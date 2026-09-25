-- name: CreateDomain :one
INSERT INTO domains (id, workspace_id, hostname, verification_token, root_redirect_url, not_found_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetDomainByID :one
SELECT * FROM domains
WHERE id = $1;

-- name: GetDomainByHostname :one
SELECT * FROM domains
WHERE hostname = $1;

-- name: ListDomainsForWorkspace :many
SELECT * FROM domains
WHERE workspace_id = $1 OR workspace_id IS NULL
ORDER BY created_at;

-- name: ListActiveDomains :many
SELECT * FROM domains
WHERE status = 'active';

-- name: UpdateDomainStatus :one
UPDATE domains
SET status = $2, tls_status = $3, verified_at = COALESCE($4, verified_at), last_checked_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDomain :exec
DELETE FROM domains
WHERE id = $1 AND workspace_id = $2;
