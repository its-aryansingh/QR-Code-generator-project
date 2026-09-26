// Package ingest turns the "scans" Redis stream into scan_events rows and rollups.
//
// Every batch runs in one Postgres transaction whose counters are fed only by rows that
// were genuinely inserted (ON CONFLICT DO NOTHING on event_id), so replaying a batch after
// a crash changes nothing: exactly-once effect on top of at-least-once delivery.
package ingest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/realtime"
	"github.com/its-aryansingh/qrit/services/internal/scan"
)

type Config struct {
	Stream        string
	Group         string
	Consumer      string
	DLQ           string
	BatchSize     int64
	BlockWait     time.Duration
	DedupeWindow  time.Duration
	VelocityLimit int64
	ClaimIdle     time.Duration
	MaxDeliveries int64
}

func (c *Config) defaults() {
	if c.Stream == "" {
		c.Stream = "scans"
	}
	if c.Group == "" {
		c.Group = "ingest"
	}
	if c.DLQ == "" {
		c.DLQ = c.Stream + ":dlq"
	}
	if c.Consumer == "" {
		h, _ := os.Hostname()
		c.Consumer = "ingest-" + h
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 1000
	}
	if c.BlockWait <= 0 {
		c.BlockWait = time.Second
	}
	if c.DedupeWindow <= 0 {
		c.DedupeWindow = 10 * time.Second
	}
	if c.VelocityLimit <= 0 {
		c.VelocityLimit = 30
	}
	if c.ClaimIdle <= 0 {
		c.ClaimIdle = 60 * time.Second
	}
	if c.MaxDeliveries <= 0 {
		c.MaxDeliveries = 5
	}
}

// Row is one enriched event, ready for COPY into scan_events.
type Row struct {
	EventID        uuid.UUID
	OccurredAt     time.Time
	WorkspaceID    uuid.UUID
	QRCodeID       uuid.UUID
	VersionID      *uuid.UUID
	CampaignID     *uuid.UUID
	DomainID       uuid.UUID
	RuleID         *string
	Outcome        string
	Method         string
	IsBot          bool
	BotReason      *string
	IsDuplicate    bool
	VisitorHash    []byte
	DeviceType     *string
	OS             *string
	OSVersion      *string
	Browser        *string
	BrowserVersion *string
	Country        *string
	Region         *string
	City           *string
	Language       *string
	ReferrerHost   *string
	UTMSource      *string
	UTMMedium      *string
	UTMCampaign    *string
}

// Counted mirrors the SQL definition of a scan.
func (r Row) Counted() bool {
	return !r.IsBot && !r.IsDuplicate && (r.Outcome == "redirect" || r.Outcome == "hosted_page" || r.Outcome == "password_ok")
}

// AfterCommit runs once a batch is durable (realtime counters, webhook fan-out).
type AfterCommit func(ctx context.Context, rows []Row)

type Consumer struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	rt     *realtime.RealtimeService
	cfg    Config
	hooks  []AfterCommit
	now    func() time.Time
	logger *slog.Logger
}

func NewConsumer(pool *pgxpool.Pool, rdb *redis.Client, cfg Config, logger *slog.Logger) *Consumer {
	cfg.defaults()
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{pool: pool, rdb: rdb, rt: realtime.New(rdb), cfg: cfg, now: time.Now, logger: logger}
}

// OnCommit registers a post-commit hook.
func (c *Consumer) OnCommit(h AfterCommit) { c.hooks = append(c.hooks, h) }

