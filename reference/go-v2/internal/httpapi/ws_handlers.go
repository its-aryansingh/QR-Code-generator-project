package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/auth"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/workspace"
)

const inviteTTL = 7 * 24 * time.Hour

// ---- workspaces -------------------------------------------------------------

func (s *Server) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListWorkspacesForUser(r.Context(), principal(r).UserID)
	if err != nil {
		fail(w, apierr.Internal("failed to list workspaces"))
		return
	}
	out := make([]workspaceDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, toWorkspaceDTO(dbgen.Workspace{ID: row.ID, Name: row.Name, Slug: row.Slug, OwnerID: row.OwnerID,
			PlanID: row.PlanID, Timezone: row.Timezone, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			OrgID: row.OrgID, IsSandbox: row.IsSandbox}, row.Role))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type createWorkspaceReq struct {
	Name  string     `json:"name"`
	Slug  string     `json:"slug"`
	OrgID *uuid.UUID `json:"org_id"`
}

func (s *Server) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req createWorkspaceReq
	if !decode(w, r, &req) {
		return
	}
	p := principal(r)
	if p.IsAPIKey() {
		fail(w, forbidden("api_key_not_allowed", "API keys cannot create workspaces"))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		fail(w, unprocessable("invalid_name", "name must be 1–80 characters"))
		return
	}
	slug := trimLower(req.Slug)
	if slug != "" {
		if err := workspace.ValidateSlug(slug); err != nil {
			fail(w, unprocessable("invalid_slug", err.Error()))
			return
		}
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Unauthorized("user not found"))
		return
	}
	// Inside an organisation: org.manage is required and the org's plan/contract caps the
	// number of workspaces. Without one, a new organisation is created and the best plan among
	// organisations the user owns caps how many they may own (a free user owns one).
	if req.OrgID != nil {
		_, perms, err := s.access.OrgPermissions(r.Context(), *req.OrgID, u.ID)
		if err != nil {
			fail(w, apierr.Internal("failed to check organisation access"))
			return
		}
		if !hasPerm(perms, authz.OrgManage) {
			fail(w, apierr.NotFound("organization not found"))
			return
		}
		eff, err := s.plans.ForOrg(r.Context(), *req.OrgID)
		if err != nil {
			fail(w, apierr.Internal("failed to check plan"))
			return
		}
		var n int
		_ = s.pool.QueryRow(r.Context(), `SELECT count(*) FROM workspaces WHERE org_id = $1 AND deleted_at IS NULL`, *req.OrgID).Scan(&n)
		if n >= eff.Limits.OwnedWorkspaces {
			pd := paymentRequired("limit_reached", "this organisation has reached its workspace limit")
			pd.RequiredPlan = string(entitlements.NextPlan(eff.Plan))
			fail(w, pd)
			return
		}
	} else {
		rows, err := s.q.ListWorkspacesForUser(r.Context(), u.ID)
		if err != nil {
			fail(w, apierr.Internal("failed to check workspace quota"))
			return
		}
		owned, limit, best := 0, entitlements.GetLimits("free").OwnedWorkspaces, entitlements.PlanFree
		for _, row := range rows {
			if row.OwnerID != u.ID {
				continue
			}
			owned++
			if l := entitlements.GetLimits(row.PlanID).OwnedWorkspaces; l > limit {
				limit, best = l, entitlements.NormalisePlan(row.PlanID)
			}
		}
		if owned >= limit {
			pd := paymentRequired("limit_reached", "you already own the maximum number of workspaces for your plan")
			pd.RequiredPlan = string(entitlements.NextPlan(best))
			fail(w, pd)
			return
		}
	}
	var ws dbgen.Workspace
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var err error
		ws, err = s.createWorkspace(r.Context(), q, tx, u, name, slug, req.OrgID)
		if err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "workspace.created", "workspace", &id, map[string]any{"name": ws.Name, "slug": ws.Slug})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("slug_exists", "a workspace with this slug already exists"))
			return
		}
		var pd *apierr.ProblemDetails
		if errors.As(err, &pd) {
			fail(w, pd)
			return
		}
		slog.Error("create workspace", "error", err)
		fail(w, apierr.Internal("failed to create workspace"))
		return
	}
	writeJSON(w, http.StatusCreated, toWorkspaceDTO(ws, "owner"))
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	g := authz.FromContext(r.Context())
	role := ""
	if p := principal(r); p != nil && !p.IsAPIKey() {
		if m, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: p.UserID}); err == nil {
			role = m.Role
		}
	}
	type folderGrant struct {
		FolderID    uuid.UUID          `json:"folder_id"`
		Permissions []authz.Permission `json:"permissions"`
	}
	var folders []folderGrant
	for _, perm := range authz.Catalogue {
		for _, f := range g.FolderGrantsFor(perm) {
			found := false
			for i := range folders {
				if folders[i].FolderID == f {
					folders[i].Permissions = append(folders[i].Permissions, perm)
					found = true
				}
			}
			if !found {
				folders = append(folders, folderGrant{FolderID: f, Permissions: []authz.Permission{perm}})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspace":          toWorkspaceDTO(ws, role),
		"permissions":        g.List(),
		"folder_permissions": folders,
		"settings":           nonNullJSON(ws.Settings, "{}"),
	})
}

