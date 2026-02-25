package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
)

// RedirectHandler handles dynamic QR redirects
type RedirectHandler struct {
	qrService        *services.QRService
	qrRepo           *repository.QRRecordRepository
	scanRepo         *repository.QRScanRepository
	analyticsService *services.AnalyticsService
}

// NewRedirectHandler creates a new redirect handler
func NewRedirectHandler(
	qrService *services.QRService,
	qrRepo *repository.QRRecordRepository,
	scanRepo *repository.QRScanRepository,
	analyticsService *services.AnalyticsService,
) *RedirectHandler {
	return &RedirectHandler{
		qrService:        qrService,
		qrRepo:           qrRepo,
		scanRepo:         scanRepo,
		analyticsService: analyticsService,
	}
}

// Redirect handles dynamic QR code redirects
// GET /r/:code
func (h *RedirectHandler) Redirect(c *gin.Context) {
	shortCode := c.Param("code")
	if shortCode == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// Find QR by short code
	record, err := h.qrService.GetByShortCode(shortCode)
	if err != nil {
		// Redirect to home page on error
		c.Redirect(http.StatusFound, "/?error=invalid")
		return
	}

	// Extract all values from gin Context BEFORE spawning goroutine
	// (gin Context is invalid after the request handler returns)
	userAgent := c.GetHeader("User-Agent")
	clientIP := getClientIP(c)
	referer := c.GetHeader("Referer")
	language := c.GetHeader("Accept-Language")

	// Track the scan asynchronously with extracted values
	go h.trackScan(userAgent, clientIP, referer, language, record)

	// Increment scan count
	h.qrRepo.IncrementScanCount(record.ID)

	// Redirect to target URL
	redirectURL := record.RedirectURL
	if redirectURL == "" {
		redirectURL = record.Content
	}

	// Ensure URL has protocol
	if !strings.HasPrefix(redirectURL, "http://") && !strings.HasPrefix(redirectURL, "https://") {
		redirectURL = "https://" + redirectURL
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// trackScan records the scan event with device/location info
// Accepts extracted values instead of gin.Context for goroutine safety
func (h *RedirectHandler) trackScan(userAgent, clientIP, referer, language string, record *models.QRRecord) {
	// Parse user agent
	ua := user_agent.New(userAgent)

	// fast fail for bots
	if ua.Bot() {
		return
	}

	browserName, browserVersion := ua.Browser()
	osInfo := ua.OS()

	// Determine device type
	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	} else if strings.Contains(strings.ToLower(userAgent), "tablet") {
		deviceType = "tablet"
	}

	// Create scan record
	scan := &models.QRScan{
		QRID:           record.ID,
		IPAddress:      clientIP,
		DeviceType:     deviceType,
		OS:             osInfo,
		Browser:        browserName,
		BrowserVersion: browserVersion,
		UserAgent:      userAgent,
		Referrer:       referer,
		Language:       language,
	}

	// Get GeoIP data if analytics service available
	if h.analyticsService != nil {
		geoData := h.analyticsService.GetGeoIP(scan.IPAddress)
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
	h.scanRepo.Create(scan)
}
