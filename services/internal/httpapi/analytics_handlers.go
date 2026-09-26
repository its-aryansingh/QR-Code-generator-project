package httpapi

import (
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/analytics"
	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/realtime"
)

func (s *Server) analyticsRoutes(r chi.Router) {
	r.With(authz.Require(authz.AnalyticsRead)).Get("/realtime", s.handleRealtime)
	r.Route("/analytics", func(r chi.Router) {
		r.Use(authz.Require(authz.AnalyticsRead))
		r.Get("/summary", s.handleAnalyticsSummary)
		r.Get("/timeseries", s.handleAnalyticsTimeseries)
		r.Get("/breakdown", s.handleAnalyticsBreakdown)
		r.Get("/top-codes", s.handleAnalyticsTopCodes)
		r.Get("/heatmap", s.handleAnalyticsHeatmap)
		r.Get("/realtime", s.handleRealtime)
		r.With(authz.Require(authz.AnalyticsRaw)).Get("/scans", s.handleAnalyticsScans)
		r.With(authz.Require(authz.AnalyticsExport)).Get("/export", s.handleAnalyticsExport)
	})
}

type rangeInfo struct {
	From          time.Time `json:"from"`
	To            time.Time `json:"to"`
	TZ            string    `json:"tz"`
	Clamped       bool      `json:"clamped"`
	AvailableFrom time.Time `json:"available_from"`
}

// analyticsFilter parses from/to (local dates, inclusive) or range=24h, tz, qr_id (repeatable)
// and campaign_id, clamps to the plan's history and applies folder-scoped access.
func (s *Server) analyticsFilter(r *http.Request) (analytics.Filter, rangeInfo, error) {
	ws := workspaceRow(r)
	q := r.URL.Query()
	tz := q.Get("tz")
	if tz == "" {
		tz = ws.Timezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_tz", "tz must be an IANA timezone")
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	var from, to time.Time
	switch rng := q.Get("range"); rng {
	case "24h":
		to = now.Truncate(15 * time.Minute).Add(15 * time.Minute)
		from = to.Add(-24 * time.Hour)
	case "", "custom":
		to = today.AddDate(0, 0, 1)
		from = today.AddDate(0, 0, -29)
		if v := q.Get("to"); v != "" {
			d, err := time.ParseInLocation("2006-01-02", v, loc)
			if err != nil {
				return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_to", "to must be YYYY-MM-DD")
			}
			to = d.AddDate(0, 0, 1)
		}
		if v := q.Get("from"); v != "" {
			d, err := time.ParseInLocation("2006-01-02", v, loc)
			if err != nil {
				return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_from", "from must be YYYY-MM-DD")
			}
			from = d
		}
	default:
		days, err := strconv.Atoi(strings.TrimSuffix(rng, "d"))
		if err != nil || !strings.HasSuffix(rng, "d") || days < 1 || days > 3650 {
			return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_range", "range must be 24h or Nd (e.g. 7d, 30d)")
		}
		to = today.AddDate(0, 0, 1)
		from = today.AddDate(0, 0, -(days - 1))
	}
	if !to.After(from) {
		return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_range", "to must be on or after from")
	}
	if to.Sub(from) > 3660*24*time.Hour {
		return analytics.Filter{}, rangeInfo{}, apierr.BadRequest("invalid_range", "ranges are limited to 10 years")
	}
	info := rangeInfo{TZ: loc.String()}
	history := s.ent().Limits(r.Context(), ws).AnalyticsHistoryDays
	info.AvailableFrom = today.AddDate(0, 0, -(history - 1))
	if from.Before(info.AvailableFrom) {
		from, info.Clamped = info.AvailableFrom, true
		if !to.After(from) {
			to = from.AddDate(0, 0, 1)
		}
	}
	info.From, info.To = from, to
	f := analytics.Filter{WorkspaceID: ws.ID, From: from.UTC(), To: to.UTC(), Loc: loc}
	if v := q.Get("campaign_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return f, info, apierr.BadRequest("invalid_campaign_id", "campaign_id must be a UUID")
		}
		f.CampaignID = &id
	}
	var requested []uuid.UUID
	for _, v := range q["qr_id"] {
		for _, part := range strings.Split(v, ",") {
			id, err := uuid.Parse(strings.TrimSpace(part))
			if err != nil {
				return f, info, apierr.BadRequest("invalid_qr_id", "qr_id must be a UUID")
			}
			requested = append(requested, id)
		}
	}
	if len(requested) > 100 {
		return f, info, apierr.BadRequest("too_many_codes", "at most 100 qr_id values")
	}
	g := authz.FromContext(r.Context())
	if len(requested) == 0 && g.HasWorkspace(authz.AnalyticsRead) {
		return f, info, nil
	}
	// Resolve which codes the caller may see (explicit list and/or folder-scoped access).
	rows, err := s.pool.Query(r.Context(), `SELECT id, folder_id FROM qr_codes
		WHERE workspace_id = $1 AND ($2::uuid[] IS NULL OR id = ANY($2)) AND deleted_at IS NULL`, ws.ID, nilIfEmpty(requested))
	if err != nil {
		return f, info, apierr.Internal("failed to resolve codes")
	}
	type code struct {
		id     uuid.UUID
		folder *uuid.UUID
	}
	var codes []code
	for rows.Next() {
		var c code
		if err := rows.Scan(&c.id, &c.folder); err == nil {
			codes = append(codes, c)
		}
	}
	rows.Close()
	allowed := []uuid.UUID{}
	for _, c := range codes {
		if s.can(r, authz.AnalyticsRead, c.folder) {
			allowed = append(allowed, c.id)
		}
	}
	if len(requested) > 0 && len(allowed) != len(uniqueIDs(requested)) {
		return f, info, apierr.NotFound("QR code not found")
	}
	f.QRIDs = allowed
	return f, info, nil
}

