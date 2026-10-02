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
	"github.com/its-aryansingh/qrit/services/internal/approval"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/jobs"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/routing"
)

// approvalGate holds destination changes for review according to the workspace policy
// (maker–checker). The held version is stored with approval_status 'pending' and is never
// served until approved (every "version in effect" query filters on approval_status).
type approvalGate struct{ s *Server }

// draftURLs lists every URL a version can send scanners to.
func draftURLs(d VersionDraft) []string {
	var out []string
	if d.DestinationURL != nil {
		out = append(out, *d.DestinationURL)
	}
	var rules []routing.Rule
	if json.Unmarshal(d.Rules, &rules) == nil {
		for _, r := range rules {
			if r.DestinationURL != "" {
				out = append(out, r.DestinationURL)
			}
			for _, v := range r.Split {
				if v.DestinationURL != "" {
					out = append(out, v.DestinationURL)
				}
			}
		}
	}
	if len(d.HostedPage) > 0 {
		var page any
		if json.Unmarshal(d.HostedPage, &page) == nil {
			collectLinks(page, &out)
		}
	}
	return out
}

// collectLinks walks a hosted page for outbound links ("url", "website"); uploaded assets
// (file_url, photo_url, avatar_url) live on our storage and are not destinations.
func collectLinks(v any, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if s, ok := val.(string); ok && (k == "url" || k == "website") && s != "" {
				*out = append(*out, s)
				continue
			}
			collectLinks(val, out)
		}
	case []any:
		for _, e := range x {
			collectLinks(e, out)
		}
	}
}

type approvalSummary struct {
	ID                uuid.UUID `json:"id"`
	Status            string    `json:"status"`
	Reasons           []string  `json:"reasons"`
	ReasonsText       []string  `json:"reasons_text"`
	RequiredApprovals int       `json:"required_approvals"`
	ExpiresAt         time.Time `json:"expires_at"`
}

func (g approvalGate) Intercept(r *http.Request, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, code dbgen.QrCode, d VersionDraft) (bool, any, error) {
	ctx := r.Context()
	wp, err := g.s.wsPolicy(ctx, ws.ID)
	if err != nil {
		return false, nil, err
	}
	if wp.ApprovalMode == approval.ModeOff || wp.ApprovalMode == "" {
		return false, nil, nil
	}
	newCode := !code.CurrentVersionID.Valid
	reasons := approval.Evaluate(approval.Policy{Mode: wp.ApprovalMode, AllowedHosts: wp.AllowedDestinationHosts},
		approval.Change{NewCode: newCode, URLs: draftURLs(d)})
	if len(reasons) == 0 {
		return false, nil, nil
	}
	p := principal(r)
	if p == nil || p.UserID == uuid.Nil {
		return false, nil, apierr.Conflict("approval_requires_user", "this change needs approval, and the API key has no owning user to request it; create keys from a user account")
	}
	kind := "destination"
	if newCode {
		kind = "create"
	}
	reqID := idgen.New()
	expires := time.Now().Add(time.Duration(wp.ApprovalExpiryHours) * time.Hour).UTC()
	if _, err := tx.Exec(ctx, `INSERT INTO approval_requests (id, workspace_id, kind, qr_code_id, reasons, requested_by,
		required_approvals, note, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		reqID, ws.ID, kind, code.ID, reasons, p.UserID, wp.ApprovalsRequired, d.ChangeNote, expires); err != nil {
		return false, nil, err
	}
	no, err := q.MaxQRVersionNo(ctx, code.ID)
	if err != nil {
		return false, nil, err
	}
	userID, keyID := actorIDs(r)
	v, err := q.CreateQRVersionFull(ctx, dbgen.CreateQRVersionFullParams{
		ID: idgen.New(), QrCodeID: code.ID, VersionNo: no + 1, DestinationKind: d.DestinationKind,
		DestinationUrl: d.DestinationURL, HostedPage: d.HostedPage, Rules: d.Rules, Utm: d.UTM,
		EffectiveAt: d.EffectiveAt, SafetyStatus: d.SafetyStatus, RestoredFrom: pgUUIDPtr(d.RestoredFrom),
		ChangeNote: d.ChangeNote, CreatedBy: pgUUIDPtr(userID), CreatedByKey: pgUUIDPtr(keyID),
	})
	if err != nil {
		return false, nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE qr_versions SET approval_status = 'pending', approval_request_id = $2 WHERE id = $1`, v.ID, reqID); err != nil {
		return false, nil, err
	}
	v.ApprovalStatus, v.ApprovalRequestID = "pending", pgUUID(reqID)
	wsID := ws.ID
	if err := g.s.audit(r, q, &wsID, "approval.requested", "approval_request", &reqID, map[string]any{
		"qr_code_id": code.ID, "version_no": v.VersionNo, "destination": d.DestinationURL, "reasons": reasons}); err != nil {
		return false, nil, err
	}
	payload := map[string]any{"workspace_id": ws.ID, "event": "approval.requested",
		"data": map[string]any{"approval_id": reqID, "qr_code_id": code.ID, "qr_name": code.Name, "reasons": reasons, "requested_by": p.UserID}}
	if _, err := jobs.Enqueue(ctx, tx, "event.fanout", payload, jobs.Options{WorkspaceID: &wsID}); err != nil {
		return false, nil, err
	}
	// Notifications go out from the worker once this transaction commits.
	if _, err := jobs.Enqueue(ctx, tx, "approval.notify", map[string]any{"approval_id": reqID}, jobs.Options{WorkspaceID: &wsID}); err != nil {
		return false, nil, err
	}
	body := map[string]any{
		"version": toVersionDTO(v, nil, time.Now()),
		"approval": approvalSummary{ID: reqID, Status: "pending", Reasons: reasons,
			ReasonsText: approval.Describe(reasons, wp.AllowedDestinationHosts), RequiredApprovals: wp.ApprovalsRequired, ExpiresAt: expires},
	}
	return true, body, nil
}

