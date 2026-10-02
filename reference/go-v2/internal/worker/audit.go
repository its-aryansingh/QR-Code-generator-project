package worker

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/jobs"
)

// RegisterAudit adds the tamper-evident audit tasks: sealing (hash chain), daily anchors
// and retention.
func RegisterAudit(s *jobs.Scheduler, d *Deps) {
	s.Add(jobs.Task{Name: "audit.seal", Every: 5 * time.Second, Timeout: 2 * time.Minute, Run: d.SealAudit})
	s.Add(jobs.Task{Name: "audit.anchor", DailyAtUTC: jobs.Daily(0, 10), Run: d.AnchorAudit})
	s.Add(jobs.Task{Name: "audit.retention", DailyAtUTC: jobs.Daily(3, 30), Run: d.AuditRetention})
}

// SealAudit chains every unsealed entry into its organisation's hash chain.
func (d *Deps) SealAudit(ctx context.Context) error {
	rows, err := d.Pool.Query(ctx, `SELECT DISTINCT org_id FROM audit_logs WHERE hash IS NULL AND org_id IS NOT NULL LIMIT 5000`)
	if err != nil {
		return err
	}
	var orgs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			orgs = append(orgs, id)
		}
	}
	rows.Close()
	for _, org := range orgs {
		for {
			var n int
			if err := d.Pool.QueryRow(ctx, `SELECT audit_seal($1, 1000)`, org).Scan(&n); err != nil {
				return err
			}
			if n < 1000 {
				break
			}
		}
	}
	return nil
}

// AnchorAudit records yesterday's chain head per organisation. The anchor lets the verifier
// detect a rewritten chain even if an attacker recomputes every hash after the anchor.
func (d *Deps) AnchorAudit(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO audit_anchors (org_id, day, last_seq, head_hash)
		SELECT DISTINCT ON (org_id) org_id, (now() AT TIME ZONE 'UTC')::date - 1, seq, hash
		FROM audit_logs
		WHERE seq IS NOT NULL AND sealed_at < date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
		ORDER BY org_id, seq DESC
		ON CONFLICT (org_id, day) DO NOTHING`)
	return err
}

// AuditRetention prunes sealed entries past the plan's retention (1 year; Enterprise keeps
// everything unless a contract sets audit_retention_days). The chain stays verifiable from
// the oldest remaining entry's prev_hash and the anchors.
func (d *Deps) AuditRetention(ctx context.Context) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL qrit.audit_retention = 'on'`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM audit_logs a USING organizations o
		WHERE o.id = a.org_id AND a.seq IS NOT NULL
		  AND a.created_at < now() - make_interval(days => COALESCE(
		        (SELECT (c.limits_override->>'audit_retention_days')::int FROM contracts c
		          WHERE c.org_id = o.id AND c.status = 'active' LIMIT 1),
		        CASE o.plan_id WHEN 'enterprise' THEN 36500 ELSE 365 END))`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
