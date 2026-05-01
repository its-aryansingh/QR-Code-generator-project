package repository

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"gorm.io/gorm"
)

type WorkspaceRepository struct {
	db *gorm.DB
}

func NewWorkspaceRepository(db *gorm.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

// ==========================================
// WORKSPACE CRUD
// ==========================================

func (r *WorkspaceRepository) Create(workspace *models.Workspace) error {
	return r.db.Create(workspace).Error
}

func (r *WorkspaceRepository) GetByID(id uuid.UUID) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.Preload("Members").Preload("Members.User").First(&workspace, "id = ?", id).Error
	return &workspace, err
}

func (r *WorkspaceRepository) GetBySlug(slug string) (*models.Workspace, error) {
	var workspace models.Workspace
	err := r.db.First(&workspace, "slug = ?", slug).Error
	return &workspace, err
}

func (r *WorkspaceRepository) Update(workspace *models.Workspace) error {
	return r.db.Save(workspace).Error
}

func (r *WorkspaceRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&models.Workspace{}, "id = ?", id).Error
}

// GetUserWorkspaces returns all workspaces a user is a member of
func (r *WorkspaceRepository) GetUserWorkspaces(userID uuid.UUID) ([]models.Workspace, error) {
	var workspaces []models.Workspace
	err := r.db.
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ?", userID).
		Preload("Members").
		Preload("Members.User").
		Find(&workspaces).Error
	return workspaces, err
}

// GetWorkspaceStats returns QR count and total scans for a workspace
func (r *WorkspaceRepository) GetWorkspaceStats(workspaceID uuid.UUID) (qrCount int64, scanCount int64, err error) {
	r.db.Model(&models.QRRecord{}).Where("workspace_id = ?", workspaceID).Count(&qrCount)
	r.db.Model(&models.QRScan{}).
		Joins("JOIN qr_records ON qr_records.id = qr_scans.qr_id").
		Where("qr_records.workspace_id = ?", workspaceID).
		Count(&scanCount)
	return
}

// ==========================================
// MEMBER MANAGEMENT
// ==========================================

func (r *WorkspaceRepository) AddMember(member *models.WorkspaceMember) error {
	return r.db.Create(member).Error
}

func (r *WorkspaceRepository) GetMember(workspaceID, userID uuid.UUID) (*models.WorkspaceMember, error) {
	var member models.WorkspaceMember
	err := r.db.First(&member, "workspace_id = ? AND user_id = ?", workspaceID, userID).Error
	return &member, err
}

func (r *WorkspaceRepository) UpdateMemberRole(workspaceID, userID uuid.UUID, role string) error {
	return r.db.Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", role).Error
}

func (r *WorkspaceRepository) RemoveMember(workspaceID, userID uuid.UUID) error {
	return r.db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Delete(&models.WorkspaceMember{}).Error
}

func (r *WorkspaceRepository) GetMembers(workspaceID uuid.UUID) ([]models.WorkspaceMember, error) {
	var members []models.WorkspaceMember
	err := r.db.Preload("User").
		Where("workspace_id = ?", workspaceID).
		Find(&members).Error
	return members, err
}

func (r *WorkspaceRepository) CountMembers(workspaceID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.WorkspaceMember{}).Where("workspace_id = ?", workspaceID).Count(&count).Error
	return count, err
}

// ==========================================
// INVITE MANAGEMENT
// ==========================================

func (r *WorkspaceRepository) CreateInvite(invite *models.WorkspaceInvite) error {
	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	invite.Token = hex.EncodeToString(bytes)
	invite.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // 7 days
	return r.db.Create(invite).Error
}

func (r *WorkspaceRepository) GetInviteByToken(token string) (*models.WorkspaceInvite, error) {
	var invite models.WorkspaceInvite
	err := r.db.Preload("Workspace").First(&invite, "token = ? AND status = ?", token, models.InvitePending).Error
	return &invite, err
}

func (r *WorkspaceRepository) GetPendingInvites(workspaceID uuid.UUID) ([]models.WorkspaceInvite, error) {
	var invites []models.WorkspaceInvite
	err := r.db.Where("workspace_id = ? AND status = ?", workspaceID, models.InvitePending).Find(&invites).Error
	return invites, err
}