// ---- API ----------------------------------------------------------------------------

func (s *Server) approvalRoutes(r chi.Router) {
	r.With(authz.Require(authz.QRRead)).Get("/approvals", s.handleListApprovals)
	r.With(authz.Require(authz.QRRead)).Get("/approvals/{id}", s.handleGetApproval)
	r.With(authz.Require(authz.QRDestinationApprove)).Post("/approvals/{id}/decisions", s.handleApprovalDecision)
	r.Post("/approvals/{id}/cancel", s.handleCancelApproval)
	r.With(s.requireStepUp(10*time.Minute)).Post("/approvals/{id}/override", s.handleOverrideApproval)
}

type decisionDTO struct {
	ApproverID   uuid.UUID `json:"approver_id"`
	ApproverName string    `json:"approver_name"`
	Decision     string    `json:"decision"`
	Comment      *string   `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type approvalDTO struct {
	ID                uuid.UUID     `json:"id"`
	WorkspaceID       uuid.UUID     `json:"workspace_id"`
	WorkspaceName     string        `json:"workspace_name,omitempty"`
	Kind              string        `json:"kind"`
	QRCodeID          *uuid.UUID    `json:"qr_code_id"`
	QRName            *string       `json:"qr_name"`
	ShortCode         *string       `json:"short_code"`
	Reasons           []string      `json:"reasons"`
	ReasonsText       []string      `json:"reasons_text"`
	RequestedBy       uuid.UUID     `json:"requested_by"`
	RequesterName     string        `json:"requester_name"`
	RequesterEmail    string        `json:"requester_email"`
	RequiredApprovals int           `json:"required_approvals"`
	Approvals         int           `json:"approvals"`
	Status            string        `json:"status"`
	Note              *string       `json:"note"`
	OverrideReason    *string       `json:"override_reason,omitempty"`
	ExpiresAt         time.Time     `json:"expires_at"`
	DecidedAt         *time.Time    `json:"decided_at"`
	CreatedAt         time.Time     `json:"created_at"`
	ProposedURL       *string       `json:"proposed_destination_url"`
	Current           *versionDTO   `json:"current_version,omitempty"`
	Versions          []versionDTO  `json:"versions,omitempty"`
	Decisions         []decisionDTO `json:"decisions,omitempty"`
	CanDecide         *bool         `json:"can_decide,omitempty"`
	folderID          *uuid.UUID
}

const approvalCols = `ar.id, ar.workspace_id, w.name, ar.kind, ar.qr_code_id, q.name, q.short_code, q.folder_id, ar.reasons,
	ar.requested_by, u.name, u.email::text, ar.required_approvals,
	(SELECT count(*) FROM approval_decisions d WHERE d.request_id = ar.id AND d.decision = 'approve')::int,
	ar.status, ar.note, ar.override_reason, ar.expires_at, ar.decided_at, ar.created_at,
	(SELECT v.destination_url FROM qr_versions v WHERE v.approval_request_id = ar.id ORDER BY v.version_no DESC LIMIT 1)`

const approvalFrom = ` FROM approval_requests ar JOIN workspaces w ON w.id = ar.workspace_id
	LEFT JOIN qr_codes q ON q.id = ar.qr_code_id JOIN users u ON u.id = ar.requested_by`

func scanApproval(row pgx.Row, allowed []string) (approvalDTO, error) {
	var a approvalDTO
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.WorkspaceName, &a.Kind, &a.QRCodeID, &a.QRName, &a.ShortCode, &a.folderID, &a.Reasons,
		&a.RequestedBy, &a.RequesterName, &a.RequesterEmail, &a.RequiredApprovals, &a.Approvals, &a.Status, &a.Note,
		&a.OverrideReason, &a.ExpiresAt, &a.DecidedAt, &a.CreatedAt, &a.ProposedURL)
	if a.Status == "pending" && !a.ExpiresAt.After(time.Now()) {
		a.Status = "expired" // the expiry task will catch up
	}
	a.ReasonsText = approval.Describe(a.Reasons, allowed)
	return a, err
}

var approvalStatuses = map[string]bool{"pending": true, "approved": true, "rejected": true, "cancelled": true, "expired": true}

// GET /workspaces/{ws}/approvals?status=
func (s *Server) handleListApprovals(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	st := r.URL.Query().Get("status")
	if st != "" && st != "all" && !approvalStatuses[st] {
		fail(w, apierr.BadRequest("invalid_status", "status must be pending, approved, rejected, cancelled, expired or all"))
		return
	}
	if st == "all" {
		st = ""
	}
	wp, _ := s.wsPolicy(r.Context(), ws.ID)
	where := ` WHERE ar.workspace_id = $1`
	args := []any{ws.ID}
	switch st {
	case "":
	case "pending":
		where += ` AND ar.status = 'pending' AND ar.expires_at > now()`
	case "expired":
		where += ` AND (ar.status = 'expired' OR (ar.status = 'pending' AND ar.expires_at <= now()))`
	default:
		where += ` AND ar.status = $2`
		args = append(args, st)
	}
	rows, err := s.pool.Query(r.Context(), `SELECT `+approvalCols+approvalFrom+where+` ORDER BY ar.created_at DESC LIMIT $`+itoa(len(args)+1),
		append(args, limitParam(r, 100, 500))...)
	if err != nil {
		fail(w, apierr.Internal("failed to list approvals"))
		return
	}
	var list []approvalDTO
	for rows.Next() {
		a, err := scanApproval(rows, wp.AllowedDestinationHosts)
		if err == nil {
			list = append(list, a)
		}
	}
	rows.Close()
	out := []approvalDTO{}
	for _, a := range list {
		if s.can(r, authz.QRRead, a.folderID) {
			out = append(out, a)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (s *Server) loadApproval(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, wsID uuid.UUID, id uuid.UUID, lock bool) (approvalDTO, error) {
	sql := `SELECT ` + approvalCols + approvalFrom + ` WHERE ar.id = $1 AND ar.workspace_id = $2`
	if lock {
		sql += ` FOR UPDATE OF ar`
	}
	wp, _ := s.wsPolicy(ctx, wsID)
	var allowed []string
	if wp != nil {
		allowed = wp.AllowedDestinationHosts
	}
	a, err := scanApproval(db.QueryRow(ctx, sql, id, wsID), allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, apierr.NotFound("approval request not found")
	}
	return a, err
}

// GET /workspaces/{ws}/approvals/{id}: request, proposed and current versions, decisions.
func (s *Server) handleGetApproval(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("approval request not found"))
		return
	}
	a, err := s.loadApproval(r.Context(), s.pool, ws.ID, id, false)
	if err != nil {
		fail(w, problemOr500(err, "failed to load approval"))
		return
	}
	if !s.can(r, authz.QRRead, a.folderID) {
		fail(w, apierr.NotFound("approval request not found"))
		return
	}
	if err := s.approvalDetail(r, &a); err != nil {
		fail(w, apierr.Internal("failed to load approval"))
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) approvalDetail(r *http.Request, a *approvalDTO) error {
	ctx := r.Context()
	now := time.Now()
	a.Versions = []versionDTO{}
	vs, err := s.q.ListApprovalVersions(ctx, pgUUID(a.ID))
	if err != nil {
		return err
	}
	for _, v := range vs {
		a.Versions = append(a.Versions, toVersionDTO(v, nil, now))
	}
	if a.QRCodeID != nil {
		if cur, err := s.q.CurrentEffectiveVersion(ctx, *a.QRCodeID); err == nil {
			d := toVersionDTO(cur, &cur.ID, now)
			a.Current = &d
		}
	}
	a.Decisions = []decisionDTO{}
	drows, err := s.pool.Query(ctx, `SELECT d.approver_id, u.name, d.decision, d.comment, d.created_at FROM approval_decisions d
		JOIN users u ON u.id = d.approver_id WHERE d.request_id = $1 ORDER BY d.created_at`, a.ID)
	if err != nil {
		return err
	}
	for drows.Next() {
		var d decisionDTO
		if err := drows.Scan(&d.ApproverID, &d.ApproverName, &d.Decision, &d.Comment, &d.CreatedAt); err == nil {
			a.Decisions = append(a.Decisions, d)
		}
	}
	drows.Close()
	can := a.Status == "pending" && s.can(r, authz.QRDestinationApprove, a.folderID)
	if p := principal(r); p != nil {
		can = can && p.UserID != a.RequestedBy && !p.IsAPIKey()
		for _, d := range a.Decisions {
			if d.ApproverID == p.UserID {
				can = false
			}
		}
	}
	a.CanDecide = &can
	return nil
}

type decisionReq struct {
	Decision string  `json:"decision"`
	Comment  *string `json:"comment"`
}

// POST /workspaces/{ws}/approvals/{id}/decisions
func (s *Server) handleApprovalDecision(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	p := principal(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("approval request not found"))
		return
	}
	var req decisionReq
	if !decode(w, r, &req) {
		return
	}
	if req.Decision != "approve" && req.Decision != "reject" {
		fail(w, unprocessable("invalid_decision", "decision must be approve or reject"))
		return
	}
	if req.Comment != nil && len(*req.Comment) > 2000 {
		fail(w, unprocessable("invalid_comment", "comment must be at most 2000 characters"))
		return
	}
	if req.Decision == "reject" && (req.Comment == nil || strings.TrimSpace(*req.Comment) == "") {
		fail(w, unprocessable("comment_required", "say why you're rejecting the change"))
		return
	}
	if p.IsAPIKey() {
		fail(w, forbidden("api_keys_cannot_approve", "approvals must be decided by a person"))
		return
	}
	var a approvalDTO
	var codeRow dbgen.QrCode
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		if a, err = s.loadApproval(r.Context(), tx, ws.ID, id, true); err != nil {
			return err
		}
		if !s.can(r, authz.QRRead, a.folderID) {
			return apierr.NotFound("approval request not found")
		}
		if !s.can(r, authz.QRDestinationApprove, a.folderID) {
			return forbidden("forbidden", "you can't approve changes to this code")
		}
		if a.Status != "pending" {
			return apierr.Conflict("approval_not_pending", "this request is already "+a.Status)
		}
		if a.RequestedBy == p.UserID {
			return forbidden("self_approval", "someone other than the requester must review this change")
		}
		if _, err := tx.Exec(r.Context(), `INSERT INTO approval_decisions (request_id, approver_id, decision, comment) VALUES ($1, $2, $3, $4)`,
			id, p.UserID, req.Decision, req.Comment); err != nil {
			if pgCode(err) == sqlUniqueViolation {
				return apierr.Conflict("already_decided", "you have already decided on this request")
			}
			if pgCode(err) == sqlCheckViolation {
				return apierr.Conflict("approval_not_pending", "this request can no longer be decided")
			}
			return err
		}
		wsID := ws.ID
		if err := s.audit(r, q, &wsID, "approval.decided", "approval_request", &id, map[string]any{
			"decision": req.Decision, "comment": req.Comment, "qr_code_id": a.QRCodeID}); err != nil {
			return err
		}
		switch {
		case req.Decision == "reject":
			if err := closeApproval(r.Context(), tx, id, "rejected"); err != nil {
				return err
			}
			a.Status = "rejected"
		case a.Approvals+1 >= a.RequiredApprovals:
			if codeRow, err = s.finalizeApproval(r, q, tx, ws, a); err != nil {
				return err
			}
			a.Status = "approved"
		default:
			a.Approvals++
			return nil
		}
		_, err = jobs.Enqueue(r.Context(), tx, "event.fanout", map[string]any{"workspace_id": ws.ID, "event": "approval.decided",
			"data": map[string]any{"approval_id": id, "status": a.Status, "qr_code_id": a.QRCodeID, "decided_by": p.UserID}},
			jobs.Options{WorkspaceID: &wsID})
		return err
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to record decision"))
		return
	}
	if codeRow.ID != uuid.Nil {
		s.invalidateCode(r.Context(), codeRow)
	}
	s.respondApproval(w, r, ws.ID, id)
}

func (s *Server) respondApproval(w http.ResponseWriter, r *http.Request, wsID, id uuid.UUID) {
	a, err := s.loadApproval(r.Context(), s.pool, wsID, id, false)
	if err == nil {
		err = s.approvalDetail(r, &a)
	}
	if err != nil {
		fail(w, apierr.Internal("saved, but failed to load the request"))
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// closeApproval ends a request without publishing (rejected, cancelled, expired).
func closeApproval(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string) error {
	vs := status
	if vs == "expired" {
		vs = "cancelled"
	}
	if _, err := tx.Exec(ctx, `UPDATE approval_requests SET status = $2, decided_at = now() WHERE id = $1 AND status = 'pending'`, id, status); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE qr_versions SET approval_status = $2 WHERE approval_request_id = $1 AND approval_status = 'pending'`, id, vs)
	return err
}

