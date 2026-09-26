// Package analytics answers dashboard questions from the scan rollups.
//
// Ranges are half-open [From, To) UTC instants that the API aligns to local midnights in
// the viewer's timezone. Charts read scan_stats_15m (15-minute buckets are exact in every
// timezone, including +05:30 and +05:45). Breakdowns of short ranges (< 3 days) read raw
// events for exactness; longer ranges read the daily dimension rollup.
package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Filter scopes every query.
type Filter struct {
	WorkspaceID uuid.UUID
	From, To    time.Time
	Loc         *time.Location
	// QRIDs restricts to these codes; nil means every code, an empty non-nil slice means none.
	QRIDs      []uuid.UUID
	CampaignID *uuid.UUID
}

func (f Filter) tz() string {
	if f.Loc == nil {
		return "UTC"
	}
	return f.Loc.String()
}

func (f Filter) loc() *time.Location {
	if f.Loc == nil {
		return time.UTC
	}
	return f.Loc
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// where builds the common predicate for rollup tables. timeCol is the bucket column.
func (f Filter) where(timeCol string, args *[]any) string {
	*args = append(*args, f.WorkspaceID, f.From, f.To)
	parts := []string{"workspace_id = $1", timeCol + " >= $2", timeCol + " < $3"}
	if f.QRIDs != nil {
		*args = append(*args, f.QRIDs)
		parts = append(parts, fmt.Sprintf("qr_code_id = ANY($%d)", len(*args)))
	}
	if f.CampaignID != nil {
		*args = append(*args, *f.CampaignID)
		parts = append(parts, fmt.Sprintf("campaign_id = $%d", len(*args)))
	}
	return strings.Join(parts, " AND ")
}

type Totals struct {
	Scans       int64 `json:"scans"`
	UniqueScans int64 `json:"unique_scans"`
	BotHits     int64 `json:"bot_hits"`
	BlockedHits int64 `json:"blocked_hits"`
}

func (s *Service) Totals(ctx context.Context, f Filter) (Totals, error) {
	var args []any
	w := f.where("bucket_start", &args)
	var t Totals
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(sum(scans),0), COALESCE(sum(unique_scans),0),
		COALESCE(sum(bot_hits),0), COALESCE(sum(blocked_hits),0) FROM scan_stats_15m WHERE `+w, args...).
		Scan(&t.Scans, &t.UniqueScans, &t.BotHits, &t.BlockedHits)
	return t, err
}

type Summary struct {
	Totals
	Previous Totals             `json:"previous"`
	Deltas   map[string]*float64 `json:"deltas"`
}

func delta(cur, prev int64) *float64 {
	if prev == 0 {
		return nil
	}
	v := float64(int64((float64(cur-prev)/float64(prev))*1000)) / 10
	return &v
}

// Summary returns totals for the range and for the preceding period of equal length.
func (s *Service) Summary(ctx context.Context, f Filter) (Summary, error) {
	cur, err := s.Totals(ctx, f)
	if err != nil {
		return Summary{}, err
	}
	p := f
	p.From, p.To = f.From.Add(-f.To.Sub(f.From)), f.From
	prev, err := s.Totals(ctx, p)
	if err != nil {
		return Summary{}, err
	}
	return Summary{Totals: cur, Previous: prev, Deltas: map[string]*float64{
		"scans": delta(cur.Scans, prev.Scans), "unique_scans": delta(cur.UniqueScans, prev.UniqueScans),
	}}, nil
}

// Granularity picks the chart bucket from the range length.
func Granularity(from, to time.Time) string {
	switch d := to.Sub(from); {
	case d <= 25*time.Hour:
		return "15m"
	case d <= 8*24*time.Hour:
		return "hour"
	case d <= 181*24*time.Hour:
		return "day"
	default:
		return "week"
	}
}

type Point struct {
	T           time.Time `json:"t"`
	Scans       int64     `json:"scans"`
	UniqueScans int64     `json:"unique_scans"`
}

// Timeseries returns a zero-filled series in local time.
func (s *Service) Timeseries(ctx context.Context, f Filter, gran string) ([]Point, error) {
	var bucket string
	switch gran {
	case "15m":
		bucket = "date_bin('15 minutes', bucket_start AT TIME ZONE %s, '2000-01-01')"
	case "hour":
		bucket = "date_trunc('hour', bucket_start AT TIME ZONE %s)"
	case "day":
		bucket = "date_trunc('day', bucket_start AT TIME ZONE %s)"
	case "week":
		bucket = "date_trunc('week', bucket_start AT TIME ZONE %s)"
	default:
		return nil, fmt.Errorf("unknown granularity %q", gran)
	}
	var args []any
	w := f.where("bucket_start", &args)
	args = append(args, f.tz())
	expr := fmt.Sprintf(bucket, fmt.Sprintf("$%d", len(args)))
	rows, err := s.pool.Query(ctx, `SELECT `+expr+` AS b, sum(scans), sum(unique_scans)
		FROM scan_stats_15m WHERE `+w+` GROUP BY 1 ORDER BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	loc := f.loc()
	got := map[time.Time]Point{}
	for rows.Next() {
		var b time.Time
		var p Point
		if err := rows.Scan(&b, &p.Scans, &p.UniqueScans); err != nil {
			return nil, err
		}
		// b is a local wall-clock time without zone.
		key := time.Date(b.Year(), b.Month(), b.Day(), b.Hour(), b.Minute(), 0, 0, loc)
		p.T = key
		got[key.UTC()] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	start := f.From.In(loc)
	switch gran {
	case "15m":
		start = start.Truncate(15 * time.Minute)
	case "hour":
		start = time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), 0, 0, 0, loc)
	case "day":
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	case "week":
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		wd := int(start.Weekday())
		if wd == 0 {
			wd = 7
		}
		start = start.AddDate(0, 0, -(wd - 1))
	}
	var out []Point
	for t := start; t.Before(f.To); {
		if p, ok := got[t.UTC()]; ok {
			out = append(out, p)
		} else {
			out = append(out, Point{T: t})
		}
		switch gran {
		case "15m":
			t = t.Add(15 * time.Minute)
		case "hour":
			t = t.Add(time.Hour)
		case "day":
			t = t.AddDate(0, 0, 1)
		case "week":
			t = t.AddDate(0, 0, 7)
		}
		if len(out) > 5000 {
			break
		}
	}
	return out, nil
}