func (r *WorkspaceRepository) AcceptInvite(token string) error {
	return r.db.Model(&models.WorkspaceInvite{}).
		Where("token = ?", token).
		Update("status", models.InviteAccepted).Error
}

func (r *WorkspaceRepository) DeleteInvite(id uuid.UUID) error {
	return r.db.Delete(&models.WorkspaceInvite{}, "id = ?", id).Error
}

// ==========================================
// FOLDER MANAGEMENT
// ==========================================

func (r *WorkspaceRepository) CreateFolder(folder *models.Folder) error {
	return r.db.Create(folder).Error
}

func (r *WorkspaceRepository) GetFolders(workspaceID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	err := r.db.Where("workspace_id = ?", workspaceID).
		Order("sort_order ASC, name ASC").
		Find(&folders).Error
	return folders, err
}

func (r *WorkspaceRepository) GetFolderByID(id uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	err := r.db.First(&folder, "id = ?", id).Error
	return &folder, err
}

func (r *WorkspaceRepository) UpdateFolder(folder *models.Folder) error {
	return r.db.Save(folder).Error
}

func (r *WorkspaceRepository) DeleteFolder(id uuid.UUID) error {
	// Set folder_id to NULL on QR records in this folder
	r.db.Model(&models.QRRecord{}).Where("folder_id = ?", id).Update("folder_id", nil)
	return r.db.Delete(&models.Folder{}, "id = ?", id).Error
}

// ==========================================
// AUDIT LOG
// ==========================================

func (r *WorkspaceRepository) CreateAuditLog(log *models.AuditLog) error {
	return r.db.Create(log).Error
}

type AuditLogFilters struct {
	Action   string
	Resource string
	From     *time.Time
	To       *time.Time
}

func (r *WorkspaceRepository) GetAuditLogs(workspaceID uuid.UUID, limit, offset int, filters AuditLogFilters) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.Where("workspace_id = ?", workspaceID)
	if filters.Action != "" {
		query = query.Where("action = ?", filters.Action)
	}
	if filters.Resource != "" {
		query = query.Where("resource = ?", filters.Resource)
	}
	if filters.From != nil {
		query = query.Where("created_at >= ?", *filters.From)
	}
	if filters.To != nil {
		query = query.Where("created_at <= ?", *filters.To)
	}

	query.Model(&models.AuditLog{}).Count(&total)

	err := query.Preload("User").
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs).Error
	return logs, total, err
}

// ==========================================
// WEBHOOK MANAGEMENT
// ==========================================

func (r *WorkspaceRepository) CreateWebhook(webhook *models.Webhook) error {
	// Generate secret
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	webhook.Secret = hex.EncodeToString(bytes)
	return r.db.Create(webhook).Error
}

func (r *WorkspaceRepository) GetWebhooks(workspaceID uuid.UUID) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.db.Where("workspace_id = ?", workspaceID).Find(&webhooks).Error
	return webhooks, err
}

func (r *WorkspaceRepository) GetWebhookByID(id uuid.UUID) (*models.Webhook, error) {
	var webhook models.Webhook
	err := r.db.First(&webhook, "id = ?", id).Error
	return &webhook, err
}

func (r *WorkspaceRepository) UpdateWebhook(webhook *models.Webhook) error {
	return r.db.Save(webhook).Error
}

func (r *WorkspaceRepository) DeleteWebhook(id uuid.UUID) error {
	return r.db.Delete(&models.Webhook{}, "id = ?", id).Error
}

func (r *WorkspaceRepository) GetWebhooksByEvent(workspaceID uuid.UUID, event string) ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.db.Where("workspace_id = ? AND is_active = true", workspaceID).Find(&webhooks).Error

	// Filter by event (stored as comma-separated)
	var matching []models.Webhook
	for _, wh := range webhooks {
		events := strings.Split(wh.Events, ",")
		for _, e := range events {
			if strings.TrimSpace(e) == event {
				matching = append(matching, wh)
				break
			}
		}
	}
	return matching, err
}

