package services

import (
	"time"

	"github.com/google/uuid"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
)

// AnalyticsService handles QR scan analytics
type AnalyticsService struct {
	scanRepo *repository.QRScanRepository
	qrRepo   *repository.QRRecordRepository
	geoIP    GeoIPProvider
}

// GeoIPProvider interface for GeoIP lookups
type GeoIPProvider interface {
	Lookup(ip string) *GeoIPData
}

// GeoIPData contains location data from IP lookup
type GeoIPData struct {
	CountryCode string  `json:"country_code"`
	CountryName string  `json:"country_name"`
	City        string  `json:"city"`
	Region      string  `json:"region"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(scanRepo *repository.QRScanRepository, qrRepo *repository.QRRecordRepository, geoIP GeoIPProvider) *AnalyticsService {
	return &AnalyticsService{
		scanRepo: scanRepo,
		qrRepo:   qrRepo,
		geoIP:    geoIP,
	}
}

// RecordScan records a scan event
func (s *AnalyticsService) RecordScan(scan *models.QRScan) error {
	// Get GeoIP data
	if s.geoIP != nil && scan.IPAddress != "" {
		geoData := s.geoIP.Lookup(scan.IPAddress)
		if geoData != nil {
			scan.CountryCode = geoData.CountryCode
			scan.CountryName = geoData.CountryName
			scan.City = geoData.City
			scan.Region = geoData.Region
			scan.Latitude = geoData.Latitude
			scan.Longitude = geoData.Longitude
		}
	}

	// Save scan
	if err := s.scanRepo.Create(scan); err != nil {
		return err
	}

	// Update scan count on QR record
	return s.qrRepo.IncrementScanCount(scan.QRID)
}

// GetAnalytics retrieves analytics for a QR code
func (s *AnalyticsService) GetAnalytics(qrID uuid.UUID, days int) (*models.ScanAnalytics, error) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	return s.scanRepo.GetAnalytics(qrID, startDate, endDate)
}

// GetRecentScans retrieves recent scans for a QR code
func (s *AnalyticsService) GetRecentScans(qrID uuid.UUID, limit int) ([]models.QRScan, error) {
	return s.scanRepo.GetByQRID(qrID, limit, 0)
}

// GetAllScans retrieves all scans for a QR code (for export)
func (s *AnalyticsService) GetAllScans(qrID uuid.UUID) ([]models.QRScan, error) {
	// 0 limit means no limit in our repo implementation
	return s.scanRepo.GetByQRID(qrID, -1, 0)
}

// GetGeoIP performs a GeoIP lookup
func (s *AnalyticsService) GetGeoIP(ip string) *GeoIPData {
	if s.geoIP == nil {
		return nil
	}
	return s.geoIP.Lookup(ip)
}

// GetUserAnalyticsSummary gets aggregate analytics for all user's QR codes
func (s *AnalyticsService) GetUserAnalyticsSummary(userID uuid.UUID) (*UserAnalyticsSummary, error) {
	summary := &UserAnalyticsSummary{}

	// Get all user's QR records
	records, err := s.qrRepo.FindByUserID(userID, 1000, 0)
	if err != nil {
		return nil, err
	}

	summary.TotalQRCodes = len(records)

	// Aggregate scan counts
	for _, record := range records {
		summary.TotalScans += int64(record.ScanCount)
		if record.IsDynamic {
			summary.DynamicQRCodes++
		}
	}

	return summary, nil
}

// UserAnalyticsSummary contains aggregate analytics for a user
type UserAnalyticsSummary struct {
	TotalQRCodes   int   `json:"total_qr_codes"`
	DynamicQRCodes int   `json:"dynamic_qr_codes"`
	TotalScans     int64 `json:"total_scans"`
}
