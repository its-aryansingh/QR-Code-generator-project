package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/its-aryansingh/qrit/services/internal/apierr"
	"github.com/its-aryansingh/qrit/services/internal/authz"
	"github.com/its-aryansingh/qrit/services/internal/entitlements"
	"github.com/its-aryansingh/qrit/services/internal/platform/crypto"
	"github.com/its-aryansingh/qrit/services/internal/platform/db/dbgen"
	"github.com/its-aryansingh/qrit/services/internal/platform/idgen"
	"github.com/its-aryansingh/qrit/services/internal/qr"
	"github.com/its-aryansingh/qrit/services/internal/resolve"
	"github.com/its-aryansingh/qrit/services/internal/routing"
	"github.com/its-aryansingh/qrit/services/internal/shortcode"
	"github.com/its-aryansingh/qrit/services/internal/version"
)

var (
	staticOnlyTypes  = map[string]bool{"text": true, "wifi": true, "upi": true, "location": true}
	dynamicOnlyTypes = map[string]bool{"links_page": true, "file": true, "app_store": true, "gs1": true}
	hostedTypes      = map[string]bool{"links_page": true, "file": true}
	allContentTypes  = map[string]bool{"url": true, "text": true, "email": true, "phone": true, "sms": true, "whatsapp": true,
		"wifi": true, "vcard": true, "event": true, "upi": true, "location": true, "links_page": true, "file": true,
		"app_store": true, "gs1": true}
)

// ---- folder-scoped permission helpers ----------------------------------------

// folderChain returns the folder and its ancestors (nearest first).
func (s *Server) folderChain(ctx context.Context, wsID uuid.UUID, folderID *uuid.UUID) ([]uuid.UUID, error) {
	if folderID == nil {
		return nil, nil
	}
	return s.q.FolderAncestors(ctx, dbgen.FolderAncestorsParams{ID: *folderID, WorkspaceID: wsID})
}

// can reports whether the request's grants allow perm on an object in folderID.
func (s *Server) can(r *http.Request, perm authz.Permission, folderID *uuid.UUID) bool {
	g := authz.FromContext(r.Context())
	if g.HasWorkspace(perm) {
		return true
	}
	if folderID == nil || !g.HasFolderGrants() {
		return false
	}
	chain, err := s.folderChain(r.Context(), workspaceRow(r).ID, folderID)
	if err != nil {
		return false
	}
	return g.HasInFolderChain(perm, chain)
}

// readableFolders returns nil when perm is held workspace-wide, otherwise every folder
// (including descendants) where it is granted.
func (s *Server) readableFolders(r *http.Request, perm authz.Permission) ([]uuid.UUID, bool, error) {
	g := authz.FromContext(r.Context())
	if g.HasWorkspace(perm) {
		return nil, true, nil
	}
	set := map[uuid.UUID]bool{}
	for _, f := range g.FolderGrantsFor(perm) {
		ids, err := s.q.FolderDescendants(r.Context(), dbgen.FolderDescendantsParams{ID: f, WorkspaceID: workspaceRow(r).ID})
		if err != nil {
			return nil, false, err
		}
		for _, id := range ids {
			set[id] = true
		}
	}
	out := make([]uuid.UUID, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out, false, nil
}

// loadQR fetches a code in the current workspace and enforces perm on its folder.
// Codes the caller cannot read are reported as 404 so their existence does not leak.
func (s *Server) loadQR(w http.ResponseWriter, r *http.Request, perm authz.Permission) (dbgen.QrCode, bool) {
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("QR code not found"))
		return dbgen.QrCode{}, false
	}
	code, err := s.q.GetQRCode(r.Context(), dbgen.GetQRCodeParams{ID: id, WorkspaceID: workspaceRow(r).ID})
	if err != nil {
		fail(w, apierr.NotFound("QR code not found"))
		return dbgen.QrCode{}, false
	}
	folder := uuidPtr(code.FolderID)
	if !s.can(r, authz.QRRead, folder) {
		fail(w, apierr.NotFound("QR code not found"))
		return dbgen.QrCode{}, false
	}
	if perm != authz.QRRead && !s.can(r, perm, folder) {
		fail(w, forbidden("forbidden", "you do not have the "+string(perm)+" permission for this QR code"))
		return dbgen.QrCode{}, false
	}
	return code, true
}

func (s *Server) checkWritable(code dbgen.QrCode) error {
	if code.IsReadOnly {
		pd := paymentRequired("read_only_over_limit", "this code is read-only because the workspace is over its plan limit; upgrade to edit it")
		return pd
	}
	if code.Status == "blocked" {
		return forbidden("code_blocked", "this code was blocked by trust & safety and cannot be edited")
	}
	return nil
}

// ---- list / get -------------------------------------------------------------

func listRowToQR(r dbgen.ListQRCodesPageRow) dbgen.QrCode {
	return dbgen.QrCode{ID: r.ID, WorkspaceID: r.WorkspaceID, CreatedBy: r.CreatedBy, Mode: r.Mode, ContentType: r.ContentType,
		Name: r.Name, DomainID: r.DomainID, ShortCode: r.ShortCode, LegacyShortCode: r.LegacyShortCode, Gs1Gtin: r.Gs1Gtin,
		StaticPayload: r.StaticPayload, StaticContent: r.StaticContent, CurrentVersionID: r.CurrentVersionID, Design: r.Design,
		DesignHash: r.DesignHash, TemplateID: r.TemplateID, FolderID: r.FolderID, CampaignID: r.CampaignID, Status: r.Status,
		IsReadOnly: r.IsReadOnly, StartsAt: r.StartsAt, ExpiresAt: r.ExpiresAt, ScanLimit: r.ScanLimit, PasswordHash: r.PasswordHash,
		FallbackUrl: r.FallbackUrl, SafetyStatus: r.SafetyStatus, TotalScans: r.TotalScans, UniqueScans: r.UniqueScans,
		LastScannedAt: r.LastScannedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ArchivedAt: r.ArchivedAt}
}