func (r *WorkspaceRepository) CreateWebhookLog(log *models.WebhookLog) error {
	return r.db.Create(log).Error
}

func (r *WorkspaceRepository) GetWebhookLogs(webhookID uuid.UUID, limit int) ([]models.WebhookLog, error) {
	var logs []models.WebhookLog
	err := r.db.Where("webhook_id = ?", webhookID).
		Order("created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// ==========================================
// QR RECORDS (workspace-scoped queries)
// ==========================================

func (r *WorkspaceRepository) GetWorkspaceQRCodes(workspaceID uuid.UUID, folderID *uuid.UUID, search string, limit, offset int) ([]models.QRRecord, int64, error) {
	var records []models.QRRecord
	var total int64

	query := r.db.Where("workspace_id = ?", workspaceID)

	if folderID != nil {
		query = query.Where("folder_id = ?", *folderID)
	}

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("title ILIKE ? OR content ILIKE ? OR tags ILIKE ?", searchTerm, searchTerm, searchTerm)
	}

	query.Model(&models.QRRecord{}).Count(&total)

	err := query.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&records).Error
	return records, total, err
}

func (r *WorkspaceRepository) MoveQRToFolder(qrID uuid.UUID, folderID *uuid.UUID) error {
	return r.db.Model(&models.QRRecord{}).Where("id = ?", qrID).Update("folder_id", folderID).Error
}

func (r *WorkspaceRepository) BulkMoveToFolder(qrIDs []uuid.UUID, folderID *uuid.UUID) error {
	return r.db.Model(&models.QRRecord{}).Where("id IN ?", qrIDs).Update("folder_id", folderID).Error
}

func (r *WorkspaceRepository) BulkDelete(qrIDs []uuid.UUID) error {
	return r.db.Where("id IN ?", qrIDs).Delete(&models.QRRecord{}).Error
}

// SlugExists checks if a workspace slug is already taken
func (r *WorkspaceRepository) SlugExists(slug string) bool {
	var count int64
	r.db.Model(&models.Workspace{}).Where("slug = ?", slug).Count(&count)
	return count > 0
}

// UpdateWebhookHealth updates fail count, active status, and last triggered time
func (r *WorkspaceRepository) UpdateWebhookHealth(wh *models.Webhook) error {
	return r.db.Model(wh).Updates(map[string]interface{}{
		"fail_count":     wh.FailCount,
		"is_active":      wh.IsActive,
		"last_triggered": wh.LastTriggered,
	}).Error
}

// ==========================================
// WORKSPACE ANALYTICS (advanced queries)
// ==========================================

// GetWorkspaceScansByDate returns scan counts per day for a workspace
func (r *WorkspaceRepository) GetWorkspaceScansByDate(workspaceID uuid.UUID, days int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT DATE(qs.scanned_at) as date, COUNT(*) as count
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ? AND qs.scanned_at >= NOW() - INTERVAL '1 day' * ?
		GROUP BY DATE(qs.scanned_at)
		ORDER BY date ASC
	`, workspaceID, days).Scan(&results).Error
	return results, err
}

// GetWorkspaceDeviceBreakdown returns device type distribution for a workspace
func (r *WorkspaceRepository) GetWorkspaceDeviceBreakdown(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT COALESCE(qs.device_type, 'Unknown') as name, COUNT(*) as value
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ?
		GROUP BY qs.device_type
		ORDER BY value DESC
	`, workspaceID).Scan(&results).Error
	return results, err
}

// GetWorkspaceCountryBreakdown returns country distribution for a workspace
func (r *WorkspaceRepository) GetWorkspaceCountryBreakdown(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT COALESCE(qs.country_name, 'Unknown') as name, COUNT(*) as value
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ?
		GROUP BY qs.country_name
		ORDER BY value DESC
		LIMIT 10
	`, workspaceID).Scan(&results).Error
	return results, err
}

// GetWorkspaceHourlyHeatmap returns scan counts by hour of day for a workspace
func (r *WorkspaceRepository) GetWorkspaceHourlyHeatmap(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT EXTRACT(DOW FROM qs.scanned_at)::int as day,
		       EXTRACT(HOUR FROM qs.scanned_at)::int as hour,
		       COUNT(*) as count
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ?
		GROUP BY day, hour
		ORDER BY day, hour
	`, workspaceID).Scan(&results).Error
	return results, err
}