// finalizeApproval publishes the held versions: a past effective time moves to now so the
// approved change becomes current, then the current pointer follows.
func (s *Server) finalizeApproval(r *http.Request, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, a approvalDTO) (dbgen.QrCode, error) {
	ctx := r.Context()
	if _, err := tx.Exec(ctx, `WITH req AS (
			UPDATE approval_requests SET status = 'approved', decided_at = now() WHERE id = $1 AND status = 'pending' RETURNING id)
		UPDATE qr_versions v SET approval_status = 'approved', effective_at = GREATEST(v.effective_at, now())
		FROM req WHERE v.approval_request_id = req.id AND v.approval_status = 'pending'`, a.ID); err != nil {
		return dbgen.QrCode{}, err
	}
	if a.QRCodeID == nil {
		return dbgen.QrCode{}, nil
	}
	code, err := q.GetQRCodeForUpdate(ctx, dbgen.GetQRCodeForUpdateParams{ID: *a.QRCodeID, WorkspaceID: ws.ID})
	if err != nil {
		return code, err
	}
	cur, err := q.CurrentEffectiveVersion(ctx, code.ID)
	if err == nil && (!code.CurrentVersionID.Valid || uuid.UUID(code.CurrentVersionID.Bytes) != cur.ID) {
		if err := q.SetQRCodeCurrentVersion(ctx, dbgen.SetQRCodeCurrentVersionParams{VersionID: pgUUID(cur.ID), ID: code.ID, WorkspaceID: ws.ID}); err != nil {
			return code, err
		}
		wsID := ws.ID
		if err := s.audit(r, q, &wsID, "qr.version.activated", "qr_code", &code.ID, map[string]any{
			"version_no": cur.VersionNo, "destination": cur.DestinationUrl, "approval_id": a.ID}); err != nil {
			return code, err
		}
		if cur.SafetyStatus == "pending" && code.SafetyStatus == "safe" {
			if err := q.SetQRCodeSafety(ctx, dbgen.SetQRCodeSafetyParams{SafetyStatus: "pending", ID: code.ID}); err != nil {
				return code, err
			}
		}
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return code, err
	}
	return code, nil
}

