package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FreeTierUsage tracks free QR generation by IP
type FreeTierUsage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	IPAddress string    `gorm:"size:45;not null" json:"ip_address"`
	SessionID string    `gorm:"size:64" json:"session_id,omitempty"`
	QRCount   int       `gorm:"default:0" json:"qr_count"`
	Date      time.Time `gorm:"type:date;default:CURRENT_DATE" json:"date"`
	FirstUsed time.Time `gorm:"autoCreateTime" json:"first_used"`
	LastUsed  time.Time `gorm:"autoUpdateTime" json:"last_used"`
}

func (f *FreeTierUsage) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	if f.Date.IsZero() {
		f.Date = time.Now().Truncate(24 * time.Hour)
	}
	return nil
}

// Free tier configuration
const (
	FreeTierDailyLimit = 10 // QRs per IP per day
)

// CanGenerate checks if the IP can generate more free QRs
func (f *FreeTierUsage) CanGenerate() bool {
	return f.QRCount < FreeTierDailyLimit
}

// RemainingQRs returns how many free QRs are left
func (f *FreeTierUsage) RemainingQRs() int {
	remaining := FreeTierDailyLimit - f.QRCount
	if remaining < 0 {
		return 0
	}
	return remaining
}