type updateWorkspaceReq struct {
	Name     *string `json:"name"`
	Timezone *string `json:"timezone"`
}

func (s *Server) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req updateWorkspaceReq
	if !decode(w, r, &req) {
		return
	}
	ws := workspaceRow(r)
	changes := map[string]any{}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 80 {
			fail(w, unprocessable("invalid_name", "name must be 1–80 characters"))
			return
		}
		req.Name = &n
		changes["name"] = map[string]any{"before": ws.Name, "after": n}
	}
	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil || *req.Timezone == "" {
			fail(w, unprocessable("invalid_timezone", "timezone must be an IANA zone name such as Asia/Kolkata"))
			return
		}
		changes["timezone"] = map[string]any{"before": ws.Timezone, "after": *req.Timezone}
	}
	var out dbgen.Workspace
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		out, err = q.UpdateWorkspaceSettings(r.Context(), dbgen.UpdateWorkspaceSettingsParams{ID: ws.ID, Name: req.Name, Timezone: req.Timezone})
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			return nil
		}
		id := ws.ID
		return s.audit(r, q, &id, "workspace.updated", "workspace", &id, changes)
	})
	if err != nil {
		fail(w, apierr.Internal("failed to update workspace"))
		return
	}
	writeJSON(w, http.StatusOK, toWorkspaceDTO(out, ""))
}

