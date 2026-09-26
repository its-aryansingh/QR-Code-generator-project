// Command worker runs periodic maintenance and the durable job queue.
//
//	worker                 run scheduler + job runner
//	worker run-task NAME   run one periodic task now (e.g. partitions, rollups.reconcile)
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/platform/db"
	"github.com/its-aryansingh/qrit/services/internal/platform/obs"
	"github.com/its-aryansingh/qrit/services/internal/platform/redisx"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
	"github.com/its-aryansingh/qrit/services/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := obs.InitLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	rdb, err := redisx.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		logger.Warn("redis unavailable; cache invalidation is skipped", "error", err)
	}
	var safety urlsafety.SafetyClient
	if cfg.WebRiskKey != "" {
		safety = urlsafety.NewWebRiskClient(cfg.WebRiskKey)
	} else if cfg.IsLocal() {
		safety = urlsafety.NewFakeSafetyClient()
	}

	deps := &worker.Deps{Pool: pool, Redis: rdb, Safety: safety, Logger: logger}
	sched := jobs.NewScheduler(pool, logger)
	worker.Register(sched, deps)
	runner := jobs.NewRunner(pool, cfg.WorkerConcurrency, logger)
	if err := registerExtensions(ctx, cfg, deps, sched, runner); err != nil {
		return err
	}

	if len(os.Args) > 2 && os.Args[1] == "run-task" {
		if err := sched.RunNow(ctx, os.Args[2]); err != nil {
			return fmt.Errorf("task %s: %w", os.Args[2], err)
		}
		logger.Info("task finished", "task", os.Args[2])
		return nil
	}

	logger.Info("worker started", "concurrency", cfg.WorkerConcurrency)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); runner.Run(ctx) }()
	go func() {
		defer wg.Done()
		if err := sched.Run(ctx); err != nil {
			logger.Error("scheduler stopped", "error", err)
			stop()
		}
	}()
	wg.Wait()
	logger.Info("worker stopped")
	return nil
}