// GET /qr-codes?search=&status=&mode=&folder_id=&cursor=&limit=
func (s *Server) handleListQRCodes(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	qs := r.URL.Query()
	params := dbgen.ListQRCodesPageParams{WorkspaceID: ws.ID, RowLimit: int32(limitParam(r, 50, 200)) + 1}
	if v := strings.TrimSpace(qs.Get("search")); v != "" {
		params.Search = &v
	}
	if v := qs.Get("status"); v != "" {
		if v != "active" && v != "paused" && v != "archived" && v != "blocked" {
			fail(w, apierr.BadRequest("invalid_status", "status must be active, paused, archived or blocked"))
			return
		}
		params.Status = &v
	}
	if v := qs.Get("mode"); v != "" {
		if v != "static" && v != "dynamic" {
			fail(w, apierr.BadRequest("invalid_mode", "mode must be static or dynamic"))
			return
		}
		params.Mode = &v
	}
	c, err := decodeCursor(qs.Get("cursor"))
	if err != nil {
		fail(w, apierr.BadRequest("invalid_cursor", "cursor is malformed"))
		return
	}
	if c != nil {
		params.CursorCreatedAt = pgTS(&c.T)
		params.CursorID = pgUUID(c.ID)
	}
	allowed, all, err := s.readableFolders(r, authz.QRRead)
	if err != nil {
		fail(w, apierr.Internal("failed to resolve folder access"))
		return
	}
	if v := qs.Get("folder_id"); v != "" {
		fid, perr := uuid.Parse(v)
		if perr != nil {
			fail(w, apierr.BadRequest("invalid_folder_id", "folder_id must be a UUID"))
			return
		}
		sub, err := s.q.FolderDescendants(r.Context(), dbgen.FolderDescendantsParams{ID: fid, WorkspaceID: ws.ID})
		if err != nil {
			fail(w, apierr.Internal("failed to list folder"))
			return
		}
		if !all {
			ok := map[uuid.UUID]bool{}
			for _, a := range allowed {
				ok[a] = true
			}
			var keep []uuid.UUID
			for _, id := range sub {
				if ok[id] {
					keep = append(keep, id)
				}
			}
			sub = keep
		}
		params.FolderIds = sub
		if params.FolderIds == nil {
			params.FolderIds = []uuid.UUID{}
		}
	} else if !all {
		params.FolderIds = allowed
	}
	rows, err := s.q.ListQRCodesPage(r.Context(), params)
	if err != nil {
		slog.Error("list qr codes", "error", err)
		fail(w, apierr.Internal("failed to list QR codes"))
		return
	}
	out := page[qrDTO]{Data: make([]qrDTO, 0, len(rows))}
	limit := int(params.RowLimit) - 1
	if len(rows) > limit {
		last := rows[limit-1]
		cur := encodeCursor(last.CreatedAt, last.ID)
		out.NextCursor = &cur
		rows = rows[:limit]
	}
	for _, row := range rows {
		d := s.toQRDTO(r.Context(), listRowToQR(row))
		d.DestinationURL = row.DestinationUrl
		out.Data = append(out.Data, d)
	}
	writeJSON(w, http.StatusOK, out)
}

// qrDetail assembles the full representation: current + scheduled version and tags.
func (s *Server) qrDetail(ctx context.Context, code dbgen.QrCode) (qrDTO, error) {
	d := s.toQRDTO(ctx, code)
	now := time.Now()
	if code.Mode == "dynamic" {
		if cur, err := s.q.CurrentEffectiveVersion(ctx, code.ID); err == nil {
			id := cur.ID
			v := toVersionDTO(cur, &id, now)
			d.CurrentVersion = &v
			d.DestinationURL = cur.DestinationUrl
		} else if !isNotFound(err) {
			return d, err
		}
		if next, err := s.q.NextScheduledVersion(ctx, code.ID); err == nil {
			v := toVersionDTO(next, nil, now)
			d.ScheduledVersion = &v
		} else if !isNotFound(err) {
			return d, err
		}
	}
	tags, err := s.q.ListQRTags(ctx, code.ID)
	if err != nil {
		return d, err
	}
	d.Tags = make([]tagRef, 0, len(tags))
	for _, t := range tags {
		d.Tags = append(d.Tags, tagRef{ID: t.ID, Name: t.Name, Color: t.Color})
	}
	return d, nil
}

