// Command ingest consumes the "scans" stream into Postgres.
//
//	ingest                         run the consumer
//	ingest rebuild --day 2026-09-25 recompute one UTC day's rollups from scan_events
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/its-aryansingh/qrit/services/internal/ingest"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/platform/redisx"
)

type Config struct {
	AppEnv       string        `env:"APP_ENV" envDefault:"local"`
	LogLevel     string        `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL  string        `env:"DATABASE_URL,required"`
	RedisURL     string        `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`
	BatchSize    int64         `env:"INGEST_BATCH_SIZE" envDefault:"1000"`
	BatchWait    time.Duration `env:"INGEST_BATCH_WAIT" envDefault:"1s"`
	ConsumerName string        `env:"INGEST_CONSUMER_NAME"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("ingest exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return err
	}
	logger := obs.InitLogger(cfg.LogLevel)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if len(os.Args) > 1 && os.Args[1] == "rebuild" {
		fs := flag.NewFlagSet("rebuild", flag.ExitOnError)
		day := fs.String("day", "", "UTC day to rebuild (YYYY-MM-DD)")
		_ = fs.Parse(os.Args[2:])
		d, err := time.Parse("2006-01-02", *day)
		if err != nil {
			return fmt.Errorf("--day must be YYYY-MM-DD: %w", err)
		}
		if err := ingest.Rebuild(ctx, pool, d); err != nil {
			return err
		}
		logger.Info("rollups rebuilt", "day", *day)
		return nil
	}

	rdb, err := redisx.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	c := ingest.NewConsumer(pool, rdb, ingest.Config{
		Consumer: cfg.ConsumerName, BatchSize: cfg.BatchSize, BlockWait: cfg.BatchWait,
	}, logger)
	logger.Info("ingest consuming", "stream", "scans", "group", "ingest")
	return c.Run(ctx)
}