// Dimensions available for breakdowns (rollup key expression, raw expression).
var Dimensions = map[string]string{
	"country":    "COALESCE(country, 'Unknown')",
	"region":     "COALESCE(country || '/' || region, 'Unknown')",
	"city":       "COALESCE(country || '/' || city, 'Unknown')",
	"device":     "COALESCE(device_type, 'other')",
	"os":         "COALESCE(os, 'Unknown')",
	"browser":    "COALESCE(browser, 'Unknown')",
	"language":   "COALESCE(language, 'Unknown')",
	"referrer":   "COALESCE(referrer_host, 'Direct')",
	"rule":       "COALESCE(rule_id, 'default')",
	"version":    "COALESCE(version_id::text, 'Unknown')",
	"utm_source": "COALESCE(utm_source, 'None')",
}

type Item struct {
	Key         string  `json:"key"`
	Scans       int64   `json:"scans"`
	UniqueScans int64   `json:"unique_scans"`
	Share       float64 `json:"share"`
}

// Breakdown groups counted scans by a dimension.
func (s *Service) Breakdown(ctx context.Context, f Filter, dim string, limit int) ([]Item, bool, error) {
	expr, ok := Dimensions[dim]
	if !ok {
		return nil, false, fmt.Errorf("unknown dimension %q", dim)
	}
	var args []any
	var sql string
	exact := f.To.Sub(f.From) < 72*time.Hour
	if exact {
		w := f.where("occurred_at", &args)
		sql = `SELECT ` + expr + ` AS k, count(*), count(*) FILTER (WHERE is_unique) FROM scan_events
			WHERE ` + w + ` AND NOT is_bot AND NOT is_duplicate AND outcome IN ('redirect','hosted_page','password_ok')
			GROUP BY 1 ORDER BY 2 DESC, 1`
	} else {
		// Daily rollups are UTC days; the edge days are approximate by < 1 day.
		args = append(args, f.WorkspaceID, f.From.UTC().Format("2006-01-02"), f.To.Add(-time.Microsecond).UTC().Format("2006-01-02"))
		w := "workspace_id = $1 AND day >= $2::date AND day <= $3::date"
		if f.QRIDs != nil {
			args = append(args, f.QRIDs)
			w += fmt.Sprintf(" AND qr_code_id = ANY($%d)", len(args))
		}
		if f.CampaignID != nil {
			args = append(args, *f.CampaignID)
			w += fmt.Sprintf(" AND campaign_id = $%d", len(args))
		}
		args = append(args, dim)
		sql = fmt.Sprintf(`SELECT key, sum(scans), sum(unique_scans) FROM scan_stats_daily_dim
			WHERE %s AND dimension = $%d GROUP BY key ORDER BY 2 DESC, 1`, w, len(args))
	}
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, exact, err
	}
	defer rows.Close()
	var out []Item
	var total int64
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.Key, &it.Scans, &it.UniqueScans); err != nil {
			return nil, exact, err
		}
		total += it.Scans
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, exact, err
	}
	for i := range out {
		if total > 0 {
			out[i].Share = float64(int64(float64(out[i].Scans)/float64(total)*10000)) / 10000
		}
	}
	if limit > 0 && len(out) > limit {
		var rest Item
		rest.Key = "Other"
		for _, it := range out[limit:] {
			rest.Scans += it.Scans
			rest.UniqueScans += it.UniqueScans
			rest.Share += it.Share
		}
		out = append(out[:limit], rest)
	}
	return out, exact, nil
}

