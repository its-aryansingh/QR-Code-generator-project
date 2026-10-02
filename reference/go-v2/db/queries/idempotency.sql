-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys
WHERE workspace_id = $1 AND key = $2;

-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (workspace_id, key, method, path, request_hash, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: SetIdempotencyResponse :exec
UPDATE idempotency_keys
SET status_code = $3,
    response_body = $4
WHERE workspace_id = $1 AND key = $2;

-- name: DeleteExpiredIdempotencyKeys :exec
DELETE FROM idempotency_keys
WHERE expires_at <= now();

-- name: DeleteIdempotencyKey :exec
DELETE FROM idempotency_keys WHERE workspace_id = $1 AND key = $2;
