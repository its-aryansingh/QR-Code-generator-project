package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Audit action types
const (
	AuditActionCreate = "create"
	AuditActionUpdate = "update"
	AuditActionDelete = "delete"
	AuditActionInvite = "invite"
	AuditActionJoin   = "join"
	AuditActionLeave  = "leave"
	AuditActionExport = "export"
	AuditActionBulk   = "bulk_generate"
)

// Audit resource types
const (
	AuditResourceQR        = "qr_code"
	AuditResourceFolder    = "folder"
	AuditResourceWorkspace = "workspace"
	AuditResourceMember    = "member"
	AuditResourceWebhook   = "webhook"
	AuditResourceAPIKey    = "api_key"
)

// AuditLog represents an audit trail entry for compliance
type AuditLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID  `gorm:"type:uuid;not null;index" json:"workspace_id"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Action      string     `gorm:"size:50;not null;index" json:"action"`
	Resource    string     `gorm:"size:50;not null" json:"resource"`
	ResourceID  *uuid.UUID `gorm:"type:uuid" json:"resource_id,omitempty"`
	Details     string     `gorm:"type:text" json:"details,omitempty"`
	IPAddress   string     `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent   string     `gorm:"type:text" json:"user_agent,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime;index" json:"created_at"`

	// Relations
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
