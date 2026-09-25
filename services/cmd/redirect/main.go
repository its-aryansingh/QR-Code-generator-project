package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/platform/redisx"
	"github.com/its-aryansingh/qrit/services/internal/resolve"
	"github.com/its-aryansingh/qrit/services/internal/scan"
	"github.com/its-aryansingh/qrit/services/internal/shortcode"
)

var statusTmpl = template.Must(template.New("status").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Title}} - QRit</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0a0a0c; color: #f4f4f5; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; padding: 20px; box-sizing: border-box; }
    .card { background: #18181b; border: 1px solid #27272a; border-radius: 12px; padding: 32px; max-width: 420px; width: 100%; text-align: center; box-shadow: 0 8px 30px rgba(0,0,0,0.4); }
    h1 { font-size: 20px; margin: 0 0 12px; color: #ffffff; }
    p { font-size: 14px; color: #a1a1aa; line-height: 1.5; margin: 0 0 24px; }
    .btn { display: inline-block; background: #2563eb; color: #fff; padding: 10px 20px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; border: none; cursor: pointer; }
    .btn:hover { background: #1d4ed8; }
    input[type="password"] { width: 100%; padding: 10px 12px; background: #09090b; border: 1px solid #3f3f46; border-radius: 6px; color: #fff; margin-bottom: 16px; box-sizing: border-box; font-size: 14px; }
    .error { color: #ef4444; font-size: 13px; margin-bottom: 12px; }
  </style>
</head>
<body>
  <div class="card">
    {{if .IsPassword}}
      <h1>Password Protected</h1>
      <p>Please enter the password to view this destination.</p>
      {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
      <form method="POST">
        <input type="password" name="password" placeholder="Enter password" required autofocus>
        <button type="submit" class="btn" style="width:100%">Continue</button>
      </form>
    {{else}}
      <h1>{{.Title}}</h1>
      <p>{{.Message}}</p>
      <a href="https://qrit.io" class="btn">Powered by QRit</a>
    {{end}}
  </div>
</body>
</html>`))

type Server struct {
	cfg        *config.Config
	pool       *pgxpool.Pool
	rdb        *redis.Client
	queries    *dbgen.Queries
	resolver   *resolve.Resolver
	domainLock sync.RWMutex
	domains    map[string]uuid.UUID // hostname -> domain_id
	saltSecret []byte
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	obs.InitLogger(cfg.LogLevel)

	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb, err := redisx.NewClient(context.Background(), cfg.RedisURL)
	if err != nil {
		slog.Warn("redis connection not available, operating without redis cache", "error", err)
	}

	queries := dbgen.New(pool)

	srv := &Server{
		cfg:        cfg,
		pool:       pool,
		rdb:        rdb,
		queries:    queries,
		domains:    make(map[string]uuid.UUID),
		saltSecret: []byte("qrit-daily-visitor-salt-secret-key-32b"),
	}

	// Initialize resolver
	fetchFunc := func(ctx context.Context, domainID uuid.UUID, code string) (*resolve.ResolvedLink, error) {
		shortCode := shortcode.Normalise(code)
		row, err := queries.GetResolvedLink(ctx, dbgen.GetResolvedLinkParams{
			DomainID:  pgtype.UUID{Bytes: domainID, Valid: true},
			ShortCode: &shortCode,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}

		var startsAt *time.Time
		if row.StartsAt.Valid {
			startsAt = &row.StartsAt.Time
		}
		var expiresAt *time.Time
		if row.ExpiresAt.Valid {
			expiresAt = &row.ExpiresAt.Time
		}

		var ver *resolve.ResolvedVersion
		if row.DestinationUrl != nil {
			ver = &resolve.ResolvedVersion{
				ID:    row.VersionID,
				No:    int(row.VersionNo),
				Kind:  row.DestinationKind,
				URL:   *row.DestinationUrl,
				Rules: row.Rules,
				UTM:   row.Utm,
			}
		}

		link := &resolve.ResolvedLink{
			QRCodeID:     row.QrCodeID,
			WorkspaceID:  row.WorkspaceID,
			DomainID:     domainID,
			Status:       row.QrStatus,
			Safety:       row.SafetyStatus,
			StartsAt:     startsAt,
			ExpiresAt:    expiresAt,
			ScanLimit:    row.ScanLimit,
			TotalScans:   row.TotalScans,
			HasPassword:  row.PasswordHash != nil && *row.PasswordHash != "",
			PasswordHash: row.PasswordHash,
			FallbackURL:  row.FallbackUrl,
			Timezone:     row.WorkspaceTimezone,
			Version:      ver,
			CachedAt:     time.Now().UTC(),
		}

		return link, nil
	}

	resolver, err := resolve.NewResolver(cfg.LRUSize, rdb, fetchFunc)
	if err != nil {
		slog.Error("failed to initialize resolver", "error", err)
		os.Exit(1)
	}
	srv.resolver = resolver

	// Pre-seed default domain
	srv.refreshDomains(context.Background())

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/{code}", srv.handleRedirect)
	r.Post("/{code}", srv.handlePasswordSubmit)

	httpServer := &http.Server{
		Addr:         ":8090",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("redirect service listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	slog.Info("redirect service gracefully stopped")
}

func (s *Server) refreshDomains(ctx context.Context) {
	domains, err := s.queries.ListActiveDomains(ctx)
	if err != nil {
		slog.Warn("could not list active domains", "error", err)
		return
	}
	s.domainLock.Lock()
	defer s.domainLock.Unlock()
	for _, d := range domains {
		s.domains[strings.ToLower(d.Hostname)] = d.ID
	}
}

func (s *Server) getDomainID(host string) (uuid.UUID, bool) {
	s.domainLock.RLock()
	defer s.domainLock.RUnlock()
	cleanHost := strings.ToLower(host)
	if idx := strings.Index(cleanHost, ":"); idx != -1 {
		cleanHost = cleanHost[:idx]
	}
	id, ok := s.domains[cleanHost]
	if !ok {
		// Fallback: check if we have any domain or match platform domain
		for _, v := range s.domains {
			return v, true
		}
	}
	return id, ok
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rawCode := chi.URLParam(r, "code")

	// Check if trailing + for preview
	isPreview := strings.HasSuffix(rawCode, "+")
	if isPreview {
		rawCode = strings.TrimSuffix(rawCode, "+")
	}

	code := shortcode.Normalise(rawCode)
	if err := shortcode.Validate(code); err != nil {
		s.renderStatus(w, http.StatusNotFound, "Not Found", "The requested QR code does not exist.")
		return
	}

	domainID, ok := s.getDomainID(r.Host)
	if !ok {
		s.renderStatus(w, http.StatusNotFound, "Domain Not Configured", "This domain is not configured on QRit.")
		return
	}

	link, err := s.resolver.Resolve(r.Context(), domainID, code)
	if err != nil || link == nil {
		s.renderStatus(w, http.StatusNotFound, "Not Found", "The requested QR code does not exist.")
		return
	}

	outcome, target := resolve.EvaluateState(link, time.Now().UTC())

	dur := float64(time.Since(start).Microseconds()) / 1000.0
	w.Header().Set("Server-Timing", fmt.Sprintf("resolve;dur=%.2f", dur))

	// If preview requested (+), show preview page without counting scan
	if isPreview {
		s.renderStatus(w, http.StatusOK, "QR Code Preview", fmt.Sprintf("Points to: %s", target))
		return
	}

	// Dispatch scan event asynchronously
	s.emitScan(r, link, string(outcome))

	switch outcome {
	case resolve.OutcomeBlocked:
		s.renderStatus(w, http.StatusGone, "Link Disabled", "This link has been disabled for violating our terms of service.")
	case resolve.OutcomePaused:
		if target != "" {
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			s.renderStatus(w, http.StatusGone, "Link Inactive", "This QR code is currently paused.")
		}
	case resolve.OutcomeNotStarted:
		if target != "" {
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			s.renderStatus(w, http.StatusNotFound, "Not Active Yet", "This QR code has not started yet.")
		}
	case resolve.OutcomeExpired:
		if target != "" {
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			s.renderStatus(w, http.StatusGone, "Link Expired", "This QR code has expired.")
		}
	case resolve.OutcomeLimitReached:
		if target != "" {
			http.Redirect(w, r, target, http.StatusFound)
		} else {
			s.renderStatus(w, http.StatusGone, "Limit Reached", "This QR code has reached its scan limit.")
		}
	case resolve.OutcomePasswordRequired:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = statusTmpl.Execute(w, map[string]interface{}{"IsPassword": true})
	case resolve.OutcomeActive:
		w.Header().Set("Cache-Control", "private, no-store, max-age=0")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		http.Redirect(w, r, target, http.StatusFound)
	}
}

func (s *Server) handlePasswordSubmit(w http.ResponseWriter, r *http.Request) {
	rawCode := chi.URLParam(r, "code")
	code := shortcode.Normalise(rawCode)
	domainID, _ := s.getDomainID(r.Host)

	link, err := s.resolver.Resolve(r.Context(), domainID, code)
	if err != nil || link == nil || link.PasswordHash == nil {
		s.renderStatus(w, http.StatusNotFound, "Not Found", "QR code not found.")
		return
	}

	pass := r.FormValue("password")
	ok, err := crypto.VerifyPassword(pass, *link.PasswordHash)
	if err != nil || !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = statusTmpl.Execute(w, map[string]interface{}{
			"IsPassword": true,
			"Error":      "Incorrect password. Please try again.",
		})
		return
	}

	target := ""
	if link.Version != nil {
		target = link.Version.URL
	}
	s.emitScan(r, link, "password_ok")
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *Server) emitScan(r *http.Request, link *resolve.ResolvedLink, outcome string) {
	if s.rdb == nil {
		return
	}

	ip, ua, lang, ref := scan.ParseRequestFacts(r)
	todaySalt := scan.SaltForDate(s.saltSecret, time.Now().UTC())
	vh := scan.ComputeVisitorHash(todaySalt, link.QRCodeID, ip, ua)

	verID := ""
	if link.Version != nil {
		verID = link.Version.ID.String()
	}

	event := scan.NewScanEvent(
		time.Now().UTC(),
		link.WorkspaceID.String(),
		link.QRCodeID.String(),
		verID,
		link.DomainID.String(),
		nil,
		nil,
		outcome,
		r.Method,
		vh,
		ua,
		false,
		scan.GeoFacts{},
		lang,
		ref,
		nil,
	)

	go func() {
		bytes, err := json.Marshal(event)
		if err != nil {
			return
		}
		_ = s.rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: "scans",
			MaxLen: 5000000,
			Approx: true,
			Values: map[string]interface{}{"e": string(bytes)},
		}).Err()
	}()
}

func (s *Server) renderStatus(w http.ResponseWriter, code int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = statusTmpl.Execute(w, map[string]interface{}{
		"Title":   title,
		"Message": message,
	})
}