func (s *Server) handleGetQRCode(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRRead)
	if !ok {
		return
	}
	d, err := s.qrDetail(r.Context(), code)
	if err != nil {
		fail(w, apierr.Internal("failed to load QR code"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ---- create -----------------------------------------------------------------

type createQRReq struct {
	Name           string          `json:"name"`
	Mode           string          `json:"mode"`
	ContentType    string          `json:"content_type"`
	DestinationURL string          `json:"destination_url"`
	HostedPage     json.RawMessage `json:"hosted_page"`
	Rules          json.RawMessage `json:"rules"`
	UTM            json.RawMessage `json:"utm"`
	StaticPayload  string          `json:"static_payload"`
	StaticContent  json.RawMessage `json:"static_content"`
	Design         *qr.DesignV1    `json:"design"`
	TemplateID     *uuid.UUID      `json:"template_id"`
	FolderID       *uuid.UUID      `json:"folder_id"`
	CampaignID     *uuid.UUID      `json:"campaign_id"`
	TagIDs         []uuid.UUID     `json:"tag_ids"`
	DomainID       *uuid.UUID      `json:"domain_id"`
	GTIN           string          `json:"gtin"`
	StartsAt       *time.Time      `json:"starts_at"`
	ExpiresAt      *time.Time      `json:"expires_at"`
	ScanLimit      *int64          `json:"scan_limit"`
	Password       *string         `json:"password"`
	FallbackURL    *string         `json:"fallback_url"`
	ChangeNote     *string         `json:"change_note"`
}

// VersionDraft is a validated destination change awaiting application.
type VersionDraft struct {
	DestinationKind string          `json:"destination_kind"`
	DestinationURL  *string         `json:"destination_url,omitempty"`
	HostedPage      json.RawMessage `json:"hosted_page,omitempty"`
	Rules           json.RawMessage `json:"rules"`
	UTM             json.RawMessage `json:"utm"`
	EffectiveAt     time.Time       `json:"effective_at"`
	SafetyStatus    string          `json:"safety_status"`
	RestoredFrom    *uuid.UUID      `json:"restored_from,omitempty"`
	ChangeNote      *string         `json:"change_note,omitempty"`
}

// VersionGate lets the enterprise approvals module hold a destination change for review.
// When Intercept returns handled=true the change is not applied; body is returned with 202.
type VersionGate interface {
	Intercept(r *http.Request, q *dbgen.Queries, tx pgx.Tx, ws dbgen.Workspace, code dbgen.QrCode, d VersionDraft) (handled bool, body any, err error)
}

// SetVersionGate installs the approvals gate.
func (s *Server) SetVersionGate(g VersionGate) { s.versionGate = g }

// versionInput is shared by create-QR and create-version.
type versionInput struct {
	ContentType    string
	DestinationURL string
	HostedPage     json.RawMessage
	Rules          json.RawMessage
	UTM            json.RawMessage
	EffectiveAt    *time.Time
	ChangeNote     *string
}

// buildDraft validates a destination change against policy, safety and plan.
func (s *Server) buildDraft(ctx context.Context, ws dbgen.Workspace, in versionInput) (VersionDraft, error) {
	d := VersionDraft{EffectiveAt: time.Now().UTC(), ChangeNote: in.ChangeNote, SafetyStatus: "safe"}
	if in.ChangeNote != nil && len(*in.ChangeNote) > 500 {
		return d, unprocessable("invalid_change_note", "change_note must be at most 500 characters")
	}
	ent := s.ent()
	if in.EffectiveAt != nil && in.EffectiveAt.After(time.Now().Add(30*time.Second)) {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureScheduling); err != nil {
			return d, featureErr(err)
		}
		if in.EffectiveAt.After(time.Now().AddDate(2, 0, 0)) {
			return d, unprocessable("invalid_effective_at", "changes can be scheduled at most two years ahead")
		}
		d.EffectiveAt = in.EffectiveAt.UTC()
	}
	hosted := len(in.HostedPage) > 0 && string(in.HostedPage) != "null"
	switch {
	case hosted:
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureHostedPages); err != nil {
			return d, featureErr(err)
		}
		page, st, err := s.normaliseHostedPage(ctx, ws, in.HostedPage)
		if err != nil {
			return d, err
		}
		d.DestinationKind, d.HostedPage, d.SafetyStatus = "hosted_page", page, st
	case hostedTypes[in.ContentType]:
		return d, unprocessable("hosted_page_required", in.ContentType+" codes need a hosted_page")
	default:
		if strings.TrimSpace(in.DestinationURL) == "" {
			return d, unprocessable("destination_required", "destination_url is required",
				apierr.FieldError{Field: "destination_url", Code: "required", Message: "required"})
		}
		if err := version.ValidateVersionInput(version.DestinationKindURL, in.DestinationURL); err != nil {
			return d, validationProblem("destination_url", err)
		}
		norm, st, err := s.checkDestination(ctx, ws, "destination_url", in.DestinationURL)
		if err != nil {
			return d, err
		}
		d.DestinationKind, d.DestinationURL, d.SafetyStatus = "url", &norm, st
	}
	rules, rst, err := s.normaliseRules(ctx, ws, in.Rules)
	if err != nil {
		return d, err
	}
	if string(rules) != "[]" {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureRules); err != nil {
			return d, featureErr(err)
		}
	}
	d.Rules, d.SafetyStatus = rules, combineSafety(d.SafetyStatus, rst)
	utm, used, err := normaliseUTM(in.UTM)
	if err != nil {
		return d, err
	}
	if used {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureUTMAppend); err != nil {
			return d, featureErr(err)
		}
	}
	d.UTM = utm
	return d, nil
}

// ApplyVersion writes a validated draft as the next immutable version and, when it is
// effective now, moves the current pointer. Exported for the approvals module.
func (s *Server) ApplyVersion(ctx context.Context, q *dbgen.Queries, code dbgen.QrCode, d VersionDraft, actorUser, actorKey *uuid.UUID) (dbgen.QrVersion, error) {
	no, err := q.MaxQRVersionNo(ctx, code.ID)
	if err != nil {
		return dbgen.QrVersion{}, err
	}
	v, err := q.CreateQRVersionFull(ctx, dbgen.CreateQRVersionFullParams{
		ID: idgen.New(), QrCodeID: code.ID, VersionNo: no + 1, DestinationKind: d.DestinationKind,
		DestinationUrl: d.DestinationURL, HostedPage: d.HostedPage, Rules: d.Rules, Utm: d.UTM,
		EffectiveAt: d.EffectiveAt, SafetyStatus: d.SafetyStatus, RestoredFrom: pgUUIDPtr(d.RestoredFrom),
		ChangeNote: d.ChangeNote, CreatedBy: pgUUIDPtr(actorUser), CreatedByKey: pgUUIDPtr(actorKey),
	})
	if err != nil {
		return v, err
	}
	if !d.EffectiveAt.After(time.Now()) {
		if err := q.SetQRCodeCurrentVersion(ctx, dbgen.SetQRCodeCurrentVersionParams{
			VersionID: pgUUID(v.ID), ID: code.ID, WorkspaceID: code.WorkspaceID}); err != nil {
			return v, err
		}
	}
	return v, nil
}

// actorIDs splits the principal into (user, api key) for created_by columns.
func actorIDs(r *http.Request) (*uuid.UUID, *uuid.UUID) {
	p := principal(r)
	if p == nil {
		return nil, nil
	}
	if p.IsAPIKey() {
		k := p.APIKeyID
		return nil, &k
	}
	u := p.UserID
	return &u, nil
}

