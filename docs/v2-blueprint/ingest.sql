-- =====================================================================
-- Ingest batch transaction (executed by cmd/ingest for every batch of
-- up to 1,000 stream messages). Exactly-once *effect*: replaying the same
-- batch changes nothing, because only rows that were actually inserted
-- into scan_events (captured in new_ev) feed the counters.
-- =====================================================================
BEGIN;
SET LOCAL TimeZone = 'UTC';

CREATE TEMP TABLE staging (LIKE scan_events INCLUDING DEFAULTS) ON COMMIT DROP;
-- Go: pgx.CopyFrom(ctx, pgx.Identifier{"staging"}, columns, rows)

CREATE TEMP TABLE new_ev (event_id uuid, occurred_at timestamptz) ON COMMIT DROP;

-- 1. Insert raw events; remember which ones are genuinely new.
WITH ins AS (
    INSERT INTO scan_events SELECT * FROM staging
    ON CONFLICT (event_id, occurred_at) DO NOTHING
    RETURNING event_id, occurred_at
)
INSERT INTO new_ev SELECT event_id, occurred_at FROM ins;

-- 2. Unique-visitor detection (per QR, per UTC day) for new counted events.
WITH cand AS (
    SELECT DISTINCT ON (s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash)
           s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date AS day,
           s.visitor_hash, s.occurred_at, s.event_id
    FROM staging s
    JOIN new_ev n USING (event_id, occurred_at)
    WHERE NOT s.is_bot AND NOT s.is_duplicate
      AND s.outcome IN ('redirect','hosted_page','password_ok')
    ORDER BY s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash, s.occurred_at
), firsts AS (
    INSERT INTO scan_visitors_daily (qr_code_id, day, visitor_hash, first_seen_at)
    SELECT qr_code_id, day, visitor_hash, occurred_at FROM cand
    ON CONFLICT DO NOTHING
    RETURNING qr_code_id, day, visitor_hash
)
UPDATE scan_events e SET is_unique = true
FROM cand c JOIN firsts f USING (qr_code_id, day, visitor_hash)
WHERE e.event_id = c.event_id AND e.occurred_at = c.occurred_at;

-- Working set with the "counted" flag resolved once.
CREATE TEMP TABLE batch ON COMMIT DROP AS
SELECT e.*,
       (NOT e.is_bot AND NOT e.is_duplicate
        AND e.outcome IN ('redirect','hosted_page','password_ok'))            AS counted,
       (NOT e.is_bot
        AND e.outcome IN ('geo_blocked','paused','expired','not_started',
                          'limit_reached','blocked'))                          AS blocked
FROM scan_events e
JOIN new_ev n USING (event_id, occurred_at);

-- 3. 15-minute rollup.
INSERT INTO scan_stats_15m AS h
       (qr_code_id, bucket_start, workspace_id, campaign_id, scans, unique_scans, bot_hits, blocked_hits)
SELECT qr_code_id,
       date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'),
       workspace_id,
       (array_agg(campaign_id ORDER BY occurred_at DESC))[1],
       count(*) FILTER (WHERE counted),
       count(*) FILTER (WHERE counted AND is_unique),
       count(*) FILTER (WHERE is_bot),
       count(*) FILTER (WHERE blocked)
FROM batch
GROUP BY qr_code_id, date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'), workspace_id
ON CONFLICT (qr_code_id, bucket_start) DO UPDATE SET
       scans        = h.scans        + EXCLUDED.scans,
       unique_scans = h.unique_scans + EXCLUDED.unique_scans,
       bot_hits     = h.bot_hits     + EXCLUDED.bot_hits,
       blocked_hits = h.blocked_hits + EXCLUDED.blocked_hits,
       campaign_id  = COALESCE(EXCLUDED.campaign_id, h.campaign_id);

-- 4. Daily dimension rollup (counted scans only).
INSERT INTO scan_stats_daily_dim AS d
       (qr_code_id, day, dimension, key, workspace_id, campaign_id, scans, unique_scans)
SELECT b.qr_code_id,
       (b.occurred_at AT TIME ZONE 'UTC')::date,
       x.dimension,
       x.key,
       b.workspace_id,
       (array_agg(b.campaign_id ORDER BY b.occurred_at DESC))[1],
       count(*),
       count(*) FILTER (WHERE b.is_unique)
FROM batch b
CROSS JOIN LATERAL (VALUES
    ('country',    COALESCE(b.country, 'Unknown')),
    ('region',     COALESCE(b.country || '/' || b.region, 'Unknown')),
    ('city',       COALESCE(b.country || '/' || b.city, 'Unknown')),
    ('device',     COALESCE(b.device_type, 'other')),
    ('os',         COALESCE(b.os, 'Unknown')),
    ('browser',    COALESCE(b.browser, 'Unknown')),
    ('language',   COALESCE(b.language, 'Unknown')),
    ('referrer',   COALESCE(b.referrer_host, 'Direct')),
    ('rule',       COALESCE(b.rule_id, 'default')),
    ('version',    COALESCE(b.version_id::text, 'Unknown')),
    ('utm_source', COALESCE(b.utm_source, 'None'))
) AS x(dimension, key)
WHERE b.counted
GROUP BY b.qr_code_id, (b.occurred_at AT TIME ZONE 'UTC')::date, x.dimension, x.key, b.workspace_id
ON CONFLICT (qr_code_id, day, dimension, key) DO UPDATE SET
       scans        = d.scans        + EXCLUDED.scans,
       unique_scans = d.unique_scans + EXCLUDED.unique_scans;

-- 5. Denormalised counters on qr_codes (list view + scan_limit enforcement).
UPDATE qr_codes q SET
       total_scans     = q.total_scans  + a.scans,
       unique_scans    = q.unique_scans + a.uniques,
       last_scanned_at = GREATEST(COALESCE(q.last_scanned_at, a.last_at), a.last_at)
FROM (
    SELECT qr_code_id,
           count(*) FILTER (WHERE counted)               AS scans,
           count(*) FILTER (WHERE counted AND is_unique) AS uniques,
           max(occurred_at) FILTER (WHERE counted)       AS last_at
    FROM batch GROUP BY qr_code_id
) a
WHERE q.id = a.qr_code_id AND a.scans > 0;

COMMIT;
-- After COMMIT: XACK every message id in the batch.