func nilIfEmpty(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]bool {
	m := map[uuid.UUID]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// cached serves an analytics response from Redis for 60 s (5 min for closed ranges).
func (s *Server) cached(w http.ResponseWriter, r *http.Request, f analytics.Filter, info rangeInfo, compute func(ctx context.Context) (any, error)) {
	h := sha1.New()
	ids := make([]string, 0, len(f.QRIDs))
	for _, id := range f.QRIDs {
		ids = append(ids, id.String())
	}
	sort.Strings(ids)
	h.Write([]byte(r.URL.Path + "?" + r.URL.Query().Encode() + "|" + strings.Join(ids, ",") + "|" + info.From.String() + info.To.String()))
	key := "an:" + f.WorkspaceID.String() + ":" + hex.EncodeToString(h.Sum(nil))
	if s.rdb != nil {
		if b, err := s.rdb.Get(r.Context(), key).Bytes(); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "hit")
			_, _ = w.Write(b)
			return
		}
	}
	v, err := compute(r.Context())
	if err != nil {
		fail(w, problemOr500(err, "failed to compute analytics"))
		return
	}
	b, _ := json.Marshal(v)
	if s.rdb != nil {
		ttl := 60 * time.Second
		if info.To.Before(time.Now().Add(-time.Hour)) {
			ttl = 5 * time.Minute
		}
		_ = s.rdb.Set(r.Context(), key, b, ttl).Err()
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func (s *Server) analyticsSvc() *analytics.Service { return analytics.NewService(s.pool) }

func (s *Server) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	s.cached(w, r, f, info, func(ctx context.Context) (any, error) {
		sum, err := s.analyticsSvc().Summary(ctx, f)
		if err != nil {
			return nil, err
		}
		return map[string]any{"range": info, "summary": sum}, nil
	})
}

func (s *Server) handleAnalyticsTimeseries(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	gran := r.URL.Query().Get("granularity")
	switch gran {
	case "", "auto":
		gran = analytics.Granularity(f.From, f.To)
	case "15m", "hour", "day", "week":
		if gran == "15m" && f.To.Sub(f.From) > 8*24*time.Hour || gran == "hour" && f.To.Sub(f.From) > 93*24*time.Hour {
			fail(w, apierr.BadRequest("granularity_too_fine", "choose a coarser granularity for this range"))
			return
		}
	default:
		fail(w, apierr.BadRequest("invalid_granularity", "granularity must be auto, 15m, hour, day or week"))
		return
	}
	s.cached(w, r, f, info, func(ctx context.Context) (any, error) {
		pts, err := s.analyticsSvc().Timeseries(ctx, f, gran)
		if err != nil {
			return nil, err
		}
		return map[string]any{"range": info, "granularity": gran, "points": pts}, nil
	})
}

func (s *Server) handleAnalyticsBreakdown(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	dim := r.URL.Query().Get("dimension")
	if _, ok := analytics.Dimensions[dim]; !ok {
		keys := make([]string, 0, len(analytics.Dimensions))
		for k := range analytics.Dimensions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fail(w, apierr.BadRequest("invalid_dimension", "dimension must be one of "+strings.Join(keys, ", ")))
		return
	}
	if dim == "rule" {
		if err := s.ent().CheckFeature(r.Context(), workspaceRow(r), entitlements.FeatureRules); err != nil {
			fail(w, featureErr(err))
			return
		}
	}
	limit := limitParam(r, 20, 250)
	s.cached(w, r, f, info, func(ctx context.Context) (any, error) {
		items, exact, err := s.analyticsSvc().Breakdown(ctx, f, dim, limit)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []analytics.Item{}
		}
		return map[string]any{"range": info, "dimension": dim, "exact": exact, "data": items}, nil
	})
}

func (s *Server) handleAnalyticsTopCodes(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	limit := limitParam(r, 10, 100)
	s.cached(w, r, f, info, func(ctx context.Context) (any, error) {
		top, err := s.analyticsSvc().TopCodes(ctx, f, limit)
		if err != nil {
			return nil, err
		}
		if top == nil {
			top = []analytics.TopCode{}
		}
		return map[string]any{"range": info, "data": top}, nil
	})
}

func (s *Server) handleAnalyticsHeatmap(w http.ResponseWriter, r *http.Request) {
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	s.cached(w, r, f, info, func(ctx context.Context) (any, error) {
		cells, err := s.analyticsSvc().Heatmap(ctx, f)
		if err != nil {
			return nil, err
		}
		return map[string]any{"range": info, "data": cells}, nil
	})
}

