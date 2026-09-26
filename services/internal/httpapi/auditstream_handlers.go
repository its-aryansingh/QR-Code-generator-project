package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auditstream"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/stdwebhook"
)

func (s *Server) auditStreamRoutes(r chi.Router) {
	r.With(requireOrg(authz.OrgAudit)).Get("/audit-streams", s.handleListStreams)
	r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Post("/audit-streams", s.handleCreateStream)
	r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Patch("/audit-streams/{id}", s.handleUpdateStream)
	r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Delete("/audit-streams/{id}", s.handleDeleteStream)
	r.With(requireOrg(authz.OrgAudit)).Post("/audit-streams/{id}/test", s.handleTestStream)
	r.With(requireOrg(authz.OrgAudit), s.requireStepUp(10*time.Minute)).Post("/audit-streams/{id}/replay", s.handleReplayStream)
}

type streamDTO struct {
	ID              uuid.UUID          `json:"id"`
	Kind            string             `json:"kind"`
	Label           string             `json:"label"`
	Config          auditstream.Config `json:"config"`
	HasSecret       bool               `json:"has_secret"`
	CursorSeq       int64              `json:"cursor_seq"`
	LatestSeq       int64              `json:"latest_seq"`
	Status          string             `json:"status"`
	LastError       *string            `json:"last_error"`
	FailingSince    *time.Time         `json:"failing_since"`
	LastDeliveredAt *time.Time         `json:"last_delivered_at"`
	CreatedAt       time.Time          `json:"created_at"`
	secretCT        []byte
}

const streamCols = `s.id, s.kind, s.label, s.config, s.secret_ct, s.cursor_seq, s.status, s.last_error, s.failing_since,
	s.last_delivered_at, s.created_at, COALESCE((SELECT max(seq) FROM audit_logs a WHERE a.org_id = s.org_id), 0)`

func scanStream(row pgx.Row) (streamDTO, error) {
	var d streamDTO
	var cfg []byte
	err := row.Scan(&d.ID, &d.Kind, &d.Label, &cfg, &d.secretCT, &d.CursorSeq, &d.Status, &d.LastError, &d.FailingSince,
		&d.LastDeliveredAt, &d.CreatedAt, &d.LatestSeq)
	_ = json.Unmarshal(cfg, &d.Config)
	d.HasSecret = len(d.secretCT) > 0
	return d, err
}

