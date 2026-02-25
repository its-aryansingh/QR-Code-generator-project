package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LeadCapturePage represents a customizable landing page for QR codes
type LeadCapturePage struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID uuid.UUID  `gorm:"type:uuid;not null;index" json:"workspace_id"`
	QRRecordID  *uuid.UUID `gorm:"type:uuid;index" json:"qr_record_id,omitempty"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	Slug        string     `gorm:"size:255;uniqueIndex" json:"slug"`

	// Page Design
	Headline        string `gorm:"size:500" json:"headline"`
	Subheadline     string `gorm:"type:text" json:"subheadline,omitempty"`
	HeroImage       string `gorm:"type:text" json:"hero_image,omitempty"`
	ButtonText      string `gorm:"size:100;default:'Submit'" json:"button_text"`
	ButtonColor     string `gorm:"size:7;default:'#8B5CF6'" json:"button_color"`
	BackgroundColor string `gorm:"size:7;default:'#09090B'" json:"background_color"`
	TextColor       string `gorm:"size:7;default:'#FAFAFA'" json:"text_color"`
	ThankYouMessage string `gorm:"type:text;default:'Thank you for signing up!'" json:"thank_you_message"`
	RedirectURL     string `gorm:"type:text" json:"redirect_url,omitempty"`

	// Form Fields (JSON array of field configs)
	// e.g., [{"name":"email","type":"email","required":true,"label":"Email Address"}]
	FormFields string `gorm:"type:text;default:'[]'" json:"form_fields"`

	// Settings
	IsActive      bool   `gorm:"default:true" json:"is_active"`
	RequiresOptIn bool   `gorm:"default:false" json:"requires_opt_in"`
	PrivacyPolicy string `gorm:"type:text" json:"privacy_policy_url,omitempty"`
	ConsentText   string `gorm:"type:text" json:"consent_text,omitempty"`

	// Stats
	Views       int `gorm:"default:0" json:"views"`
	Submissions int `gorm:"default:0" json:"submissions"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	Leads     []Lead     `gorm:"foreignKey:PageID;constraint:OnDelete:CASCADE" json:"leads,omitempty"`
}

func (p *LeadCapturePage) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// ConversionRate returns the conversion rate as a percentage
func (p *LeadCapturePage) ConversionRate() float64 {
	if p.Views == 0 {
		return 0
	}
	return float64(p.Submissions) / float64(p.Views) * 100
}

// Lead represents a captured lead from a landing page
type Lead struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	PageID    uuid.UUID `gorm:"type:uuid;not null;index" json:"page_id"`
	Data      string    `gorm:"type:text;not null" json:"data"` // JSON form data
	Email     string    `gorm:"size:255;index" json:"email,omitempty"`
	IPAddress string    `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent string    `gorm:"type:text" json:"user_agent,omitempty"`
	Source    string    `gorm:"size:255" json:"source,omitempty"` // QR code ID or direct
	OptedIn   bool      `gorm:"default:false" json:"opted_in"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (l *Lead) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
