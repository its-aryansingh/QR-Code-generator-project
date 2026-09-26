package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/qr"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// organizeRoutes mounts folders, tags, campaigns and templates under a workspace.
func (s *Server) organizeRoutes(r chi.Router) {
	r.Route("/folders", func(r chi.Router) {
		r.With(authz.Require(authz.QRRead)).Get("/", s.handleListFolders)
		r.With(authz.Require(authz.FolderManage)).Post("/", s.handleCreateFolder)
		r.With(authz.Require(authz.FolderManage)).Patch("/{id}", s.handleUpdateFolder)
		r.With(authz.Require(authz.FolderManage)).Delete("/{id}", s.handleDeleteFolder)
	})
	r.Route("/tags", func(r chi.Router) {
		r.With(authz.Require(authz.QRRead)).Get("/", s.handleListTags)
		r.With(authz.Require(authz.QRUpdate)).Post("/", s.handleCreateTag)
		r.With(authz.Require(authz.QRUpdate)).Patch("/{id}", s.handleUpdateTag)
		r.With(authz.RequireWorkspaceWide(authz.FolderManage)).Delete("/{id}", s.handleDeleteTag)
	})
	r.Route("/campaigns", func(r chi.Router) {
		r.With(authz.Require(authz.QRRead)).Get("/", s.handleListCampaigns)
		r.With(authz.RequireWorkspaceWide(authz.CampaignManage)).Post("/", s.handleCreateCampaign)
		r.With(authz.Require(authz.QRRead)).Get("/{id}", s.handleGetCampaign)
		r.With(authz.RequireWorkspaceWide(authz.CampaignManage)).Patch("/{id}", s.handleUpdateCampaign)
		r.With(authz.RequireWorkspaceWide(authz.CampaignManage)).Delete("/{id}", s.handleDeleteCampaign)
	})
	r.Route("/templates", func(r chi.Router) {
		r.With(authz.Require(authz.QRRead)).Get("/", s.handleListTemplates)
		r.With(authz.RequireWorkspaceWide(authz.TemplateManage)).Post("/", s.handleCreateTemplate)
		r.With(authz.RequireWorkspaceWide(authz.TemplateManage)).Patch("/{id}", s.handleUpdateTemplate)
		r.With(authz.RequireWorkspaceWide(authz.TemplateManage)).Delete("/{id}", s.handleDeleteTemplate)
	})
}

// ---- folders ----------------------------------------------------------------

