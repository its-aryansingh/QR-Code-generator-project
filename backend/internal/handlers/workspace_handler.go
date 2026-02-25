package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
)

type WorkspaceHandler struct {
	repo     *repository.WorkspaceRepository
	userRepo repository.UserRepository
}

func NewWorkspaceHandler(repo *repository.WorkspaceRepository, userRepo repository.UserRepository) *WorkspaceHandler {
	return &WorkspaceHandler{repo: repo, userRepo: userRepo}
}

// ==========================================
// WORKSPACE CRUD
// ==========================================

func (h *WorkspaceHandler) CreateWorkspace(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}

	// Generate slug from name
	slug := generateSlug(req.Name)
	if h.repo.SlugExists(slug) {
		slug = slug + "-" + uuid.New().String()[:6]
	}

	workspace := &models.Workspace{
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		OwnerID:     userID,
		Plan:        models.PlanFree,
		MaxMembers:  5,
		MaxQRCodes:  100,
		MaxFolders:  10,
	}

	if err := h.repo.Create(workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create workspace"})
		return
	}

	// Add owner as member
	member := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		UserID:      userID,
		Role:        models.RoleOwner,
		InvitedBy:   userID,
	}
	h.repo.AddMember(member)

	// Create audit log
	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: workspace.ID,
		UserID:      userID,
		Action:      models.AuditActionCreate,
		Resource:    models.AuditResourceWorkspace,
		ResourceID:  &workspace.ID,
		Details:     "Created workspace: " + workspace.Name,
		IPAddress:   c.ClientIP(),
		UserAgent:   c.Request.UserAgent(),
	})

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": workspace})
}

