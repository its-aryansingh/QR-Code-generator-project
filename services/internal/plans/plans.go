// Package plans resolves effective entitlements: the organisation's plan, merged with its
// active enterprise contract (limit and feature overrides), minus features a workspace
// policy switched off.
package plans

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
)

// Effective is what an organisation (or a workspace in it) may use.
type Effective struct {
	Plan       entitlements.Plan   `json:"plan"`
	Limits     entitlements.Limits `json:"limits"`
	Features   []string            `json:"features"`
	ContractID *uuid.UUID          `json:"contract_id,omitempty"`
	features   map[string]bool
}

// Has reports whether a feature is enabled.
func (e *Effective) Has(f string) bool { return e.features[f] }

type Service struct {
	pool  *pgxpool.Pool
	cache *expirable.LRU[string, *Effective]
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, cache: expirable.NewLRU[string, *Effective](10000, nil, 60*time.Second)}
}

// Invalidate drops cached entitlements (after plan, contract or policy changes).
func (s *Service) Invalidate() { s.cache.Purge() }

// contractOverride is the limits_override JSON of a contract.
type contractOverride struct {
	entitlements.Limits
	Features         []string `json:"features"`
	DisabledFeatures []string `json:"disabled_features"`
}

// ForOrg resolves plan ⊕ active contract.
func (s *Service) ForOrg(ctx context.Context, orgID uuid.UUID) (*Effective, error) {
	key := orgID.String()
	if e, ok := s.cache.Get(key); ok {
		return e, nil
	}
	var planID string
	if err := s.pool.QueryRow(ctx, `SELECT plan_id FROM organizations WHERE id = $1`, orgID).Scan(&planID); err != nil {
		return nil, err
	}
	plan := entitlements.NormalisePlan(planID)
	e := &Effective{Plan: plan, Limits: entitlements.GetLimits(planID), features: map[string]bool{}}
	for _, f := range entitlements.AllFeatures {
		if entitlements.HasFeature(planID, f) {
			e.features[f] = true
		}
	}
	var (
		cid      uuid.UUID
		seats    int
		override []byte
	)
	err := s.pool.QueryRow(ctx, `SELECT id, seats, limits_override FROM contracts
		WHERE org_id = $1 AND status = 'active' AND starts_on <= current_date AND ends_on >= current_date`, orgID).
		Scan(&cid, &seats, &override)
	switch {
	case err == nil:
		e.ContractID = &cid
		var o contractOverride
		if len(override) > 0 {
			// Unmarshal over the plan limits: only keys present in the override change.
			o.Limits = e.Limits
			if err := json.Unmarshal(override, &o); err == nil {
				e.Limits = o.Limits
			}
		}
		if seats > 0 {
			e.Limits.Seats = seats
		}
		for _, f := range o.Features {
			e.features[f] = true
		}
		for _, f := range o.DisabledFeatures {
			delete(e.features, f)
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	e.Features = sortedKeys(e.features)
	s.cache.Add(key, e)
	return e, nil
}

// ForWorkspace applies the workspace policy's disabled features on top of the org.
func (s *Service) ForWorkspace(ctx context.Context, ws dbgen.Workspace) (*Effective, error) {
	key := "ws:" + ws.ID.String()
	if e, ok := s.cache.Get(key); ok {
		return e, nil
	}
	base, err := s.ForOrg(ctx, ws.OrgID)
	if err != nil {
		return nil, err
	}
	var disabled []string
	err = s.pool.QueryRow(ctx, `SELECT disabled_features FROM workspace_policies WHERE workspace_id = $1`, ws.ID).Scan(&disabled)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	e := &Effective{Plan: base.Plan, Limits: base.Limits, ContractID: base.ContractID, features: map[string]bool{}}
	for f := range base.features {
		e.features[f] = true
	}
	for _, f := range disabled {
		delete(e.features, f)
	}
	e.Features = sortedKeys(e.features)
	s.cache.Add(key, e)
	return e, nil
}

// CheckFeature returns an entitlement error when the workspace cannot use f.
func (s *Service) CheckFeature(ctx context.Context, ws dbgen.Workspace, f string) error {
	e, err := s.ForWorkspace(ctx, ws)
	if err != nil {
		return err
	}
	if e.Has(f) {
		return nil
	}
	if entitlements.HasFeature(string(e.Plan), f) {
		// Available on the plan but switched off for this workspace by an admin.
		return &entitlements.EntitlementError{Code: "feature_disabled", Message: "feature \"" + f + "\" is disabled for this workspace by an administrator"}
	}
	return entitlements.ErrUpgradeRequired(f, entitlements.RequiredPlan(f))
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