// EnsureGroup creates the consumer group (and stream) if missing.
func (c *Consumer) EnsureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.cfg.Stream, c.cfg.Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Run consumes until ctx is cancelled. Pending messages of crashed consumers are reclaimed
// every 30 s; messages delivered more than MaxDeliveries times go to the DLQ.
func (c *Consumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		return err
	}
	lastClaim := time.Time{}
	for ctx.Err() == nil {
		if time.Since(lastClaim) > 30*time.Second {
			if err := c.Reclaim(ctx); err != nil && ctx.Err() == nil {
				c.logger.Warn("reclaim pending scans", "error", err)
			}
			lastClaim = time.Now()
		}
		streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.cfg.Group, Consumer: c.cfg.Consumer, Streams: []string{c.cfg.Stream, ">"},
			Count: c.cfg.BatchSize, Block: c.cfg.BlockWait,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			if strings.Contains(err.Error(), "NOGROUP") {
				_ = c.EnsureGroup(ctx)
				continue
			}
			c.logger.Warn("read scans", "error", err)
			time.Sleep(time.Second)
			continue
		}
		for _, st := range streams {
			if len(st.Messages) == 0 {
				continue
			}
			if _, err := c.Process(ctx, st.Messages); err != nil {
				// Not acknowledged: the messages stay pending and are retried by Reclaim.
				c.logger.Error("ingest batch failed", "count", len(st.Messages), "error", err)
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
	return nil
}

// Result summarises one processed batch.
type Result struct {
	Inserted int
	DLQ      int
}

// Process validates, enriches and stores messages, then acknowledges them.
func (c *Consumer) Process(ctx context.Context, msgs []redis.XMessage) (Result, error) {
	var res Result
	rows := make([]Row, 0, len(msgs))
	var ack []string
	var dead []redis.XMessage
	var reasons []string
	now := c.now().UTC()
	for _, m := range msgs {
		row, reason := c.decode(m, now)
		if reason != "" {
			dead = append(dead, m)
			reasons = append(reasons, reason)
			continue
		}
		rows = append(rows, row)
		ack = append(ack, m.ID)
	}
	if err := c.enrich(ctx, rows); err != nil {
		return res, fmt.Errorf("enrich: %w", err)
	}
	if len(rows) > 0 {
		n, err := WriteBatch(ctx, c.pool, rows)
		if err != nil {
			return res, err
		}
		res.Inserted = n
	}
	for i, m := range dead {
		c.toDLQ(ctx, m, reasons[i])
		ack = append(ack, m.ID)
	}
	res.DLQ = len(dead)
	if len(ack) > 0 {
		if err := c.rdb.XAck(ctx, c.cfg.Stream, c.cfg.Group, ack...).Err(); err != nil {
			c.logger.Warn("xack", "error", err)
		}
	}
	for _, r := range rows {
		if r.Counted() {
			_ = c.rt.RecordScan(ctx, r.WorkspaceID.String(), r.QRCodeID.String(), r.OccurredAt)
		}
	}
	for _, h := range c.hooks {
		h(ctx, rows)
	}
	return res, nil
}

func (c *Consumer) toDLQ(ctx context.Context, m redis.XMessage, reason string) {
	vals := []any{"reason", reason, "source_id", m.ID}
	for k, v := range m.Values {
		vals = append(vals, k, v)
	}
	if err := c.rdb.XAdd(ctx, &redis.XAddArgs{Stream: c.cfg.DLQ, MaxLen: 1_000_000, Approx: true, Values: vals}).Err(); err != nil {
		c.logger.Error("dlq write failed", "error", err)
	}
}

var countedOutcomes = map[string]bool{"redirect": true, "hosted_page": true, "password_prompt": true, "password_ok": true,
	"password_fail": true, "geo_blocked": true, "paused": true, "expired": true, "not_started": true,
	"limit_reached": true, "blocked": true}

func optUUID(s *string) (*uuid.UUID, bool) {
	if s == nil || *s == "" {
		return nil, true
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, false
	}
	return &id, true
}

func optStr(s *string, max int) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	if r := []rune(v); len(r) > max {
		v = string(r[:max])
	}
	return &v
}

func str(s string, max int) *string { return optStr(&s, max) }

