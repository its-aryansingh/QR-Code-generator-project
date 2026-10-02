// Package redirect is the scan path: short host + code → destination, in ~1 ms on a cache
// hit. It never writes to Postgres; every request becomes one event on the "scans" stream.
package redirect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/abuse"
	"github.com/its-aryansingh/qrit/services/internal/hosted"
	"github.com/its-aryansingh/qrit/services/internal/netutil"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/resolve"
	"github.com/its-aryansingh/qrit/services/internal/routing"
	"github.com/its-aryansingh/qrit/services/internal/scan"
	"github.com/its-aryansingh/qrit/services/internal/shortcode"
	"github.com/its-aryansingh/qrit/services/internal/version"
)

// DomainInvalidateChannel is published when domains are added, verified or removed.
const DomainInvalidateChannel = "domain:invalidate"

type Options struct {
	Pool           *pgxpool.Pool
	Redis          *redis.Client
	TrustedProxies []*net.IPNet
	AppBaseURL     string
	LRUSize        int
	LRUTTL         time.Duration
	EventBuffer    int
	// Fetch overrides the Postgres link loader (tests).
	Fetch resolve.FetchFunc
	// Extra routes (e.g. the GS1 Digital Link resolver) mounted before /{code}.
	Mount func(r chi.Router, s *Server)
}

type Server struct {
	pool       *pgxpool.Pool
	rdb        *redis.Client
	Resolver   *resolve.Resolver
	Emitter    *Emitter
	salts      *SaltStore
	ips        *netutil.ClientIPResolver
	limiter    abuse.VelocityLimiter
	appBaseURL string
	mount      func(r chi.Router, s *Server)

	domMu   sync.RWMutex
	domains map[string]Domain
}

func New(o Options) (*Server, error) {
	fetch := o.Fetch
	if fetch == nil {
		fetch = FetchLink(o.Pool)
	}
	size := o.LRUSize
	if size <= 0 {
		size = 100_000
	}
	res, err := resolve.NewResolverTTL(size, o.LRUTTL, o.Redis, fetch)
	if err != nil {
		return nil, err
	}
	var lim abuse.VelocityLimiter = abuse.NewMemoryVelocityLimiter()
	if o.Redis != nil {
		lim = abuse.NewRedisVelocityLimiter(o.Redis)
	}
	return &Server{
		pool: o.Pool, rdb: o.Redis, Resolver: res, Emitter: NewEmitter(o.Redis, o.EventBuffer),
		salts: NewSaltStore(o.Redis), ips: netutil.NewClientIPResolver(o.TrustedProxies), limiter: lim,
		appBaseURL: strings.TrimRight(o.AppBaseURL, "/"), mount: o.Mount, domains: map[string]Domain{},
	}, nil
}

// Start loads domains and runs the background loops until ctx ends.
func (s *Server) Start(ctx context.Context) error {
	if err := s.ReloadDomains(ctx); err != nil {
		return err
	}
	go s.Emitter.Run(ctx)
	go s.Resolver.Subscribe(ctx)
	go s.domainLoop(ctx)
	return nil
}

// ReloadDomains refreshes the host → domain map.
func (s *Server) ReloadDomains(ctx context.Context) error {
	if s.pool == nil {
		return nil
	}
	m, err := loadDomains(ctx, s.pool)
	if err != nil {
		return err
	}
	s.domMu.Lock()
	s.domains = m
	s.domMu.Unlock()
	return nil
}

// SetDomains replaces the host map (tests).
func (s *Server) SetDomains(ds ...Domain) {
	m := map[string]Domain{}
	for _, d := range ds {
		m[strings.ToLower(d.Hostname)] = d
	}
	s.domMu.Lock()
	s.domains = m
	s.domMu.Unlock()
}

func (s *Server) domainLoop(ctx context.Context) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	var ch <-chan *redis.Message
	if s.rdb != nil {
		sub := s.rdb.Subscribe(ctx, DomainInvalidateChannel)
		defer sub.Close()
		ch = sub.Channel()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-ch:
		}
		if err := s.ReloadDomains(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("reload domains", "error", err)
		}
	}
}