func (s *Server) handleCreateQRCode(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	var req createQRReq
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		fail(w, unprocessable("invalid_name", "name must be 1–120 characters",
			apierr.FieldError{Field: "name", Code: "invalid", Message: "1–120 characters"}))
		return
	}
	if req.Mode == "" {
		req.Mode = "dynamic"
	}
	if req.ContentType == "" {
		req.ContentType = "url"
	}
	if req.Mode != "static" && req.Mode != "dynamic" {
		fail(w, unprocessable("invalid_mode", "mode must be static or dynamic"))
		return
	}
	if !allContentTypes[req.ContentType] {
		fail(w, unprocessable("invalid_content_type", "unsupported content_type "+req.ContentType))
		return
	}
	if req.Mode == "dynamic" && staticOnlyTypes[req.ContentType] {
		fail(w, unprocessable("static_only_type", req.ContentType+" codes must be static (the payload is read offline by the phone)"))
		return
	}
	if req.Mode == "static" && dynamicOnlyTypes[req.ContentType] {
		fail(w, unprocessable("dynamic_only_type", req.ContentType+" codes must be dynamic"))
		return
	}
	if !s.can(r, authz.QRCreate, req.FolderID) {
		fail(w, forbidden("forbidden", "you cannot create QR codes in this folder"))
		return
	}
	ctx := r.Context()
	ent := s.ent()

	// Lifecycle controls are plan features and only meaningful on dynamic codes.
	if req.Mode == "static" && (req.StartsAt != nil || req.ExpiresAt != nil || req.ScanLimit != nil || req.Password != nil ||
		req.FallbackURL != nil || req.CampaignID != nil || len(req.Rules) > 0 && string(req.Rules) != "null") {
		fail(w, unprocessable("static_no_lifecycle", "schedules, expiry, scan limits, passwords, rules and campaigns need a dynamic code"))
		return
	}
	if req.StartsAt != nil || req.ExpiresAt != nil {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureExpiry); err != nil {
			fail(w, featureErr(err))
			return
		}
		if req.StartsAt != nil && req.ExpiresAt != nil && !req.ExpiresAt.After(*req.StartsAt) {
			fail(w, unprocessable("invalid_window", "expires_at must be after starts_at"))
			return
		}
	}
	if req.ScanLimit != nil {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureScanLimit); err != nil {
			fail(w, featureErr(err))
			return
		}
		if *req.ScanLimit <= 0 {
			fail(w, unprocessable("invalid_scan_limit", "scan_limit must be positive"))
			return
		}
	}
	if req.CampaignID != nil {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureCampaigns); err != nil {
			fail(w, featureErr(err))
			return
		}
		if _, err := s.q.GetCampaign(ctx, dbgen.GetCampaignParams{ID: *req.CampaignID, WorkspaceID: ws.ID}); err != nil {
			fail(w, unprocessable("invalid_campaign", "campaign not found in this workspace"))
			return
		}
	}
	if req.FolderID != nil {
		if _, err := s.q.GetFolder(ctx, dbgen.GetFolderParams{ID: *req.FolderID, WorkspaceID: ws.ID}); err != nil {
			fail(w, unprocessable("invalid_folder", "folder not found in this workspace"))
			return
		}
	}
	if req.TemplateID != nil {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureTemplates); err != nil {
			fail(w, featureErr(err))
			return
		}
	} else if wp, err := s.wsPolicy(ctx, ws.ID); err == nil && wp.RequireTemplate && !s.can(r, authz.QRDesignBypassLock, req.FolderID) {
		fail(w, unprocessable("template_required", "this workspace requires every new code to use a brand template"))
		return
	}
	design, designHash, tplID, err := s.resolveDesign(ctx, ws, req.TemplateID, req.Design, s.can(r, authz.QRDesignBypassLock, req.FolderID))
	if err != nil {
		fail(w, err)
		return
	}
	var fallback *string
	if req.FallbackURL != nil && strings.TrimSpace(*req.FallbackURL) != "" {
		norm, _, err := s.checkDestination(ctx, ws, "fallback_url", *req.FallbackURL)
		if err != nil {
			fail(w, err)
			return
		}
		fallback = &norm
	}
	var pwHash *string
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 4 || len(*req.Password) > 128 {
			fail(w, unprocessable("invalid_password", "password must be 4–128 characters"))
			return
		}
		h, err := crypto.HashPassword(*req.Password, crypto.QRPasswordConfig)
		if err != nil {
			fail(w, apierr.Internal("failed to hash password"))
			return
		}
		pwHash = &h
	}

	params := dbgen.CreateQRCodeFullParams{
		ID: idgen.New(), WorkspaceID: ws.ID, Mode: req.Mode, ContentType: req.ContentType, Name: req.Name,
		Design: design, DesignHash: designHash, TemplateID: pgUUIDPtr(tplID), FolderID: pgUUIDPtr(req.FolderID),
		CampaignID: pgUUIDPtr(req.CampaignID), Status: "active", StartsAt: pgTS(req.StartsAt), ExpiresAt: pgTS(req.ExpiresAt),
		ScanLimit: req.ScanLimit, PasswordHash: pwHash, FallbackUrl: fallback, SafetyStatus: "safe",
	}
	userID, keyID := actorIDs(r)
	params.CreatedBy = pgUUIDPtr(userID)

	var draft VersionDraft
	var domainID uuid.UUID
	if req.Mode == "static" {
		payload, content, safety, err := s.encodeStatic(ctx, ws, req.ContentType, req.StaticContent, req.StaticPayload)
		if err != nil {
			fail(w, err)
			return
		}
		params.StaticPayload, params.StaticContent, params.SafetyStatus = &payload, content, safety
	} else {
		domainID = s.platformDomainID
		if req.DomainID != nil && *req.DomainID != s.platformDomainID {
			d, err := s.q.GetDomainByID(ctx, *req.DomainID)
			if err != nil || !d.WorkspaceID.Valid || uuid.UUID(d.WorkspaceID.Bytes) != ws.ID || d.Status != "active" {
				fail(w, unprocessable("invalid_domain", "domain must be an active custom domain of this workspace"))
				return
			}
			domainID = d.ID
		}
		if req.ContentType == "gs1" {
			if err := ent.CheckFeature(ctx, ws, entitlements.FeatureGS1); err != nil {
				fail(w, featureErr(err))
				return
			}
			gtin, err := qr.ValidateAndPadGTIN14(req.GTIN)
			if err != nil {
				fail(w, unprocessable("invalid_gtin", "gtin must be a valid GTIN-8/12/13/14 with a correct check digit"))
				return
			}
			params.Gs1Gtin = &gtin
		}
		in := versionInput{ContentType: req.ContentType, DestinationURL: req.DestinationURL, HostedPage: req.HostedPage,
			Rules: req.Rules, UTM: req.UTM, ChangeNote: req.ChangeNote}
		// Contact-style dynamic codes may be given as structured content and become a URL destination.
		if in.DestinationURL == "" && len(req.StaticContent) > 0 {
			u, err := dynamicContactURL(req.ContentType, req.StaticContent)
			if err != nil {
				fail(w, err)
				return
			}
			in.DestinationURL = u
		}
		draft, err = s.buildDraft(ctx, ws, in)
		if err != nil {
			fail(w, err)
			return
		}
		params.SafetyStatus = draft.SafetyStatus
		params.DomainID = pgUUID(domainID)
	}

	var created dbgen.QrCode
	var pending any
	err = s.inTx(ctx, func(q *dbgen.Queries, tx pgx.Tx) error {
		if req.Mode == "dynamic" {
			// Serialise quota checks per workspace so concurrent creates cannot overshoot.
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('qr-quota:' || $1::text, 0))`, ws.ID); err != nil {
				return err
			}
			n, err := q.CountActiveDynamicQRCodes(ctx, ws.ID)
			if err != nil {
				return err
			}
			if lim := ent.Limits(ctx, ws).DynamicCodes; int(n) >= lim {
				p := entitlements.NormalisePlan(ws.PlanID)
				pd := paymentRequired("limit_reached", "this workspace has reached its dynamic QR code limit")
				pd.RequiredPlan = string(entitlements.NextPlan(p))
				return pd
			}
			if err := s.insertWithShortCode(ctx, q, tx, &params, domainID, &created); err != nil {
				return err
			}
		} else {
			var err error
			created, err = q.CreateQRCodeFull(ctx, params)
			if err != nil {
				return err
			}
		}
		if len(req.TagIDs) > 0 {
			if err := q.AddQRTags(ctx, dbgen.AddQRTagsParams{QrCodeID: created.ID, WorkspaceID: ws.ID, TagIds: req.TagIDs}); err != nil {
				return err
			}
		}
		if req.Mode == "dynamic" {
			if s.versionGate != nil {
				handled, body, err := s.versionGate.Intercept(r, q, tx, ws, created, draft)
				if err != nil {
					return err
				}
				if handled {
					pending = body
				}
			}
			if pending == nil {
				v, err := s.ApplyVersion(ctx, q, created, draft, userID, keyID)
				if err != nil {
					return err
				}
				created.CurrentVersionID = pgUUID(v.ID)
			}
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "qr.created", "qr_code", &created.ID, map[string]any{
			"name": created.Name, "mode": created.Mode, "content_type": created.ContentType,
			"short_code": created.ShortCode, "destination": draft.DestinationURL,
		})
	})
	if err != nil {
		if pgCode(err) == sqlUniqueViolation && strings.Contains(pgConstraint(err), "gtin") {
			fail(w, apierr.Conflict("gtin_exists", "a GS1 code for this GTIN already exists on this domain"))
			return
		}
		slogIf(err, "create qr code")
		fail(w, problemOr500(err, "failed to create QR code"))
		return
	}
	if created.Mode == "dynamic" && created.ShortCode != nil {
		s.InvalidateLink(ctx, domainID, *created.ShortCode) // clears any negative-cache entry
	}
	d, err := s.qrDetail(ctx, created)
	if err != nil {
		fail(w, apierr.Internal("created, but failed to load QR code"))
		return
	}
	w.Header().Set("Location", r.URL.Path+"/"+created.ID.String())
	if pending != nil {
		writeJSON(w, http.StatusCreated, map[string]any{"qr_code": d, "pending_approval": pending})
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// insertWithShortCode allocates an unused short code and inserts the row, retrying on the
// rare race where two requests pick the same code (savepoint keeps the transaction usable).
func (s *Server) insertWithShortCode(ctx context.Context, q *dbgen.Queries, tx pgx.Tx, params *dbgen.CreateQRCodeFullParams, domainID uuid.UUID, out *dbgen.QrCode) error {
	for attempt := 0; attempt < 8; attempt++ {
		code, err := shortcode.Generate()
		if err != nil {
			return err
		}
		taken, err := q.ShortCodeTaken(ctx, dbgen.ShortCodeTakenParams{DomainID: pgUUID(domainID), ShortCode: &code})
		if err != nil {
			return err
		}
		if taken {
			continue
		}
		params.ShortCode = &code
		sp, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		row, err := q.WithTx(sp).CreateQRCodeFull(ctx, *params)
		if err != nil {
			_ = sp.Rollback(ctx)
			if pgCode(err) == sqlUniqueViolation && pgConstraint(err) == "qr_codes_domain_code_uniq" {
				continue
			}
			return err
		}
		if err := sp.Commit(ctx); err != nil {
			return err
		}
		*out = row
		return nil
	}
	return errors.New("could not allocate a unique short code")
}

func slogIf(err error, msg string) {
	var pd *apierr.ProblemDetails
	if !errors.As(err, &pd) {
		slog.Error(msg, "error", err)
	}
}

// ---- update -----------------------------------------------------------------

type updateQRReq struct {
	Name        opt[string]      `json:"name"`
	Design      opt[qr.DesignV1] `json:"design"`
	FolderID    opt[uuid.UUID]   `json:"folder_id"`
	CampaignID  opt[uuid.UUID]   `json:"campaign_id"`
	TagIDs      opt[[]uuid.UUID] `json:"tag_ids"`
	StartsAt    opt[time.Time]   `json:"starts_at"`
	ExpiresAt   opt[time.Time]   `json:"expires_at"`
	ScanLimit   opt[int64]       `json:"scan_limit"`
	Password    opt[string]      `json:"password"`
	FallbackURL opt[string]      `json:"fallback_url"`
}

func (s *Server) handleUpdateQRCode(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRUpdate)
	if !ok {
		return
	}
	if err := s.checkWritable(code); err != nil {
		fail(w, err)
		return
	}
	var req updateQRReq
	if !decode(w, r, &req) {
		return
	}
	ws := workspaceRow(r)
	ctx := r.Context()
	ent := s.ent()
	p := dbgen.UpdateQRCodeSettingsParams{ID: code.ID, WorkspaceID: ws.ID}
	changes := map[string]any{}

	if req.Name.Set {
		n := strings.TrimSpace(req.Name.Value)
		if req.Name.Null || n == "" || len(n) > 120 {
			fail(w, unprocessable("invalid_name", "name must be 1–120 characters"))
			return
		}
		p.Name = &n
		changes["name"] = map[string]any{"before": code.Name, "after": n}
	}
	if req.Design.Set && !req.Design.Null {
		d := req.Design.Value
		design, hash, _, err := s.resolveDesign(ctx, ws, uuidPtr(code.TemplateID), &d, s.can(r, authz.QRDesignBypassLock, uuidPtr(code.FolderID)))
		if err != nil {
			fail(w, err)
			return
		}
		p.Design, p.DesignHash = design, hash
		changes["design"] = "updated"
	}
	if req.FolderID.Set {
		target := req.FolderID.ptr()
		if target != nil {
			if _, err := s.q.GetFolder(ctx, dbgen.GetFolderParams{ID: *target, WorkspaceID: ws.ID}); err != nil {
				fail(w, unprocessable("invalid_folder", "folder not found in this workspace"))
				return
			}
		}
		// Moving needs create rights in the destination as well as update rights here.
		if !s.can(r, authz.QRCreate, target) {
			fail(w, forbidden("forbidden", "you cannot move codes into that folder"))
			return
		}
		p.SetFolder, p.FolderID = true, pgUUIDPtr(target)
		changes["folder_id"] = map[string]any{"before": uuidPtr(code.FolderID), "after": target}
	}
	dynamicOnly := req.CampaignID.Set || req.StartsAt.Set || req.ExpiresAt.Set || req.ScanLimit.Set || req.Password.Set || req.FallbackURL.Set
	if dynamicOnly && code.Mode != "dynamic" {
		fail(w, unprocessable("static_no_lifecycle", "campaigns, schedules, expiry, scan limits, passwords and fallbacks need a dynamic code"))
		return
	}
	if req.CampaignID.Set {
		if c := req.CampaignID.ptr(); c != nil {
			if err := ent.CheckFeature(ctx, ws, entitlements.FeatureCampaigns); err != nil {
				fail(w, featureErr(err))
				return
			}
			if _, err := s.q.GetCampaign(ctx, dbgen.GetCampaignParams{ID: *c, WorkspaceID: ws.ID}); err != nil {
				fail(w, unprocessable("invalid_campaign", "campaign not found in this workspace"))
				return
			}
		}
		p.SetCampaign, p.CampaignID = true, pgUUIDPtr(req.CampaignID.ptr())
		changes["campaign_id"] = req.CampaignID.ptr()
	}
	if (req.StartsAt.Set && !req.StartsAt.Null) || (req.ExpiresAt.Set && !req.ExpiresAt.Null) {
		if err := ent.CheckFeature(ctx, ws, entitlements.FeatureExpiry); err != nil {
			fail(w, featureErr(err))
			return
		}
	}
	starts, expires := tsPtr(code.StartsAt), tsPtr(code.ExpiresAt)
	if req.StartsAt.Set {
		starts = req.StartsAt.ptr()
		p.SetStartsAt, p.StartsAt = true, pgTS(starts)
		changes["starts_at"] = starts
	}
	if req.ExpiresAt.Set {
		expires = req.ExpiresAt.ptr()
		p.SetExpiresAt, p.ExpiresAt = true, pgTS(expires)
		changes["expires_at"] = expires
	}
	if starts != nil && expires != nil && !expires.After(*starts) {
		fail(w, unprocessable("invalid_window", "expires_at must be after starts_at"))
		return
	}
	if req.ScanLimit.Set {
		if v := req.ScanLimit.ptr(); v != nil {
			if err := ent.CheckFeature(ctx, ws, entitlements.FeatureScanLimit); err != nil {
				fail(w, featureErr(err))
				return
			}
			if *v <= 0 {
				fail(w, unprocessable("invalid_scan_limit", "scan_limit must be positive"))
				return
			}
		}
		p.SetScanLimit, p.ScanLimit = true, req.ScanLimit.ptr()
		changes["scan_limit"] = req.ScanLimit.ptr()
	}
	if req.FallbackURL.Set {
		var fb *string
		if v := req.FallbackURL.ptr(); v != nil && strings.TrimSpace(*v) != "" {
			norm, _, err := s.checkDestination(ctx, ws, "fallback_url", *v)
			if err != nil {
				fail(w, err)
				return
			}
			fb = &norm
		}
		p.SetFallback, p.FallbackUrl = true, fb
		changes["fallback_url"] = fb
	}
	if req.Password.Set {
		var h *string
		if v := req.Password.ptr(); v != nil && *v != "" {
			if len(*v) < 4 || len(*v) > 128 {
				fail(w, unprocessable("invalid_password", "password must be 4–128 characters"))
				return
			}
			hv, err := crypto.HashPassword(*v, crypto.QRPasswordConfig)
			if err != nil {
				fail(w, apierr.Internal("failed to hash password"))
				return
			}
			h = &hv
		}
		p.SetPassword, p.PasswordHash = true, h
		changes["password"] = map[bool]string{true: "set", false: "cleared"}[h != nil]
	}
	var updated dbgen.QrCode
	err := s.inTx(ctx, func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		updated, err = q.UpdateQRCodeSettings(ctx, p)
		if err != nil {
			return err
		}
		if req.TagIDs.Set {
			if err := q.ClearQRTags(ctx, code.ID); err != nil {
				return err
			}
			if ids := req.TagIDs.Value; !req.TagIDs.Null && len(ids) > 0 {
				if err := q.AddQRTags(ctx, dbgen.AddQRTagsParams{QrCodeID: code.ID, WorkspaceID: ws.ID, TagIds: ids}); err != nil {
					return err
				}
			}
			changes["tag_ids"] = req.TagIDs.Value
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "qr.updated", "qr_code", &code.ID, changes)
	})
	if err != nil {
		slogIf(err, "update qr code")
		fail(w, problemOr500(err, "failed to update QR code"))
		return
	}
	s.invalidateCode(ctx, updated)
	d, err := s.qrDetail(ctx, updated)
	if err != nil {
		fail(w, apierr.Internal("failed to load QR code"))
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) invalidateCode(ctx context.Context, c dbgen.QrCode) {
	if c.Mode == "dynamic" && c.ShortCode != nil && c.DomainID.Valid {
		s.InvalidateLink(ctx, uuid.UUID(c.DomainID.Bytes), *c.ShortCode)
	}
}

// ---- lifecycle --------------------------------------------------------------

func (s *Server) handleSetStatus(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code, ok := s.loadQR(w, r, authz.QRUpdate)
		if !ok {
			return
		}
		if code.Status == "blocked" {
			fail(w, forbidden("code_blocked", "this code was blocked by trust & safety; contact support"))
			return
		}
		if code.IsReadOnly && status == "active" {
			fail(w, paymentRequired("read_only_over_limit", "this code is read-only because the workspace is over its plan limit"))
			return
		}
		if code.Status == status {
			d, _ := s.qrDetail(r.Context(), code)
			writeJSON(w, http.StatusOK, d)
			return
		}
		var updated dbgen.QrCode
		err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
			var err error
			updated, err = q.SetQRCodeStatus(r.Context(), dbgen.SetQRCodeStatusParams{Status: status, ID: code.ID, WorkspaceID: code.WorkspaceID})
			if err != nil {
				return err
			}
			wsID := code.WorkspaceID
			return s.audit(r, q, &wsID, "qr.status.changed", "qr_code", &code.ID, map[string]any{"before": code.Status, "after": status})
		})
		if err != nil {
			fail(w, apierr.Internal("failed to change status"))
			return
		}
		s.invalidateCode(r.Context(), updated)
		d, _ := s.qrDetail(r.Context(), updated)
		writeJSON(w, http.StatusOK, d)
	}
}

func (s *Server) handleDeleteQRCode(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRDelete)
	if !ok {
		return
	}
	var deleted dbgen.QrCode
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		var err error
		deleted, err = q.SoftDeleteQRCodeScoped(r.Context(), dbgen.SoftDeleteQRCodeScopedParams{ID: code.ID, WorkspaceID: code.WorkspaceID})
		if err != nil {
			return err
		}
		wsID := code.WorkspaceID
		return s.audit(r, q, &wsID, "qr.deleted", "qr_code", &code.ID, map[string]any{"name": code.Name, "short_code": code.ShortCode})
	})
	if err != nil {
		fail(w, apierr.Internal("failed to delete QR code"))
		return
	}
	s.invalidateCode(r.Context(), deleted)
	w.WriteHeader(http.StatusNoContent)
}

// POST /qr-codes/{id}/restore: undo a soft delete within 30 days (re-checks the dynamic quota).
func (s *Server) handleRestoreQRCode(w http.ResponseWriter, r *http.Request) {
	ws := workspaceRow(r)
	id, ok := uuidParam(r, "id")
	if !ok {
		fail(w, apierr.NotFound("QR code not found"))
		return
	}
	var restored dbgen.QrCode
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		var folder *uuid.UUID
		var mode string
		err := tx.QueryRow(r.Context(), `SELECT folder_id, mode FROM qr_codes
			WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NOT NULL AND deleted_at > now() - interval '30 days'`,
			id, ws.ID).Scan(&folder, &mode)
		if err != nil {
			if isNotFound(err) {
				return apierr.NotFound("no deleted QR code with this id can be restored")
			}
			return err
		}
		if !s.can(r, authz.QRDelete, folder) {
			return apierr.NotFound("no deleted QR code with this id can be restored")
		}
		if mode == "dynamic" {
			if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended('qr-quota:' || $1::text, 0))`, ws.ID); err != nil {
				return err
			}
			n, err := q.CountActiveDynamicQRCodes(r.Context(), ws.ID)
			if err != nil {
				return err
			}
			if int(n) >= s.ent().Limits(r.Context(), ws).DynamicCodes {
				return paymentRequired("limit_reached", "restoring this code would exceed the dynamic QR code limit")
			}
		}
		restored, err = q.RestoreQRCode(r.Context(), dbgen.RestoreQRCodeParams{ID: id, WorkspaceID: ws.ID})
		if err != nil {
			return err
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, "qr.restored", "qr_code", &id, nil)
	})
	if err != nil {
		slogIf(err, "restore qr code")
		fail(w, problemOr500(err, "failed to restore QR code"))
		return
	}
	s.invalidateCode(r.Context(), restored)
	d, _ := s.qrDetail(r.Context(), restored)
	writeJSON(w, http.StatusOK, d)
}