// decode parses and validates one stream message; a non-empty reason sends it to the DLQ.
func (c *Consumer) decode(m redis.XMessage, now time.Time) (Row, string) {
	raw, ok := m.Values["e"].(string)
	if !ok {
		raw, ok = m.Values["data"].(string) // pre-v2 producers
	}
	if !ok {
		return Row{}, "missing_payload"
	}
	var ev scan.ScanEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return Row{}, "invalid_json"
	}
	var r Row
	var err error
	if r.EventID, err = uuid.Parse(ev.ID); err != nil {
		return r, "invalid_id"
	}
	if r.WorkspaceID, err = uuid.Parse(ev.WorkspaceID); err != nil {
		return r, "invalid_workspace"
	}
	if r.QRCodeID, err = uuid.Parse(ev.QRCodeID); err != nil {
		return r, "invalid_qr"
	}
	if r.DomainID, err = uuid.Parse(ev.DomainID); err != nil {
		return r, "invalid_domain"
	}
	if r.VersionID, ok = optUUID(ev.VersionID); !ok {
		return r, "invalid_version"
	}
	if r.CampaignID, ok = optUUID(ev.CampaignID); !ok {
		return r, "invalid_campaign"
	}
	if ev.Timestamp.IsZero() || ev.Timestamp.After(now.Add(5*time.Minute)) || ev.Timestamp.Before(now.Add(-7*24*time.Hour)) {
		return r, "timestamp_out_of_window"
	}
	if !countedOutcomes[ev.Outcome] {
		return r, "invalid_outcome"
	}
	vh, err := base64.StdEncoding.DecodeString(ev.VisitorHash)
	if err != nil || len(vh) != 16 {
		return r, "invalid_visitor_hash"
	}
	r.OccurredAt, r.Outcome, r.VisitorHash = ev.Timestamp.UTC(), ev.Outcome, vh
	r.Method = strings.ToUpper(ev.Method)
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "POST" {
		r.Method = "GET"
	}
	r.RuleID = optStr(ev.Rule, 100)
	if cc := strings.ToUpper(strings.TrimSpace(ev.Geo.Country)); len(cc) == 2 {
		r.Country = &cc
	}
	r.Region, r.City = str(ev.Geo.Region, 64), str(ev.Geo.City, 80)
	r.Language = optStr(ev.Language, 16)
	r.ReferrerHost = optStr(ev.Referrer, 253)
	if ev.UTM != nil {
		r.UTMSource, r.UTMMedium, r.UTMCampaign = optStr(ev.UTM.Source, 200), optStr(ev.UTM.Medium, 200), optStr(ev.UTM.Campaign, 200)
	}
	ua := scan.ParseUserAgent(ev.UserAgent)
	device := ua.DeviceType
	switch device {
	case "mobile", "tablet", "desktop":
	default:
		device = "other"
	}
	r.DeviceType = &device
	r.OS, r.OSVersion = str(ua.OS, 40), str(ua.OSVersion, 20)
	r.Browser, r.BrowserVersion = str(ua.Browser, 40), str(ua.BrowserVersion, 20)
	v := scan.ClassifyBot(r.Method, ev.UserAgent, ev.Datacenter && (device == "desktop" || strings.Contains(strings.ToLower(ev.UserAgent), "headless")))
	if v.IsBot {
		r.IsBot, r.BotReason = true, str(v.Reason, 40)
	}
	return r, ""
}

// dedupeScript marks a repeat of the same visitor on the same code within the window as a
// duplicate (a re-delivered copy of the same event is not), and counts velocity.
// ARGV: event id, dedupe window ms, velocity window ms, "1" when the event can count.
var dedupeScript = redis.NewScript(`
local dup = 0
if ARGV[4] == '1' then
  local ok = redis.call('SET', KEYS[1], ARGV[1], 'NX', 'PX', ARGV[2])
  if not ok then
    local cur = redis.call('GET', KEYS[1])
    if cur and cur ~= ARGV[1] then dup = 1 end
  end
end
local n = redis.call('INCR', KEYS[2])
if n == 1 then redis.call('PEXPIRE', KEYS[2], ARGV[3]) end
return {dup, n}`)