// domainFor matches the Host header exactly (with port first, then without). Unknown hosts
// never fall back to another domain.
func (s *Server) domainFor(host string) *Domain {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	s.domMu.RLock()
	defer s.domMu.RUnlock()
	if d, ok := s.domains[h]; ok {
		return &d
	}
	if i := strings.LastIndex(h, ":"); i > 0 && !strings.Contains(h[i:], "]") {
		if d, ok := s.domains[h[:i]]; ok {
			return &d
		}
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	r.Get("/readyz", s.handleReady)
	r.Get("/metrics", s.handleMetrics)
	r.Get("/robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})
	r.Get("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if s.mount != nil {
		s.mount(r, s)
	}
	r.Get("/", s.handleRoot)
	r.Get("/p/{code}", s.handleHosted)
	r.Head("/p/{code}", s.handleHosted)
	r.Get("/{code}", s.handleCode)
	r.Head("/{code}", s.handleCode)
	r.Post("/{code}", s.handleCode)
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if d := s.domainFor(r.Host); d != nil {
			s.notFound(w, r, d)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	return r
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ok := s.Resolver.Len() > 0
	if s.rdb != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
		defer cancel()
		ok = ok || s.rdb.Ping(ctx).Err() == nil
	}
	if !ok {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte("ready"))
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE scan_events_dropped_total counter\nscan_events_dropped_total %d\n", s.Emitter.Dropped.Load())
	fmt.Fprintf(w, "# TYPE scan_events_sent_total counter\nscan_events_sent_total %d\n", s.Emitter.Sent.Load())
	fmt.Fprintf(w, "# TYPE scan_events_failed_total counter\nscan_events_failed_total %d\n", s.Emitter.Failed.Load())
	fmt.Fprintf(w, "# TYPE scan_events_backlog gauge\nscan_events_backlog %d\n", s.Emitter.Backlog())
	fmt.Fprintf(w, "# TYPE resolver_cache_entries gauge\nresolver_cache_entries %d\n", s.Resolver.Len())
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	d := s.domainFor(r.Host)
	if d == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	target := s.appBaseURL
	if d.RootRedirectURL != nil && *d.RootRedirectURL != "" {
		target = *d.RootRedirectURL
	}
	s.redirectTo(w, r, target)
}

// redirectTo sends the canonical scan response: 302, never cached, no cookies.
func (s *Server) redirectTo(w http.ResponseWriter, r *http.Request, target string) {
	h := w.Header()
	h.Set("Location", target)
	h.Set("Cache-Control", "private, no-store, max-age=0")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(http.StatusFound)
}

// requestFacts are computed once per request. The client IP stays in this struct and is
// only used to compute the visitor hash and the A/B bucket; it is never emitted or logged.
type requestFacts struct {
	ip, ua, lang   string
	ref            *string
	geo            scan.GeoFacts
	device, osName string
}

func (s *Server) facts(r *http.Request) requestFacts {
	f := requestFacts{ip: s.ips.ClientIPString(r), ua: r.UserAgent(),
		lang: scan.ExtractPrimaryLanguage(r.Header.Get("Accept-Language")), ref: scan.ExtractReferrerHost(r.Referer())}
	if len(f.ua) > 512 {
		f.ua = f.ua[:512]
	}
	if s.ips.TrustedPeer(r) {
		cc := strings.ToUpper(strings.TrimSpace(r.Header.Get("CF-IPCountry")))
		if len(cc) == 2 && cc != "XX" && cc != "T1" {
			f.geo.Country = cc
		}
		f.geo.Region = truncateRunes(r.Header.Get("cf-region-code"), 64)
		f.geo.City = truncateRunes(r.Header.Get("cf-ipcity"), 80)
	}
	ua := scan.ParseUserAgent(f.ua)
	f.device, f.osName = ua.DeviceType, strings.ToLower(ua.OS)
	return f
}

func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

func (s *Server) emit(r *http.Request, link *resolve.ResolvedLink, f requestFacts, now time.Time, outcome, ruleID string) {
	ev := &scan.ScanEvent{
		ID: idgen.New().String(), Timestamp: now, WorkspaceID: link.WorkspaceID.String(), QRCodeID: link.QRCodeID.String(),
		DomainID: link.DomainID.String(), Outcome: outcome, Method: r.Method, UserAgent: f.ua, Geo: f.geo, Referrer: f.ref,
	}
	if f.lang != "" {
		l := f.lang
		ev.Language = &l
	}
	salt := s.salts.For(r.Context(), now)
	ev.VisitorHash = scan.ComputeVisitorHash(salt, link.QRCodeID, f.ip, f.ua)
	if link.Version != nil {
		v := link.Version.ID.String()
		ev.VersionID = &v
		var utm version.UTMConfig
		if json.Unmarshal(link.Version.UTM, &utm) == nil && (utm.Source != "" || utm.Medium != "" || utm.Campaign != "") {
			ev.UTM = &scan.UTMParams{Source: strPtr(utm.Source), Medium: strPtr(utm.Medium), Campaign: strPtr(utm.Campaign)}
		}
	}
	if link.CampaignID != nil {
		c := link.CampaignID.String()
		ev.CampaignID = &c
	}
	if ruleID != "" {
		ev.Rule = &ruleID
	}
	s.Emitter.Emit(ev)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

var safeSchemes = map[string]bool{"http": true, "https": true, "mailto": true, "tel": true, "sms": true, "smsto": true,
	"geo": true, "upi": true, "market": true, "itms-apps": true, "whatsapp": true, "maps": true}

// finalTarget applies UTM (only missing keys) and re-checks the scheme as defence in depth.
func finalTarget(dest string, utmRaw json.RawMessage) (string, bool) {
	var utm version.UTMConfig
	_ = json.Unmarshal(utmRaw, &utm)
	if withUTM, err := version.AppendUTM(dest, utm); err == nil {
		dest = withUTM
	}
	u, err := url.Parse(dest)
	if err != nil || !safeSchemes[strings.ToLower(u.Scheme)] {
		return "", false
	}
	return dest, true
}

func (s *Server) handleCode(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	dom := s.domainFor(r.Host)
	if dom == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	raw := chi.URLParam(r, "code")
	if u, err := url.PathUnescape(raw); err == nil {
		raw = u
	}
	preview := strings.HasSuffix(raw, "+")
	code := shortcode.Normalise(strings.TrimSuffix(raw, "+"))
	if shortcode.Validate(code) != nil {
		s.notFound(w, r, dom)
		return
	}
	link, err := s.Resolver.Resolve(r.Context(), dom.ID, code)
	switch {
	case errors.Is(err, resolve.ErrNotFound):
		s.notFound(w, r, dom)
		return
	case err != nil:
		slog.Error("resolve failed", "error", err)
		w.Header().Set("Retry-After", "30")
		s.renderPage(w, r, http.StatusServiceUnavailable, pageData{Title: "Temporarily unavailable",
			Message: "We couldn't open this code right now. Please try again in a moment."})
		return
	}
	w.Header().Set("Server-Timing", fmt.Sprintf("resolve;dur=%.2f", float64(time.Since(start).Microseconds())/1000))
	now := time.Now().UTC()
	f := s.facts(r)
	outcome, fallback := resolve.EvaluateState(link, now)
	if outcome == resolve.OutcomeActive && link.Version == nil {
		outcome = resolve.OutcomeNotStarted // created, but its first version awaits approval
	}

	if preview {
		d := pageData{Title: "Preview", Preview: true}
		if outcome == resolve.OutcomeActive || outcome == resolve.OutcomePasswordRequired {
			res := s.route(link, f, now)
			if res.URL != "" {
				if u, err := url.Parse(res.URL); err == nil {
					d.Host = u.Hostname()
					if d.Host == "" {
						d.Host = u.Scheme + ":"
					}
				}
				if outcome == resolve.OutcomeActive {
					d.Target, _ = finalTarget(res.URL, link.Version.UTM)
				}
			} else {
				d.Host = r.Host + " (hosted page)"
			}
		} else {
			d.Host = "nothing right now (the code is not active)"
		}
		s.renderPage(w, r, http.StatusOK, d)
		return
	}

	switch outcome {
	case resolve.OutcomeBlocked:
		s.emit(r, link, f, now, "blocked", "")
		s.renderPage(w, r, http.StatusGone, pageData{Title: "Link disabled",
			Message: "This link has been disabled for violating our terms."})
		return
	case resolve.OutcomePaused, resolve.OutcomeNotStarted, resolve.OutcomeExpired, resolve.OutcomeLimitReached:
		s.emit(r, link, f, now, string(outcome), "")
		if fallback != "" {
			s.redirectTo(w, r, fallback)
			return
		}
		status, title, msg := http.StatusGone, "This QR code is not active", "Its owner has paused it."
		switch outcome {
		case resolve.OutcomeNotStarted:
			status, title, msg = http.StatusNotFound, "Not active yet", "This QR code isn't live yet. Please try again later."
		case resolve.OutcomeExpired:
			title, msg = "This QR code has expired", "The campaign behind this code has ended."
		case resolve.OutcomeLimitReached:
			title, msg = "This QR code has reached its limit", "It can't be scanned any more."
		}
		s.renderPage(w, r, status, pageData{Title: title, Message: msg})
		return
	case resolve.OutcomePasswordRequired:
		if r.Method != http.MethodPost {
			s.emit(r, link, f, now, "password_prompt", "")
			s.renderPage(w, r, http.StatusOK, pageData{Title: "Password required", Password: true})
			return
		}
		if !s.checkPassword(w, r, link, f, now) {
			return
		}
		s.deliver(w, r, link, f, now, "password_ok", code)
		return
	}
	s.deliver(w, r, link, f, now, "", code)
}

func (s *Server) route(link *resolve.ResolvedLink, f requestFacts, now time.Time) routing.Result {
	var rules []routing.Rule
	_ = json.Unmarshal(link.Version.Rules, &rules)
	loc, err := time.LoadLocation(link.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return routing.Evaluate(routing.Version{DefaultDestination: link.Version.URL, Rules: rules}, routing.RequestFacts{
		QRCodeID: link.QRCodeID.String(), IP: f.ip, UserAgent: f.ua, Country: f.geo.Country, Region: f.geo.Region,
		DeviceType: f.device, OS: f.osName, Language: f.lang, ScanCount: link.TotalScans,
	}, now, loc)
}

// deliver sends the scan to its destination. outcome overrides "redirect"/"hosted_page"
// (used for password_ok).
func (s *Server) deliver(w http.ResponseWriter, r *http.Request, link *resolve.ResolvedLink, f requestFacts, now time.Time, outcome, code string) {
	res := s.route(link, f, now)
	if res.Blocked {
		s.emit(r, link, f, now, "geo_blocked", res.RuleID)
		s.renderPage(w, r, http.StatusUnavailableForLegalReasons, pageData{Title: "Not available here",
			Message: "This QR code isn't available in your region."})
		return
	}
	if res.URL == "" && link.Version.Kind == "hosted_page" {
		if outcome == "" {
			outcome = "hosted_page"
		}
		s.emit(r, link, f, now, outcome, res.RuleID)
		if outcome == "password_ok" {
			// The visitor proved the password on this request: render inline rather than
			// redirecting to /p/{code}, which would ask again.
			s.renderHosted(w, r, link, code)
			return
		}
		s.redirectTo(w, r, "/p/"+code)
		return
	}
	target, ok := finalTarget(res.URL, link.Version.UTM)
	if !ok {
		slog.Error("refusing unsafe destination scheme", "qr", link.QRCodeID)
		s.renderPage(w, r, http.StatusGone, pageData{Title: "Link unavailable", Message: "This destination can't be opened."})
		return
	}
	if outcome == "" {
		outcome = "redirect"
	}
	s.emit(r, link, f, now, outcome, res.RuleID)
	s.redirectTo(w, r, target)
}

func (s *Server) checkPassword(w http.ResponseWriter, r *http.Request, link *resolve.ResolvedLink, f requestFacts, now time.Time) bool {
	vh := scan.ComputeVisitorHash(s.salts.For(r.Context(), now), link.QRCodeID, f.ip, f.ua)
	okV, _, _, _ := s.limiter.Allow(r.Context(), "pw:"+link.QRCodeID.String()+":"+vh, 5, 15*time.Minute)
	okQ, _, _, _ := s.limiter.Allow(r.Context(), "pw:"+link.QRCodeID.String(), 100, 15*time.Minute)
	if !okV || !okQ {
		w.Header().Set("Retry-After", "900")
		s.renderPage(w, r, http.StatusTooManyRequests, pageData{Title: "Too many attempts",
			Message: "Please wait 15 minutes before trying again."})
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	pw := r.PostFormValue("password")
	ok := false
	if link.PasswordHash != nil && pw != "" {
		ok, _ = crypto.VerifyPassword(pw, *link.PasswordHash)
	}
	if !ok {
		s.emit(r, link, f, now, "password_fail", "")
		s.renderPage(w, r, http.StatusUnauthorized, pageData{Title: "Password required", Password: true,
			Error: "That password isn't right. Please try again."})
		return false
	}
	return true
}

// handleHosted serves /p/{code} (and .vcf / .ics downloads). It is reached after the scan
// was recorded on /{code}, so it records nothing itself.
func (s *Server) handleHosted(w http.ResponseWriter, r *http.Request) {
	dom := s.domainFor(r.Host)
	if dom == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	raw := chi.URLParam(r, "code")
	suffix := ""
	for _, ext := range []string{".vcf", ".ics"} {
		if strings.HasSuffix(strings.ToLower(raw), ext) {
			suffix, raw = ext, raw[:len(raw)-len(ext)]
		}
	}
	code := shortcode.Normalise(raw)
	if shortcode.Validate(code) != nil {
		s.notFound(w, r, dom)
		return
	}
	link, err := s.Resolver.Resolve(r.Context(), dom.ID, code)
	if err != nil {
		s.notFound(w, r, dom)
		return
	}
	outcome, _ := resolve.EvaluateState(link, time.Now().UTC())
	if outcome == resolve.OutcomePasswordRequired {
		s.redirectTo(w, r, "/"+code)
		return
	}
	if outcome != resolve.OutcomeActive || link.Version == nil || link.Version.Kind != "hosted_page" {
		s.redirectTo(w, r, "/"+code) // let the scan path explain (paused, expired, moved to a URL...)
		return
	}
	page, err := hosted.Parse(link.Version.HostedPage, func(_, u string) (string, error) { return u, nil })
	if err != nil {
		slog.Error("stored hosted page invalid", "qr", link.QRCodeID, "error", err)
		s.notFound(w, r, dom)
		return
	}
	switch {
	case suffix == ".vcf" && page.VCard != nil:
		w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="contact.vcf"`)
		_, _ = w.Write([]byte(page.VCard.VCF()))
		return
	case suffix == ".ics" && page.Event != nil:
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="event.ics"`)
		_, _ = w.Write([]byte(page.Event.ICS(link.QRCodeID.String())))
		return
	case suffix != "":
		s.notFound(w, r, dom)
		return
	}
	s.writeHosted(w, r, page, code)
}

func (s *Server) renderHosted(w http.ResponseWriter, r *http.Request, link *resolve.ResolvedLink, code string) {
	page, err := hosted.Parse(link.Version.HostedPage, func(_, u string) (string, error) { return u, nil })
	if err != nil {
		s.renderPage(w, r, http.StatusGone, pageData{Title: "Page unavailable", Message: "This page can't be shown."})
		return
	}
	s.writeHosted(w, r, page, "p/"+code)
}

func (s *Server) writeHosted(w http.ResponseWriter, r *http.Request, page *hosted.Page, code string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "private, no-store, max-age=0")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src https: data:; frame-ancestors 'none'; base-uri 'none'")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	// code is the relative prefix for the .vcf/.ics links ("CODE" on /p/CODE, "p/CODE" on /CODE).
	_ = hosted.Render(w, page, hosted.RenderOptions{Code: code, ReportURL: s.reportURL(r)})
}