// ---- versions ---------------------------------------------------------------

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRRead)
	if !ok {
		return
	}
	rows, err := s.q.ListQRVersionsScoped(r.Context(), dbgen.ListQRVersionsScopedParams{QrCodeID: code.ID, WorkspaceID: code.WorkspaceID})
	if err != nil {
		fail(w, apierr.Internal("failed to list versions"))
		return
	}
	var currentID *uuid.UUID
	if cur, err := s.q.CurrentEffectiveVersion(r.Context(), code.ID); err == nil {
		currentID = &cur.ID
	}
	now := time.Now()
	out := make([]versionDTO, 0, len(rows))
	for _, v := range rows {
		out = append(out, toVersionDTO(v, currentID, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type createVersionReq struct {
	DestinationURL string          `json:"destination_url"`
	HostedPage     json.RawMessage `json:"hosted_page"`
	Rules          json.RawMessage `json:"rules"`
	UTM            json.RawMessage `json:"utm"`
	EffectiveAt    *time.Time      `json:"effective_at"`
	ChangeNote     *string         `json:"change_note"`
}

func (s *Server) handleCreateVersion(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRDestinationUpdate)
	if !ok {
		return
	}
	if code.Mode != "dynamic" {
		fail(w, unprocessable("static_immutable", "static codes encode their content directly and cannot be redirected"))
		return
	}
	if err := s.checkWritable(code); err != nil {
		fail(w, err)
		return
	}
	var req createVersionReq
	if !decode(w, r, &req) {
		return
	}
	ws := workspaceRow(r)
	draft, err := s.buildDraft(r.Context(), ws, versionInput{ContentType: code.ContentType, DestinationURL: req.DestinationURL,
		HostedPage: req.HostedPage, Rules: req.Rules, UTM: req.UTM, EffectiveAt: req.EffectiveAt, ChangeNote: req.ChangeNote})
	if err != nil {
		fail(w, err)
		return
	}
	s.commitVersion(w, r, code, draft, "qr.version.created")
}

// commitVersion runs the approvals gate, applies the draft and invalidates caches.
func (s *Server) commitVersion(w http.ResponseWriter, r *http.Request, code dbgen.QrCode, draft VersionDraft, action string) {
	ws := workspaceRow(r)
	userID, keyID := actorIDs(r)
	var v dbgen.QrVersion
	var pending any
	err := s.inTx(r.Context(), func(q *dbgen.Queries, tx pgx.Tx) error {
		// Lock the code so concurrent edits get sequential version numbers.
		locked, err := q.GetQRCodeForUpdate(r.Context(), dbgen.GetQRCodeForUpdateParams{ID: code.ID, WorkspaceID: ws.ID})
		if err != nil {
			return err
		}
		if s.versionGate != nil {
			handled, body, err := s.versionGate.Intercept(r, q, tx, ws, locked, draft)
			if err != nil {
				return err
			}
			if handled {
				pending = body
				return nil
			}
		}
		v, err = s.ApplyVersion(r.Context(), q, locked, draft, userID, keyID)
		if err != nil {
			return err
		}
		if draft.SafetyStatus == "pending" && locked.SafetyStatus == "safe" {
			if err := q.SetQRCodeSafety(r.Context(), dbgen.SetQRCodeSafetyParams{SafetyStatus: "pending", ID: locked.ID}); err != nil {
				return err
			}
		}
		wsID := ws.ID
		return s.audit(r, q, &wsID, action, "qr_code", &code.ID, map[string]any{
			"version_no": v.VersionNo, "destination": draft.DestinationURL, "effective_at": draft.EffectiveAt,
			"restored_from": draft.RestoredFrom,
		})
	})
	if err != nil {
		slogIf(err, "create version")
		fail(w, problemOr500(err, "failed to create version"))
		return
	}
	if pending != nil {
		writeJSON(w, http.StatusAccepted, map[string]any{"pending_approval": pending})
		return
	}
	s.invalidateCode(r.Context(), code)
	var currentID *uuid.UUID
	if !v.EffectiveAt.After(time.Now()) {
		currentID = &v.ID
	}
	writeJSON(w, http.StatusCreated, toVersionDTO(v, currentID, time.Now()))
}

// POST /versions/{versionId}/restore: re-publish an old destination as a new version.
func (s *Server) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRDestinationUpdate)
	if !ok {
		return
	}
	if err := s.checkWritable(code); err != nil {
		fail(w, err)
		return
	}
	vid, ok := uuidParam(r, "versionId")
	if !ok {
		fail(w, apierr.NotFound("version not found"))
		return
	}
	old, err := s.q.GetQRVersionScoped(r.Context(), dbgen.GetQRVersionScopedParams{VersionID: vid, QrCodeID: code.ID, WorkspaceID: code.WorkspaceID})
	if err != nil {
		fail(w, apierr.NotFound("version not found"))
		return
	}
	ws := workspaceRow(r)
	// Re-validate: policy or reputation may have changed since the old version was written.
	var destURL string
	if old.DestinationUrl != nil {
		destURL = *old.DestinationUrl
	}
	note := "Restored version " + itoa(int(old.VersionNo))
	draft, err := s.buildDraft(r.Context(), ws, versionInput{ContentType: code.ContentType, DestinationURL: destURL,
		HostedPage: old.HostedPage, Rules: old.Rules, UTM: old.Utm, ChangeNote: &note})
	if err != nil {
		fail(w, err)
		return
	}
	draft.RestoredFrom = &old.ID
	s.commitVersion(w, r, code, draft, "qr.version.restored")
}