// POST /approvals/{id}/cancel: the requester (or a policy manager) withdraws the change.
func (s *Server) handleCancelApproval(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	p := principal(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("approval request not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		a, err := s.loadApproval(r.Context(), tx, ws.ID, id, true)
		if err != nil {
			return err
		}
		if !s.can(r, authz.QRRead, a.folderID) {
			return apierr.NotFound("approval request not found")
		}
		if a.RequestedBy != p.UserID && !authz.FromContext(r.Context()).HasWorkspace(authz.PolicyManage) {
			return forbidden("forbidden", "only the requester or a policy manager can cancel this request")
		}
		if a.Status != "pending" {
			return apierr.Conflict("approval_not_pending", "this request is already "+a.Status)
		}
		if err := closeApproval(r.Context(), tx, id, "cancelled"); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "approval.cancelled", "approval_request", &id, map[string]any{"qr_code_id": a.QRCodeID})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to cancel"))
		return
	}
	s.respondApproval(w, r, ws.ID, id)
}

type overrideReq struct {
	Reason string `json:"reason"`
}

// POST /approvals/{id}/override: break-glass publish by an organisation owner (step-up).
func (s *Server) handleOverrideApproval(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	p := principal(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("approval request not found"))
		return
	}
	var req overrideReq
	if !decode(w, r, &req) {
		return
	}
	if len(strings.TrimSpace(req.Reason)) < 10 {
		fail(w, unprocessable("reason_required", "explain the emergency in at least 10 characters; it goes into the audit log"))
		return
	}
	var role string
	_ = s.pool.QueryRow(r.Context(), `SELECT org_role FROM org_members WHERE org_id = $1 AND user_id = $2 AND status = 'active'`,
		ws.OrgID, p.UserID).Scan(&role)
	if role != "org_owner" {
		fail(w, forbidden("owner_required", "only the organisation owner can publish without approval"))
		return
	}
	var code dbgen.QrCode
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		a, err := s.loadApproval(r.Context(), tx, ws.ID, id, true)
		if err != nil {
			return err
		}
		if a.Status != "pending" {
			return apierr.Conflict("approval_not_pending", "this request is already "+a.Status)
		}
		if _, err := tx.Exec(r.Context(), `UPDATE approval_requests SET override_reason = $2 WHERE id = $1`, id, strings.TrimSpace(req.Reason)); err != nil {
			return err
		}
		if code, err = s.finalizeApproval(r, q, tx, ws, a); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "approval.overridden", "approval_request", &id, map[string]any{
			"reason": strings.TrimSpace(req.Reason), "qr_code_id": a.QRCodeID})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to publish"))
		return
	}
	if code.ID != uuid.Nil {
		s.invalidateCode(r.Context(), code)
	}
	s.respondApproval(w, r, ws.ID, id)
}