func (c *Consumer) enrich(ctx context.Context, rows []Row) error {
	if len(rows) == 0 || c.rdb == nil {
		return nil
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*redis.Cmd, len(rows))
	for i, r := range rows {
		vh := base64.RawStdEncoding.EncodeToString(r.VisitorHash)
		cmds[i] = dedupeScript.Run(ctx, pipe, []string{"dup:" + r.QRCodeID.String() + ":" + vh, "v:" + r.QRCodeID.String() + ":" + vh},
			r.EventID.String(), c.cfg.DedupeWindow.Milliseconds(), int64(60000), dedupeFlag(r))
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		// NOSCRIPT on first use inside a pipeline: load and retry once.
		if strings.Contains(err.Error(), "NOSCRIPT") {
			if err := dedupeScript.Load(ctx, c.rdb).Err(); err != nil {
				return err
			}
			return c.enrich(ctx, rows)
		}
		return err
	}
	for i, cmd := range cmds {
		vals, err := cmd.Slice()
		if err != nil || len(vals) != 2 {
			continue
		}
		if d, _ := vals[0].(int64); d == 1 {
			rows[i].IsDuplicate = true
		}
		if n, _ := vals[1].(int64); n > c.cfg.VelocityLimit && !rows[i].IsBot {
			rows[i].IsBot, rows[i].BotReason = true, str("velocity", 40)
		}
	}
	return nil
}

// dedupeFlag limits duplicate detection to requests that would count as a scan: a password
// prompt followed by the correct password moments later is one scan, not a duplicate.
func dedupeFlag(r Row) string {
	if !r.IsBot && (r.Outcome == "redirect" || r.Outcome == "hosted_page" || r.Outcome == "password_ok") {
		return "1"
	}
	return "0"
}

var copyColumns = []string{"event_id", "occurred_at", "workspace_id", "qr_code_id", "version_id", "campaign_id",
	"domain_id", "rule_id", "outcome", "method", "is_bot", "bot_reason", "is_duplicate", "is_unique", "visitor_hash",
	"device_type", "os", "os_version", "browser", "browser_version", "country", "region", "city", "language",
	"referrer_host", "utm_source", "utm_medium", "utm_campaign"}

// batchSQL is the verified ingest transaction (docs/v2-blueprint/ingest.sql), minus COPY.
var batchSQL = []string{
	`CREATE TEMP TABLE new_ev (event_id uuid, occurred_at timestamptz) ON COMMIT DROP`,
	`WITH ins AS (
	    INSERT INTO scan_events SELECT * FROM staging
	    ON CONFLICT (event_id, occurred_at) DO NOTHING
	    RETURNING event_id, occurred_at
	) INSERT INTO new_ev SELECT event_id, occurred_at FROM ins`,
	`WITH cand AS (
	    SELECT DISTINCT ON (s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash)
	           s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date AS day,
	           s.visitor_hash, s.occurred_at, s.event_id
	    FROM staging s JOIN new_ev n USING (event_id, occurred_at)
	    WHERE NOT s.is_bot AND NOT s.is_duplicate AND s.outcome IN ('redirect','hosted_page','password_ok')
	    ORDER BY s.qr_code_id, (s.occurred_at AT TIME ZONE 'UTC')::date, s.visitor_hash, s.occurred_at
	), firsts AS (
	    INSERT INTO scan_visitors_daily (qr_code_id, day, visitor_hash, first_seen_at)
	    SELECT qr_code_id, day, visitor_hash, occurred_at FROM cand
	    ON CONFLICT DO NOTHING
	    RETURNING qr_code_id, day, visitor_hash
	)
	UPDATE scan_events e SET is_unique = true
	FROM cand c JOIN firsts f USING (qr_code_id, day, visitor_hash)
	WHERE e.event_id = c.event_id AND e.occurred_at = c.occurred_at`,
	`CREATE TEMP TABLE batch ON COMMIT DROP AS
	SELECT e.*,
	       (NOT e.is_bot AND NOT e.is_duplicate AND e.outcome IN ('redirect','hosted_page','password_ok')) AS counted,
	       (NOT e.is_bot AND e.outcome IN ('geo_blocked','paused','expired','not_started','limit_reached','blocked')) AS blocked
	FROM scan_events e JOIN new_ev n USING (event_id, occurred_at)`,
	rollup15mSQL("batch"),
	rollupDimSQL("batch"),
	`UPDATE qr_codes q SET
	       total_scans     = q.total_scans  + a.scans,
	       unique_scans    = q.unique_scans + a.uniques,
	       last_scanned_at = GREATEST(COALESCE(q.last_scanned_at, a.last_at), a.last_at)
	FROM (
	    SELECT qr_code_id, count(*) FILTER (WHERE counted) AS scans,
	           count(*) FILTER (WHERE counted AND is_unique) AS uniques,
	           max(occurred_at) FILTER (WHERE counted) AS last_at
	    FROM batch GROUP BY qr_code_id
	) a
	WHERE q.id = a.qr_code_id AND a.scans > 0`,
}

