package repository

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/qrapp/backend/internal/models"
)

// FreeTierRepository handles free tier usage tracking
type FreeTierRepository struct {
	db *gorm.DB
}

// NewFreeTierRepository creates a new free tier repository
func NewFreeTierRepository(db *gorm.DB) *FreeTierRepository {
	return &FreeTierRepository{db: db}
}

// GetOrCreate gets existing usage record or creates new one for IP + date
func (r *FreeTierRepository) GetOrCreate(ipAddress string) (*models.FreeTierUsage, error) {
	today := time.Now().Truncate(24 * time.Hour)

	usage := &models.FreeTierUsage{
		IPAddress: ipAddress,
		Date:      today,
		QRCount:   0,
	}

	// Upsert: insert or get existing record for this IP + date
	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ip_address"}, {Name: "date"}},
		DoNothing: true,
	}).Create(usage).Error

	if err != nil {
		return nil, err
	}

	// Fetch the actual record
	var existing models.FreeTierUsage
	if err := r.db.Where("ip_address = ? AND date = ?", ipAddress, today).First(&existing).Error; err != nil {
		return usage, nil
	}

	return &existing, nil
}

// IncrementUsage increments the QR count for an IP
func (r *FreeTierRepository) IncrementUsage(ipAddress string) error {
	today := time.Now().Truncate(24 * time.Hour)

	return r.db.Model(&models.FreeTierUsage{}).
		Where("ip_address = ? AND date = ?", ipAddress, today).
		UpdateColumns(map[string]interface{}{
			"qr_count":  gorm.Expr("qr_count + 1"),
			"last_used": time.Now(),
		}).Error
}

// GetUsage gets the current usage for an IP
func (r *FreeTierRepository) GetUsage(ipAddress string) (*models.FreeTierUsage, error) {
	today := time.Now().Truncate(24 * time.Hour)

	var usage models.FreeTierUsage
	err := r.db.Where("ip_address = ? AND date = ?", ipAddress, today).First(&usage).Error
	if err == gorm.ErrRecordNotFound {
		return &models.FreeTierUsage{
			IPAddress: ipAddress,
			Date:      today,
			QRCount:   0,
		}, nil
	}
	return &usage, err
}

// CanGenerate checks if IP can generate more free QRs
func (r *FreeTierRepository) CanGenerate(ipAddress string) (bool, int, error) {
	usage, err := r.GetUsage(ipAddress)
	if err != nil {
		return false, 0, err
	}

	remaining := models.FreeTierDailyLimit - usage.QRCount
	if remaining < 0 {
		remaining = 0
	}

	return usage.CanGenerate(), remaining, nil
}

// CleanupOld removes usage records older than specified days
func (r *FreeTierRepository) CleanupOld(days int) error {
	cutoff := time.Now().AddDate(0, 0, -days).Truncate(24 * time.Hour)
	return r.db.Where("date < ?", cutoff).Delete(&models.FreeTierUsage{}).Error
}
