package main

import (
	"context"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/sso"
	"github.com/its-aryansingh/qrit/services/internal/worker"
)

// registerExtensions adds job handlers and periodic tasks of the enterprise modules.
func registerExtensions(ctx context.Context, cfg *config.Config, d *worker.Deps, s *jobs.Scheduler, r *jobs.Runner) error {
	worker.RegisterIdentity(s, d, sso.DoH{Endpoint: cfg.DoHURL, Client: sso.NewHTTPClient(false)})
	return nil
}
