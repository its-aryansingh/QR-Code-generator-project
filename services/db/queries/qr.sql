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
           AND v2.approval_status IN ('not_required','approved')
       ) AS next_change_at
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
      AND v1.approval_status IN ('not_required','approved')
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = $1 AND q.short_code = $2 AND q.deleted_at IS NULL;

-- name: GetQRCodeForUpdate :one
SELECT * FROM qr_codes
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
FOR UPDATE;

-- name: ListQRCodesPage :many
-- Cursor pagination on (created_at, id) descending. Folder filter supports folder-scoped access.
SELECT q.*, v.destination_url, v.destination_kind
FROM qr_codes q
LEFT JOIN qr_versions v ON v.id = q.current_version_id
WHERE q.workspace_id = sqlc.arg(workspace_id)
  AND q.deleted_at IS NULL
  AND (sqlc.narg(search)::text IS NULL OR q.name ILIKE '%' || sqlc.narg(search) || '%' OR q.short_code ILIKE '%' || sqlc.narg(search) || '%')
  AND (sqlc.narg(status)::text IS NULL OR q.status = sqlc.narg(status))
  AND (sqlc.narg(mode)::text IS NULL OR q.mode = sqlc.narg(mode))
  AND (sqlc.narg(folder_ids)::uuid[] IS NULL OR q.folder_id = ANY(sqlc.narg(folder_ids)::uuid[]))
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL
       OR (q.created_at, q.id) < (sqlc.narg(cursor_created_at)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY q.created_at DESC, q.id DESC
LIMIT sqlc.arg(row_limit);

-- name: UpdateQRCodeSettings :one
-- Explicit set_* flags allow clearing nullable fields.
UPDATE qr_codes SET
    name          = COALESCE(sqlc.narg(name), name),
    design        = COALESCE(sqlc.narg(design), design),
    design_hash   = COALESCE(sqlc.narg(design_hash), design_hash),
    folder_id     = CASE WHEN sqlc.arg(set_folder)::bool     THEN sqlc.narg(folder_id)     ELSE folder_id END,
    campaign_id   = CASE WHEN sqlc.arg(set_campaign)::bool   THEN sqlc.narg(campaign_id)   ELSE campaign_id END,
    starts_at     = CASE WHEN sqlc.arg(set_starts_at)::bool  THEN sqlc.narg(starts_at)     ELSE starts_at END,
    expires_at    = CASE WHEN sqlc.arg(set_expires_at)::bool THEN sqlc.narg(expires_at)    ELSE expires_at END,
    scan_limit    = CASE WHEN sqlc.arg(set_scan_limit)::bool THEN sqlc.narg(scan_limit)    ELSE scan_limit END,
    fallback_url  = CASE WHEN sqlc.arg(set_fallback)::bool   THEN sqlc.narg(fallback_url)  ELSE fallback_url END,
    password_hash = CASE WHEN sqlc.arg(set_password)::bool   THEN sqlc.narg(password_hash) ELSE password_hash END,
    updated_at    = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL
RETURNING *;

-- name: SetQRCodeStatus :one
UPDATE qr_codes SET
    status = sqlc.arg(status),
    archived_at = CASE WHEN sqlc.arg(status) = 'archived' THEN now() ELSE NULL END,
    updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id) AND deleted_at IS NULL
RETURNING *;

-- name: SetQRCodeCurrentVersion :exec
UPDATE qr_codes SET current_version_id = sqlc.arg(version_id), updated_at = now()
WHERE id = sqlc.arg(id) AND workspace_id = sqlc.arg(workspace_id);

-- name: RestoreQRCode :one
UPDATE qr_codes SET deleted_at = NULL, updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NOT NULL AND deleted_at > now() - interval '30 days'
RETURNING *;

-- name: SoftDeleteQRCodeScoped :one
UPDATE qr_codes SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: MaxQRVersionNo :one
SELECT COALESCE(max(version_no), 0)::int FROM qr_versions WHERE qr_code_id = $1;

-- name: GetQRVersionScoped :one
SELECT v.* FROM qr_versions v
JOIN qr_codes q ON q.id = v.qr_code_id
WHERE v.id = sqlc.arg(version_id) AND v.qr_code_id = sqlc.arg(qr_code_id) AND q.workspace_id = sqlc.arg(workspace_id);

-- name: ListQRVersionsScoped :many
SELECT v.* FROM qr_versions v
JOIN qr_codes q ON q.id = v.qr_code_id
WHERE v.qr_code_id = sqlc.arg(qr_code_id) AND q.workspace_id = sqlc.arg(workspace_id)
ORDER BY v.version_no DESC;