func (s *Server) handleListStreams(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `SELECT `+streamCols+` FROM audit_streams s WHERE s.org_id = $1 ORDER BY s.created_at`, orgRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list streams"))
		return
	}
	defer rows.Close()
	out := []streamDTO{}
	for rows.Next() {
		if d, err := scanStream(rows); err == nil {
			out = append(out, d)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type streamReq struct {
	Kind   string              `json:"kind"`
	Label  *string             `json:"label"`
	Config *auditstream.Config `json:"config"`
	Secret *string             `json:"secret"`
	Status *string             `json:"status"`
}

func (s *Server) loadStream(ctx context.Context, orgID, id uuid.UUID) (streamDTO, error) {
	d, err := scanStream(s.pool.QueryRow(ctx, `SELECT `+streamCols+` FROM audit_streams s WHERE s.id = $1 AND s.org_id = $2`, id, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return d, apierr.NotFound("audit stream not found")
	}
	return d, err
}

func (s *Server) handleCreateStream(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	if err := s.requireFeatureOrg(r, entitlements.FeatureAuditStreams); err != nil {
		fail(w, err)
		return
	}
	var req streamReq
	if !decode(w, r, &req) {
		return
	}
	if req.Config == nil {
		req.Config = &auditstream.Config{}
	}
	if err := auditstream.Validate(req.Kind, *req.Config, s.cfg.IsLocal()); err != nil {
		fail(w, unprocessable("invalid_stream_config", err.Error()))
		return
	}
	var n int
	_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM audit_streams WHERE org_id = $1`, o.ID).Scan(&n)
	if n >= 5 {
		fail(w, apierr.Conflict("too_many_streams", "an organisation can have at most 5 audit streams"))
		return
	}
	secret, generated := "", ""
	switch {
	case req.Secret != nil && strings.TrimSpace(*req.Secret) != "":
		secret = strings.TrimSpace(*req.Secret)
		if req.Kind == "webhook" && !strings.HasPrefix(secret, "whsec_") {
			fail(w, unprocessable("invalid_secret", "webhook secrets use the whsec_ format; omit secret to have one generated"))
			return
		}
	case req.Kind == "webhook":
		secret = stdwebhook.NewSecret()
		generated = secret
	default:
		fail(w, unprocessable("secret_required", "provide the credential for "+req.Kind))
		return
	}
	ct, err := s.keyring.Encrypt(r.Context(), o.ID, []byte(secret))
	if err != nil {
		fail(w, apierr.Internal("failed to store the credential"))
		return
	}
	cfg, _ := json.Marshal(req.Config)
	label := req.Kind
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		label = strings.TrimSpace(*req.Label)
	}
	id := idgen.New()
	uid, _ := actorIDs(r)
	// New streams start from the current end of the chain unless replayed explicitly.
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `INSERT INTO audit_streams (id, org_id, kind, label, config, secret_ct, cursor_seq, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, COALESCE((SELECT max(seq) FROM audit_logs WHERE org_id = $2), 0), $7)`,
			id, o.ID, req.Kind, label, cfg, ct, uid); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "audit_stream.created", "audit_stream", &id, map[string]any{"kind": req.Kind, "label": label,
			"config": req.Config})
	})
	if err != nil {
		fail(w, apierr.Internal("failed to create the stream"))
		return
	}
	d, _ := s.loadStream(r.Context(), o.ID, id)
	resp := map[string]any{"stream": d}
	if generated != "" {
		resp["secret"] = generated // shown once
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleUpdateStream(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("audit stream not found"))
		return
	}
	var req streamReq
	if !decode(w, r, &req) {
		return
	}
	cur, err := s.loadStream(r.Context(), o.ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load stream"))
		return
	}
	cfg := cur.Config
	if req.Config != nil {
		if err := auditstream.Validate(cur.Kind, *req.Config, s.cfg.IsLocal()); err != nil {
			fail(w, unprocessable("invalid_stream_config", err.Error()))
			return
		}
		cfg = *req.Config
	}
	status := cur.Status
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "paused" {
			fail(w, unprocessable("invalid_status", "status must be active or paused"))
			return
		}
		status = *req.Status
	}
	label := cur.Label
	if req.Label != nil && strings.TrimSpace(*req.Label) != "" {
		label = strings.TrimSpace(*req.Label)
	}
	ct := cur.secretCT
	if req.Secret != nil && strings.TrimSpace(*req.Secret) != "" {
		if ct, err = s.keyring.Encrypt(r.Context(), o.ID, []byte(strings.TrimSpace(*req.Secret))); err != nil {
			fail(w, apierr.Internal("failed to store the credential"))
			return
		}
	}
	cb, _ := json.Marshal(cfg)
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		// Re-activating clears the failure state; delivery resumes from the cursor.
		if _, err := tx.Exec(r.Context(), `UPDATE audit_streams SET label = $3, config = $4, secret_ct = $5, status = $6,
			failing_since = CASE WHEN $6 = 'active' AND status <> 'active' THEN NULL ELSE failing_since END,
			last_error = CASE WHEN $6 = 'active' AND status <> 'active' THEN NULL ELSE last_error END, updated_at = now()
			WHERE id = $1 AND org_id = $2`, id, o.ID, label, cb, ct, status); err != nil {
			return err
		}
		return s.auditOrg(r, q, o.ID, "audit_stream.updated", "audit_stream", &id, map[string]any{"status": status, "label": label,
			"config_changed": req.Config != nil, "secret_rotated": req.Secret != nil})
	})
	if err != nil {
		fail(w, apierr.Internal("failed to update the stream"))
		return
	}
	d, _ := s.loadStream(r.Context(), o.ID, id)
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleDeleteStream(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("audit stream not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `DELETE FROM audit_streams WHERE id = $1 AND org_id = $2`, id, o.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("audit stream not found")
		}
		return s.auditOrg(r, q, o.ID, "audit_stream.deleted", "audit_stream", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete the stream"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// streamSender builds a sender for a stream (decrypting its credential).
func (s *Server) streamSender(ctx context.Context, orgID uuid.UUID, d streamDTO) (auditstream.Sender, error) {
	secret, err := s.keyring.Decrypt(ctx, orgID, d.secretCT)
	if err != nil {
		return auditstream.Sender{}, err
	}
	return auditstream.Sender{Kind: d.Kind, Config: d.Config, Secret: string(secret), HTTP: s.streamHTTP}, nil
}

// POST /audit-streams/{id}/test: deliver one synthetic event (seq 0) and report the outcome.
func (s *Server) handleTestStream(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("audit stream not found"))
		return
	}
	d, err := s.loadStream(r.Context(), o.ID, id)
	if err != nil {
		fail(w, problemOr500(err, "failed to load stream"))
		return
	}
	snd, err := s.streamSender(r.Context(), o.ID, d)
	if err != nil {
		fail(w, apierr.Internal("failed to read the credential"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ev := auditstream.Event{OrgID: o.ID, Seq: 0, ActorType: "system", Action: "audit_stream.test", TargetType: "audit_stream",
		TargetID: &id, Changes: json.RawMessage(`{"test":true}`), CreatedAt: time.Now().UTC()}
	res := map[string]any{"ok": true}
	if err := snd.Send(ctx, []auditstream.Event{ev}); err != nil {
		res = map[string]any{"ok": false, "error": err.Error()}
	}
	_ = s.auditOrg(r, s.q, o.ID, "audit_stream.tested", "audit_stream", &id, res)
	writeJSON(w, http.StatusOK, res)
}

type replayReq struct {
	FromSeq int64 `json:"from_seq"`
}

// POST /audit-streams/{id}/replay {from_seq}: re-deliver from a sequence number.
func (s *Server) handleReplayStream(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("audit stream not found"))
		return
	}
	var req replayReq
	if !decode(w, r, &req) {
		return
	}
	if req.FromSeq < 1 {
		fail(w, unprocessable("invalid_from_seq", "from_seq must be at least 1"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `UPDATE audit_streams SET cursor_seq = $3 - 1, updated_at = now() WHERE id = $1 AND org_id = $2`,
			id, o.ID, req.FromSeq)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.NotFound("audit stream not found")
		}
		return s.auditOrg(r, q, o.ID, "audit_stream.replayed", "audit_stream", &id, map[string]any{"from_seq": req.FromSeq})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to replay"))
		return
	}
	d, _ := s.loadStream(r.Context(), o.ID, id)
	writeJSON(w, http.StatusOK, d)
}
