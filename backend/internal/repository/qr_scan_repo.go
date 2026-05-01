package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/qrapp/backend/internal/models"
)

// QRScanRepository handles QR scan analytics database operations
type QRScanRepository struct {
	db *gorm.DB
}

// NewQRScanRepository creates a new QR scan repository
func NewQRScanRepository(db *gorm.DB) *QRScanRepository {
	return &QRScanRepository{db: db}
}

// Create records a new scan event
func (r *QRScanRepository) Create(scan *models.QRScan) error {
	return r.db.Create(scan).Error
}

// GetByQRID retrieves all scans for a specific QR code
func (r *QRScanRepository) GetByQRID(qrID uuid.UUID, limit, offset int) ([]models.QRScan, error) {
	var scans []models.QRScan
	err := r.db.Where("qr_id = ?", qrID).
		Order("scanned_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&scans).Error
	return scans, err
}

// CountByQRID counts total scans for a QR code
func (r *QRScanRepository) CountByQRID(qrID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.QRScan{}).Where("qr_id = ?", qrID).Count(&count).Error
	return count, err
}

// CountUniqueByQRID counts unique IPs that scanned a QR code
func (r *QRScanRepository) CountUniqueByQRID(qrID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.QRScan{}).
		Where("qr_id = ?", qrID).
		Distinct("ip_address").
		Count(&count).Error
	return count, err
}

// GetScansByDate retrieves scan counts grouped by date
func (r *QRScanRepository) GetScansByDate(qrID uuid.UUID, startDate, endDate time.Time) ([]models.DateCount, error) {
	var results []models.DateCount
	err := r.db.Model(&models.QRScan{}).
		Select("DATE(scanned_at) as date, COUNT(*) as count").
		Where("qr_id = ? AND scanned_at >= ? AND scanned_at <= ?", qrID, startDate, endDate).
		Group("DATE(scanned_at)").
		Order("date ASC").
		Scan(&results).Error
	return results, err
}

// GetScansByCountry retrieves scan counts grouped by country
func (r *QRScanRepository) GetScansByCountry(qrID uuid.UUID) ([]models.CountryCount, error) {
	var results []models.CountryCount
	err := r.db.Model(&models.QRScan{}).
		Select("country_code, country_name, COUNT(*) as count").
		Where("qr_id = ? AND country_code IS NOT NULL", qrID).
		Group("country_code, country_name").
		Order("count DESC").
		Limit(20).
		Scan(&results).Error
	return results, err
}

// GetScansByDevice retrieves scan counts grouped by device type
func (r *QRScanRepository) GetScansByDevice(qrID uuid.UUID) ([]models.DeviceCount, error) {
	var results []models.DeviceCount
	err := r.db.Model(&models.QRScan{}).
		Select("device_type, COUNT(*) as count").
		Where("qr_id = ?", qrID).
		Group("device_type").
		Order("count DESC").
		Scan(&results).Error
	return results, err
}

// GetScansByBrowser retrieves scan counts grouped by browser
func (r *QRScanRepository) GetScansByBrowser(qrID uuid.UUID) ([]models.BrowserCount, error) {
	var results []models.BrowserCount
	err := r.db.Model(&models.QRScan{}).
		Select("browser, COUNT(*) as count").
		Where("qr_id = ? AND browser IS NOT NULL", qrID).
		Group("browser").
		Order("count DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

// GetScansByOS retrieves scan counts grouped by operating system
func (r *QRScanRepository) GetScansByOS(qrID uuid.UUID) ([]models.OSCount, error) {
	var results []models.OSCount
	err := r.db.Model(&models.QRScan{}).
		Select("os, COUNT(*) as count").
		Where("qr_id = ? AND os IS NOT NULL", qrID).
		Group("os").
		Order("count DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

// GetScansByReferrer retrieves scan counts grouped by referrer
func (r *QRScanRepository) GetScansByReferrer(qrID uuid.UUID) ([]models.ReferrerCount, error) {
	var results []models.ReferrerCount
	err := r.db.Model(&models.QRScan{}).
		Select("referrer, COUNT(*) as count").
		Where("qr_id = ? AND referrer != ''", qrID).
		Group("referrer").
		Order("count DESC").
		Limit(10).
		Scan(&results).Error
	return results, err
}

// GetTopCities retrieves top cities by scan count
func (r *QRScanRepository) GetTopCities(qrID uuid.UUID, limit int) ([]models.CityCount, error) {
	var results []models.CityCount
	err := r.db.Model(&models.QRScan{}).
		Select("city, country_code, COUNT(*) as count").
		Where("qr_id = ? AND city IS NOT NULL", qrID).
		Group("city, country_code").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	return results, err
}

// GetAnalytics retrieves full analytics for a QR code
func (r *QRScanRepository) GetAnalytics(qrID uuid.UUID, startDate, endDate time.Time) (*models.ScanAnalytics, error) {
	analytics := &models.ScanAnalytics{}

	// Total scans
	r.db.Model(&models.QRScan{}).Where("qr_id = ?", qrID).Count(&analytics.TotalScans)

	// Unique scans
	r.db.Model(&models.QRScan{}).Where("qr_id = ?", qrID).Distinct("ip_address").Count(&analytics.UniqueScans)

	// Scans by date
	analytics.ScansByDate, _ = r.GetScansByDate(qrID, startDate, endDate)

	// Scans by country
	analytics.ScansByCountry, _ = r.GetScansByCountry(qrID)

	// Scans by device
	analytics.ScansByDevice, _ = r.GetScansByDevice(qrID)

	// Scans by browser
	analytics.ScansByBrowser, _ = r.GetScansByBrowser(qrID)

	// Scans by OS
	analytics.ScansByOS, _ = r.GetScansByOS(qrID)

	// Scans by Referrer
	analytics.ScansByReferrer, _ = r.GetScansByReferrer(qrID)

	// Top cities
	analytics.TopCities, _ = r.GetTopCities(qrID, 10)

	return analytics, nil
}
