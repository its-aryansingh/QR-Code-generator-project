// Package flags reads feature flags: a per-organisation value overrides the global
// default (org_id NULL); unknown flags are off.
package flags

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Flags struct {
	pool  *pgxpool.Pool
	cache *expirable.LRU[string, map[string]bool]
}

func New(pool *pgxpool.Pool) *Flags {
	return &Flags{pool: pool, cache: expirable.NewLRU[string, map[string]bool](5000, nil, 30*time.Second)}
}

// All returns every flag's effective value for the organisation.
func (f *Flags) All(ctx context.Context, org uuid.UUID) (map[string]bool, error) {
	if m, ok := f.cache.Get(org.String()); ok {
		return m, nil
	}
	rows, err := f.pool.Query(ctx, `SELECT DISTINCT ON (key) key, enabled FROM feature_flags
		WHERE org_id IS NULL OR org_id = $1 ORDER BY key, org_id NULLS LAST`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]bool{}
	for rows.Next() {
		var k string
		var on bool
		if err := rows.Scan(&k, &on); err != nil {
			return nil, err
		}
		m[k] = on
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	f.cache.Add(org.String(), m)
	return m, nil
}

// Enabled reports one flag (false on errors: flags gate new behaviour, so fail closed).
func (f *Flags) Enabled(ctx context.Context, key string, org uuid.UUID) bool {
	m, err := f.All(ctx, org)
	return err == nil && m[key]
}

// Set upserts a flag (org nil = global default).
func (f *Flags) Set(ctx context.Context, key string, org *uuid.UUID, enabled bool, by *uuid.UUID) error {
	_, err := f.pool.Exec(ctx, `INSERT INTO feature_flags (key, org_id, enabled, updated_by) VALUES ($1, $2, $3, $4)
		ON CONFLICT (key, org_id) DO UPDATE SET enabled = EXCLUDED.enabled, updated_by = EXCLUDED.updated_by, updated_at = now()`,
		key, org, enabled, by)
	f.cache.Purge()
	return err
}