func (h *WorkspaceHandler) GetWorkspaces(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	workspaces, err := h.repo.GetUserWorkspaces(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch workspaces"})
		return
	}

	// Enrich with stats
	type WorkspaceWithStats struct {
		models.Workspace
		QRCount   int64 `json:"qr_count"`
		ScanCount int64 `json:"scan_count"`
	}

	var result []WorkspaceWithStats
	for _, ws := range workspaces {
		qrCount, scanCount, _ := h.repo.GetWorkspaceStats(ws.ID)
		result = append(result, WorkspaceWithStats{
			Workspace: ws,
			QRCount:   qrCount,
			ScanCount: scanCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func (h *WorkspaceHandler) GetWorkspace(c *gin.Context) {
	wsID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid workspace ID"})
		return
	}

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	qrCount, scanCount, _ := h.repo.GetWorkspaceStats(wsID)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       workspace,
		"qr_count":   qrCount,
		"scan_count": scanCount,
	})
}

func (h *WorkspaceHandler) UpdateWorkspace(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		LogoURL     *string `json:"logo_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.Name != nil {
		workspace.Name = *req.Name
	}
	if req.Description != nil {
		workspace.Description = *req.Description
	}
	if req.LogoURL != nil {
		workspace.LogoURL = *req.LogoURL
	}

	if err := h.repo.Update(workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update"})
		return
	}

	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionUpdate, Resource: models.AuditResourceWorkspace,
		ResourceID: &wsID, IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "data": workspace})
}

func (h *WorkspaceHandler) DeleteWorkspace(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))

	member, err := h.repo.GetMember(wsID, userID)
	if err != nil || !member.CanDeleteWorkspace() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Only workspace owner can delete"})
		return
	}

	if err := h.repo.Delete(wsID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Workspace deleted"})
}

// ==========================================
// MEMBER MANAGEMENT
// ==========================================

func (h *WorkspaceHandler) InviteMember(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))

	// Check permission
	member, err := h.repo.GetMember(wsID, userID)
	if err != nil || !member.CanManageMembers() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Insufficient permissions"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
		Role  string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	// Validate role
	if req.Role != models.RoleAdmin && req.Role != models.RoleEditor && req.Role != models.RoleViewer {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid role"})
		return
	}

	// Check member limit
	count, _ := h.repo.CountMembers(wsID)
	workspace, _ := h.repo.GetByID(wsID)
	if int(count) >= workspace.MaxMembers {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Member limit reached. Upgrade your plan."})
		return
	}

	invite := &models.WorkspaceInvite{
		WorkspaceID: wsID,
		Email:       req.Email,
		Role:        req.Role,
		InvitedBy:   userID,
		Status:      models.InvitePending,
	}

	if err := h.repo.CreateInvite(invite); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create invite"})
		return
	}

	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionInvite, Resource: models.AuditResourceMember,
		Details: "Invited " + req.Email + " as " + req.Role, IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": invite})
}

func (h *WorkspaceHandler) AcceptInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	token := c.Param("token")

	invite, err := h.repo.GetInviteByToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Invalid or expired invite"})
		return
	}

	if invite.IsExpired() {
		c.JSON(http.StatusGone, gin.H{"success": false, "error": "Invite has expired"})
		return
	}

	// Add as member
	member := &models.WorkspaceMember{
		WorkspaceID: invite.WorkspaceID,
		UserID:      userID,
		Role:        invite.Role,
		InvitedBy:   invite.InvitedBy,
	}

	if err := h.repo.AddMember(member); err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": "Already a member of this workspace"})
		return
	}

	h.repo.AcceptInvite(token)

	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: invite.WorkspaceID, UserID: userID,
		Action: models.AuditActionJoin, Resource: models.AuditResourceMember,
		IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Joined workspace", "workspace_id": invite.WorkspaceID})
}

func (h *WorkspaceHandler) GetMembers(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	members, err := h.repo.GetMembers(wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": members})
}

func (h *WorkspaceHandler) UpdateMemberRole(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))
	memberID, _ := uuid.Parse(c.Param("memberID"))

	// Check permission
	actor, err := h.repo.GetMember(wsID, userID)
	if err != nil || !actor.CanManageMembers() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Insufficient permissions"})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if err := h.repo.UpdateMemberRole(wsID, memberID, req.Role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Role updated"})
}

func (h *WorkspaceHandler) RemoveMember(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))
	memberID, _ := uuid.Parse(c.Param("memberID"))

	actor, err := h.repo.GetMember(wsID, userID)
	if err != nil || !actor.CanManageMembers() {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Insufficient permissions"})
		return
	}

	if err := h.repo.RemoveMember(wsID, memberID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Member removed"})
}

// ==========================================
// FOLDER MANAGEMENT
// ==========================================

func (h *WorkspaceHandler) CreateFolder(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Color       string `json:"color"`
		ParentID    string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Name is required"})
		return
	}

	folder := &models.Folder{
		WorkspaceID: wsID,
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
	}

	if req.ParentID != "" {
		parentID, _ := uuid.Parse(req.ParentID)
		folder.ParentID = &parentID
	}
	if folder.Color == "" {
		folder.Color = "#8B5CF6"
	}

	if err := h.repo.CreateFolder(folder); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create folder"})
		return
	}

	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionCreate, Resource: models.AuditResourceFolder,
		ResourceID: &folder.ID, Details: "Created folder: " + folder.Name, IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": folder})
}

func (h *WorkspaceHandler) GetFolders(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	folders, err := h.repo.GetFolders(wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch folders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": folders})
}

func (h *WorkspaceHandler) UpdateFolder(c *gin.Context) {
	folderID, _ := uuid.Parse(c.Param("folderID"))

	folder, err := h.repo.GetFolderByID(folderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Folder not found"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
	}
	c.ShouldBindJSON(&req)

	if req.Name != nil {
		folder.Name = *req.Name
	}
	if req.Description != nil {
		folder.Description = *req.Description
	}
	if req.Color != nil {
		folder.Color = *req.Color
	}

	h.repo.UpdateFolder(folder)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": folder})
}

func (h *WorkspaceHandler) DeleteFolder(c *gin.Context) {
	folderID, _ := uuid.Parse(c.Param("folderID"))

	if err := h.repo.DeleteFolder(folderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete folder"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Folder deleted"})
}

// ==========================================
// WORKSPACE QR CODES
// ==========================================

func (h *WorkspaceHandler) GetWorkspaceQRCodes(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	search := c.Query("search")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var folderID *uuid.UUID
	if fid := c.Query("folder_id"); fid != "" {
		id, _ := uuid.Parse(fid)
		folderID = &id
	}

	records, total, err := h.repo.GetWorkspaceQRCodes(wsID, folderID, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch QR codes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *WorkspaceHandler) BulkMoveToFolder(c *gin.Context) {
	var req struct {
		QRIDs    []string `json:"qr_ids" binding:"required"`
		FolderID *string  `json:"folder_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var ids []uuid.UUID
	for _, id := range req.QRIDs {
		uid, _ := uuid.Parse(id)
		ids = append(ids, uid)
	}

	var folderID *uuid.UUID
	if req.FolderID != nil {
		id, _ := uuid.Parse(*req.FolderID)
		folderID = &id
	}

	if err := h.repo.BulkMoveToFolder(ids, folderID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to move"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "QR codes moved"})
}

func (h *WorkspaceHandler) BulkDeleteQR(c *gin.Context) {
	var req struct {
		QRIDs []string `json:"qr_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	var ids []uuid.UUID
	for _, id := range req.QRIDs {
		uid, _ := uuid.Parse(id)
		ids = append(ids, uid)
	}

	if err := h.repo.BulkDelete(ids); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "QR codes deleted"})
}

// ==========================================
// AUDIT LOGS
// ==========================================

func (h *WorkspaceHandler) GetAuditLogs(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := h.repo.GetAuditLogs(wsID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs, "total": total})
}

// ==========================================
// WEBHOOKS
// ==========================================

func (h *WorkspaceHandler) CreateWebhook(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	wsID, _ := uuid.Parse(c.Param("id"))

	var req struct {
		URL         string   `json:"url" binding:"required,url"`
		Events      []string `json:"events" binding:"required"`
		Description string   `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	webhook := &models.Webhook{
		WorkspaceID: wsID,
		URL:         req.URL,
		Events:      strings.Join(req.Events, ","),
		Description: req.Description,
		IsActive:    true,
	}

	if err := h.repo.CreateWebhook(webhook); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create webhook"})
		return
	}

	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionCreate, Resource: models.AuditResourceWebhook,
		ResourceID: &webhook.ID, IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": webhook})
}

func (h *WorkspaceHandler) GetWebhooks(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	webhooks, err := h.repo.GetWebhooks(wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch webhooks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": webhooks})
}

func (h *WorkspaceHandler) DeleteWebhook(c *gin.Context) {
	webhookID, _ := uuid.Parse(c.Param("webhookID"))

	if err := h.repo.DeleteWebhook(webhookID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete webhook"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Webhook deleted"})
}

func (h *WorkspaceHandler) GetWebhookLogs(c *gin.Context) {
	webhookID, _ := uuid.Parse(c.Param("webhookID"))

	logs, err := h.repo.GetWebhookLogs(webhookID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}

// ==========================================
// WORKSPACE ANALYTICS
// ==========================================

// GetWorkspaceAnalytics returns comprehensive analytics for a workspace
func (h *WorkspaceHandler) GetWorkspaceAnalytics(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	qrCount, scanCount, _ := h.repo.GetWorkspaceStats(wsID)
	scansByDate, _ := h.repo.GetWorkspaceScansByDate(wsID, days)
	devices, _ := h.repo.GetWorkspaceDeviceBreakdown(wsID)
	countries, _ := h.repo.GetWorkspaceCountryBreakdown(wsID)
	browsers, _ := h.repo.GetWorkspaceBrowserBreakdown(wsID)
	referrers, _ := h.repo.GetWorkspaceReferrerBreakdown(wsID)
	topQR, _ := h.repo.GetTopPerformingQR(wsID, 10)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"summary": gin.H{
				"total_qr_codes": qrCount,
				"total_scans":    scanCount,
			},
			"scans_by_date": scansByDate,
			"devices":       devices,
			"countries":     countries,
			"browsers":      browsers,
			"referrers":     referrers,
			"top_qr_codes":  topQR,
		},
	})
}

// GetWorkspaceHeatmap returns scan activity heatmap (day x hour)
func (h *WorkspaceHandler) GetWorkspaceHeatmap(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	heatmap, err := h.repo.GetWorkspaceHourlyHeatmap(wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch heatmap"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": heatmap})
}

// GetTopQRCodes returns top performing QR codes in the workspace
func (h *WorkspaceHandler) GetTopQRCodes(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	records, err := h.repo.GetTopPerformingQR(wsID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch top QR codes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": records})
}

// ==========================================
// HELPERS
// ==========================================

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.TrimSpace(slug)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug
}