func rollup15mSQL(src string) string {
	return `INSERT INTO scan_stats_15m AS h
	       (qr_code_id, bucket_start, workspace_id, campaign_id, scans, unique_scans, bot_hits, blocked_hits)
	SELECT qr_code_id, date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'), workspace_id,
	       (array_agg(campaign_id ORDER BY occurred_at DESC))[1],
	       count(*) FILTER (WHERE counted), count(*) FILTER (WHERE counted AND is_unique),
	       count(*) FILTER (WHERE is_bot), count(*) FILTER (WHERE blocked)
	FROM ` + src + `
	GROUP BY qr_code_id, date_bin('15 minutes', occurred_at, '2000-01-01 00:00+00'), workspace_id
	ON CONFLICT (qr_code_id, bucket_start) DO UPDATE SET
	       scans        = h.scans        + EXCLUDED.scans,
	       unique_scans = h.unique_scans + EXCLUDED.unique_scans,
	       bot_hits     = h.bot_hits     + EXCLUDED.bot_hits,
	       blocked_hits = h.blocked_hits + EXCLUDED.blocked_hits,
	       campaign_id  = COALESCE(EXCLUDED.campaign_id, h.campaign_id)`
}

func rollupDimSQL(src string) string {
	return `INSERT INTO scan_stats_daily_dim AS d
	       (qr_code_id, day, dimension, key, workspace_id, campaign_id, scans, unique_scans)
	SELECT b.qr_code_id, (b.occurred_at AT TIME ZONE 'UTC')::date, x.dimension, x.key, b.workspace_id,
	       (array_agg(b.campaign_id ORDER BY b.occurred_at DESC))[1], count(*), count(*) FILTER (WHERE b.is_unique)
	FROM ` + src + ` b
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
	       unique_scans = d.unique_scans + EXCLUDED.unique_scans`
}

// WriteBatch stores rows and updates rollups atomically; returns how many were new.
func WriteBatch(ctx context.Context, pool *pgxpool.Pool, rows []Row) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL TimeZone = 'UTC'`); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `CREATE TEMP TABLE staging (LIKE scan_events INCLUDING DEFAULTS) ON COMMIT DROP`); err != nil {
		return 0, err
	}
	src := make([][]any, len(rows))
	for i, r := range rows {
		src[i] = []any{r.EventID, r.OccurredAt, r.WorkspaceID, r.QRCodeID, r.VersionID, r.CampaignID, r.DomainID, r.RuleID,
			r.Outcome, r.Method, r.IsBot, r.BotReason, r.IsDuplicate, false, r.VisitorHash, r.DeviceType, r.OS, r.OSVersion,
			r.Browser, r.BrowserVersion, r.Country, r.Region, r.City, r.Language, r.ReferrerHost, r.UTMSource, r.UTMMedium, r.UTMCampaign}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"staging"}, copyColumns, pgx.CopyFromRows(src)); err != nil {
		return 0, fmt.Errorf("copy staging: %w", err)
	}
	for _, q := range batchSQL {
		if _, err := tx.Exec(ctx, q); err != nil {
			return 0, fmt.Errorf("batch step: %w", err)
		}
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM new_ev`).Scan(&n); err != nil {
		return 0, err
	}
	return n, tx.Commit(ctx)
}

