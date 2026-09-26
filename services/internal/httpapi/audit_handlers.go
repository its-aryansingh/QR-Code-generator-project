package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
)

func jsonBody(v any) (io.ReadCloser, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

type auditActor struct {
	Type  string     `json:"type"`
	ID    *uuid.UUID `json:"id"`
	Email *string    `json:"email,omitempty"`
	Name  *string    `json:"name,omitempty"`
}

type auditEntryDTO struct {
	ID          int64           `json:"id"`
	Seq         *int64          `json:"seq"`
	OrgID       *uuid.UUID      `json:"org_id"`
	WorkspaceID *uuid.UUID      `json:"workspace_id"`
	Actor       auditActor      `json:"actor"`
	Action      string          `json:"action"`
	TargetType  string          `json:"target_type"`
	TargetID    *uuid.UUID      `json:"target_id"`
	Changes     json.RawMessage `json:"changes"`
	IPPrefix    *string         `json:"ip_prefix"`
	UserAgent   *string         `json:"user_agent"`
	RequestID   *string         `json:"request_id"`
	CreatedAt   time.Time       `json:"created_at"`
	Hash        *string         `json:"hash"`
}

type auditFilter struct {
	orgID       uuid.UUID
	workspaceID *uuid.UUID
	action      string
	actorID     *uuid.UUID
	targetType  string
	targetID    *uuid.UUID
	from, to    *time.Time
	beforeID    int64
	limit       int
}

func parseAuditFilter(r *http.Request) (auditFilter, error) {
	q := r.URL.Query()
	f := auditFilter{action: strings.TrimSpace(q.Get("action")), targetType: q.Get("target_type"), limit: limitParam(r, 50, 500)}
	for name, dst := range map[string]**uuid.UUID{"actor_id": &f.actorID, "target_id": &f.targetID, "workspace_id": &f.workspaceID} {
		if v := q.Get(name); v != "" {
			id, err := uuid.Parse(v)
			if err != nil {
				return f, apierr.BadRequest("invalid_"+name, name+" must be a UUID")
			}
			*dst = &id
		}
	}
	for name, dst := range map[string]**time.Time{"from": &f.from, "to": &f.to} {
		if v := q.Get(name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return f, apierr.BadRequest("invalid_"+name, name+" must be RFC 3339")
			}
			*dst = &t
		}
	}
	if c := q.Get("cursor"); c != "" {
		n, err := strconv.ParseInt(c, 10, 64)
		if err != nil || n <= 0 {
			return f, apierr.BadRequest("invalid_cursor", "cursor is malformed")
		}
		f.beforeID = n
	}
	return f, nil
}

func (s *Server) queryAudit(r *http.Request, f auditFilter) ([]auditEntryDTO, error) {
	args := []any{f.orgID}
	where := []string{"a.org_id = $1"}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.workspaceID != nil {
		add("a.workspace_id = ?", *f.workspaceID)
	}
	if f.action != "" {
		if strings.HasSuffix(f.action, "*") {
			add("a.action LIKE ?", strings.TrimSuffix(f.action, "*")+"%")
		} else {
			add("a.action = ?", f.action)
		}
	}
	if f.actorID != nil {
		add("a.actor_id = ?", *f.actorID)
	}
	if f.targetType != "" {
		add("a.target_type = ?", f.targetType)
	}
	if f.targetID != nil {
		add("a.target_id = ?", *f.targetID)
	}
	if f.from != nil {
		add("a.created_at >= ?", *f.from)
	}
	if f.to != nil {
		add("a.created_at < ?", *f.to)
	}
	if f.beforeID > 0 {
		add("a.id < ?", f.beforeID)
	}
	args = append(args, f.limit)
	rows, err := s.pool.Query(r.Context(), fmt.Sprintf(`SELECT a.id, a.seq, a.org_id, a.workspace_id, a.actor_type, a.actor_id,
		u.email::text, u.name, a.action, a.target_type, a.target_id, a.changes, a.ip_prefix, a.user_agent, a.request_id,
		a.created_at, a.hash
		FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_id AND a.actor_type IN ('user','staff')
		WHERE %s ORDER BY a.id DESC LIMIT $%d`, strings.Join(where, " AND "), len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []auditEntryDTO{}
	for rows.Next() {
		var e auditEntryDTO
		var hash []byte
		if err := rows.Scan(&e.ID, &e.Seq, &e.OrgID, &e.WorkspaceID, &e.Actor.Type, &e.Actor.ID, &e.Actor.Email, &e.Actor.Name,
			&e.Action, &e.TargetType, &e.TargetID, &e.Changes, &e.IPPrefix, &e.UserAgent, &e.RequestID, &e.CreatedAt, &hash); err != nil {
			return nil, err
		}
		if hash != nil {
			h := hex.EncodeToString(hash)
			e.Hash = &h
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Server) writeAuditPage(w http.ResponseWriter, r *http.Request, f auditFilter) {
	rows, err := s.queryAudit(r, f)
	if err != nil {
		fail(w, apierr.Internal("failed to read the audit log"))
		return
	}
	resp := map[string]any{"data": rows, "next_cursor": nil}
	if len(rows) == f.limit && f.limit > 0 {
		resp["next_cursor"] = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /orgs/{org}/audit-logs (org.audit, Business+).
func (s *Server) handleOrgAuditLogs(w http.ResponseWriter, r *http.Request) {
	if err := s.requireFeatureOrg(r, entitlements.FeatureAuditLog); err != nil {
		fail(w, err)
		return
	}
	f, err := parseAuditFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	f.orgID = orgRow(r).ID
	s.writeAuditPage(w, r, f)
}

// GET /workspaces/{ws}/audit-logs (audit.read).
func (s *Server) handleWorkspaceAuditLogs(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	if err := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureAuditLog); err != nil {
		fail(w, featureErr(err))
		return
	}
	f, err := parseAuditFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	f.orgID, f.workspaceID = ws.OrgID, &ws.ID
	s.writeAuditPage(w, r, f)
}

// GET /orgs/{org}/audit-logs/verify: recompute the hash chain and compare daily anchors.
func (s *Server) handleAuditVerify(w http.ResponseWriter, r *http.Request) {
	o := orgRow(r)
	ctx := r.Context()
	var broken *int64
	if err := s.pool.QueryRow(ctx, `SELECT audit_verify($1)`, o.ID).Scan(&broken); err != nil {
		fail(w, apierr.Internal("verification failed"))
		return
	}
	var sealed, unsealed int64
	var head *int64
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE seq IS NOT NULL), count(*) FILTER (WHERE seq IS NULL), max(seq)
		FROM audit_logs WHERE org_id = $1`, o.ID).Scan(&sealed, &unsealed, &head)
	status := "intact"
	resp := map[string]any{"sealed_entries": sealed, "unsealed_entries": unsealed, "head_seq": head}
	if broken != nil {
		status = "broken"
		resp["broken_at_seq"] = *broken
	}
	rows, err := s.pool.Query(ctx, `SELECT an.day, an.last_seq, an.head_hash = a.hash, a.hash IS NULL
		FROM audit_anchors an LEFT JOIN audit_logs a ON a.org_id = an.org_id AND a.seq = an.last_seq
		WHERE an.org_id = $1 ORDER BY an.day`, o.ID)
	if err == nil {
		checked := 0
		for rows.Next() {
			var day time.Time
			var seq int64
			var match *bool
			var pruned bool
			if rows.Scan(&day, &seq, &match, &pruned) != nil {
				continue
			}
			if pruned {
				continue // entry removed by retention; its successor's prev_hash still anchors the chain
			}
			checked++
			if match == nil || !*match {
				status = "anchor_mismatch"
				resp["anchor_mismatch_day"] = day.Format("2006-01-02")
			}
		}
		rows.Close()
		resp["anchors_checked"] = checked
	}
	resp["status"] = status
	writeJSON(w, http.StatusOK, resp)
}

// GET /orgs/{org}/audit-logs/export?format=jsonl|csv (org.audit + step-up). The export is
// itself audited.
func (s *Server) handleAuditExport(w http.ResponseWriter, r *http.Request) {
	if err := s.requireFeatureOrg(r, entitlements.FeatureAuditLog); err != nil {
		fail(w, err)
		return
	}
	f, err := parseAuditFilter(r)
	if err != nil {
		fail(w, err)
		return
	}
	o := orgRow(r)
	f.orgID, f.limit = o.ID, 5000
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "jsonl"
	}
	if format != "jsonl" && format != "csv" {
		fail(w, apierr.BadRequest("invalid_format", "format must be jsonl or csv"))
		return
	}
	_ = s.auditOrg(r, s.q, o.ID, "audit.exported", "organization", &o.ID, map[string]any{"format": format})
	name := "qrit-audit-" + o.Slug + "-" + time.Now().UTC().Format("20060102T150405Z") + "." + format
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/x-ndjson")
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	var cw *csv.Writer
	if format == "csv" {
		cw = csv.NewWriter(w)
		_ = cw.Write([]string{"id", "seq", "created_at", "workspace_id", "actor_type", "actor_id", "actor_email", "action",
			"target_type", "target_id", "ip_prefix", "request_id", "changes", "hash"})
	}
	enc := json.NewEncoder(w)
	for total := 0; total < 2_000_000; {
		rows, err := s.queryAudit(r, f)
		if err != nil || len(rows) == 0 {
			break
		}
		for _, e := range rows {
			if cw != nil {
				str := func(p *string) string {
					if p == nil {
						return ""
					}
					return csvSafe(p)
				}
				id := func(p *uuid.UUID) string {
					if p == nil {
						return ""
					}
					return p.String()
				}
				seq := ""
				if e.Seq != nil {
					seq = strconv.FormatInt(*e.Seq, 10)
				}
				_ = cw.Write([]string{strconv.FormatInt(e.ID, 10), seq, e.CreatedAt.Format(time.RFC3339Nano), id(e.WorkspaceID),
					e.Actor.Type, id(e.Actor.ID), str(e.Actor.Email), e.Action, e.TargetType, id(e.TargetID), str(e.IPPrefix),
					str(e.RequestID), string(e.Changes), str(e.Hash)})
			} else {
				_ = enc.Encode(e)
			}
		}
		total += len(rows)
		f.beforeID = rows[len(rows)-1].ID
		if cw != nil {
			cw.Flush()
		}
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}
	if cw != nil {
		cw.Flush()
	}
}
