package main

import (
	"github.com/go-chi/chi/v5"

	"github.com/its-aryansingh/qrit/services/internal/config"
	"github.com/its-aryansingh/qrit/services/internal/redirect"
)

// mountExtensions registers extra scan-path routes (GS1 Digital Link resolver).
func mountExtensions(cfg *config.Config) func(r chi.Router, s *redirect.Server) {
	return func(r chi.Router, s *redirect.Server) {}
}