// DELETE /workspaces/{ws}: owner only (billing.manage is owner-only via "*").
func (s *Server) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		if err := q.SoftDeleteWorkspace(r.Context(), ws.ID); err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "workspace.deleted", "workspace", &id, map[string]any{"slug": ws.Slug})
	})
	if err != nil {
		fail(w, apierr.Internal("failed to delete workspace"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// usage returns live counters used by entitlements and the billing page.
func (s *Server) usage(ctx context.Context, wsID uuid.UUID) (map[string]int, error) {
	dyn, err := s.q.CountActiveDynamicQRCodes(ctx, wsID)
	if err != nil {
		return nil, err
	}
	members, err := s.q.CountWorkspaceMembers(ctx, wsID)
	if err != nil {
		return nil, err
	}
	invites, err := s.q.CountPendingInvites(ctx, wsID)
	if err != nil {
		return nil, err
	}
	templates, err := s.q.CountTemplates(ctx, wsID)
	if err != nil {
		return nil, err
	}
	domains, err := s.q.ListDomainsForWorkspace(ctx, pgUUID(wsID))
	if err != nil {
		return nil, err
	}
	return map[string]int{
		"dynamic_codes": int(dyn), "seats": int(members + invites), "members": int(members),
		"pending_invites": int(invites), "templates": int(templates), "custom_domains": len(domains),
	}, nil
}

func (s *Server) handleGetEntitlements(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	u, err := s.usage(r.Context(), ws.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to compute usage"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plan":     entitlements.NormalisePlan(ws.PlanID),
		"limits":   s.ent().Limits(r.Context(), ws),
		"features": s.ent().Features(r.Context(), ws),
		"usage":    u,
	})
}

// ---- members ----------------------------------------------------------------

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListWorkspaceMembers(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list members"))
		return
	}
	out := make([]memberDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, memberDTO{UserID: m.UserID, Email: m.Email, Name: m.Name, AvatarURL: m.AvatarUrl, Role: m.Role, JoinedAt: m.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// assignableMemberRole reports whether role may be granted through membership (never owner).
func assignableMemberRole(role string) bool {
	if role == "owner" {
		return false
	}
	_, ok := authz.SystemRoles[role]
	return ok
}

type updateMemberReq struct {
	Role string `json:"role"`
}

func (s *Server) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	target, ok := uuidParam(r, "userId")
	if !ok {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	var req updateMemberReq
	if !decode(w, r, &req) {
		return
	}
	role := trimLower(req.Role)
	if !assignableMemberRole(role) {
		fail(w, unprocessable("invalid_role", "role must be one of admin, editor, reviewer, analyst (use transfer-ownership for owner)"))
		return
	}
	actor := principal(r)
	g := authz.FromContext(r.Context())
	cur, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: target})
	if err != nil {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	if cur.Role == "owner" {
		fail(w, forbidden("owner_immutable", "the owner's role cannot be changed; transfer ownership instead"))
		return
	}
	// Only the owner may create or demote admins; admins cannot escalate peers.
	if (role == "admin" || cur.Role == "admin") && !g.IsOwner() {
		fail(w, forbidden("owner_required", "only the workspace owner can grant or revoke admin"))
		return
	}
	if target == actor.UserID {
		fail(w, forbidden("self_role_change", "you cannot change your own role"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if err := q.UpdateWorkspaceMemberRole(r.Context(), dbgen.UpdateWorkspaceMemberRoleParams{WorkspaceID: ws.ID, UserID: target, Role: role}); err != nil {
			return err
		}
		if err := s.mem().MemberRoleChanged(r.Context(), q, tx, ws, target, role); err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "member.role.changed", "user", &target, map[string]any{"before": cur.Role, "after": role})
	})
	if err != nil {
		if pgCode(err) == sqlCheckViolation {
			fail(w, unprocessable("invalid_role", "role is not supported by this workspace"))
			return
		}
		fail(w, err)
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user_id": target, "role": role})
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	target, ok := uuidParam(r, "userId")
	if !ok {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	cur, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: target})
	if err != nil {
		fail(w, apierr.NotFound("member not found"))
		return
	}
	if cur.Role == "owner" {
		fail(w, forbidden("owner_immutable", "the owner cannot be removed; transfer ownership first"))
		return
	}
	if cur.Role == "admin" && !authz.FromContext(r.Context()).IsOwner() {
		fail(w, forbidden("owner_required", "only the workspace owner can remove an admin"))
		return
	}
	if err := s.removeMember(r, ws, target, cur.Role, "member.removed"); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /workspaces/{ws}/leave: any non-owner member may leave.
func (s *Server) handleLeaveWorkspace(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	p := principal(r)
	cur, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: p.UserID})
	if err != nil {
		fail(w, apierr.NotFound("you are not a member of this workspace"))
		return
	}
	if cur.Role == "owner" {
		fail(w, forbidden("owner_cannot_leave", "transfer ownership before leaving the workspace"))
		return
	}
	if err := s.removeMember(r, ws, p.UserID, cur.Role, "member.left"); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeMember(r *http.Request, ws dbgen.Workspace, target uuid.UUID, role, action string) error {
	defer s.access.Invalidate(r.Context())
	return s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		if err := q.RemoveWorkspaceMember(r.Context(), dbgen.RemoveWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: target}); err != nil {
			return err
		}
		if err := s.mem().MemberRemoved(r.Context(), q, tx, ws, target); err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, action, "user", &target, map[string]any{"role": role})
	})
}

type transferReq struct {
	UserID uuid.UUID `json:"user_id"`
}

// POST /workspaces/{ws}/transfer-ownership: owner hands the workspace to an existing member,
// who becomes owner; the previous owner becomes admin.
func (s *Server) handleTransferOwnership(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req transferReq
	if !decode(w, r, &req) {
		return
	}
	p := principal(r)
	if req.UserID == p.UserID {
		fail(w, unprocessable("already_owner", "you already own this workspace"))
		return
	}
	if _, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: req.UserID}); err != nil {
		fail(w, unprocessable("not_a_member", "the new owner must already be a member of the workspace"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		// Demote first: a partial unique index allows only one owner row.
		if err := q.UpdateWorkspaceMemberRole(r.Context(), dbgen.UpdateWorkspaceMemberRoleParams{WorkspaceID: ws.ID, UserID: p.UserID, Role: "admin"}); err != nil {
			return err
		}
		if err := q.UpdateWorkspaceMemberRole(r.Context(), dbgen.UpdateWorkspaceMemberRoleParams{WorkspaceID: ws.ID, UserID: req.UserID, Role: "owner"}); err != nil {
			return err
		}
		if err := q.TransferWorkspaceOwnership(r.Context(), dbgen.TransferWorkspaceOwnershipParams{ID: ws.ID, OwnerID: req.UserID}); err != nil {
			return err
		}
		if err := s.mem().MemberRoleChanged(r.Context(), q, tx, ws, p.UserID, "admin"); err != nil {
			return err
		}
		if err := s.mem().MemberRoleChanged(r.Context(), q, tx, ws, req.UserID, "owner"); err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "workspace.ownership.transferred", "workspace", &id,
			map[string]any{"from": p.UserID, "to": req.UserID})
	})
	if err != nil {
		slog.Error("transfer ownership", "error", err)
		fail(w, apierr.Internal("failed to transfer ownership"))
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"owner_id": req.UserID})
}