// DELETE /versions/{versionId}: cancel a scheduled (future) change.
func (s *Server) handleCancelScheduledVersion(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRDestinationUpdate)
	if !ok {
		return
	}
	vid, ok := uuidParam(r, "versionId")
	if !ok {
		fail(w, apierr.NotFound("version not found"))
		return
	}
	err := s.inTx(r.Context(), func(q *dbgen.Queries, _ pgx.Tx) error {
		n, err := q.DeleteScheduledVersion(r.Context(), dbgen.DeleteScheduledVersionParams{VersionID: vid, QrCodeID: code.ID, WorkspaceID: code.WorkspaceID})
		if err != nil {
			return err
		}
		if n == 0 {
			return apierr.Conflict("not_scheduled", "only versions scheduled in the future can be cancelled")
		}
		wsID := code.WorkspaceID
		return s.audit(r, q, &wsID, "qr.version.cancelled", "qr_code", &code.ID, map[string]any{"version_id": vid})
	})
	if err != nil {
		fail(w, problemOr500(err, "failed to cancel version"))
		return
	}
	s.invalidateCode(r.Context(), code)
	w.WriteHeader(http.StatusNoContent)
}

// ---- resolve preview --------------------------------------------------------

type previewReq struct {
	Country    string     `json:"country"`
	Region     string     `json:"region"`
	DeviceType string     `json:"device_type"`
	OS         string     `json:"os"`
	Language   string     `json:"language"`
	UserAgent  string     `json:"user_agent"`
	At         *time.Time `json:"at"`
}

