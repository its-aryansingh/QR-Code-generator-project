package httpapi

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/ingest"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/redirect"
	"github.com/its-aryansingh/qrit/services/internal/worker"
)

const iphoneUA = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1"
const androidUA = "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Mobile Safari/537.36"

type scanClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func (c scanClient) do(method, path, ip, ua, country string, form url.Values) (*http.Response, string) {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, _ := http.NewRequest(method, c.base+path, body)
	req.Host = "qr.test"
	req.Header.Set("User-Agent", ua)
	if ip != "" {
		req.Header.Set("CF-Connecting-IP", ip)
	}
	if country != "" {
		req.Header.Set("CF-IPCountry", country)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestScanPipelineEndToEnd(t *testing.T) {
	h := newHarness(t)
	if h.rdb == nil {
		t.Skip("QRIT_TEST_REDIS_URL not set")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, ws, _ := h.register("pipe")
	h.setPlan(ws, "business")
	base := "/v1/workspaces/" + ws + "/qr-codes"
	q := c.must(201, "POST", base, map[string]any{"name": "Poster", "destination_url": "https://example.com/menu?table=4",
		"utm": map[string]any{"source": "qr", "medium": "poster"},
		"rules": []any{map[string]any{"id": "india", "enabled": true, "destination_url": "https://example.in/menu",
			"when": map[string]any{"all": []any{map[string]any{"field": "country", "op": "in", "value": []any{"IN"}}}}},
			map[string]any{"id": "noeu", "enabled": true, "block": true,
				"when": map[string]any{"all": []any{map[string]any{"field": "country", "op": "in", "value": []any{"DE"}}}}}}})
	code, id := q["short_code"].(string), q["id"].(string)

	_, loop, _ := net.ParseCIDR("127.0.0.0/8")
	rs, err := redirect.New(redirect.Options{Pool: h.pool, Redis: h.rdb, TrustedProxies: []*net.IPNet{loop},
		AppBaseURL: "http://app.test", LRUSize: 1000, LRUTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Start(ctx); err != nil {
		t.Fatal(err)
	}
	rts := httptest.NewServer(rs.Handler())
	defer rts.Close()
	sc := scanClient{t: t, base: rts.URL, http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}

	// Visitor A in India: rule destination with UTM appended (existing params kept).
	res, _ := sc.do("GET", "/"+code, "203.0.113.10", iphoneUA, "IN", nil)
	if res.StatusCode != 302 || res.Header.Get("Location") != "https://example.in/menu?utm_source=qr&utm_medium=poster" {
		t.Fatalf("IN redirect: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	if res.Header.Get("Cache-Control") != "private, no-store, max-age=0" || res.Header.Get("Set-Cookie") != "" ||
		res.Header.Get("X-Robots-Tag") == "" || res.Header.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatalf("redirect headers: %v", res.Header)
	}
	sc.do("GET", "/"+strings.ToLower(code), "203.0.113.10", iphoneUA, "IN", nil) // duplicate (and case-insensitive)
	res, _ = sc.do("GET", "/"+code, "198.51.100.7", androidUA, "US", nil)
	if res.Header.Get("Location") != "https://example.com/menu?table=4&utm_source=qr&utm_medium=poster" {
		t.Fatalf("default redirect: %s", res.Header.Get("Location"))
	}
	res, _ = sc.do("GET", "/"+code, "192.0.2.44", androidUA, "DE", nil)
	if res.StatusCode != http.StatusUnavailableForLegalReasons {
		t.Fatalf("geo block: %d", res.StatusCode)
	}
	res, _ = sc.do("HEAD", "/"+code, "198.51.100.8", androidUA, "US", nil)
	if res.StatusCode != 302 {
		t.Fatalf("HEAD: %d", res.StatusCode)
	}
	sc.do("GET", "/"+code, "198.51.100.9", "WhatsApp/2.23.20.0 A", "US", nil)
	res, body := sc.do("GET", "/"+code+"+", "198.51.100.10", androidUA, "US", nil)
	if res.StatusCode != 200 || !strings.Contains(body, "example.com") {
		t.Fatalf("preview: %d", res.StatusCode)
	}
	res, _ = sc.do("GET", "/ZZZZZZZ", "198.51.100.10", androidUA, "US", nil)
	if res.StatusCode != 404 {
		t.Fatalf("unknown code: %d", res.StatusCode)
	}
	req, _ := http.NewRequest("GET", rts.URL+"/"+code, nil)
	req.Host = "evil.example"
	if r2, _ := http.DefaultClient.Do(req); r2 == nil || r2.StatusCode != 404 {
		t.Fatal("unknown host must 404")
	}

	// Pausing through the API takes effect on the redirect immediately (pub/sub eviction).
	c.must(200, "POST", base+"/"+id+"/pause", nil)
	waitFor(t, func() bool {
		res, _ := sc.do("GET", "/"+code+"+", "198.51.100.11", androidUA, "US", nil)
		_ = res
		r, _ := sc.do("HEAD", "/"+code, "198.51.100.11", androidUA, "US", nil)
		return r.StatusCode == http.StatusGone
	})
	c.must(200, "POST", base+"/"+id+"/resume", nil)
	c.must(200, "PATCH", base+"/"+id, map[string]any{"password": "open sesame"})
	waitFor(t, func() bool {
		r, b := sc.do("GET", "/"+code, "100.64.0.1", iphoneUA, "IN", nil)
		return r.StatusCode == 200 && strings.Contains(b, `type="password"`)
	})
	res, _ = sc.do("POST", "/"+code, "100.64.0.1", iphoneUA, "IN", url.Values{"password": {"wrong"}})
	if res.StatusCode != 401 {
		t.Fatalf("wrong password: %d", res.StatusCode)
	}
	res, _ = sc.do("POST", "/"+code, "100.64.0.1", iphoneUA, "IN", url.Values{"password": {"open sesame"}})
	if res.StatusCode != 302 || !strings.HasPrefix(res.Header.Get("Location"), "https://example.in/menu") {
		t.Fatalf("password ok: %d %s", res.StatusCode, res.Header.Get("Location"))
	}

	// Hosted vCard page served on the short host.
	hq := c.must(201, "POST", base, map[string]any{"name": "Card", "content_type": "vcard",
		"hosted_page": map[string]any{"kind": "vcard", "vcard": map[string]any{"first_name": "Asha", "last_name": "Rao",
			"phones": []any{map[string]any{"type": "cell", "value": "+919876543210"}}}}})
	hcode := hq["short_code"].(string)
	res, _ = sc.do("GET", "/"+hcode, "203.0.113.20", iphoneUA, "IN", nil)
	if res.StatusCode != 302 || res.Header.Get("Location") != "/p/"+hcode {
		t.Fatalf("hosted redirect: %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	res, body = sc.do("GET", "/p/"+hcode, "203.0.113.20", iphoneUA, "IN", nil)
	if res.StatusCode != 200 || !strings.Contains(body, "Asha") || !strings.Contains(body, hcode+".vcf") {
		t.Fatalf("hosted page: %d", res.StatusCode)
	}
	res, body = sc.do("GET", "/p/"+hcode+".vcf", "203.0.113.20", iphoneUA, "IN", nil)
	if !strings.Contains(body, "TEL;TYPE=CELL:+919876543210") || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/vcard") {
		t.Fatalf("vcf: %s", body)
	}

	// Drain the emitter, then ingest the stream.
	// 11 events: IN, duplicate, US, DE (geo), HEAD, WhatsApp, paused, password prompt/fail/ok, vCard.
	waitFor(t, func() bool { return rs.Emitter.Backlog() == 0 && h.rdb.XLen(ctx, "scans").Val() >= 11 })
	cons := ingest.NewConsumer(h.pool, h.rdb, ingest.Config{Consumer: "test", BlockWait: 100 * time.Millisecond}, nil)
	if err := cons.EnsureGroup(ctx); err != nil {
		t.Fatal(err)
	}
	var all []redis.XMessage
	for {
		st, err := h.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{Group: "ingest", Consumer: "test", Streams: []string{"scans", ">"},
			Count: 1000, Block: 100 * time.Millisecond}).Result()
		if err != nil || len(st) == 0 || len(st[0].Messages) == 0 {
			break
		}
		if _, err := cons.Process(ctx, st[0].Messages); err != nil {
			t.Fatal(err)
		}
		all = append(all, st[0].Messages...)
	}
	var total, uniques int64
	if err := h.pool.QueryRow(ctx, `SELECT total_scans, unique_scans FROM qr_codes WHERE id = $1`, id).Scan(&total, &uniques); err != nil {
		t.Fatal(err)
	}
	if total != 3 || uniques != 3 {
		t.Fatalf("counted scans: total=%d unique=%d (want 3/3: IN visitor, US visitor, password_ok)", total, uniques)
	}
	var bots, dups, outcomes int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE is_bot), count(*) FILTER (WHERE is_duplicate), count(DISTINCT outcome)
		FROM scan_events WHERE qr_code_id = $1`, id).Scan(&bots, &dups, &outcomes)
	if bots < 2 || dups != 1 || outcomes < 5 {
		t.Fatalf("classification: bots=%d dups=%d outcomes=%d", bots, dups, outcomes)
	}

	// Exactly-once effect: replaying the same messages changes nothing.
	if r, err := cons.Process(ctx, all); err != nil || r.Inserted != 0 {
		t.Fatalf("replay: %+v %v", r, err)
	}
	_ = h.pool.QueryRow(ctx, `SELECT total_scans FROM qr_codes WHERE id = $1`, id).Scan(&total)
	if total != 3 {
		t.Fatalf("replay changed totals: %d", total)
	}
	// A rebuild from raw events agrees with the incremental rollups.
	if err := ingest.Rebuild(ctx, h.pool, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	_ = h.pool.QueryRow(ctx, `SELECT total_scans FROM qr_codes WHERE id = $1`, id).Scan(&total)
	if total != 3 {
		t.Fatalf("rebuild totals: %d", total)
	}

	// Analytics API over the rollups.
	an := "/v1/workspaces/" + ws + "/analytics"
	sum := c.must(200, "GET", an+"/summary?range=7d", nil)
	if dig(sum, "summary", "scans") != float64(4) || dig(sum, "summary", "bot_hits").(float64) < 2 {
		t.Fatalf("summary: %v", sum) // 3 on the poster + 1 on the vCard
	}
	per := c.must(200, "GET", an+"/summary?range=7d&qr_id="+id, nil)
	if dig(per, "summary", "scans") != float64(3) || dig(per, "summary", "unique_scans") != float64(3) {
		t.Fatalf("per-code summary: %v", per)
	}
	br := c.must(200, "GET", an+"/breakdown?dimension=country&range=7d&qr_id="+id, nil)
	countries := map[string]float64{}
	for _, it := range br["data"].([]any) {
		m := it.(map[string]any)
		countries[m["key"].(string)] = m["scans"].(float64)
	}
	if countries["IN"] != 2 || countries["US"] != 1 || br["exact"] != false {
		t.Fatalf("country breakdown: %v", br)
	}
	ts := c.must(200, "GET", an+"/timeseries?range=24h", nil)
	var sumTS float64
	for _, p := range ts["points"].([]any) {
		sumTS += p.(map[string]any)["scans"].(float64)
	}
	if ts["granularity"] != "15m" || sumTS != 4 || len(ts["points"].([]any)) != 96 {
		t.Fatalf("timeseries: gran=%v sum=%v n=%d", ts["granularity"], sumTS, len(ts["points"].([]any)))
	}
	dayTS := c.must(200, "GET", an+"/timeseries?range=30d&tz=Asia/Kolkata", nil)
	if dayTS["granularity"] != "day" || len(dayTS["points"].([]any)) != 30 {
		t.Fatalf("daily timeseries: %v", dayTS["granularity"])
	}
	top := c.must(200, "GET", an+"/top-codes?range=7d", nil)["data"].([]any)
	if len(top) != 2 || top[0].(map[string]any)["qr_code_id"] != id {
		t.Fatalf("top codes: %v", top)
	}
	hm := c.must(200, "GET", an+"/heatmap?range=7d", nil)["data"].([]any)
	if len(hm) != 168 {
		t.Fatalf("heatmap cells: %d", len(hm))
	}
	rt := c.must(200, "GET", "/v1/workspaces/"+ws+"/realtime?minutes=15", nil)
	if rt["total"].(float64) < 4 {
		t.Fatalf("realtime: %v", rt)
	}
	raw := c.must(200, "GET", an+"/scans?range=7d&limit=5", nil)
	if len(raw["data"].([]any)) != 5 || raw["next_cursor"] == nil {
		t.Fatalf("raw scans: %v", raw)
	}
	if strings.Contains(string(c.do("GET", an+"/scans?range=7d", nil).Body), "203.0.113") {
		t.Fatal("an IP address was stored")
	}
	exp := c.do("GET", an+"/export?report=daily&range=7d", nil)
	if exp.Status != 200 || !strings.HasPrefix(string(exp.Body), "date,scans,unique_scans") {
		t.Fatalf("export: %d %s", exp.Status, exp.Body)
	}
	// Free plans see at most 30 days of history.
	h.setPlan(ws, "free")
	cl := c.must(200, "GET", an+"/summary?range=90d", nil)
	if dig(cl, "range", "clamped") != true {
		t.Fatalf("history clamp: %v", cl)
	}
	c.must(402, "GET", an+"/scans?range=7d", nil)

	// Worker: partitions ahead, scheduled-version pointer, plan read-only enforcement.
	d := &worker.Deps{Pool: h.pool, Redis: h.rdb}
	if err := d.EnsurePartitions(ctx); err != nil {
		t.Fatal(err)
	}
	var parts int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FROM pg_inherits WHERE inhparent = 'scan_events'::regclass`).Scan(&parts)
	if parts < 5 {
		t.Fatalf("partitions: %d", parts)
	}
	h.exec(`INSERT INTO qr_versions (id, qr_code_id, version_no, destination_kind, destination_url, effective_at, safety_status)
		VALUES (gen_random_uuid(), $1, 99, 'url', 'https://example.com/due', clock_timestamp(), 'safe')`, id)
	if err := d.ActivateScheduledVersions(ctx); err != nil {
		t.Fatal(err)
	}
	var cur string
	_ = h.pool.QueryRow(ctx, `SELECT v.destination_url FROM qr_codes q JOIN qr_versions v ON v.id = q.current_version_id WHERE q.id = $1`, id).Scan(&cur)
	if cur != "https://example.com/due" {
		t.Fatalf("activation: %s", cur)
	}
	if err := d.EnforcePlanReadOnly(ctx); err != nil {
		t.Fatal(err)
	}
	var ro int
	_ = h.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE is_read_only) FROM qr_codes WHERE workspace_id = $1`, ws).Scan(&ro)
	if ro != 0 { // 2 dynamic codes <= free limit of 3
		t.Fatalf("read-only: %d", ro)
	}
}

func TestJobQueue(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.exec(`DELETE FROM job_queue WHERE kind LIKE 'test.%'`)
	r := jobs.NewRunner(h.pool, 2, nil)
	r.SetBackoff(func(int) time.Duration { return 0 })
	attempts := 0
	r.Handle("test.flaky", func(ctx context.Context, j *jobs.Job) error {
		attempts++
		if attempts < 3 {
			return io.ErrUnexpectedEOF
		}
		return nil
	})
	r.Handle("test.bad", func(ctx context.Context, j *jobs.Job) error { return jobs.Permanent(io.EOF) })
	id1, err := jobs.Enqueue(ctx, h.pool, "test.flaky", map[string]any{"n": 1}, jobs.Options{UniqueKey: "only-one"})
	if err != nil || id1 == 0 {
		t.Fatal(err)
	}
	if id, _ := jobs.Enqueue(ctx, h.pool, "test.flaky", map[string]any{"n": 2}, jobs.Options{UniqueKey: "only-one"}); id != 0 {
		t.Fatal("unique key not honoured")
	}
	id2, _ := jobs.Enqueue(ctx, h.pool, "test.bad", nil, jobs.Options{})
	id3, _ := jobs.Enqueue(ctx, h.pool, "test.unknown", nil, jobs.Options{})
	for i := 0; i < 5; i++ {
		_, _ = r.RunOnce(ctx)
	}
	state := func(id int64) string {
		var s string
		_ = h.pool.QueryRow(ctx, `SELECT state FROM job_queue WHERE id = $1`, id).Scan(&s)
		return s
	}
	if state(id1) != "completed" || attempts != 3 || state(id2) != "failed" || state(id3) != "available" {
		t.Fatalf("states: %s(%d) %s %s", state(id1), attempts, state(id2), state(id3))
	}
	// Periodic tasks run once per slot across replicas.
	s1, s2 := jobs.NewScheduler(h.pool, nil), jobs.NewScheduler(h.pool, nil)
	runs := 0
	task := jobs.Task{Name: "test.every-hour-" + time.Now().Format("150405.000"), Every: time.Hour, Run: func(context.Context) error { runs++; return nil }}
	s1.Add(task)
	s2.Add(task)
	s1.RunDue(ctx)
	s2.RunDue(ctx)
	s1.RunDue(ctx)
	if runs != 1 {
		t.Fatalf("periodic ran %d times", runs)
	}
	h.exec(`DELETE FROM job_queue WHERE kind LIKE 'test.%'`)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met in time")
}
