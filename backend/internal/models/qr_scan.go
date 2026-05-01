package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// QRScan represents a scan event for analytics tracking
type QRScan struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	QRID      uuid.UUID `gorm:"type:uuid;not null;index" json:"qr_id"`
	ScannedAt time.Time `gorm:"autoCreateTime;index" json:"scanned_at"`

	// Network info
	IPAddress string `gorm:"size:45" json:"ip_address,omitempty"` // IPv6 max length

	// GeoIP data (MaxMind)
	CountryCode string  `gorm:"size:2" json:"country_code,omitempty"`
	CountryName string  `gorm:"size:100" json:"country_name,omitempty"`
	City        string  `gorm:"size:100" json:"city,omitempty"`
	Region      string  `gorm:"size:100" json:"region,omitempty"`
	Latitude    float64 `gorm:"type:decimal(9,6)" json:"latitude,omitempty"`
	Longitude   float64 `gorm:"type:decimal(9,6)" json:"longitude,omitempty"`

	// Device info
	DeviceType     string `gorm:"size:20" json:"device_type,omitempty"` // mobile, tablet, desktop
	OS             string `gorm:"size:50" json:"os,omitempty"`
	OSVersion      string `gorm:"size:20" json:"os_version,omitempty"`
	Browser        string `gorm:"size:50" json:"browser,omitempty"`
	BrowserVersion string `gorm:"size:20" json:"browser_version,omitempty"`

	// Request info
	UserAgent string `gorm:"type:text" json:"user_agent,omitempty"`
	Referrer  string `gorm:"type:text" json:"referrer,omitempty"`
	Language  string `gorm:"size:10" json:"language,omitempty"`

	// Relations
	QRRecord *QRRecord `gorm:"foreignKey:QRID" json:"qr_record,omitempty"`
}

func (s *QRScan) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// ScanAnalytics represents aggregated scan statistics
type ScanAnalytics struct {
	TotalScans      int64           `json:"total_scans"`
	UniqueScans     int64           `json:"unique_scans"`
	ScansByDate     []DateCount     `json:"scans_by_date,omitempty"`
	ScansByCountry  []CountryCount  `json:"scans_by_country,omitempty"`
	ScansByDevice   []DeviceCount   `json:"scans_by_device,omitempty"`
	ScansByBrowser  []BrowserCount  `json:"scans_by_browser,omitempty"`
	ScansByOS       []OSCount       `json:"scans_by_os,omitempty"`
	ScansByReferrer []ReferrerCount `json:"scans_by_referrer,omitempty"`
	TopCities       []CityCount     `json:"top_cities,omitempty"`
}

type DateCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type CountryCount struct {
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	Count       int64  `json:"count"`
}

type DeviceCount struct {
	DeviceType string `json:"device_type"`
	Count      int64  `json:"count"`
}

type BrowserCount struct {
	Browser string `json:"browser"`
	Count   int64  `json:"count"`
}

type OSCount struct {
	OS    string `json:"os"`
	Count int64  `json:"count"`
}

type ReferrerCount struct {
	Referrer string `json:"referrer"`
	Count    int64  `json:"count"`
}

type CityCount struct {
	City        string `json:"city"`
	CountryCode string `json:"country_code"`
	Count       int64  `json:"count"`
}