// POST /qr-codes/{id}/resolve-preview: where would a scan with these facts go, and why.
func (s *Server) handleResolvePreview(w http.ResponseWriter, r *http.Request) {
	code, ok := s.loadQR(w, r, authz.QRRead)
	if !ok {
		return
	}
	var req previewReq
	if !decodeOptional(w, r, &req) {
		return
	}
	if code.Mode != "dynamic" {
		writeJSON(w, http.StatusOK, map[string]any{"outcome": "static", "payload": code.StaticPayload})
		return
	}
	at := time.Now().UTC()
	if req.At != nil {
		at = req.At.UTC()
	}
	ws := workspaceRow(r)
	loc, err := time.LoadLocation(ws.Timezone)
	if err != nil {
		loc = time.UTC
	}
	// Pick the version effective at the requested instant.
	versions, err := s.q.ListQRVersionsScoped(r.Context(), dbgen.ListQRVersionsScopedParams{QrCodeID: code.ID, WorkspaceID: code.WorkspaceID})
	if err != nil {
		fail(w, apierr.Internal("failed to load versions"))
		return
	}
	var effective *dbgen.QrVersion
	for i := range versions {
		v := &versions[i]
		if v.EffectiveAt.After(at) {
			continue
		}
		if effective == nil || v.EffectiveAt.After(effective.EffectiveAt) ||
			(v.EffectiveAt.Equal(effective.EffectiveAt) && v.VersionNo > effective.VersionNo) {
			effective = v
		}
	}
	link := &resolve.ResolvedLink{Status: code.Status, Safety: code.SafetyStatus, StartsAt: tsPtr(code.StartsAt),
		ExpiresAt: tsPtr(code.ExpiresAt), ScanLimit: code.ScanLimit, TotalScans: code.TotalScans,
		HasPassword: code.PasswordHash != nil && *code.PasswordHash != "", FallbackURL: code.FallbackUrl}
	resp := map[string]any{"at": at}
	if effective != nil {
		resp["version_id"] = effective.ID
		resp["version_no"] = effective.VersionNo
		url := ""
		if effective.DestinationUrl != nil {
			url = *effective.DestinationUrl
		}
		link.Version = &resolve.ResolvedVersion{ID: effective.ID, URL: url, Kind: effective.DestinationKind}
	}
	outcome, target := resolve.EvaluateState(link, at)
	resp["outcome"] = outcome
	if outcome == resolve.OutcomeActive && effective != nil {
		var rules []routing.Rule
		_ = json.Unmarshal(effective.Rules, &rules)
		res := routing.Evaluate(routing.Version{DefaultDestination: target, Rules: rules}, routing.RequestFacts{
			QRCodeID: code.ID.String(), UserAgent: req.UserAgent, Country: strings.ToUpper(req.Country), Region: req.Region,
			DeviceType: req.DeviceType, OS: req.OS, Language: req.Language, ScanCount: code.TotalScans,
		}, at, loc)
		dest, ruleID := res.URL, res.RuleID
		if res.Blocked {
			resp["outcome"] = "geo_blocked"
		}
		var utm version.UTMConfig
		_ = json.Unmarshal(effective.Utm, &utm)
		if dest != "" {
			if withUTM, err := version.AppendUTM(dest, utm); err == nil {
				dest = withUTM
			}
		}
		if effective.DestinationKind == "hosted_page" && ruleID == "" {
			resp["hosted_page"] = true
			resp["outcome"] = "hosted_page"
		}
		target = dest
		resp["rule_id"] = ruleID
	}
	resp["destination"] = target
	writeJSON(w, http.StatusOK, resp)
}
