// Package worker holds the periodic maintenance tasks and on-demand job handlers run by
// cmd/worker.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/ingest"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/resolve"
	"github.com/its-aryansingh/qrit/services/internal/urlsafety"
)

// Deps are shared by tasks.
type Deps struct {
	Pool   *pgxpool.Pool
	Redis  *redis.Client
	Safety urlsafety.SafetyClient
	Logger *slog.Logger
	Now    func() time.Time
}

func (d *Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

func (d *Deps) log() *slog.Logger {
	if d.Logger == nil {
		return slog.Default()
	}
	return d.Logger
}

// Register adds the core maintenance tasks to the scheduler.
func Register(s *jobs.Scheduler, d *Deps) {
	s.Add(jobs.Task{Name: "partitions", DailyAtUTC: jobs.Daily(1, 0), Timeout: 20 * time.Minute, Run: d.Partitions})
	s.Add(jobs.Task{Name: "partitions.boot", Every: 6 * time.Hour, Run: d.EnsurePartitions})
	s.Add(jobs.Task{Name: "versions.activate", Every: 30 * time.Second, Timeout: 2 * time.Minute, Run: d.ActivateScheduledVersions})
	s.Add(jobs.Task{Name: "safety.pending", Every: 10 * time.Minute, Timeout: 10 * time.Minute, Run: d.RecheckPendingSafety})
	s.Add(jobs.Task{Name: "safety.rescan", DailyAtUTC: jobs.Daily(3, 0), Timeout: time.Hour, Run: d.RescanActiveDestinations})
	s.Add(jobs.Task{Name: "rollups.reconcile", DailyAtUTC: jobs.Daily(2, 0), Timeout: time.Hour, Run: d.ReconcileYesterday})
	s.Add(jobs.Task{Name: "qr.purge_deleted", DailyAtUTC: jobs.Daily(4, 0), Run: d.PurgeDeletedCodes})
	s.Add(jobs.Task{Name: "plans.read_only", Every: time.Hour, Run: d.EnforcePlanReadOnly})
	s.Add(jobs.Task{Name: "cleanup", DailyAtUTC: jobs.Daily(5, 0), Run: d.Cleanup})
}

var partitionName = regexp.MustCompile(`^scan_events_(\d{4})_(\d{2})$`)

// EnsurePartitions creates monthly scan_events partitions from last month to three months
// ahead. Rows that landed in the default partition for a new month are moved into it.
func (d *Deps) EnsurePartitions(ctx context.Context) error {
	now := d.now().UTC()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for i := -1; i <= 3; i++ {
		if err := d.ensureMonth(ctx, first.AddDate(0, i, 0)); err != nil {
			return err
		}
	}
	return nil
}

func (d *Deps) ensureMonth(ctx context.Context, from time.Time) error {
	to := from.AddDate(0, 1, 0)
	name := fmt.Sprintf("scan_events_%04d_%02d", from.Year(), int(from.Month()))
	var attached bool
	if err := d.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'scan_events'::regclass AND c.relname = $1)`, name).Scan(&attached); err != nil {
		return err
	}
	if attached {
		return nil
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (LIKE scan_events INCLUDING DEFAULTS INCLUDING CONSTRAINTS)`, name),
		fmt.Sprintf(`WITH moved AS (DELETE FROM scan_events_default WHERE occurred_at >= '%s' AND occurred_at < '%s' RETURNING *)
			INSERT INTO %s SELECT * FROM moved`, from.Format(time.RFC3339), to.Format(time.RFC3339), name),
		fmt.Sprintf(`ALTER TABLE scan_events ATTACH PARTITION %s FOR VALUES FROM ('%s') TO ('%s')`,
			name, from.Format(time.RFC3339), to.Format(time.RFC3339)),
	}
	for _, q := range stmts {
		if _, err := tx.Exec(ctx, q); err != nil {
			return fmt.Errorf("partition %s: %w", name, err)
		}
	}
	d.log().Info("created scan_events partition", "partition", name)
	return tx.Commit(ctx)
}

