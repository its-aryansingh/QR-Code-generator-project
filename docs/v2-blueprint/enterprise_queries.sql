-- =====================================================================
-- Enterprise queries (sqlc: services/db/queries/enterprise_*.sql)
-- Every query below was executed against fixture data (plan §E-V).
-- =====================================================================

-- name: EffectivePermissions :many
-- All permissions a user holds in a workspace, from direct and group bindings.
-- folder_id NULL = workspace-wide; otherwise the grant applies to that folder subtree.
WITH RECURSIVE principal AS (
    SELECT 'user'::text AS principal_type, sqlc.arg(user_id)::uuid AS principal_id
    UNION ALL
    SELECT 'group', gm.group_id FROM group_members gm WHERE gm.user_id = sqlc.arg(user_id)::uuid
)
SELECT DISTINCT unnest(r.permissions) AS permission, b.folder_id
FROM role_bindings b
JOIN principal p ON p.principal_type = b.principal_type AND p.principal_id = b.principal_id
JOIN roles r ON r.id = b.role_id
JOIN org_members om ON om.org_id = b.org_id AND om.user_id = sqlc.arg(user_id)::uuid AND om.status = 'active'
WHERE b.workspace_id = sqlc.arg(workspace_id)::uuid;

-- name: FolderAncestors :many
-- A folder-scoped grant on F applies to F and every descendant; the service checks
-- whether any granted folder_id is in the ancestor chain of the target QR's folder.
WITH RECURSIVE chain AS (
    SELECT id, parent_id FROM folders WHERE id = sqlc.arg(folder_id)::uuid AND workspace_id = sqlc.arg(workspace_id)::uuid
    UNION ALL
    SELECT f.id, f.parent_id FROM folders f JOIN chain c ON f.id = c.parent_id
)
SELECT id FROM chain;

-- name: GetResolvedLink :one
-- Redirect resolution. Only versions that need no approval or were approved are eligible.
SELECT q.id, q.workspace_id, q.domain_id, q.campaign_id, q.status, q.safety_status,
       q.starts_at, q.expires_at, q.scan_limit, q.total_scans, q.password_hash, q.fallback_url,
       w.timezone,
       v.id AS version_id, v.version_no, v.destination_kind, v.destination_url, v.rules, v.utm,
       (SELECT min(v2.effective_at) FROM qr_versions v2
         WHERE v2.qr_code_id = q.id AND v2.effective_at > now()
           AND v2.approval_status IN ('not_required','approved')) AS next_change_at
FROM qr_codes q
JOIN workspaces w ON w.id = q.workspace_id
LEFT JOIN LATERAL (
    SELECT * FROM qr_versions v1
    WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
      AND v1.approval_status IN ('not_required','approved')
    ORDER BY v1.effective_at DESC, v1.version_no DESC
    LIMIT 1
) v ON true
WHERE q.domain_id = sqlc.arg(domain_id)::uuid AND q.short_code = sqlc.arg(short_code)::text;

-- name: GS1Linkset :many
-- Conformant-resolver lookup with qualifier inheritance: every item whose qualifiers are
-- NULL or equal to the request's is a candidate; more specific items rank first.
-- The service builds the RFC 9264 linkset from all rows, and picks the default link
-- (or the requested linkType / language) from the most specific item that has one.
SELECT i.id AS item_id, i.qr_code_id, i.title AS item_title,
       (i.cpv IS NOT NULL)::int + (i.lot IS NOT NULL)::int + (i.serial IS NOT NULL)::int AS specificity,
       l.link_type, l.href, l.title, l.hreflang, l.media_type, l.is_default, l.position
FROM gs1_items i
JOIN gs1_links l ON l.item_id = i.id
WHERE i.domain_id = sqlc.arg(domain_id)::uuid
  AND i.gtin = sqlc.arg(gtin)::text
  AND (i.cpv    IS NULL OR i.cpv    = sqlc.narg(cpv)::text)
  AND (i.lot    IS NULL OR i.lot    = sqlc.narg(lot)::text)
  AND (i.serial IS NULL OR i.serial = sqlc.narg(serial)::text)
ORDER BY specificity DESC, l.is_default DESC, l.position, l.link_type;

-- name: ScanSpikeCandidates :many
-- Anomaly detection (every 5 min): last complete 15-min bucket vs the same bucket on each of
-- the previous 7 days. Returns codes whose z-score >= z_min and scans >= min_scans (spike),
-- or scans <= baseline_mean * drop_ratio when the baseline is meaningful (drop).
WITH cur AS (
    SELECT qr_code_id, workspace_id, scans
    FROM scan_stats_15m
    WHERE bucket_start = sqlc.arg(bucket)::timestamptz
      AND (sqlc.narg(workspace_id)::uuid IS NULL OR workspace_id = sqlc.narg(workspace_id)::uuid)
), hist AS (
    SELECT s.qr_code_id, d AS days_ago, COALESCE(s.scans, 0) AS scans
    FROM generate_series(1, 7) d
    CROSS JOIN (SELECT DISTINCT qr_code_id FROM cur) c
    LEFT JOIN scan_stats_15m s
           ON s.qr_code_id = c.qr_code_id
          AND s.bucket_start = sqlc.arg(bucket)::timestamptz - make_interval(days => d)
), base AS (
    SELECT qr_code_id, avg(scans)::float8 AS mean, stddev_pop(scans)::float8 AS sd
    FROM hist GROUP BY qr_code_id
)
SELECT c.qr_code_id, c.workspace_id, c.scans, b.mean, b.sd,
       CASE WHEN b.sd > 0 THEN (c.scans - b.mean) / b.sd ELSE NULL END AS z