type folderDTO struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id"`
	Name      string     `json:"name"`
	Position  int32      `json:"position"`
	QRCount   int32      `json:"qr_count"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListFolders(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list folders"))
		return
	}
	allowed, all, err := s.readableFolders(r, authz.QRRead)
	if err != nil {
		fail(w, apierr.Internal("failed to resolve folder access"))
		return
	}
	ok := map[uuid.UUID]bool{}
	for _, a := range allowed {
		ok[a] = true
	}
	out := make([]folderDTO, 0, len(rows))
	for _, f := range rows {
		if !all && !ok[f.ID] {
			continue
		}
		out = append(out, folderDTO{ID: f.ID, ParentID: uuidPtr(f.ParentID), Name: f.Name, Position: f.Position,
			QRCount: f.QrCount, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type folderReq struct {
	Name     opt[string]    `json:"name"`
	ParentID opt[uuid.UUID] `json:"parent_id"`
	Position opt[int32]     `json:"position"`
}

func validFolderName(n string) (string, bool) {
	n = strings.TrimSpace(n)
	return n, n != "" && len(n) <= 80 && !strings.ContainsAny(n, "/\\")
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req folderReq
	if !decode(w, r, &req) {
		return
	}
	name, ok := validFolderName(req.Name.Value)
	if !ok {
		fail(w, unprocessable("invalid_name", "folder name must be 1–80 characters without slashes"))
		return
	}
	parent := req.ParentID.ptr()
	if parent != nil {
		chain, err := s.folderChain(r.Context(), ws.ID, parent)
		if err != nil || len(chain) == 0 {
			fail(w, unprocessable("invalid_parent", "parent folder not found"))
			return
		}
		if len(chain) >= 8 {
			fail(w, unprocessable("too_deep", "folders can be nested at most 8 levels"))
			return
		}
	}
	if !s.can(r, authz.FolderManage, parent) {
		fail(w, forbidden("forbidden", "you cannot create folders here"))
		return
	}
	var pos int32
	if req.Position.Set && !req.Position.Null {
		pos = req.Position.Value
	}
	var f dbgen.Folder
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		f, err = q.CreateFolder(r.Context(), dbgen.CreateFolderParams{ID: idgen.New(), WorkspaceID: ws.ID, ParentID: pgUUIDPtr(parent), Name: name, Position: pos})
		if err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "folder.created", "folder", &f.ID, map[string]any{"name": name, "parent_id": parent})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("folder_exists", "a folder with this name already exists here"))
			return
		}
		fail(w, problemOr500(err, "failed to create folder"))
		return
	}
	writeJSON(w, http.StatusCreated, folderDTO{ID: f.ID, ParentID: uuidPtr(f.ParentID), Name: f.Name, Position: f.Position, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt})
}

func (s *Server) handleUpdateFolder(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("folder not found"))
		return
	}
	if _, err := s.q.GetFolder(r.Context(), dbgen.GetFolderParams{ID: id, WorkspaceID: ws.ID}); err != nil || !s.can(r, authz.FolderManage, &id) {
		fail(w, apierr.NotFound("folder not found"))
		return
	}
	var req folderReq
	if !decode(w, r, &req) {
		return
	}
	p := dbgen.UpdateFolderParams{ID: id, WorkspaceID: ws.ID}
	if req.Name.Set {
		n, ok := validFolderName(req.Name.Value)
		if !ok || req.Name.Null {
			fail(w, unprocessable("invalid_name", "folder name must be 1–80 characters without slashes"))
			return
		}
		p.Name = &n
	}
	if req.ParentID.Set {
		parent := req.ParentID.ptr()
		if parent != nil {
			chain, err := s.folderChain(r.Context(), ws.ID, parent)
			if err != nil || len(chain) == 0 {
				fail(w, unprocessable("invalid_parent", "parent folder not found"))
				return
			}
			for _, c := range chain {
				if c == id {
					fail(w, unprocessable("folder_cycle", "a folder cannot be moved inside itself"))
					return
				}
			}
		}
		if !s.can(r, authz.FolderManage, parent) {
			fail(w, forbidden("forbidden", "you cannot move folders there"))
			return
		}
		p.SetParent, p.ParentID = true, pgUUIDPtr(parent)
	}
	if req.Position.Set && !req.Position.Null {
		v := req.Position.Value
		p.Position = &v
	}
	var f dbgen.Folder
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		if f, err = q.UpdateFolder(r.Context(), p); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "folder.updated", "folder", &id, map[string]any{"name": p.Name, "parent_id": req.ParentID.ptr()})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("folder_exists", "a folder with this name already exists here"))
			return
		}
		fail(w, problemOr500(err, "failed to update folder"))
		return
	}
	writeJSON(w, http.StatusOK, folderDTO{ID: f.ID, ParentID: uuidPtr(f.ParentID), Name: f.Name, Position: f.Position, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt})
}

// DELETE /folders/{id}: only empty folders (codes must be moved first; subfolders cascade only when empty).
func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok || !s.can(r, authz.FolderManage, &id) {
		fail(w, apierr.NotFound("folder not found"))
		return
	}
	tree, err := s.q.FolderDescendants(r.Context(), dbgen.FolderDescendantsParams{ID: id, WorkspaceID: ws.ID})
	if err != nil || len(tree) == 0 {
		fail(w, apierr.NotFound("folder not found"))
		return
	}
	for _, f := range tree {
		n, err := s.q.CountQRCodesInFolder(r.Context(), dbgen.CountQRCodesInFolderParams{WorkspaceID: ws.ID, FolderID: pgUUID(f)})
		if err != nil {
			fail(w, apierr.Internal("failed to check folder"))
			return
		}
		if n > 0 {
			fail(w, apierr.Conflict("folder_not_empty", "move or delete the QR codes in this folder (and its subfolders) first"))
			return
		}
	}
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		if _, err := q.DeleteFolder(r.Context(), dbgen.DeleteFolderParams{ID: id, WorkspaceID: ws.ID}); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "folder.deleted", "folder", &id, nil)
	})
	if err != nil {
		fail(w, apierr.Internal("failed to delete folder"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- tags -------------------------------------------------------------------

type tagReq struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListTags(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list tags"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}

func (s *Server) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req tagReq
	if !decode(w, r, &req) {
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 40 {
		fail(w, unprocessable("invalid_name", "tag name must be 1–40 characters"))
		return
	}
	color := "#64748B"
	if req.Color != nil {
		if !hexColor.MatchString(*req.Color) {
			fail(w, unprocessable("invalid_color", "color must be #RRGGBB"))
			return
		}
		color = *req.Color
	}
	t, err := s.q.CreateTag(r.Context(), dbgen.CreateTagParams{ID: idgen.New(), WorkspaceID: ws.ID, Name: strings.TrimSpace(*req.Name), Color: color})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("tag_exists", "a tag with this name already exists"))
			return
		}
		fail(w, apierr.Internal("failed to create tag"))
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleUpdateTag(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("tag not found"))
		return
	}
	var req tagReq
	if !decode(w, r, &req) {
		return
	}
	if req.Color != nil && !hexColor.MatchString(*req.Color) {
		fail(w, unprocessable("invalid_color", "color must be #RRGGBB"))
		return
	}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 40 {
			fail(w, unprocessable("invalid_name", "tag name must be 1–40 characters"))
			return
		}
		req.Name = &n
	}
	t, err := s.q.UpdateTag(r.Context(), dbgen.UpdateTagParams{ID: id, WorkspaceID: ws.ID, Name: req.Name, Color: req.Color})
	if err != nil {
		if isNotFound(err) {
			fail(w, apierr.NotFound("tag not found"))
			return
		}
		if pgCode(err) == sqlUniqueViolation {
			fail(w, apierr.Conflict("tag_exists", "a tag with this name already exists"))
			return
		}
		fail(w, apierr.Internal("failed to update tag"))
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("tag not found"))
		return
	}
	n, err := s.q.DeleteTag(r.Context(), dbgen.DeleteTagParams{ID: id, WorkspaceID: workspaceRow(r).ID})
	if err != nil || n == 0 {
		fail(w, apierr.NotFound("tag not found"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- campaigns --------------------------------------------------------------

type campaignReq struct {
	Name      opt[string]          `json:"name"`
	Status    opt[string]          `json:"status"`
	StartsAt  opt[time.Time]       `json:"starts_at"`
	EndsAt    opt[time.Time]       `json:"ends_at"`
	GoalScans opt[int64]           `json:"goal_scans"`
	UTM       opt[json.RawMessage] `json:"utm"`
}

var campaignStatuses = map[string]bool{"draft": true, "active": true, "paused": true, "ended": true, "archived": true}

func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListCampaigns(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list campaigns"))
		return
	}
	type campaignDTO struct {
		dbgen.ListCampaignsRow
		StartsAt *time.Time      `json:"starts_at"`
		EndsAt   *time.Time      `json:"ends_at"`
		UTM      json.RawMessage `json:"utm"`
	}
	out := make([]campaignDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, campaignDTO{ListCampaignsRow: c, StartsAt: tsPtr(c.StartsAt), EndsAt: tsPtr(c.EndsAt), UTM: nonNullJSON(c.Utm, "{}")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func campaignJSON(c dbgen.Campaign) map[string]any {
	return map[string]any{"id": c.ID, "workspace_id": c.WorkspaceID, "name": c.Name, "status": c.Status,
		"starts_at": tsPtr(c.StartsAt), "ends_at": tsPtr(c.EndsAt), "goal_scans": c.GoalScans,
		"utm": nonNullJSON(c.Utm, "{}"), "created_by": uuidPtr(c.CreatedBy), "created_at": c.CreatedAt, "updated_at": c.UpdatedAt}
}

func (s *Server) handleGetCampaign(w http.ResponseWriter, r *http.Request) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("campaign not found"))
		return
	}
	c, err := s.q.GetCampaign(r.Context(), dbgen.GetCampaignParams{ID: id, WorkspaceID: workspaceRow(r).ID})
	if err != nil {
		fail(w, apierr.NotFound("campaign not found"))
		return
	}
	writeJSON(w, http.StatusOK, campaignJSON(c))
}

func (s *Server) validateCampaign(req campaignReq, existing *dbgen.Campaign) error {
	if req.Name.Set {
		n := strings.TrimSpace(req.Name.Value)
		if req.Name.Null || n == "" || len(n) > 120 {
			return unprocessable("invalid_name", "campaign name must be 1–120 characters")
		}
	}
	if req.Status.Set && !campaignStatuses[req.Status.Value] {
		return unprocessable("invalid_status", "status must be draft, active, paused, ended or archived")
	}
	if req.GoalScans.Set && !req.GoalScans.Null && req.GoalScans.Value <= 0 {
		return unprocessable("invalid_goal", "goal_scans must be positive")
	}
	starts, ends := (*time.Time)(nil), (*time.Time)(nil)
	if existing != nil {
		starts, ends = tsPtr(existing.StartsAt), tsPtr(existing.EndsAt)
	}
	if req.StartsAt.Set {
		starts = req.StartsAt.ptr()
	}
	if req.EndsAt.Set {
		ends = req.EndsAt.ptr()
	}
	if starts != nil && ends != nil && !ends.After(*starts) {
		return unprocessable("invalid_window", "ends_at must be after starts_at")
	}
	if req.UTM.Set && !req.UTM.Null {
		if _, _, err := normaliseUTM(req.UTM.Value); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	if err := s.ent().CheckFeature(r.Context(), ws, entitlements.FeatureCampaigns); err != nil {
		fail(w, featureErr(err))
		return
	}
	var req campaignReq
	if !decode(w, r, &req) {
		return
	}
	if !req.Name.Set {
		fail(w, unprocessable("invalid_name", "campaign name is required"))
		return
	}
	if err := s.validateCampaign(req, nil); err != nil {
		fail(w, err)
		return
	}
	status := "active"
	if req.Status.Set {
		status = req.Status.Value
	}
	utm := json.RawMessage("{}")
	if req.UTM.Set && !req.UTM.Null {
		utm, _, _ = normaliseUTM(req.UTM.Value)
	}
	userID, _ := actorIDs(r)
	var c dbgen.Campaign
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		c, err = q.CreateCampaign(r.Context(), dbgen.CreateCampaignParams{ID: idgen.New(), WorkspaceID: ws.ID,
			Name: strings.TrimSpace(req.Name.Value), Status: status, StartsAt: pgTS(req.StartsAt.ptr()), EndsAt: pgTS(req.EndsAt.ptr()),
			GoalScans: req.GoalScans.ptr(), Utm: utm, CreatedBy: pgUUIDPtr(userID)})
		if err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "campaign.created", "campaign", &c.ID, map[string]any{"name": c.Name})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create campaign"))
		return
	}
	writeJSON(w, http.StatusCreated, campaignJSON(c))
}

func (s *Server) handleUpdateCampaign(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("campaign not found"))
		return
	}
	existing, err := s.q.GetCampaign(r.Context(), dbgen.GetCampaignParams{ID: id, WorkspaceID: ws.ID})
	if err != nil {
		fail(w, apierr.NotFound("campaign not found"))
		return
	}
	var req campaignReq
	if !decode(w, r, &req) {
		return
	}
	if err := s.validateCampaign(req, &existing); err != nil {
		fail(w, err)
		return
	}
	p := dbgen.UpdateCampaignParams{ID: id, WorkspaceID: ws.ID}
	if req.Name.Set {
		n := strings.TrimSpace(req.Name.Value)
		p.Name = &n
	}
	if req.Status.Set {
		p.Status = &req.Status.Value
	}
	if req.StartsAt.Set {
		p.SetStartsAt, p.StartsAt = true, pgTS(req.StartsAt.ptr())
	}
	if req.EndsAt.Set {
		p.SetEndsAt, p.EndsAt = true, pgTS(req.EndsAt.ptr())
	}
	if req.GoalScans.Set {
		p.SetGoal, p.GoalScans = true, req.GoalScans.ptr()
	}
	if req.UTM.Set {
		utm := json.RawMessage("{}")
		if !req.UTM.Null {
			utm, _, _ = normaliseUTM(req.UTM.Value)
		}
		p.Utm = utm
	}
	var c dbgen.Campaign
	err = s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		if c, err = q.UpdateCampaign(r.Context(), p); err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "campaign.updated", "campaign", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update campaign"))
		return
	}
	writeJSON(w, http.StatusOK, campaignJSON(c))
}

func (s *Server) handleDeleteCampaign(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("campaign not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		n, err := q.DeleteCampaign(r.Context(), dbgen.DeleteCampaignParams{ID: id, WorkspaceID: ws.ID})
		if err != nil {
			return err
		}
		if n == 0 {
			return apierr.NotFound("campaign not found")
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "campaign.deleted", "campaign", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete campaign"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- templates --------------------------------------------------------------

type templateReq struct {
	Name      *string      `json:"name"`
	Design    *qr.DesignV1 `json:"design"`
	IsLocked  *bool        `json:"is_locked"`
	IsDefault *bool        `json:"is_default"`
}

func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := s.q.ListTemplates(r.Context(), workspaceRow(r).ID)
	if err != nil {
		fail(w, apierr.Internal("failed to list templates"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}

func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	ctx := r.Context()
	if err := s.ent().CheckFeature(ctx, ws, entitlements.FeatureTemplates); err != nil {
		fail(w, featureErr(err))
		return
	}
	var req templateReq
	if !decode(w, r, &req) {
		return
	}
	if req.Name == nil || strings.TrimSpace(*req.Name) == "" || len(*req.Name) > 80 {
		fail(w, unprocessable("invalid_name", "template name must be 1–80 characters"))
		return
	}
	if req.Design == nil {
		fail(w, unprocessable("design_required", "design is required"))
		return
	}
	canon, _, err := qr.CanonicalDesignJSON(req.Design)
	if err != nil {
		fail(w, unprocessable("invalid_design", "design: "+err.Error()))
		return
	}
	locked := req.IsLocked != nil && *req.IsLocked
	if locked {
		if err := s.ent().CheckFeature(ctx, ws, entitlements.FeatureLockedTemplates); err != nil {
			fail(w, featureErr(err))
			return
		}
	}
	n, err := s.q.CountTemplates(ctx, ws.ID)
	if err != nil {
		fail(w, apierr.Internal("failed to count templates"))
		return
	}
	if int(n) >= s.ent().Limits(ctx, ws).Templates {
		fail(w, paymentRequired("limit_reached", "template limit reached for this plan"))
		return
	}
	isDefault := req.IsDefault != nil && *req.IsDefault
	userID, _ := actorIDs(r)
	var t dbgen.Template
	err = s.inTx(ctx, func(q *dbgen.Queries, _ pgx.Tx) error {
		if isDefault {
			if err := q.ClearDefaultTemplate(ctx, ws.ID); err != nil {
				return err
			}
		}
		var err error
		t, err = q.CreateTemplate(ctx, dbgen.CreateTemplateParams{ID: idgen.New(), WorkspaceID: ws.ID, Name: strings.TrimSpace(*req.Name),
			Design: canon, IsLocked: locked, IsDefault: isDefault, CreatedBy: pgUUIDPtr(userID)})
		if err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "template.created", "template", &t.ID, map[string]any{"name": t.Name, "locked": locked, "default": isDefault})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to create template"))
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	ctx := r.Context()
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("template not found"))
		return
	}
	var req templateReq
	if !decode(w, r, &req) {
		return
	}
	p := dbgen.UpdateTemplateParams{ID: id, WorkspaceID: ws.ID, IsLocked: req.IsLocked, IsDefault: req.IsDefault}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 80 {
			fail(w, unprocessable("invalid_name", "template name must be 1–80 characters"))
			return
		}
		p.Name = &n
	}
	if req.Design != nil {
		canon, _, err := qr.CanonicalDesignJSON(req.Design)
		if err != nil {
			fail(w, unprocessable("invalid_design", "design: "+err.Error()))
			return
		}
		p.Design = canon
	}
	if req.IsLocked != nil && *req.IsLocked {
		if err := s.ent().CheckFeature(ctx, ws, entitlements.FeatureLockedTemplates); err != nil {
			fail(w, featureErr(err))
			return
		}
	}
	var t dbgen.Template
	err := s.inTx(ctx, func(q *dbgen.Queries, _ pgx.Tx) error {
		if req.IsDefault != nil && *req.IsDefault {
			if err := q.ClearDefaultTemplate(ctx, ws.ID); err != nil {
				return err
			}
		}
		var err error
		if t, err = q.UpdateTemplate(ctx, p); err != nil {
			if isNotFound(err) {
				return apierr.NotFound("template not found")
			}
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "template.updated", "template", &id, map[string]any{"locked": req.IsLocked, "default": req.IsDefault})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to update template"))
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("template not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		n, err := q.DeleteTemplate(r.Context(), dbgen.DeleteTemplateParams{ID: id, WorkspaceID: ws.ID})
		if err != nil {
			return err
		}
		if n == 0 {
			return apierr.NotFound("template not found")
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "template.deleted", "template", &id, nil)
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to delete template"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
