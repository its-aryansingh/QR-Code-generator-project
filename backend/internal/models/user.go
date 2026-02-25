package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User subscription plans
const (
	PlanFree       = "free"
	PlanStarter    = "starter"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null;size:255" json:"email"`
	PasswordHash string    `gorm:"not null;size:255" json:"-"`

	// Profile
	Name      string `gorm:"size:255" json:"name,omitempty"`
	Company   string `gorm:"size:255" json:"company,omitempty"`
	AvatarURL string `gorm:"type:text" json:"avatar_url,omitempty"`

	// Subscription
	Plan               string     `gorm:"size:20;default:'free'" json:"plan"`
	PlanExpiresAt      *time.Time `json:"plan_expires_at,omitempty"`
	SubscriptionStatus string     `gorm:"size:20;default:''" json:"subscription_status,omitempty"`
	SubscriptionEndsAt *time.Time `json:"subscription_ends_at,omitempty"`

	// API Access
	APIKey          string    `gorm:"size:64;uniqueIndex" json:"api_key,omitempty"`
	APICallsToday   int       `gorm:"default:0" json:"api_calls_today"`
	APICallsResetAt time.Time `gorm:"type:date" json:"-"`

	// Stripe
	StripeCustomerID     string `gorm:"size:255" json:"-"`
	StripeSubscriptionID string `gorm:"size:255" json:"-"`

	// Timestamps
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	QRRecords []QRRecord `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"qr_records,omitempty"`
	Files     []QRFile   `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// GenerateAPIKey creates a new API key for the user
func (u *User) GenerateAPIKey() error {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return err
	}
	u.APIKey = hex.EncodeToString(bytes)
	return nil
}

// IsPremium returns true if user has an active paid subscription
func (u *User) IsPremium() bool {
	if u.Plan == PlanFree {
		return false
	}
	if u.PlanExpiresAt != nil && u.PlanExpiresAt.Before(time.Now()) {
		return false
	}
	return true
}

// CanUseDynamicQR returns true if user's plan supports dynamic QR
func (u *User) CanUseDynamicQR() bool {
	return u.Plan != PlanFree
}

// CanUseAPI returns true if user's plan includes API access
func (u *User) CanUseAPI() bool {
	return u.Plan == PlanPro || u.Plan == PlanEnterprise
}

// GetDynamicQRLimit returns max dynamic QRs for user's plan
func (u *User) GetDynamicQRLimit() int {
	switch u.Plan {
	case PlanStarter:
		return 10
	case PlanPro:
		return 100
	case PlanEnterprise:
		return -1 // Unlimited
	default:
		return 0
	}
}

// GetAPICallsLimit returns daily API call limit
func (u *User) GetAPICallsLimit() int {
	switch u.Plan {
	case PlanPro:
		return 1000
	case PlanEnterprise:
		return 10000
	default:
		return 0
	}
}