// Partitions is the daily maintenance run: ensure partitions, drop those older than 13
// months, prune the unique-visitor ledger and expired idempotency keys.
func (d *Deps) Partitions(ctx context.Context) error {
	if err := d.EnsurePartitions(ctx); err != nil {
		return err
	}
	cutoff := time.Date(d.now().UTC().Year(), d.now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -13, 0)
	rows, err := d.Pool.Query(ctx, `SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'scan_events'::regclass`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil {
			names = append(names, n)
		}
	}
	rows.Close()
	for _, n := range names {
		m := partitionName.FindStringSubmatch(n)
		if m == nil {
			continue
		}
		var y, mo int
		_, _ = fmt.Sscanf(m[1]+" "+m[2], "%d %d", &y, &mo)
		if time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0).After(cutoff) {
			continue
		}
		if _, err := d.Pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE scan_events DETACH PARTITION %s`, n)); err != nil {
			return err
		}
		if _, err := d.Pool.Exec(ctx, fmt.Sprintf(`DROP TABLE %s`, n)); err != nil {
			return err
		}
		d.log().Info("dropped expired scan_events partition", "partition", n)
	}
	if _, err := d.Pool.Exec(ctx, `DELETE FROM scan_visitors_daily WHERE day < (now() AT TIME ZONE 'UTC')::date - 2`); err != nil {
		return err
	}
	if _, err := d.Pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE expires_at <= now()`); err != nil {
		return err
	}
	var stray int64
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM scan_events_default`).Scan(&stray); err == nil && stray > 0 {
		d.log().Warn("scan events in the default partition", "rows", stray)
	}
	return nil
}

func publishInvalidate(ctx context.Context, rdb *redis.Client, domainID uuid.UUID, code string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(ctx, fmt.Sprintf("link:v1:%s:%s", domainID, code)).Err()
	_ = rdb.Publish(ctx, resolve.InvalidateChannel, domainID.String()+":"+code).Err()
}

// ActivateScheduledVersions moves the denormalised current_version_id pointer when a
// scheduled version becomes effective and evicts cached links. (The redirect already
// honours effective_at through next_change_at; this keeps lists and webhooks exact.)
func (d *Deps) ActivateScheduledVersions(ctx context.Context) error {
	q := dbgen.New(d.Pool)
	due, err := q.ScheduledVersionsDue(ctx)
	if err != nil {
		return err
	}
	for _, v := range due {
		err := q.SetQRCodeCurrentVersion(ctx, dbgen.SetQRCodeCurrentVersionParams{
			VersionID: pgUUID(v.VersionID), ID: v.QrCodeID, WorkspaceID: v.WorkspaceID})
		if err != nil {
			return err
		}
		if _, err := jobs.Enqueue(ctx, d.Pool, "event.fanout", map[string]any{
			"workspace_id": v.WorkspaceID, "event": "qr.version.activated",
			"data": map[string]any{"qr_code_id": v.QrCodeID, "version_id": v.VersionID}}, jobs.Options{WorkspaceID: &v.WorkspaceID}); err != nil {
			d.log().Warn("enqueue activation event", "error", err)
		}
		if v.DomainID.Valid && v.ShortCode != nil {
			publishInvalidate(ctx, d.Redis, uuid.UUID(v.DomainID.Bytes), *v.ShortCode)
		}
	}
	return nil
}

type safetyTarget struct {
	versionID, codeID, workspaceID uuid.UUID
	domainID                       *uuid.UUID
	shortCode                      *string
	url                            string
}

func (d *Deps) loadTargets(ctx context.Context, sql string, args ...any) ([]safetyTarget, error) {
	rows, err := d.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []safetyTarget
	for rows.Next() {
		var t safetyTarget
		if err := rows.Scan(&t.versionID, &t.codeID, &t.workspaceID, &t.domainID, &t.shortCode, &t.url); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// currentVersionsSQL selects each live code's version in effect now.
const currentVersionsSQL = `
SELECT v.id, q.id, q.workspace_id, q.domain_id, q.short_code, v.destination_url
FROM qr_codes q
JOIN LATERAL (SELECT * FROM qr_versions v1 WHERE v1.qr_code_id = q.id AND v1.effective_at <= now()
              ORDER BY v1.effective_at DESC, v1.version_no DESC LIMIT 1) v ON true
