// Command api is the QRit control-plane HTTP API.
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

	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/email"
	"github.com/its-aryansingh/qrit/services/internal/httpapi"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	obs.InitLogger(cfg.LogLevel)
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	ropts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(ropts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		// Redis backs rate limits and link invalidation; run degraded rather than refuse to boot.
		slog.Warn("redis unavailable at startup", "error", err)
	}

	key, err := cfg.JWTPrivateKey()
	if err != nil {
		return err
	}
	tm, err := auth.NewTokenManager(cfg.JWTKeyID, key)
	if err != nil {
		return err
	}

	var sender email.Sender = &email.ConsoleSender{}
	if cfg.SMTPAddr != "" {
		sender = email.NewSMTPSender(cfg.SMTPAddr, cfg.EmailFrom)
	} else if !cfg.IsLocal() {
		slog.Warn("SMTP_ADDR is not set: transactional email is logged, not delivered")
	}
	var safety urlsafety.SafetyClient = urlsafety.NewFakeSafetyClient()
	if cfg.WebRiskKey != "" {
		safety = urlsafety.NewWebRiskClient(cfg.WebRiskKey)
	} else if !cfg.IsLocal() {
		slog.Warn("WEB_RISK_API_KEY is not set: destinations are not reputation-checked")
	}

	srv, err := httpapi.New(httpapi.Deps{Config: cfg, Pool: pool, Redis: rdb, Tokens: tm, Email: sender, Safety: safety})
	if err != nil {
		return err
	}
	if err := srv.Init(ctx); err != nil {
		return err
	}
	if err := mountEnterprise(ctx, cfg, srv); err != nil {
		return err
	}

	hs := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	errc := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	slog.Info("api shutting down")
	return hs.Shutdown(shutdownCtx)
}
