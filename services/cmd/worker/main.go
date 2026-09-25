package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
)

type Config struct {
	AppEnv      string `env:"APP_ENV" envDefault:"local"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL string `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/qrit?sslmode=disable"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Failed to parse worker configuration", "err", err)
		os.Exit(1)
	}

	logger := obs.InitLogger(cfg.LogLevel)
	logger.Info("Starting QRit Background Worker", "env", cfg.AppEnv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pgPool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("Database connection failed, worker running in offline check mode", "err", err)
	}
	if pgPool != nil {
		defer pgPool.Close()
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	logger.Info("Worker jobs loop active")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Worker shutting down gracefully")
			return
		case <-ticker.C:
			logger.Info("Executing periodic partition maintenance and reconciliation")
		}
	}
}
