-- =====================================================================
-- Analytics read queries (sqlc, internal/analytics/queries.sql).
-- Parameters: $ws workspace id, $from/$to timestamptz (half-open [from,to)),
-- $tz IANA zone, $qr uuid[] (NULL = all), $campaign uuid (NULL = all).
-- All ranges are computed in the viewer's timezone by the API and passed as
-- UTC instants aligned to local midnight.
-- =====================================================================

-- name: SummaryTotals :one
SELECT COALESCE(sum(scans),0)::bigint        AS scans,
       COALESCE(sum(unique_scans),0)::bigint AS unique_scans,
       COALESCE(sum(bot_hits),0)::bigint     AS bot_hits,
       COALESCE(sum(blocked_hits),0)::bigint AS blocked_hits
FROM scan_stats_15m
WHERE workspace_id = sqlc.arg(ws)
  AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
  AND (sqlc.narg(campaign)::uuid IS NULL OR campaign_id = sqlc.narg(campaign)::uuid);

-- name: TimeseriesDaily :many
-- Zero-filled local-day series.
WITH series AS (
    SELECT generate_series(
             (sqlc.arg(from_ts)::timestamptz AT TIME ZONE sqlc.arg(tz)::text)::date,
             ((sqlc.arg(to_ts)::timestamptz - interval '1 microsecond') AT TIME ZONE sqlc.arg(tz)::text)::date,
             interval '1 day')::date AS day
), agg AS (
    SELECT (bucket_start AT TIME ZONE sqlc.arg(tz)::text)::date AS day,
           sum(scans) AS scans, sum(unique_scans) AS unique_scans
    FROM scan_stats_15m
    WHERE workspace_id = sqlc.arg(ws)
      AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
      AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
      AND (sqlc.narg(campaign)::uuid IS NULL OR campaign_id = sqlc.narg(campaign)::uuid)
    GROUP BY 1
)
SELECT s.day, COALESCE(a.scans,0)::bigint AS scans, COALESCE(a.unique_scans,0)::bigint AS unique_scans
FROM series s LEFT JOIN agg a USING (day)
ORDER BY s.day;

-- name: Heatmap :many
-- Weekday (1=Mon..7=Sun) x local hour-of-day.
SELECT extract(isodow FROM bucket_start AT TIME ZONE sqlc.arg(tz)::text)::int AS weekday,
       extract(hour   FROM bucket_start AT TIME ZONE sqlc.arg(tz)::text)::int AS hour_of_day,
       sum(scans)::bigint AS scans
FROM scan_stats_15m
WHERE workspace_id = sqlc.arg(ws)
  AND bucket_start >= sqlc.arg(from_ts) AND bucket_start < sqlc.arg(to_ts)
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
GROUP BY 1, 2;

-- name: BreakdownRollup :many
-- Ranges >= 3 days: UTC-day dimension rollups (edge days approximate by < 1 day of data).
SELECT key, sum(scans)::bigint AS scans, sum(unique_scans)::bigint AS unique_scans
FROM scan_stats_daily_dim
WHERE workspace_id = sqlc.arg(ws)
  AND dimension = sqlc.arg(dimension)
  AND day >= (sqlc.arg(from_ts)::timestamptz AT TIME ZONE 'UTC')::date
  AND day <= ((sqlc.arg(to_ts)::timestamptz - interval '1 microsecond') AT TIME ZONE 'UTC')::date
  AND (sqlc.narg(qr_ids)::uuid[] IS NULL OR qr_code_id = ANY(sqlc.narg(qr_ids)::uuid[]))
GROUP BY key
ORDER BY scans DESC
LIMIT 20;

-- name: TopQRCodes :many
SELECT q.id, q.name, q.short_code,
       COALESCE(sum(s.scans),0)::bigint AS scans,
       COALESCE(sum(s.unique_scans),0)::bigint AS unique_scans
FROM qr_codes q
LEFT JOIN scan_stats_15m s ON s.qr_code_id = q.id
  AND s.bucket_start >= sqlc.arg(from_ts) AND s.bucket_start < sqlc.arg(to_ts)
WHERE q.workspace_id = sqlc.arg(ws)
GROUP BY q.id, q.name, q.short_code
ORDER BY scans DESC
LIMIT 10;
