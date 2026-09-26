package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/auditstream"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
)

// StreamPauseAfter is how long a stream may keep failing before it is paused.
const StreamPauseAfter = 24 * time.Hour

// RegisterAuditStreams delivers sealed audit entries to configured SIEM streams.
func RegisterAuditStreams(s *jobs.Scheduler, d *Deps, client *http.Client) {
	s.Add(jobs.Task{Name: "audit.stream", Every: 15 * time.Second, Timeout: 2 * time.Minute,
		Run: func(ctx context.Context) error { _, err := d.DeliverAuditStreams(ctx, client, 20); return err }})
}

// DeliverAuditStreams runs up to maxBatches batches (≤ 500 entries each) per active stream,
// in seq order. The cursor advances only after the receiver accepts a batch.
func (d *Deps) DeliverAuditStreams(ctx context.Context, client *http.Client, maxBatches int) (int, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id FROM audit_streams WHERE status = 'active' ORDER BY last_delivered_at NULLS FIRST`)
	if err != nil {
		return 0, err
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	sent := 0
	for _, id := range ids {
		n, err := d.deliverStream(ctx, id, client, maxBatches)
		sent += n
		if err != nil {
			d.log().Warn("audit stream", "stream", id, "error", err)
		}
	}
	return sent, nil
}

func (d *Deps) deliverStream(ctx context.Context, id uuid.UUID, client *http.Client, maxBatches int) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var (
		orgID   uuid.UUID
		kind    string
		cfgRaw  []byte
		ct      []byte
		cursor  int64
		failing *time.Time
	)
	err = tx.QueryRow(ctx, `SELECT org_id, kind, config, secret_ct, cursor_seq, failing_since FROM audit_streams
		WHERE id = $1 AND status = 'active' FOR UPDATE SKIP LOCKED`, id).Scan(&orgID, &kind, &cfgRaw, &ct, &cursor, &failing)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil // another worker holds it, or it was paused
	}
	if err != nil {
		return 0, err
	}
	var cfg auditstream.Config
	_ = json.Unmarshal(cfgRaw, &cfg)
	secret, err := d.Keyring.Decrypt(ctx, orgID, ct)
	if err != nil {
		return 0, err
	}
	snd := auditstream.Sender{Kind: kind, Config: cfg, Secret: string(secret), HTTP: client}
	sent := 0
	for i := 0; i < maxBatches; i++ {
		evs, err := loadAuditBatch(ctx, tx, orgID, cursor, 500)
		if err != nil {
			return sent, err
		}
		if len(evs) == 0 {
			break
		}
		sctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = snd.Send(sctx, evs)
		cancel()
		if err != nil {
			msg := err.Error()
			if len(msg) > 500 {
				msg = msg[:500]
			}
			if _, e := tx.Exec(ctx, `UPDATE audit_streams SET last_error = $2, failing_since = COALESCE(failing_since, now()),
				updated_at = now() WHERE id = $1`, id, msg); e != nil {
				return sent, e
			}
			if failing != nil && time.Since(*failing) > StreamPauseAfter {
				if _, e := tx.Exec(ctx, `UPDATE audit_streams SET status = 'error' WHERE id = $1`, id); e != nil {
					return sent, e
				}
				if _, e := tx.Exec(ctx, `INSERT INTO audit_logs (org_id, actor_type, action, target_type, target_id, changes)
					VALUES ($1, 'system', 'audit_stream.paused', 'audit_stream', $2, jsonb_build_object('error', $3::text))`, orgID, id, msg); e != nil {
					return sent, e
				}
				if _, e := jobs.Enqueue(ctx, tx, "org.alert", map[string]any{"org_id": orgID, "event": "audit_stream.paused",
					"data": map[string]any{"stream_id": id, "error": msg}}, jobs.Options{}); e != nil {
					return sent, e
				}
			}
			return sent, tx.Commit(ctx)
		}
		cursor = evs[len(evs)-1].Seq
		if _, err := tx.Exec(ctx, `UPDATE audit_streams SET cursor_seq = $2, last_error = NULL, failing_since = NULL,
			last_delivered_at = now(), updated_at = now() WHERE id = $1`, id, cursor); err != nil {
			return sent, err
		}
		failing = nil
		sent += len(evs)
		if len(evs) < 500 {
			break
		}
	}
	return sent, tx.Commit(ctx)
}

func loadAuditBatch(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, after int64, limit int) ([]auditstream.Event, error) {
	rows, err := tx.Query(ctx, `SELECT org_id, seq, id, workspace_id, actor_type, actor_id, action, target_type, target_id, changes,
		ip_prefix, user_agent, request_id, created_at, encode(hash, 'hex')
		FROM audit_logs WHERE org_id = $1 AND seq > $2 ORDER BY seq LIMIT $3`, orgID, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []auditstream.Event
	for rows.Next() {
		var e auditstream.Event
		if err := rows.Scan(&e.OrgID, &e.Seq, &e.ID, &e.WorkspaceID, &e.ActorType, &e.ActorID, &e.Action, &e.TargetType, &e.TargetID,
			&e.Changes, &e.IPPrefix, &e.UserAgent, &e.RequestID, &e.CreatedAt, &e.Hash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