// GET /realtime?minutes=15: per-minute counted scans from Redis (live, ~1 s behind).
func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request) {
	if s.rdb == nil {
		writeJSON(w, http.StatusOK, map[string]any{"total": 0, "data": []any{}})
		return
	}
	if !authz.FromContext(r.Context()).HasWorkspace(authz.AnalyticsRead) {
		fail(w, forbidden("forbidden", "realtime analytics need workspace-wide analytics access"))
		return
	}
	minutes, _ := strconv.Atoi(r.URL.Query().Get("minutes"))
	counts, total, err := realtime.New(s.rdb).GetRecentCounts(r.Context(), workspaceRow(r).ID.String(), minutes)
	if err != nil {
		fail(w, apierr.Internal("realtime unavailable"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "data": counts})
}

// GET /analytics/scans: raw scan log (business plan), newest first.
func (s *Server) handleAnalyticsScans(w http.ResponseWriter, r *http.Request) {
	if err := s.ent().CheckFeature(r.Context(), workspaceRow(r), entitlements.FeatureRawScanLog); err != nil {
		fail(w, featureErr(err))
		return
	}
	f, _, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	c, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		fail(w, apierr.BadRequest("invalid_cursor", "cursor is malformed"))
		return
	}
	limit := limitParam(r, 50, 200)
	var before *time.Time
	var beforeID *uuid.UUID
	if c != nil {
		before, beforeID = &c.T, &c.ID
	}
	rows, err := s.analyticsSvc().Scans(r.Context(), f, before, beforeID, limit+1)
	if err != nil {
		fail(w, apierr.Internal("failed to list scans"))
		return
	}
	out := page[analytics.Scan]{Data: rows}
	if out.Data == nil {
		out.Data = []analytics.Scan{}
	}
	if len(rows) > limit {
		last := rows[limit-1]
		cur := encodeCursor(last.OccurredAt, last.EventID)
		out.NextCursor = &cur
		out.Data = rows[:limit]
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /analytics/export?report=daily|scans: CSV download.
func (s *Server) handleAnalyticsExport(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	if err := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureCSVExport); err != nil {
		fail(w, featureErr(err))
		return
	}
	f, info, err := s.analyticsFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	report := r.URL.Query().Get("report")
	if report == "" {
		report = "daily"
	}
	name := "qrit-" + ws.Slug + "-" + report + "-" + info.From.Format("20060102") + "-" + info.To.AddDate(0, 0, -1).Format("20060102") + ".csv"
	switch report {
	case "daily":
		pts, err := s.analyticsSvc().Timeseries(r.Context(), f, "day")
		if err != nil {
			fail(w, apierr.Internal("export failed"))
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"date", "scans", "unique_scans"})
		for _, p := range pts {
			_ = cw.Write([]string{p.T.Format("2006-01-02"), strconv.FormatInt(p.Scans, 10), strconv.FormatInt(p.UniqueScans, 10)})
		}
		cw.Flush()
	case "scans":
		if err := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureRawScanLog); err != nil {
			fail(w, featureErr(err))
			return
		}
		if !authz.FromContext(r.Context()).HasAnywhere(authz.AnalyticsRaw) {
			fail(w, forbidden("forbidden", "raw scan export needs the analytics.raw permission"))
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"occurred_at", "qr_code_id", "outcome", "is_bot", "is_duplicate", "is_unique", "device_type", "os",
			"browser", "country", "region", "city", "language", "referrer_host", "rule_id", "utm_source", "utm_medium", "utm_campaign"})
		var before *time.Time
		var beforeID *uuid.UUID
		written := 0
		for written < 1_000_000 {
			rows, err := s.analyticsSvc().Scans(r.Context(), f, before, beforeID, 5000)
			if err != nil || len(rows) == 0 {
				break
			}
			for _, e := range rows {
				_ = cw.Write([]string{e.OccurredAt.In(f.Loc).Format(time.RFC3339), e.QRCodeID.String(), e.Outcome,
					strconv.FormatBool(e.IsBot), strconv.FormatBool(e.IsDuplicate), strconv.FormatBool(e.IsUnique),
					csvSafe(e.DeviceType), csvSafe(e.OS), csvSafe(e.Browser), csvSafe(e.Country), csvSafe(e.Region), csvSafe(e.City),
					csvSafe(e.Language), csvSafe(e.Referrer), csvSafe(e.RuleID), csvSafe(e.UTMSource), csvSafe(e.UTMMedium), csvSafe(e.UTMCampaign)})
			}
			written += len(rows)
			last := rows[len(rows)-1]
			before, beforeID = &last.OccurredAt, &last.EventID
			cw.Flush()
		}
		cw.Flush()
	default:
		fail(w, apierr.BadRequest("invalid_report", "report must be daily or scans"))
	}
}

// csvSafe neutralises spreadsheet formula injection (=, +, -, @ prefixes).
func csvSafe(p *string) string {
	if p == nil {
		return ""
	}
	v := *p
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
