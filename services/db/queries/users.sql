-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, avatar_url, locale, timezone, is_staff)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    locale = COALESCE($4, locale),
    timezone = COALESCE($5, timezone),
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: CreateOAuthAccount :one
INSERT INTO oauth_accounts (id, user_id, provider, provider_user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOAuthAccount :one
SELECT * FROM oauth_accounts
WHERE provider = $1 AND provider_user_id = $2;

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, family_id, refresh_token_hash, user_agent, ip_prefix, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSessionByHash :one
SELECT * FROM sessions
WHERE refresh_token_hash = $1;

-- name: RotateSession :one
UPDATE sessions
SET replaced_by = $2,
    last_used_at = now()
WHERE id = $1
RETURNING *;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = now()
WHERE id = $1;

-- name: RevokeSessionFamily :exec
UPDATE sessions
SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: CreateEmailToken :one
INSERT INTO email_tokens (id, user_id, purpose, token_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetEmailToken :one
SELECT * FROM email_tokens
WHERE token_hash = $1 AND expires_at > now() AND used_at IS NULL;

-- name: MarkEmailTokenUsed :exec
UPDATE email_tokens
SET used_at = now()
WHERE id = $1;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: TouchSession :exec
UPDATE sessions SET last_used_at = now() WHERE id = $1;

-- name: RevokeUserSessions :exec
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: ListUserSessions :many
SELECT * FROM sessions
WHERE user_id = $1 AND revoked_at IS NULL AND replaced_by IS NULL AND expires_at > now()
ORDER BY last_used_at DESC;

-- name: RevokeUserSession :execrows
UPDATE sessions SET revoked_at = now() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()), updated_at = now() WHERE id = $1;

-- name: SetLastLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: GetValidEmailToken :one
SELECT * FROM email_tokens
WHERE token_hash = $1 AND purpose = $2 AND used_at IS NULL AND expires_at > now();

-- name: ConsumeEmailToken :execrows
UPDATE email_tokens SET used_at = now() WHERE id = $1 AND used_at IS NULL;

-- name: InvalidateEmailTokens :exec
UPDATE email_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL;