type TopCode struct {
	QRCodeID    uuid.UUID `json:"qr_code_id"`
	Name        string    `json:"name"`
	ShortCode   *string   `json:"short_code"`
	ContentType string    `json:"content_type"`
	Scans       int64     `json:"scans"`
	UniqueScans int64     `json:"unique_scans"`
}

func (s *Service) TopCodes(ctx context.Context, f Filter, limit int) ([]TopCode, error) {
	var args []any
	w := f.where("s.bucket_start", &args)
	w = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(w, "workspace_id = $1", "s.workspace_id = $1"),
		"qr_code_id = ANY", "s.qr_code_id = ANY"), "campaign_id =", "s.campaign_id =")
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT s.qr_code_id, q.name, q.short_code, q.content_type,
		sum(s.scans)::bigint, sum(s.unique_scans)::bigint
		FROM scan_stats_15m s JOIN qr_codes q ON q.id = s.qr_code_id AND q.workspace_id = s.workspace_id
		WHERE %s GROUP BY 1, 2, 3, 4 HAVING sum(s.scans) > 0 ORDER BY 5 DESC LIMIT $%d`, w, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopCode
	for rows.Next() {
		var t TopCode
		if err := rows.Scan(&t.QRCodeID, &t.Name, &t.ShortCode, &t.ContentType, &t.Scans, &t.UniqueScans); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type Cell struct {
	Weekday int   `json:"weekday"` // 1 = Monday … 7 = Sunday
	Hour    int   `json:"hour"`
	Scans   int64 `json:"scans"`
}

// Heatmap is scans by local weekday × hour (always 7×24 cells).
func (s *Service) Heatmap(ctx context.Context, f Filter) ([]Cell, error) {
	var args []any
	w := f.where("bucket_start", &args)
	args = append(args, f.tz())
	n := len(args)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT extract(isodow FROM bucket_start AT TIME ZONE $%d)::int,
		extract(hour FROM bucket_start AT TIME ZONE $%d)::int, sum(scans)::bigint
		FROM scan_stats_15m WHERE %s GROUP BY 1, 2`, n, n, w), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grid := make([]Cell, 0, 168)
	idx := map[[2]int]int64{}
	for rows.Next() {
		var d, h int
		var c int64
		if err := rows.Scan(&d, &h, &c); err != nil {
			return nil, err
		}
		idx[[2]int{d, h}] = c
	}
	for d := 1; d <= 7; d++ {
		for h := 0; h < 24; h++ {
			grid = append(grid, Cell{Weekday: d, Hour: h, Scans: idx[[2]int{d, h}]})
		}
	}
	return grid, rows.Err()
}

type Scan struct {
	EventID     uuid.UUID  `json:"event_id"`
	OccurredAt  time.Time  `json:"occurred_at"`
	QRCodeID    uuid.UUID  `json:"qr_code_id"`
	VersionID   *uuid.UUID `json:"version_id"`
	Outcome     string     `json:"outcome"`
	Method      string     `json:"method"`
	IsBot       bool       `json:"is_bot"`
	BotReason   *string    `json:"bot_reason"`
	IsDuplicate bool       `json:"is_duplicate"`
	IsUnique    bool       `json:"is_unique"`
	DeviceType  *string    `json:"device_type"`
	OS          *string    `json:"os"`
	Browser     *string    `json:"browser"`
	Country     *string    `json:"country"`
	Region      *string    `json:"region"`
	City        *string    `json:"city"`
	Language    *string    `json:"language"`
	Referrer    *string    `json:"referrer_host"`
	RuleID      *string    `json:"rule_id"`
	UTMSource   *string    `json:"utm_source"`
	UTMMedium   *string    `json:"utm_medium"`
	UTMCampaign *string    `json:"utm_campaign"`
}

// Scans returns raw events newest first, keyset-paginated on (occurred_at, event_id).
func (s *Service) Scans(ctx context.Context, f Filter, before *time.Time, beforeID *uuid.UUID, limit int) ([]Scan, error) {
	var args []any
	w := f.where("occurred_at", &args)
	if before != nil && beforeID != nil {
		args = append(args, *before, *beforeID)
		w += fmt.Sprintf(" AND (occurred_at, event_id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, limit)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT event_id, occurred_at, qr_code_id, version_id, outcome, method, is_bot,
		bot_reason, is_duplicate, is_unique, device_type, os, browser, country::text, region, city, language, referrer_host,
		rule_id, utm_source, utm_medium, utm_campaign
		FROM scan_events WHERE %s ORDER BY occurred_at DESC, event_id DESC LIMIT $%d`, w, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scan
	for rows.Next() {
		var e Scan
		if err := rows.Scan(&e.EventID, &e.OccurredAt, &e.QRCodeID, &e.VersionID, &e.Outcome, &e.Method, &e.IsBot, &e.BotReason,
			&e.IsDuplicate, &e.IsUnique, &e.DeviceType, &e.OS, &e.Browser, &e.Country, &e.Region, &e.City, &e.Language,
			&e.Referrer, &e.RuleID, &e.UTMSource, &e.UTMMedium, &e.UTMCampaign); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
