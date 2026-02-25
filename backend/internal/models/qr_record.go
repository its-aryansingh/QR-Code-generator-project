package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// QRRecord represents a generated QR code
type QRRecord struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`

	// Enterprise: workspace & folder scoping
	WorkspaceID *uuid.UUID `gorm:"type:uuid;index" json:"workspace_id,omitempty"`
	FolderID    *uuid.UUID `gorm:"type:uuid;index" json:"folder_id,omitempty"`
	Tags        string     `gorm:"type:text" json:"tags,omitempty"` // comma-separated tags

	// Basic info
	Title    string `gorm:"size:255" json:"title,omitempty"`
	Content  string `gorm:"type:text;not null" json:"content"`
	QRType   string `gorm:"size:50;default:'url'" json:"qr_type"`
	QRTypeID string `gorm:"size:50" json:"qr_type_id,omitempty"`
	Size     int    `gorm:"default:256" json:"size"`

	// Dynamic QR support
	ShortCode   string `gorm:"size:20;uniqueIndex" json:"short_code,omitempty"`
	IsDynamic   bool   `gorm:"default:false" json:"is_dynamic"`
	RedirectURL string `gorm:"type:text" json:"redirect_url,omitempty"`

	// Type-specific metadata (vCard fields, WiFi settings, etc.)
	Metadata datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty"`

	// Customization (colors, logo, shape)
	Customization datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"customization,omitempty"`

	// Enterprise: Security & Scheduling
	Password        string     `gorm:"size:255" json:"-"`                           // Password-protected QR
	MaxScans        *int       `json:"max_scans,omitempty"`                         // Scan limit before deactivation
	ScheduledAt     *time.Time `json:"scheduled_at,omitempty"`                      // Activation start time
	GeoRestrictions string     `gorm:"type:text" json:"geo_restrictions,omitempty"` // comma-separated country codes

	// Lifecycle
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	ScanCount int        `gorm:"default:0" json:"scan_count"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`

	// Relations
	User      *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	Folder    *Folder    `gorm:"foreignKey:FolderID" json:"folder,omitempty"`
	Scans     []QRScan   `gorm:"foreignKey:QRID" json:"scans,omitempty"`
	Files     []QRFile   `gorm:"foreignKey:QRID" json:"files,omitempty"`
}

func (q *QRRecord) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

// QRCustomization represents customization options for QR codes
type QRCustomization struct {
	ForegroundColor string    `json:"foreground_color,omitempty"` // Hex color
	BackgroundColor string    `json:"background_color,omitempty"` // Hex color
	CornerStyle     string    `json:"corner_style,omitempty"`     // square, rounded, dots
	BodyStyle       string    `json:"body_style,omitempty"`       // square, rounded, dots
	Logo            *Logo     `json:"logo,omitempty"`
	Frame           *Frame    `json:"frame,omitempty"`
	Gradient        *Gradient `json:"gradient,omitempty"`
}

type Gradient struct {
	Type       string  `json:"type"`        // "linear", "radial"
	StartColor string  `json:"start_color"` // Hex
	EndColor   string  `json:"end_color"`   // Hex
	Rotation   float64 `json:"rotation"`    // Degrees (for linear)
}

type Logo struct {
	URL  string  `json:"url"`
	Size float64 `json:"size"` // 0.2 - 0.4 of QR size
}

type Frame struct {
	Style string `json:"style"` // none, bottom-text, top-text
	Text  string `json:"text"`
	Color string `json:"color"`
}

// WiFi metadata structure
type WiFiMetadata struct {
	SSID       string `json:"ssid"`
	Password   string `json:"password,omitempty"`
	Encryption string `json:"encryption"` // WPA, WEP, nopass
	Hidden     bool   `json:"hidden"`
}

// VCard metadata structure
type VCardMetadata struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Mobile     string `json:"mobile,omitempty"`
	Company    string `json:"company,omitempty"`
	JobTitle   string `json:"job_title,omitempty"`
	Website    string `json:"website,omitempty"`
	Address    string `json:"address,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Email metadata structure
type EmailMetadata struct {
	To      string `json:"to"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// SMS metadata structure
type SMSMetadata struct {
	Phone   string `json:"phone"`
	Message string `json:"message,omitempty"`
}

// Social media metadata
type SocialMetadata struct {
	Platform string `json:"platform"` // facebook, instagram, twitter, etc.
	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}