// ---- invites ----------------------------------------------------------------

func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListInvites(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list invites"))
		return
	}
	out := make([]inviteDTO, 0, len(rows))
	for _, i := range rows {
		out = append(out, toInviteDTO(i))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type createInviteReq struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req createInviteReq
	if !decode(w, r, &req) {
		return
	}
	emailAddr, ok := validEmail(req.Email)
	if !ok {
		fail(w, unprocessable("invalid_email", "a valid email address is required"))
		return
	}
	role := trimLower(req.Role)
	if role == "" {
		role = "editor"
	}
	if !assignableMemberRole(role) {
		fail(w, unprocessable("invalid_role", "role must be one of admin, editor, reviewer, analyst"))
		return
	}
	g := authz.FromContext(r.Context())
	if role == "admin" && !g.IsOwner() {
		fail(w, forbidden("owner_required", "only the workspace owner can invite admins"))
		return
	}
	if pol, err := s.access.Policy(r.Context(), ws.OrgID); err == nil && len(pol.InviteEmailDomains) > 0 {
		domain := emailAddr[strings.LastIndex(emailAddr, "@")+1:]
		allowed := false
		for _, d := range pol.InviteEmailDomains {
			if strings.EqualFold(d, domain) {
				allowed = true
			}
		}
		if !allowed {
			fail(w, forbidden("invite_domain_not_allowed", "your organisation only allows invites to "+strings.Join(pol.InviteEmailDomains, ", ")))
			return
		}
	}
	actor := principal(r)
	if u, err := s.q.GetUserByEmail(r.Context(), emailAddr); err == nil {
		if _, err := s.q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: u.ID}); err == nil {
			fail(w, apierr.Conflict("already_member", "this person is already a member of the workspace"))
			return
		}
	}
	if err := s.checkSeat(r.Context(), ws); err != nil {
		fail(w, err)
		return
	}
	plain, hash, err := workspace.GenerateInviteToken()
	if err != nil {
		fail(w, apierr.Internal("failed to create invite"))
		return
	}
	var inv dbgen.Invite
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		inv, err = q.CreateInvite(r.Context(), dbgen.CreateInviteParams{
			ID: idgen.New(), WorkspaceID: ws.ID, Email: emailAddr, Role: role, TokenHash: hash,
			InvitedBy: actor.UserID, ExpiresAt: time.Now().UTC().Add(inviteTTL),
		})
		if err != nil {
			return err
		}
		id := ws.ID
		return s.audit(r, q, &id, "invite.created", "invite", &inv.ID, map[string]any{"email": emailAddr, "role": role})
	})
	if err != nil {
		switch pgCode(err) {
		case sqlUniqueViolation:
			fail(w, apierr.Conflict("invite_exists", "an active invite already exists for this email"))
		case sqlCheckViolation:
			fail(w, unprocessable("invalid_role", "role is not supported by this workspace"))
		default:
			slog.Error("create invite", "error", err)
			fail(w, apierr.Internal("failed to create invite"))
		}
		return
	}
	inviter := "A teammate"
	if u, err := s.q.GetUserByID(r.Context(), actor.UserID); err == nil {
		inviter = u.Name
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.mail.SendInvite(ctx, emailAddr, inviter, ws.Name, plain, s.cfg.AppBaseURL); err != nil {
			slog.Error("send invite email", "error", err)
		}
	}()
	resp := map[string]any{"invite": toInviteDTO(inv)}
	if s.cfg.IsLocal() {
		resp["accept_url"] = strings.TrimRight(s.cfg.AppBaseURL, "/") + "/invites/" + plain
	}
	writeJSON(w, http.StatusCreated, resp)
}

