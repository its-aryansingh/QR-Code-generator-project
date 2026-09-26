// Command redirect serves short links (GET|HEAD|POST /{code}) and hosted pages.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/platform/redisx"
	"github.com/its-aryansingh/qrit/services/internal/redirect"
)

func main() {
	if err := run(); err != nil {
		slog.Error("redirect exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	obs.InitLogger(cfg.LogLevel)
	addr := os.Getenv("REDIRECT_HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	rdb, err := redisx.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		// Scans keep working from Postgres + LRU; events are buffered until Redis returns.
		slog.Warn("redis unavailable at startup", "error", err)
	}
	trusted, err := cfg.TrustedProxies()
	if err != nil {
		return err
	}
	srv, err := redirect.New(redirect.Options{
		Pool: pool, Redis: rdb, TrustedProxies: trusted, AppBaseURL: cfg.AppBaseURL,
		LRUSize: cfg.LRUSize, LRUTTL: cfg.LRUTTLDuration(), EventBuffer: cfg.EventBuffer,
		Mount: mountExtensions(cfg),
	})
	if err != nil {
		return err
	}
	bg, cancelBG := context.WithCancel(context.Background())
	if err := srv.Start(bg); err != nil {
		cancelBG()
		return err
	}

	hs := &http.Server{
		Addr: addr, Handler: srv.Handler(),
		ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 15,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("redirect listening", "addr", addr)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		cancelBG()
		return err
	case <-ctx.Done():
	}
	// Stop accepting, then flush buffered scan events (up to 10 s) before exiting.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = hs.Shutdown(shutdownCtx)
	cancelBG()
	srv.Emitter.Wait()
	slog.Info("redirect stopped", "events_sent", srv.Emitter.Sent.Load(), "events_dropped", srv.Emitter.Dropped.Load())
	return nil
}