WHERE q.mode = 'dynamic' AND q.deleted_at IS NULL AND q.status <> 'blocked' AND v.destination_url LIKE 'http%' `

// RecheckPendingSafety re-asks the reputation service about destinations that could not
// be checked when they were saved.
func (d *Deps) RecheckPendingSafety(ctx context.Context) error {
	ts, err := d.loadTargets(ctx, currentVersionsSQL+` AND v.safety_status = 'pending' LIMIT 500`)
	if err != nil {
		return err
	}
	return d.applyVerdicts(ctx, ts, true)
}

// RescanActiveDestinations re-checks active destinations daily: a site that turned
// malicious after the code was printed gets the code blocked.
func (d *Deps) RescanActiveDestinations(ctx context.Context) error {
	ts, err := d.loadTargets(ctx, currentVersionsSQL+` AND q.status = 'active' ORDER BY q.last_scanned_at DESC NULLS LAST LIMIT 5000`)
	if err != nil {
		return err
	}
	return d.applyVerdicts(ctx, ts, false)
}

func (d *Deps) applyVerdicts(ctx context.Context, ts []safetyTarget, markSafe bool) error {
	if d.Safety == nil {
		return nil
	}
	for _, t := range ts {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		verdict, err := d.Safety.Check(cctx, t.url)
		cancel()
		if err != nil {
			continue
		}
		switch verdict {
		case urlsafety.VerdictSafe:
			if markSafe {
				if _, err := d.Pool.Exec(ctx, `UPDATE qr_versions SET safety_status = 'safe' WHERE id = $1`, t.versionID); err != nil {
					return err
				}
				if _, err := d.Pool.Exec(ctx, `UPDATE qr_codes SET safety_status = 'safe' WHERE id = $1 AND safety_status = 'pending'`, t.codeID); err != nil {
					return err
				}
			}
		case urlsafety.VerdictUnsafe:
			if err := d.blockUnsafe(ctx, t); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Deps) blockUnsafe(ctx context.Context, t safetyTarget) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE qr_versions SET safety_status = 'flagged' WHERE id = $1`, t.versionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE qr_codes SET safety_status = 'blocked', updated_at = now() WHERE id = $1`, t.codeID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (workspace_id, actor_type, action, target_type, target_id, changes)
		VALUES ($1, 'system', 'qr.safety.blocked', 'qr_code', $2, jsonb_build_object('destination', $3::text))`,
		t.workspaceID, t.codeID, t.url); err != nil {
		return err
	}
	if _, err := jobs.Enqueue(ctx, tx, "event.fanout", map[string]any{"workspace_id": t.workspaceID, "event": "qr.safety.blocked",
		"data": map[string]any{"qr_code_id": t.codeID, "destination": t.url}}, jobs.Options{WorkspaceID: &t.workspaceID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	d.log().Warn("blocked code with unsafe destination", "qr", t.codeID)
	if t.domainID != nil && t.shortCode != nil {
		publishInvalidate(ctx, d.Redis, *t.domainID, *t.shortCode)
	}
	return nil
}

// ReconcileYesterday rebuilds yesterday's rollups from raw events (heals any drift).
func (d *Deps) ReconcileYesterday(ctx context.Context) error {
	return ingest.Rebuild(ctx, d.Pool, d.now().UTC().Add(-24*time.Hour))
}

// PurgeDeletedCodes hard-deletes codes soft-deleted more than 30 days ago; their short
// codes are tombstoned so they can never be reissued.
func (d *Deps) PurgeDeletedCodes(ctx context.Context) error {
	for {
		tag, err := d.Pool.Exec(ctx, `
			WITH gone AS (
			    SELECT id, domain_id, short_code FROM qr_codes
			    WHERE deleted_at < now() - interval '30 days' LIMIT 500 FOR UPDATE SKIP LOCKED
			), tomb AS (
			    INSERT INTO short_code_tombstones (domain_id, short_code)
			    SELECT domain_id, short_code FROM gone WHERE short_code IS NOT NULL AND domain_id IS NOT NULL
			    ON CONFLICT DO NOTHING
			)
			DELETE FROM qr_codes WHERE id IN (SELECT id FROM gone)`)
		if err != nil {
			return err
		}
		if tag.RowsAffected() < 500 {
			return nil
		}
	}
}

// EnforcePlanReadOnly marks dynamic codes beyond the plan's quota read-only (newest
// first) and lifts the flag when the workspace is back under its limit. Read-only codes
// keep redirecting; only edits are refused.
func (d *Deps) EnforcePlanReadOnly(ctx context.Context) error {
	var vals []string
	for plan, l := range entitlements.PlanLimits {
		vals = append(vals, fmt.Sprintf("('%s', %d)", plan, l.DynamicCodes))
	}
	_, err := d.Pool.Exec(ctx, `
		WITH limits(plan_id, lim) AS (VALUES `+strings.Join(vals, ",")+`),
		ranked AS (
		    SELECT q.id, row_number() OVER (PARTITION BY q.workspace_id ORDER BY q.created_at, q.id) AS rn,
		           COALESCE(l.lim, (SELECT lim FROM limits WHERE plan_id = 'free')) AS lim, q.is_read_only
		    FROM qr_codes q JOIN workspaces w ON w.id = q.workspace_id
		    LEFT JOIN limits l ON l.plan_id = w.plan_id
		    WHERE q.mode = 'dynamic' AND q.deleted_at IS NULL
		)
		UPDATE qr_codes q SET is_read_only = (r.rn > r.lim), updated_at = now()
		FROM ranked r WHERE q.id = r.id AND q.is_read_only <> (r.rn > r.lim)`)
	return err
}

// Cleanup removes expired sessions, used tokens and old finished jobs.
func (d *Deps) Cleanup(ctx context.Context) error {
	for _, q := range []string{
		`DELETE FROM sessions WHERE expires_at < now() - interval '7 days' OR revoked_at < now() - interval '7 days'`,
		`DELETE FROM email_tokens WHERE expires_at < now() - interval '7 days'`,
		`DELETE FROM invites WHERE expires_at < now() - interval '90 days' AND accepted_at IS NULL`,
	} {
		if _, err := d.Pool.Exec(ctx, q); err != nil {
			return err
		}
	}
	_, err := jobs.Prune(ctx, d.Pool, 7*24*time.Hour)
	return err
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }
