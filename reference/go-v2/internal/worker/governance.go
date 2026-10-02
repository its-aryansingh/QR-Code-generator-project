package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/approval"
	"github.com/its-aryansingh/qrit/services/internal/email"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
)

// RegisterGovernance adds approval notifications and expiry.
func RegisterGovernance(s *jobs.Scheduler, r *jobs.Runner, d *Deps) {
	r.Handle("approval.notify", d.NotifyApprovers)
	s.Add(jobs.Task{Name: "approvals.expire", Every: 5 * time.Minute, Timeout: 2 * time.Minute,
		Run: func(ctx context.Context) error { _, err := d.ExpireApprovals(ctx); return err }})
}

// Approvers lists people who may decide on a request: active org owners/admins and anyone
// holding qr.destination.approve (directly or via a group) at workspace scope or on the
// code's folder chain. The requester is excluded (four-eyes).
func (d *Deps) Approvers(ctx context.Context, requestID uuid.UUID) ([]string, error) {
	rows, err := d.Pool.Query(ctx, `
		WITH req AS (
		    SELECT ar.id, ar.workspace_id, ar.requested_by, w.org_id, q.folder_id
		    FROM approval_requests ar JOIN workspaces w ON w.id = ar.workspace_id
		    LEFT JOIN qr_codes q ON q.id = ar.qr_code_id WHERE ar.id = $1
		), chain AS (
		    SELECT f.id FROM req, LATERAL (
		        WITH RECURSIVE up AS (
		            SELECT id, parent_id FROM folders WHERE id = req.folder_id
		            UNION ALL SELECT p.id, p.parent_id FROM folders p JOIN up ON p.id = up.parent_id)
		        SELECT id FROM up) f
		)
		SELECT DISTINCT u.email::text FROM req
		JOIN org_members om ON om.org_id = req.org_id AND om.status = 'active'
		JOIN users u ON u.id = om.user_id
		WHERE u.id <> req.requested_by AND (
		    om.org_role IN ('org_owner','org_admin')
		 OR EXISTS (SELECT 1 FROM role_bindings b JOIN roles r ON r.id = b.role_id
		            WHERE b.workspace_id = req.workspace_id
		              AND ('qr.destination.approve' = ANY(r.permissions) OR '*' = ANY(r.permissions))
		              AND (b.folder_id IS NULL OR b.folder_id IN (SELECT id FROM chain))
		              AND ((b.principal_type = 'user' AND b.principal_id = u.id)
		                OR (b.principal_type = 'group' AND b.principal_id IN (SELECT gm.group_id FROM group_members gm WHERE gm.user_id = u.id)))))
		ORDER BY 1`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NotifyApprovers emails everyone who can decide on a new request.
func (d *Deps) NotifyApprovers(ctx context.Context, j *jobs.Job) error {
	var p struct {
		ApprovalID uuid.UUID `json:"approval_id"`
	}
	if err := j.Decode(&p); err != nil {
		return jobs.Permanent(err)
	}
	var (
		wsID                    uuid.UUID
		wsName, requester, stat string
		qrName, dest            *string
		reasons                 []string
		allowed                 []string
		expires                 time.Time
	)
	err := d.Pool.QueryRow(ctx, `SELECT ar.workspace_id, w.name, u.name, ar.status, q.name,
			(SELECT v.destination_url FROM qr_versions v WHERE v.approval_request_id = ar.id ORDER BY v.version_no DESC LIMIT 1),
			ar.reasons, COALESCE(p.allowed_destination_hosts, '{}'), ar.expires_at
		FROM approval_requests ar JOIN workspaces w ON w.id = ar.workspace_id JOIN users u ON u.id = ar.requested_by
		LEFT JOIN qr_codes q ON q.id = ar.qr_code_id LEFT JOIN workspace_policies p ON p.workspace_id = ar.workspace_id
		WHERE ar.id = $1`, p.ApprovalID).Scan(&wsID, &wsName, &requester, &stat, &qrName, &dest, &reasons, &allowed, &expires)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && stat != "pending") {
		return nil
	}
	if err != nil {
		return err
	}
	if d.Mail == nil {
		return nil
	}
	to, err := d.Approvers(ctx, p.ApprovalID)
	if err != nil {
		return err
	}
	name := "a QR code"
	if qrName != nil {
		name = "“" + *qrName + "”"
	}
	link := strings.TrimRight(d.AppBaseURL, "/") + "/w/" + wsID.String() + "/approvals/" + p.ApprovalID.String()
	subject := fmt.Sprintf("Approval needed: %s in %s", strings.Trim(name, "“”"), wsName)
	var b strings.Builder
	fmt.Fprintf(&b, "%s asked to change %s in %s.\n\n", requester, name, wsName)
	if dest != nil {
		fmt.Fprintf(&b, "New destination: %s\n", *dest)
	}
	for _, reason := range approval.Describe(reasons, allowed) {
		fmt.Fprintf(&b, "Why it needs approval: %s\n", reason)
	}
	fmt.Fprintf(&b, "\nReview it here: %s\n\nThe request expires on %s.\n", link, expires.UTC().Format("2 Jan 2006 15:04 MST"))
	for _, addr := range to {
		if err := email.Notify(d.Mail, ctx, addr, subject, b.String()); err != nil {
			d.log().Warn("approval email", "to", addr, "error", err)
		}
	}
	return nil
}

// ExpireApprovals closes requests past their deadline; their versions are never published.
func (d *Deps) ExpireApprovals(ctx context.Context) (int, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT id, workspace_id, qr_code_id FROM approval_requests
		WHERE status = 'pending' AND expires_at <= now() ORDER BY expires_at LIMIT 500 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return 0, err
	}
	type exp struct {
		id, ws uuid.UUID
		code   *uuid.UUID
	}
	var list []exp
	for rows.Next() {
		var e exp
		if err := rows.Scan(&e.id, &e.ws, &e.code); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, e)
	}
	rows.Close()
	for _, e := range list {
		if _, err := tx.Exec(ctx, `UPDATE approval_requests SET status = 'expired', decided_at = now() WHERE id = $1`, e.id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `UPDATE qr_versions SET approval_status = 'cancelled' WHERE approval_request_id = $1 AND approval_status = 'pending'`, e.id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (workspace_id, actor_type, action, target_type, target_id, changes)
			VALUES ($1, 'system', 'approval.expired', 'approval_request', $2, jsonb_build_object('qr_code_id', $3::uuid))`, e.ws, e.id, e.code); err != nil {
			return 0, err
		}
		ws := e.ws
		if _, err := jobs.Enqueue(ctx, tx, "event.fanout", map[string]any{"workspace_id": e.ws, "event": "approval.expired",
			"data": map[string]any{"approval_id": e.id, "qr_code_id": e.code}}, jobs.Options{WorkspaceID: &ws}); err != nil {
			return 0, err
		}
	}
	return len(list), tx.Commit(ctx)
}