// GET /me/approvals: the caller's inbox across workspaces.
func (s *Server) handleMyApprovals(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	wss, err := s.q.ListWorkspacesForUser(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list workspaces"))
		return
	}
	grants := map[uuid.UUID]*authz.Grants{}
	var ids []uuid.UUID
	for _, row := range wss {
		ws := dbgen.Workspace{ID: row.ID, OrgID: row.OrgID}
		g, err := s.access.Grants(r.Context(), p, ws)
		if err != nil || !g.HasAnywhere(authz.QRDestinationApprove) {
			continue
		}
		if s.checkOrgAccess(r, p, row.OrgID) != nil {
			continue // SSO/IP/MFA policy of that organisation isn't met by this session
		}
		grants[row.ID] = g
		ids = append(ids, row.ID)
	}
	out := []approvalDTO{}
	if len(ids) > 0 {
		rows, err := s.pool.Query(r.Context(), `SELECT `+approvalCols+approvalFrom+`
			WHERE ar.status = 'pending' AND ar.expires_at > now() AND ar.workspace_id = ANY($1) AND ar.requested_by <> $2
			  AND NOT EXISTS (SELECT 1 FROM approval_decisions d WHERE d.request_id = ar.id AND d.approver_id = $2)
			ORDER BY ar.created_at LIMIT 500`, ids, p.UserID)
		if err != nil {
			fail(w, apierr.Internal("failed to load approvals"))
			return
		}
		var list []approvalDTO
		for rows.Next() {
			if a, err := scanApproval(rows, nil); err == nil {
				list = append(list, a)
			}
		}
		rows.Close()
		for _, a := range list {
			g := grants[a.WorkspaceID]
			if !g.HasWorkspace(authz.QRDestinationApprove) {
				chain, err := s.folderChain(r.Context(), a.WorkspaceID, a.folderID)
				if err != nil || !g.HasInFolderChain(authz.QRDestinationApprove, chain) {
					continue
				}
			}
			out = append(out, a)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "count": len(out)})
}