// GetTopPerformingQR returns the QR codes with most scans in a workspace
func (r *WorkspaceRepository) GetTopPerformingQR(workspaceID uuid.UUID, limit int) ([]models.QRRecord, error) {
	var records []models.QRRecord
	err := r.db.Where("workspace_id = ? AND scan_count > 0", workspaceID).
		Order("scan_count DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

// GetWorkspaceBrowserBreakdown returns browser distribution for a workspace
func (r *WorkspaceRepository) GetWorkspaceBrowserBreakdown(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT COALESCE(qs.browser, 'Unknown') as name, COUNT(*) as value
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ?
		GROUP BY qs.browser
		ORDER BY value DESC
		LIMIT 10
	`, workspaceID).Scan(&results).Error
	return results, err
}

// GetWorkspaceReferrerBreakdown returns referrer distribution for a workspace
func (r *WorkspaceRepository) GetWorkspaceReferrerBreakdown(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.Raw(`
		SELECT COALESCE(qs.referrer, 'Direct') as name, COUNT(*) as value
		FROM qr_scans qs
		JOIN qr_records qr ON qr.id = qs.qr_id
		WHERE qr.workspace_id = ? AND qs.referrer != ''
		GROUP BY qs.referrer
		ORDER BY value DESC
		LIMIT 10
	`, workspaceID).Scan(&results).Error
	return results, err
}

// ==========================================
// LEAD CAPTURE
// ==========================================

func (r *WorkspaceRepository) CreateLeadCapturePage(page *models.LeadCapturePage) error {
	return r.db.Create(page).Error
}

func (r *WorkspaceRepository) GetLeadCapturePages(workspaceID uuid.UUID) ([]models.LeadCapturePage, error) {
	var pages []models.LeadCapturePage
	err := r.db.Where("workspace_id = ?", workspaceID).
		Order("created_at DESC").
		Find(&pages).Error
	return pages, err
}

func (r *WorkspaceRepository) GetLeadCapturePageBySlug(slug string) (*models.LeadCapturePage, error) {
	var page models.LeadCapturePage
	err := r.db.First(&page, "slug = ?", slug).Error
	return &page, err
}

func (r *WorkspaceRepository) DeleteLeadCapturePage(id uuid.UUID) error {
	return r.db.Delete(&models.LeadCapturePage{}, "id = ?", id).Error
}

func (r *WorkspaceRepository) IncrementPageViews(pageID uuid.UUID) {
	r.db.Model(&models.LeadCapturePage{}).Where("id = ?", pageID).
		UpdateColumn("views", gorm.Expr("views + 1"))
}

func (r *WorkspaceRepository) IncrementPageSubmissions(pageID uuid.UUID) {
	r.db.Model(&models.LeadCapturePage{}).Where("id = ?", pageID).
		UpdateColumn("submissions", gorm.Expr("submissions + 1"))
}

func (r *WorkspaceRepository) CreateLead(lead *models.Lead) error {
	return r.db.Create(lead).Error
}

func (r *WorkspaceRepository) GetLeads(workspaceID uuid.UUID, pageID *uuid.UUID, limit, offset int) ([]models.Lead, int64, error) {
	var leads []models.Lead
	var total int64

	query := r.db.Joins("JOIN lead_capture_pages ON lead_capture_pages.id = leads.page_id").
		Where("lead_capture_pages.workspace_id = ?", workspaceID)

	if pageID != nil {
		query = query.Where("leads.page_id = ?", *pageID)
	}

	query.Model(&models.Lead{}).Count(&total)

	err := query.Order("leads.created_at DESC").
		Limit(limit).Offset(offset).
		Find(&leads).Error
	return leads, total, err
}
