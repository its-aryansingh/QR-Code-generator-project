package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
)

// PublicHandler handles public (no auth) QR generation
// PublicHandler handles public (no auth) QR generation
type PublicHandler struct {
	qrService        *services.QRService
	freeTierRepo     *repository.FreeTierRepository
	analyticsService *services.AnalyticsService
}

// NewPublicHandler creates a new public handler
func NewPublicHandler(qrService *services.QRService, freeTierRepo *repository.FreeTierRepository, analyticsService *services.AnalyticsService) *PublicHandler {
	return &PublicHandler{
		qrService:        qrService,
		freeTierRepo:     freeTierRepo,
		analyticsService: analyticsService,
	}
}

// PublicGenerateRequest is the request for free QR generation
type PublicGenerateRequest struct {
	Content       string                 `json:"content"` // May be empty for types that use metadata
	QRType        string                 `json:"qr_type"`
	Size          int                    `json:"size"`
	Format        string                 `json:"format"`
	IsDynamic     bool                   `json:"is_dynamic"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Customization *BasicCustomization    `json:"customization,omitempty"`
}

// BasicCustomization is limited customization for free tier
type BasicCustomization struct {
	ForegroundColor string      `json:"foreground_color"`
	BackgroundColor string      `json:"background_color"`
	CornerStyle     string      `json:"corner_style"`
	BodyStyle       string      `json:"body_style"`
	Frame           *BasicFrame `json:"frame,omitempty"`
	Logo            *BasicLogo  `json:"logo,omitempty"`
}

// BasicFrame for frame customization
type BasicFrame struct {
	Style string `json:"style"`
	Text  string `json:"text"`
}

// BasicLogo used for logo customization
type BasicLogo struct {
	URL  string  `json:"url"`
	Size float64 `json:"size"`
}

// Generate handles free QR code generation
// POST /api/v1/public/generate
func (h *PublicHandler) Generate(c *gin.Context) {
	var req PublicGenerateRequest
	// Parse request
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[PublicHandler] BindJSON Failed: %v", err)
		utils.BadRequest(c, err.Error())
		return
	}

	log.Printf("[PublicHandler] Received Request. Type: %s, Dynamic: %v", req.QRType, req.IsDynamic)

	// Get client IP
	clientIP := getClientIP(c)

	// Check rate limit
	canGenerate, remaining, err := h.freeTierRepo.CanGenerate(clientIP)
	if err != nil {
		utils.InternalError(c, "Failed to check rate limit")
		return
	}

	if !canGenerate {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success":   false,
			"error":     "Daily limit reached. Sign up for unlimited QR codes!",
			"remaining": 0,
			"limit":     models.FreeTierDailyLimit,
		})
		return
	}

	// Check if premium type requested
	if models.IsPremiumType(req.QRType) {
		utils.Forbidden(c, "Premium QR type. Please sign up for access.")
		return
	}

	// Convert to service request
	var customization *models.QRCustomization
	if req.Customization != nil {
		customization = &models.QRCustomization{
			ForegroundColor: req.Customization.ForegroundColor,
			BackgroundColor: req.Customization.BackgroundColor,
			CornerStyle:     req.Customization.CornerStyle,
			BodyStyle:       req.Customization.BodyStyle,
		}
		// Map frame if provided
		if req.Customization.Frame != nil {
			customization.Frame = &models.Frame{
				Style: req.Customization.Frame.Style,
				Text:  req.Customization.Frame.Text,
			}
		}
		// Map logo if provided
		if req.Customization.Logo != nil {
			customization.Logo = &models.Logo{
				URL:  req.Customization.Logo.URL,
				Size: req.Customization.Logo.Size,
			}
		}
	}

	// Generate QR
	result, err := h.qrService.Generate(services.GenerateRequest{
		Content:       req.Content,
		QRType:        req.QRType,
		Size:          req.Size,
		Metadata:      req.Metadata,
		Customization: customization,
		IsDynamic:     req.IsDynamic, // Allowed for testing/demo
	})

	if err != nil {
		log.Printf("[PublicHandler] Generate Failed: %v", err)
		utils.BadRequest(c, err.Error())
		return
	}
	log.Println("[PublicHandler] Generation Successful.")

	// Increment usage
	h.freeTierRepo.GetOrCreate(clientIP)
	h.freeTierRepo.IncrementUsage(clientIP)

	// Generate SVG/PDF if requested
	var qrData string
	if req.Format == "svg" {
		svg, err := h.qrService.GenerateQRSVG(result.Record.Content, req.Size, customization)
		if err != nil {
			utils.InternalError(c, "Failed to generate SVG")
			return
		}
		qrData = svg
	} else if req.Format == "pdf" {
		pdfBase64, err := h.qrService.GenerateQRPDF(result.Record.Content, req.Size, customization, nil)
		if err != nil {
			utils.InternalError(c, "Failed to generate PDF")
			return
		}
		// Strip data URI prefix
		if len(pdfBase64) > 28 && pdfBase64[:28] == "data:application/pdf;base64," {
			qrData = pdfBase64[28:]
		} else {
			qrData = pdfBase64
		}
	} else {
		qrData = result.QRBase64
	}

	// Return result with remaining count
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"qr_base64":  qrData,
		"content":    result.Record.Content, // For dynamic, this is the original content
		"qr_type":    result.Record.QRType,
		"size":       result.Record.Size,
		"remaining":  remaining - 1,
		"limit":      models.FreeTierDailyLimit,
		"short_code": result.Record.ShortCode,
		"is_dynamic": result.Record.IsDynamic,
	})
}

// GetQRTypes returns all available QR types
// GET /api/v1/public/types
func (h *PublicHandler) GetQRTypes(c *gin.Context) {
	types := []gin.H{
		{"id": "url", "name": "Website", "description": "Link to any website URL", "icon": "globe", "category": "links", "is_premium": false},
		{"id": "text", "name": "Text", "description": "Plain text message", "icon": "file-text", "category": "basic", "is_premium": false},
		{"id": "wifi", "name": "WiFi", "description": "Connect to a Wi-Fi network", "icon": "wifi", "category": "technical", "is_premium": false},
		{"id": "vcard", "name": "vCard", "description": "Share a digital business card", "icon": "user-plus", "category": "business", "is_premium": false},
		{"id": "email", "name": "Email", "description": "Send an email", "icon": "mail", "category": "basic", "is_premium": false},
		{"id": "sms", "name": "SMS", "description": "Send a text message", "icon": "message-square", "category": "basic", "is_premium": false},
		{"id": "phone", "name": "Phone", "description": "Make a phone call", "icon": "phone", "category": "basic", "is_premium": false},
		{"id": "pdf", "name": "PDF", "description": "Show a PDF document", "icon": "file", "category": "media", "is_premium": true},
		{"id": "images", "name": "Images", "description": "Share multiple images", "icon": "image", "category": "media", "is_premium": true},
		{"id": "video", "name": "Video", "description": "Show a video", "icon": "video", "category": "media", "is_premium": true},
		{"id": "mp3", "name": "MP3", "description": "Share an audio file", "icon": "music", "category": "media", "is_premium": true},
		{"id": "facebook", "name": "Facebook", "description": "Share your Facebook page", "icon": "facebook", "category": "social", "is_premium": false},
		{"id": "instagram", "name": "Instagram", "description": "Share your Instagram", "icon": "instagram", "category": "social", "is_premium": false},
		{"id": "whatsapp", "name": "WhatsApp", "description": "Get WhatsApp messages", "icon": "message-circle", "category": "social", "is_premium": false},
		{"id": "social", "name": "Social Media", "description": "Share your social channels", "icon": "share-2", "category": "social", "is_premium": false},
		{"id": "apps", "name": "Apps", "description": "Redirect to an app store", "icon": "smartphone", "category": "technical", "is_premium": false},
		{"id": "menu", "name": "Menu", "description": "Create a restaurant menu", "icon": "menu", "category": "business", "is_premium": true},
		{"id": "coupon", "name": "Coupon", "description": "Share a coupon", "icon": "tag", "category": "business", "is_premium": true},
		{"id": "business", "name": "Business", "description": "Share information about your business", "icon": "briefcase", "category": "business", "is_premium": true},
		{"id": "links", "name": "List of Links", "description": "Share multiple links", "icon": "link", "category": "links", "is_premium": false},
	}

	utils.Success(c, types)
}

// GetRemainingQuota returns remaining free QR quota
// GET /api/v1/public/quota
func (h *PublicHandler) GetRemainingQuota(c *gin.Context) {
	clientIP := getClientIP(c)

	_, remaining, err := h.freeTierRepo.CanGenerate(clientIP)
	if err != nil {
		utils.InternalError(c, "Failed to check quota")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"remaining": remaining,
		"limit":     models.FreeTierDailyLimit,
	})
}

// getClientIP extracts the real client IP from request
func getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header first (for reverse proxy)
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	return c.ClientIP()
}

// GetQRAnalytics retrieves analytics for a public QR code by short code
// GET /api/v1/public/analytics/:code
func (h *PublicHandler) GetQRAnalytics(c *gin.Context) {
	shortCode := c.Param("code")
	if shortCode == "" {
		utils.BadRequest(c, "Short code required")
		return
	}

	// Find QR by short code
	record, err := h.qrService.GetByShortCode(shortCode)
	if err != nil {
		utils.NotFound(c, "QR code not found")
		return
	}

	// For public analytics, we only show data for dynamic QRs
	if !record.IsDynamic {
		utils.BadRequest(c, "Analytics only available for dynamic QR codes")
		return
	}

	// Get analytics (last 7 days by default for public)
	days := 7
	analytics, err := h.analyticsService.GetAnalytics(record.ID, days)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve analytics")
		return
	}

	// Get recent scans (limit 10 for public)
	recentScans, err := h.analyticsService.GetRecentScans(record.ID, 10)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve recent scans")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":           record.ID,
			"analytics":    analytics,
			"recent_scans": recentScans,
		},
	})
}
