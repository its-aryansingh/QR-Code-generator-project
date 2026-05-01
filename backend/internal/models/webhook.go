package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook event types
const (
	WebhookEventQRCreated  = "qr.created"
	WebhookEventQRUpdated  = "qr.updated"
	WebhookEventQRDeleted  = "qr.deleted"
	WebhookEventQRScanned  = "qr.scanned"
	WebhookEventBulkDone   = "bulk.completed"
	WebhookEventMemberJoin = "member.joined"
)

// Webhook represents a webhook endpoint configured by the workspace
type Webhook struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID `gorm:"type:uuid;not null;index" json:"workspace_id"`
	URL         string    `gorm:"type:text;not null" json:"url"`
	Secret      string    `gorm:"size:64" json:"-"`
	Events      string    `gorm:"type:text;not null" json:"events"` // comma-separated event types
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Description string    `gorm:"type:text" json:"description,omitempty"`

	// Health tracking
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
	FailCount     int        `gorm:"default:0" json:"fail_count"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Workspace *Workspace   `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	Logs      []WebhookLog `gorm:"foreignKey:WebhookID;constraint:OnDelete:CASCADE" json:"logs,omitempty"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// WebhookLog represents a single webhook delivery attempt
type WebhookLog struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	WebhookID  uuid.UUID `gorm:"type:uuid;not null;index" json:"webhook_id"`
	Event      string    `gorm:"size:50;not null" json:"event"`
	Payload    string    `gorm:"type:text" json:"payload,omitempty"`
	StatusCode int       `gorm:"default:0" json:"status_code"`
	Response   string    `gorm:"type:text" json:"response,omitempty"`
	Success    bool      `gorm:"default:false" json:"success"`
	Duration   int       `gorm:"default:0" json:"duration_ms"`
	Error      string    `gorm:"type:text" json:"error,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (l *WebhookLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