FROM cur c JOIN base b USING (qr_code_id)
WHERE c.scans >= sqlc.arg(min_scans)::int
  AND ((b.sd > 0 AND (c.scans - b.mean) / b.sd >= sqlc.arg(z_min)::float8)
       OR (b.sd = 0 AND c.scans >= GREATEST(b.mean * 3, sqlc.arg(min_scans)::int)));

-- name: RecordSerialVerification :one
-- Called synchronously by POST /v1/public/verify/{serial} (issued by the verify page's JS, so
-- link previews don't burn counts). Applies the batch rules atomically and returns the verdict.
-- Scan analytics for the same visit still flow through the normal stream/ingest path.
WITH b AS (
    SELECT sb.rules FROM serial_batches sb JOIN serial_codes sc ON sc.batch_id = sb.id
    WHERE sc.serial = sqlc.arg(serial)::text
)
UPDATE serial_codes sc SET
    scan_count    = sc.scan_count + 1,
    first_scan_at = COALESCE(sc.first_scan_at, sqlc.arg(at)::timestamptz),
    first_country = COALESCE(sc.first_country, sqlc.narg(country)::char(2)),
    first_city    = COALESCE(sc.first_city, sqlc.narg(city)::text),
    last_scan_at  = sqlc.arg(at)::timestamptz,
    countries     = CASE WHEN sqlc.narg(country)::char(2) IS NULL OR sqlc.narg(country)::char(2) = ANY(sc.countries)
                         THEN sc.countries ELSE sc.countries || sqlc.narg(country)::char(2) END,
    status        = CASE
        WHEN sc.status = 'void' THEN 'void'
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'flagged'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'flagged'
        ELSE sc.status END,
    flagged_reason = CASE
        WHEN sc.status IN ('flagged','void') THEN sc.flagged_reason
        WHEN sc.scan_count + 1 > (b.rules->>'max_scans')::int THEN 'max_scans_exceeded'
        WHEN sqlc.narg(country)::char(2) IS NOT NULL AND sc.first_country IS NOT NULL
             AND sqlc.narg(country)::char(2) <> sc.first_country
             AND sqlc.arg(at)::timestamptz - sc.first_scan_at
                 < make_interval(hours => (b.rules->>'multi_country_window_hours')::int) THEN 'multi_country'
        ELSE NULL END
FROM b
WHERE sc.serial = sqlc.arg(serial)::text
RETURNING sc.serial, sc.status, sc.flagged_reason, sc.scan_count, sc.first_scan_at, sc.first_country, sc.first_city;

-- name: PendingApprovalsForApprover :many
-- The approver's inbox: pending requests in workspaces where they hold qr.destination.approve,
-- excluding their own requests and ones they already decided. Single-code requests show the
-- proposed destination; bulk requests show the number of versions they cover.
SELECT ar.id, ar.workspace_id, ar.kind, ar.reasons, ar.requested_by, ar.required_approvals,
       ar.expires_at, ar.created_at, q.name AS qr_name,
       v.destination_url, v.effective_at,
       (SELECT count(*) FROM qr_versions vv WHERE vv.approval_request_id = ar.id) AS version_count
FROM approval_requests ar
LEFT JOIN qr_codes q ON q.id = ar.qr_code_id
LEFT JOIN LATERAL (
    SELECT destination_url, effective_at FROM qr_versions
    WHERE approval_request_id = ar.id ORDER BY version_no DESC LIMIT 1
) v ON ar.kind <> 'bulk_update'
WHERE ar.status = 'pending' AND ar.expires_at > now()
  AND ar.workspace_id = ANY(sqlc.arg(workspace_ids)::uuid[])
  AND ar.requested_by <> sqlc.arg(user_id)::uuid
  AND NOT EXISTS (SELECT 1 FROM approval_decisions d WHERE d.request_id = ar.id AND d.approver_id = sqlc.arg(user_id)::uuid)
ORDER BY ar.created_at;

-- name: FinalizeApproval :exec
-- Run in the decision transaction once approvals >= required_approvals.
-- Approved versions become eligible; a past effective_at is moved to the approval instant so
-- the approved version becomes current rather than slotting in behind newer versions.
WITH req AS (
    UPDATE approval_requests SET status = 'approved', decided_at = now()
    WHERE id = sqlc.arg(request_id)::uuid AND status = 'pending'
    RETURNING id
)
UPDATE qr_versions v SET approval_status = 'approved', effective_at = GREATEST(v.effective_at, now())
FROM req WHERE v.approval_request_id = req.id AND v.approval_status = 'pending';