-- name: CreateQRVersionFull :one
INSERT INTO qr_versions (
    id, qr_code_id, version_no, destination_kind, destination_url, hosted_page,
    rules, utm, effective_at, safety_status, restored_from, change_note, created_by, created_by_key
) VALUES (
    sqlc.arg(id), sqlc.arg(qr_code_id), sqlc.arg(version_no), sqlc.arg(destination_kind), sqlc.narg(destination_url),
    sqlc.narg(hosted_page), sqlc.arg(rules), sqlc.arg(utm), sqlc.arg(effective_at), sqlc.arg(safety_status),
    sqlc.narg(restored_from), sqlc.narg(change_note), sqlc.narg(created_by), sqlc.narg(created_by_key)
) RETURNING *;

-- name: CurrentEffectiveVersion :one
SELECT * FROM qr_versions
WHERE qr_code_id = $1 AND effective_at <= now() AND approval_status IN ('not_required','approved')
ORDER BY effective_at DESC, version_no DESC
LIMIT 1;

-- name: ScheduledVersionsDue :many
-- Versions whose effective_at has passed but are not yet the denormalised current pointer.
SELECT v.id AS version_id, q.id AS qr_code_id, q.workspace_id, q.domain_id, q.short_code
FROM qr_versions v
JOIN qr_codes q ON q.id = v.qr_code_id
WHERE v.effective_at <= now() AND v.effective_at > now() - interval '2 days'
  AND v.approval_status IN ('not_required','approved')
  AND q.current_version_id IS DISTINCT FROM v.id
  AND v.id = (SELECT v3.id FROM qr_versions v3 WHERE v3.qr_code_id = q.id AND v3.effective_at <= now()
              AND v3.approval_status IN ('not_required','approved')
              ORDER BY v3.effective_at DESC, v3.version_no DESC LIMIT 1)
LIMIT 500;

-- name: DeleteScheduledVersion :execrows
DELETE FROM qr_versions v
USING qr_codes q
WHERE v.id = sqlc.arg(version_id) AND v.qr_code_id = sqlc.arg(qr_code_id)
  AND q.id = v.qr_code_id AND q.workspace_id = sqlc.arg(workspace_id)
  AND v.effective_at > now() AND v.approval_status IN ('not_required','approved');

-- name: CreateQRCodeFull :one
INSERT INTO qr_codes (
    id, workspace_id, created_by, mode, content_type, name, domain_id, short_code, gs1_gtin,
    static_payload, static_content, design, design_hash, template_id, folder_id, campaign_id,
    status, starts_at, expires_at, scan_limit, password_hash, fallback_url, safety_status
) VALUES (
    sqlc.arg(id), sqlc.arg(workspace_id), sqlc.narg(created_by), sqlc.arg(mode), sqlc.arg(content_type), sqlc.arg(name),
    sqlc.narg(domain_id), sqlc.narg(short_code), sqlc.narg(gs1_gtin), sqlc.narg(static_payload), sqlc.narg(static_content),
    sqlc.arg(design), sqlc.narg(design_hash), sqlc.narg(template_id), sqlc.narg(folder_id), sqlc.narg(campaign_id),
    sqlc.arg(status), sqlc.narg(starts_at), sqlc.narg(expires_at), sqlc.narg(scan_limit), sqlc.narg(password_hash),
    sqlc.narg(fallback_url), sqlc.arg(safety_status)
) RETURNING *;

-- name: ShortCodeTaken :one
-- A short code is burned forever: live rows, soft-deleted rows and purge tombstones all count.
SELECT (EXISTS (SELECT 1 FROM qr_codes q WHERE q.domain_id = $1 AND q.short_code = $2)
     OR EXISTS (SELECT 1 FROM short_code_tombstones t WHERE t.domain_id = $1 AND t.short_code = $2))::bool;

-- name: SetQRCodeSafety :exec
UPDATE qr_codes SET safety_status = sqlc.arg(safety_status), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: SetQRVersionSafety :exec
UPDATE qr_versions SET safety_status = sqlc.arg(safety_status) WHERE id = sqlc.arg(id);

-- name: CountQRCodesInFolder :one
SELECT count(*)::int FROM qr_codes WHERE workspace_id = $1 AND folder_id = $2 AND deleted_at IS NULL;

-- name: NextScheduledVersion :one
SELECT * FROM qr_versions
WHERE qr_code_id = $1 AND effective_at > now() AND approval_status IN ('not_required','approved')
ORDER BY effective_at, version_no
LIMIT 1;

-- name: ListDomainsUsable :many
-- Domains a workspace may issue codes on: the platform domain plus its own active domains.
SELECT * FROM domains WHERE (workspace_id IS NULL OR workspace_id = $1) AND status = 'active';

-- name: ListApprovalVersions :many
SELECT * FROM qr_versions WHERE approval_request_id = $1 ORDER BY version_no;
