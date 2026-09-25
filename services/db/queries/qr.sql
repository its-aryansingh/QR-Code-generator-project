-- name: CreateQRCode :one
INSERT INTO qr_codes (
    id, workspace_id, domain_id, short_code, mode, content_type,
    name, status, design, design_hash, static_payload, static_content
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetQRCode :one
SELECT * FROM qr_codes
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL;

-- name: GetQRCodeByDomainAndCode :one
SELECT * FROM qr_codes
WHERE domain_id = $1 AND short_code = $2 AND deleted_at IS NULL;

-- name: ListQRCodes :many
SELECT q.*, v.destination_url, v.destination_kind
FROM qr_codes q
LEFT JOIN qr_versions v ON v.id = q.current_version_id
WHERE q.workspace_id = sqlc.arg(workspace_id)
  AND q.deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR q.name ILIKE '%' || sqlc.narg(search) || '%' OR q.short_code ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(status)::text IS NULL OR q.status = sqlc.narg(status))
ORDER BY q.created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: CountActiveDynamicQRCodes :one
SELECT count(*)::int FROM qr_codes
WHERE workspace_id = $1 AND mode = 'dynamic' AND deleted_at IS NULL;

-- name: UpdateQRCode :one
UPDATE qr_codes
SET name = COALESCE(sqlc.narg(name), name),
    design = COALESCE(sqlc.narg(design), design),
    design_hash = COALESCE(sqlc.narg(design_hash), design_hash),
    status = COALESCE(sqlc.narg(status), status),
    folder_id = COALESCE(sqlc.narg(folder_id), folder_id),
    current_version_id = COALESCE(sqlc.narg(current_version_id), current_version_id),
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteQRCode :exec
UPDATE qr_codes
SET deleted_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: CreateQRVersion :one
INSERT INTO qr_versions (
    id, qr_code_id, version_no, destination_kind, destination_url,
    hosted_page, rules, utm, effective_at, change_note, created_by
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetLatestQRVersion :one
SELECT * FROM qr_versions
WHERE qr_code_id = $1
ORDER BY version_no DESC
LIMIT 1;

-- name: ListQRVersions :many
SELECT * FROM qr_versions
WHERE qr_code_id = $1
ORDER BY version_no DESC;

-- name: GetResolvedLink :one
SELECT q.id AS qr_code_id,
       q.workspace_id,
       q.status AS qr_status,
       q.safety_status,
       q.starts_at,
       q.expires_at,
       q.scan_limit,
       q.total_scans,
       q.password_hash,
       q.fallback_url,
       w.timezone AS workspace_timezone,
       v.id AS version_id,
       v.version_no,
       v.destination_kind,
       v.destination_url,
       v.rules,
       v.utm,
       v.hosted_page,
       (
         SELECT min(v2.effective_at)
         FROM qr_versions v2
         WHERE v2.qr_code_id = q.id AND v2.effective_at > now()
       ) AS next_change_at
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = $1 AND q.short_code = $2 AND q.deleted_at IS NULL;
