package main

import (
	"context"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/httpapi"
)

// mountEnterprise wires organisation, identity and governance modules into the API.
func mountEnterprise(ctx context.Context, cfg *config.Config, srv *httpapi.Server) error {
	return nil
}
