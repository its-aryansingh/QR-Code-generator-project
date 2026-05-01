package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/qrapp/backend/internal/models"
)

// QRRecordRepository handles QR record database operations
type QRRecordRepository struct {
	db *gorm.DB
}

// NewQRRecordRepository creates a new QR record repository
func NewQRRecordRepository(db *gorm.DB) *QRRecordRepository {
	return &QRRecordRepository{db: db}
}

// Create creates a new QR record
func (r *QRRecordRepository) Create(record *models.QRRecord) error {
	if record.UserID == uuid.Nil {
		return r.db.Omit("UserID").Create(record).Error
	}
	return r.db.Create(record).Error
}

// FindByID finds a QR record by its ID
func (r *QRRecordRepository) FindByID(id uuid.UUID) (*models.QRRecord, error) {
	var record models.QRRecord
	if err := r.db.First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindByShortCode finds a QR record by its short code (for dynamic QR redirect)
func (r *QRRecordRepository) FindByShortCode(shortCode string) (*models.QRRecord, error) {
	var record models.QRRecord
	if err := r.db.First(&record, "short_code = ? AND is_active = true", shortCode).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindByUserID retrieves paginated QR records for a user
func (r *QRRecordRepository) FindByUserID(userID uuid.UUID, limit, offset int) ([]models.QRRecord, error) {
	var records []models.QRRecord
	err := r.db.Where("user_id = ? AND is_active = ?", userID, true).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	return records, err
}

// CountByUserID counts total QR records for a user
func (r *QRRecordRepository) CountByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.QRRecord{}).Where("user_id = ? AND is_active = ?", userID, true).Count(&count).Error
	return count, err
}

// CountDynamicByUserID counts dynamic QR records for a user
func (r *QRRecordRepository) CountDynamicByUserID(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.QRRecord{}).
		Where("user_id = ? AND is_dynamic = ? AND is_active = ?", userID, true, true).
		Count(&count).Error
	return count, err
}

// Update updates a QR record
func (r *QRRecordRepository) Update(record *models.QRRecord) error {
	return r.db.Save(record).Error
}

// Delete soft-deletes a QR record (sets is_active = false)
func (r *QRRecordRepository) Delete(id uuid.UUID, userID uuid.UUID) error {
	return r.db.Model(&models.QRRecord{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_active", false).Error
}

// IncrementScanCount increments the scan count for a QR record
func (r *QRRecordRepository) IncrementScanCount(id uuid.UUID) error {
	return r.db.Model(&models.QRRecord{}).
		Where("id = ?", id).
		UpdateColumn("scan_count", gorm.Expr("scan_count + 1")).Error
}

// FindExpired finds all expired QR records
func (r *QRRecordRepository) FindExpired() ([]models.QRRecord, error) {
	var records []models.QRRecord
	err := r.db.Where("expires_at IS NOT NULL AND expires_at < ? AND is_active = true", time.Now()).
		Find(&records).Error
	return records, err
}

// DeactivateExpired deactivates all expired QR records
func (r *QRRecordRepository) DeactivateExpired() error {
	return r.db.Model(&models.QRRecord{}).
		Where("expires_at IS NOT NULL AND expires_at < ? AND is_active = true", time.Now()).
		Update("is_active", false).Error
}
