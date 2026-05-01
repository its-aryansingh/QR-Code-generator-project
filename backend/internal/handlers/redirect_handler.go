package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"
	"golang.org/x/crypto/bcrypt"

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
	webhookService   *services.WebhookService
}

// NewRedirectHandler creates a new redirect handler
func NewRedirectHandler(
	qrService *services.QRService,
	qrRepo *repository.QRRecordRepository,
	scanRepo *repository.QRScanRepository,
	analyticsService *services.AnalyticsService,
	webhookService *services.WebhookService,
) *RedirectHandler {
	return &RedirectHandler{
		qrService:        qrService,
		qrRepo:           qrRepo,
		scanRepo:         scanRepo,
		analyticsService: analyticsService,
		webhookService:   webhookService,
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

	record, err := h.qrService.GetByShortCode(shortCode)
	if err != nil {
		c.Redirect(http.StatusFound, "/?error=invalid")
		return
	}

	// Expiry check
	if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
		c.Data(http.StatusGone, "text/html; charset=utf-8", expiredPage())
		return
	}

	// Max-scans check
	if record.MaxScans != nil && record.ScanCount >= *record.MaxScans {
		c.Data(http.StatusGone, "text/html; charset=utf-8", expiredPage())
		return
	}

	// Geo-restriction check
	if record.GeoRestrictions != "" {
		clientIP := getClientIP(c)
		countryCode := h.resolveCountry(clientIP)
		if !isAllowedCountry(record.GeoRestrictions, countryCode) {
			c.Data(http.StatusForbidden, "text/html; charset=utf-8", geoBlockedPage())
			return
		}
	}

	// Password gate
	if record.Password != "" {
		submitted := c.PostForm("qr_password")
		if submitted == "" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", passwordPage(shortCode, false))
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(record.Password), []byte(submitted)); err != nil {
			c.Data(http.StatusOK, "text/html; charset=utf-8", passwordPage(shortCode, true))
			return
		}
	}

	// Extract context values before goroutine
	userAgent := c.GetHeader("User-Agent")
	clientIP := getClientIP(c)
	referer := c.GetHeader("Referer")
	language := c.GetHeader("Accept-Language")

	go h.trackScan(userAgent, clientIP, referer, language, record)

	h.qrRepo.IncrementScanCount(record.ID)

	// Fire webhook asynchronously
	if h.webhookService != nil && record.WorkspaceID != nil {
		go h.webhookService.TriggerEvent(*record.WorkspaceID, "qr.scanned", map[string]interface{}{
			"qr_id":      record.ID,
			"short_code": record.ShortCode,
			"title":      record.Title,
			"scan_count": record.ScanCount + 1,
			"ip":         clientIP,
			"user_agent": userAgent,
		})
	}

	redirectURL := record.RedirectURL
	if redirectURL == "" {
		redirectURL = record.Content
	}

	if !strings.HasPrefix(redirectURL, "http://") && !strings.HasPrefix(redirectURL, "https://") {
		redirectURL = "https://" + redirectURL
	}

	c.Redirect(http.StatusFound, redirectURL)
}

func (h *RedirectHandler) trackScan(userAgent, clientIP, referer, language string, record *models.QRRecord) {
	ua := user_agent.New(userAgent)
	if ua.Bot() {
		return
	}

	browserName, browserVersion := ua.Browser()
	osInfo := ua.OS()

	deviceType := "desktop"
	if ua.Mobile() {
		deviceType = "mobile"
	} else if strings.Contains(strings.ToLower(userAgent), "tablet") {
		deviceType = "tablet"
	}

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

	h.scanRepo.Create(scan)
}

func (h *RedirectHandler) resolveCountry(ip string) string {
	if h.analyticsService == nil {
		return ""
	}
	geo := h.analyticsService.GetGeoIP(ip)
	if geo == nil {
		return ""
	}
	return geo.CountryCode
}

func isAllowedCountry(restrictions, countryCode string) bool {
	if restrictions == "" {
		return true
	}
	for _, code := range strings.Split(restrictions, ",") {
		if strings.EqualFold(strings.TrimSpace(code), countryCode) {
			return true
		}
	}
	return false
}

func expiredPage() []byte {
	return []byte(`<!doctype html><html><head><meta charset="utf-8">
<title>Link Expired</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f0f0f;color:#e4e4e7}
.box{text-align:center;padding:2rem}h1{font-size:1.5rem;margin-bottom:.5rem}p{color:#71717a}</style>
</head><body><div class="box"><h1>Link Expired</h1><p>This QR code is no longer active.</p></div></body></html>`)
}

func geoBlockedPage() []byte {
	return []byte(`<!doctype html><html><head><meta charset="utf-8">
<title>Not Available</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f0f0f;color:#e4e4e7}
.box{text-align:center;padding:2rem}h1{font-size:1.5rem;margin-bottom:.5rem}p{color:#71717a}</style>
</head><body><div class="box"><h1>Not Available</h1><p>This content is not available in your region.</p></div></body></html>`)
}

func passwordPage(shortCode string, invalid bool) []byte {
	errMsg := ""
	if invalid {
		errMsg = `<p style="color:#f87171;font-size:.875rem;margin-bottom:1rem">Incorrect password. Try again.</p>`
	}
	html := fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8">
<title>Protected QR</title>
<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#0f0f0f;color:#e4e4e7}
.box{background:#18181b;border:1px solid #27272a;border-radius:12px;padding:2rem;width:320px;text-align:center}
h1{font-size:1.2rem;margin-bottom:.5rem}p{color:#71717a;font-size:.875rem;margin-bottom:1.5rem}
input{width:100%%;background:#0f0f0f;border:1px solid #3f3f46;border-radius:8px;color:#e4e4e7;padding:.6rem .75rem;font-size:.875rem;box-sizing:border-box;margin-bottom:1rem}
button{width:100%%;background:#7c3aed;color:#fff;border:none;border-radius:8px;padding:.6rem;font-size:.875rem;cursor:pointer}</style>
</head><body><div class="box">
<h1>🔒 Protected Link</h1>
<p>Enter the password to continue.</p>
%s
<form method="POST" action="/r/%s">
<input type="password" name="qr_password" placeholder="Password" autofocus required>
<button type="submit">Continue</button>
</form></div></body></html>`, errMsg, shortCode)
	return []byte(html)
}
