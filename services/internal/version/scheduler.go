package version

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Scheduler struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

func NewScheduler(pool *pgxpool.Pool, rdb *redis.Client) *Scheduler {
	return &Scheduler{
		pool: pool,
		rdb:  rdb,
	}
}

// ActivateScheduledVersion sets current_version_id on qr_codes and invalidates the resolution cache.
func (s *Scheduler) ActivateScheduledVersion(ctx context.Context, qrID, versionID uuid.UUID, shortCode string) error {
	if s.pool != nil {
		query := `UPDATE qr_codes SET current_version_id = $1, updated_at = now() WHERE id = $2`
		_, err := s.pool.Exec(ctx, query, versionID, qrID)
		if err != nil {
			return fmt.Errorf("update current version: %w", err)
		}
	}

	// Invalidate resolution cache in Redis
	if s.rdb != nil && shortCode != "" {
		cacheKey := fmt.Sprintf("res:%s", shortCode)
		_ = s.rdb.Del(ctx, cacheKey).Err()
	}

	return nil
}

// CheckDueVersions scans for versions where effective_at <= now that need activation.
func (s *Scheduler) CheckDueVersions(ctx context.Context, now time.Time) (int, error) {
	if s.pool == nil {
		return 0, nil
	}

	query := `
		SELECT q.id, v.id, q.short_code
		FROM qr_versions v
		JOIN qr_codes q ON q.id = v.qr_code_id
		WHERE v.effective_at <= $1
		  AND (q.current_version_id IS NULL OR q.current_version_id != v.id)
		ORDER BY v.effective_at DESC
		LIMIT 100
	`
	rows, err := s.pool.Query(ctx, query, now)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var qrID, versionID uuid.UUID
		var shortCode string
		if err := rows.Scan(&qrID, &versionID, &shortCode); err == nil {
			_ = s.ActivateScheduledVersion(ctx, qrID, versionID, shortCode)
			count++
		}
	}

	return count, nil
}