// Reclaim takes over messages another consumer read but never acknowledged.
func (c *Consumer) Reclaim(ctx context.Context) error {
	pend, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.cfg.Stream, Group: c.cfg.Group, Idle: c.cfg.ClaimIdle, Start: "-", End: "+", Count: c.cfg.BatchSize,
	}).Result()
	if err != nil || len(pend) == 0 {
		return err
	}
	var retry, dead []string
	for _, p := range pend {
		if p.RetryCount > c.cfg.MaxDeliveries {
			dead = append(dead, p.ID)
		} else {
			retry = append(retry, p.ID)
		}
	}
	if len(dead) > 0 {
		msgs, err := c.rdb.XClaim(ctx, &redis.XClaimArgs{Stream: c.cfg.Stream, Group: c.cfg.Group,
			Consumer: c.cfg.Consumer, MinIdle: c.cfg.ClaimIdle, Messages: dead}).Result()
		if err == nil {
			for _, m := range msgs {
				c.toDLQ(ctx, m, "max_deliveries")
			}
			_ = c.rdb.XAck(ctx, c.cfg.Stream, c.cfg.Group, dead...).Err()
		}
	}
	if len(retry) == 0 {
		return nil
	}
	msgs, err := c.rdb.XClaim(ctx, &redis.XClaimArgs{Stream: c.cfg.Stream, Group: c.cfg.Group,
		Consumer: c.cfg.Consumer, MinIdle: c.cfg.ClaimIdle, Messages: retry}).Result()
	if err != nil || len(msgs) == 0 {
		return err
	}
	_, err = c.Process(ctx, msgs)
	return err
}

// Rebuild recomputes one UTC day's rollups from scan_events and re-derives the affected
// codes' denormalised totals. Safe to run while ingest is live (row locks serialise it).
func Rebuild(ctx context.Context, pool *pgxpool.Pool, day time.Time) error {
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	steps := []struct {
		sql  string
		args []any
	}{
		{`SET LOCAL TimeZone = 'UTC'`, nil},
		{`CREATE TEMP TABLE batch ON COMMIT DROP AS
		  SELECT e.*,
		         (NOT e.is_bot AND NOT e.is_duplicate AND e.outcome IN ('redirect','hosted_page','password_ok')) AS counted,
		         (NOT e.is_bot AND e.outcome IN ('geo_blocked','paused','expired','not_started','limit_reached','blocked')) AS blocked
		  FROM scan_events e WHERE e.occurred_at >= $1 AND e.occurred_at < $2`, []any{from, to}},
		{`CREATE TEMP TABLE affected ON COMMIT DROP AS
		  SELECT qr_code_id FROM scan_stats_15m WHERE bucket_start >= $1 AND bucket_start < $2
		  UNION SELECT qr_code_id FROM batch`, []any{from, to}},
		{`DELETE FROM scan_stats_15m WHERE bucket_start >= $1 AND bucket_start < $2`, []any{from, to}},
		{`DELETE FROM scan_stats_daily_dim WHERE day = $1::date`, []any{from}},
		{rollup15mSQL("batch"), nil},
		{rollupDimSQL("batch"), nil},
		{`UPDATE qr_codes q SET total_scans = COALESCE(t.scans, 0), unique_scans = COALESCE(t.uniques, 0)
		  FROM affected a LEFT JOIN (
		      SELECT qr_code_id, sum(scans)::bigint AS scans, sum(unique_scans)::bigint AS uniques
		      FROM scan_stats_15m WHERE qr_code_id IN (SELECT qr_code_id FROM affected) GROUP BY qr_code_id
		  ) t USING (qr_code_id)
		  WHERE q.id = a.qr_code_id`, nil},
	}
	for _, st := range steps {
		if _, err := tx.Exec(ctx, st.sql, st.args...); err != nil {
			return fmt.Errorf("rebuild: %w", err)
		}
	}
	return tx.Commit(ctx)
}