// checkSeat enforces the seat limit: members + pending invites.
func (s *Server) checkSeat(ctx context.Context, ws dbgen.Workspace) error {
	members, err := s.q.CountWorkspaceMembers(ctx, ws.ID)
	if err != nil {
		return apierr.Internal("failed to check seats")
	}
	pending, err := s.q.CountPendingInvites(ctx, ws.ID)
	if err != nil {
		return apierr.Internal("failed to check seats")
	}
	limit := s.ent().Limits(ctx, ws).Seats
	if int(members+pending) >= limit {
		p := entitlements.NormalisePlan(ws.PlanID)
		pd := paymentRequired("seat_limit_reached", "all seats on this plan are in use; remove a member or upgrade")
		pd.RequiredPlan = string(entitlements.NextPlan(p))
		return pd
	}
	return nil
}

func (s *Server) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("invite not found"))
		return
	}
	inv, err := s.q.GetInviteScoped(r.Context(), dbgen.GetInviteScopedParams{ID: id, WorkspaceID: ws.ID})
	if err != nil || inv.AcceptedAt.Valid || inv.RevokedAt.Valid {
		fail(w, apierr.NotFound("invite not found"))
		return
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		if err := q.RevokeInvite(r.Context(), dbgen.RevokeInviteParams{ID: id, WorkspaceID: ws.ID}); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "invite.revoked", "invite", &id, map[string]any{"email": inv.Email})
	})
	if err != nil {
		fail(w, apierr.Internal("failed to revoke invite"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /invites/{token}: unauthenticated preview for the accept page.
func (s *Server) handleInvitePreview(w http.ResponseWriter, r *http.Request) {
	inv, err := s.q.GetInviteByHashAny(r.Context(), auth.HashToken(chi.URLParam(r, "token")))
	if err != nil {
		fail(w, apierr.NotFound("invite not found"))
		return
	}
	status := "pending"
	switch {
	case inv.AcceptedAt.Valid:
		status = "accepted"
	case inv.RevokedAt.Valid:
		status = "revoked"
	case time.Now().After(inv.ExpiresAt):
		status = "expired"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspace_name": inv.WorkspaceName, "workspace_slug": inv.WorkspaceSlug,
		"email": maskEmail(inv.Email), "role": inv.Role, "expires_at": inv.ExpiresAt, "status": status,
	})
}

// POST /invites/{token}/accept: the signed-in user's email must match the invite.
func (s *Server) handleInviteAccept(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if p.IsAPIKey() {
		fail(w, forbidden("api_key_not_allowed", "API keys cannot accept invites"))
		return
	}
	u, err := s.q.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		fail(w, apierr.Unauthorized("user not found"))
		return
	}
	hash := auth.HashToken(chi.URLParam(r, "token"))
	inv, err := s.q.GetInviteByHash(r.Context(), hash)
	if err != nil {
		fail(w, apierr.New(http.StatusGone, "invite_invalid", "Gone", "this invite is invalid, expired, revoked or already used"))
		return
	}
	if !strings.EqualFold(inv.Email, u.Email) {
		fail(w, forbidden("email_mismatch", "this invite was sent to a different email address; sign in with that account"))
		return
	}
	ws, err := s.q.GetWorkspaceByID(r.Context(), inv.WorkspaceID)
	if err != nil {
		fail(w, apierr.NotFound("workspace not found"))
		return
	}
	role := inv.Role
	err = s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		tag, err := tx.Exec(r.Context(), `UPDATE invites SET accepted_at = now()
			WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > now()`, inv.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apierr.New(http.StatusGone, "invite_invalid", "Gone", "this invite was already used")
		}
		existing, err := q.GetWorkspaceMember(r.Context(), dbgen.GetWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: u.ID})
		switch {
		case err == nil:
			role = existing.Role // never downgrade (or overwrite the owner) through an invite
		case isNotFound(err):
			if err := q.AddWorkspaceMember(r.Context(), dbgen.AddWorkspaceMemberParams{WorkspaceID: ws.ID, UserID: u.ID, Role: inv.Role}); err != nil {
				return err
			}
			if err := s.mem().MemberAdded(r.Context(), q, tx, ws, u.ID, inv.Role); err != nil {
				return err
			}
		default:
			return err
		}
		// The invite link proves control of the mailbox.
		if !u.EmailVerifiedAt.Valid {
			if err := q.MarkEmailVerified(r.Context(), u.ID); err != nil {
				return err
			}
		}
		id := ws.ID
		return s.audit(r, q, &id, "invite.accepted", "invite", &inv.ID, map[string]any{"email": inv.Email, "role": role})
	})
	if err != nil {
		fail(w, err)
		return
	}
	s.access.Invalidate(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"workspace": toWorkspaceDTO(ws, role)})
}

func hasPerm(perms []authz.Permission, p authz.Permission) bool {
	for _, x := range perms {
		if x == p || x == authz.All {
			return true
		}
	}
	return false
}
