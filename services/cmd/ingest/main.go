package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/ingest"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/platform/redisx"
	"github.com/its-aryansingh/qrit/services/internal/realtime"
)

type Config struct {
	AppEnv       string        `env:"APP_ENV" envDefault:"local"`
	LogLevel     string        `env:"LOG_LEVEL" envDefault:"info"`
	DatabaseURL  string        `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/qrit?sslmode=disable"`
	RedisURL     string        `env:"REDIS_URL" envDefault:"redis://localhost:6379/0"`
	BatchSize    int64         `env:"INGEST_BATCH_SIZE" envDefault:"1000"`
	BatchWait    time.Duration `env:"INGEST_BATCH_WAIT" envDefault:"1s"`
	ConsumerName string        `env:"INGEST_CONSUMER_NAME"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		slog.Error("Failed to parse ingest configuration", "err", err)
		os.Exit(1)
	}

	logger := obs.InitLogger(cfg.LogLevel)
	logger.Info("Starting QRit Ingest Service", "env", cfg.AppEnv)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pgPool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("Database connection failed, running in resilient mode", "err", err)
	}
	if pgPool != nil {
		defer pgPool.Close()
	}

	rdb, err := redisx.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		logger.Error("Failed to connect to Redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	rt := realtime.New(rdb)

	consumerName := cfg.ConsumerName
	if consumerName == "" {
		host, _ := os.Hostname()
		consumerName = "ingest-" + host
	}

	batchCfg := ingest.BatchConfig{
		StreamName:   "scans",
		GroupName:    "ingest",
		ConsumerName: consumerName,
		BatchSize:    cfg.BatchSize,
		BatchWait:    cfg.BatchWait,
	}

	consumer := ingest.NewConsumer(pgPool, rdb, rt, batchCfg, logger)
	if err := consumer.InitGroup(ctx); err != nil {
		logger.Warn("Consumer group initialization error", "err", err)
	}

	logger.Info("Ingest consumer loop running", "group", batchCfg.GroupName, "consumer", batchCfg.ConsumerName)

	ticker := time.NewTicker(cfg.BatchWait)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Ingest service shutting down gracefully")
			return
		case <-ticker.C:
			streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    batchCfg.GroupName,
				Consumer: batchCfg.ConsumerName,
				Streams:  []string{batchCfg.StreamName, ">"},
				Count:    batchCfg.BatchSize,
				Block:    batchCfg.BatchWait,
			}).Result()

			if err != nil && err != redis.Nil {
				if ctx.Err() == nil {
					logger.Debug("XReadGroup tick", "err", err)
				}
				continue
			}

			for _, s := range streams {
				if len(s.Messages) > 0 {
					n, err := consumer.ProcessBatch(ctx, s.Messages)
					if err != nil {
						logger.Error("Failed to process batch", "err", err)
					} else {
						logger.Info("Processed scan batch", "count", n)
					}
				}
			}
		}
	}
}
