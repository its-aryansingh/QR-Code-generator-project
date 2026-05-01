package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
)

// AnalyticsHandler handles analytics endpoints
type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
	qrService        *services.QRService
}

// NewAnalyticsHandler creates a new analytics handler
func NewAnalyticsHandler(analyticsService *services.AnalyticsService, qrService *services.QRService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
		qrService:        qrService,
	}
}

// GetQRAnalytics retrieves analytics for a specific QR code
// GET /api/v1/qr/:id/analytics
func (h *AnalyticsHandler) GetQRAnalytics(c *gin.Context) {
	// Get QR ID
	qrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	// Get user ID from context
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	// Verify ownership
	record, err := h.qrService.GetByID(qrID, userID)
	if err != nil {
		utils.NotFound(c, "QR code not found")
		return
	}

	// Get days parameter (default 30)
	days := 30
	if daysParam := c.Query("days"); daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
			days = d
		}
	}

	// Get analytics
	analytics, err := h.analyticsService.GetAnalytics(record.ID, days)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve analytics")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"qr_id":     qrID,
			"title":     record.Title,
			"analytics": analytics,
		},
	})
}

// GetRecentScans retrieves recent scans for a QR code
// GET /api/v1/qr/:id/scans
func (h *AnalyticsHandler) GetRecentScans(c *gin.Context) {
	// Get QR ID
	qrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	// Get user ID from context
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	// Verify ownership
	_, err = h.qrService.GetByID(qrID, userID)
	if err != nil {
		utils.NotFound(c, "QR code not found")
		return
	}

	// Get limit (default 50)
	limit := 50

	// Get recent scans
	scans, err := h.analyticsService.GetRecentScans(qrID, limit)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve scans")
		return
	}

	utils.Success(c, scans)
}

// GetUserSummary retrieves analytics summary for user
// GET /api/v1/analytics/summary
func (h *AnalyticsHandler) GetUserSummary(c *gin.Context) {
	// Get user ID from context
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	// Get summary
	summary, err := h.analyticsService.GetUserAnalyticsSummary(userID)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve summary")
		return
	}

	utils.Success(c, summary)
}

// ExportAnalytics exports analytics data as CSV
// GET /api/v1/qr/:id/analytics/export
func (h *AnalyticsHandler) ExportAnalytics(c *gin.Context) {
	// Get QR ID
	qrID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	// Get user ID from context
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	// Verify ownership
	_, err = h.qrService.GetByID(qrID, userID)
	if err != nil {
		utils.NotFound(c, "QR code not found")
		return
	}

	// Get all scans
	scans, err := h.analyticsService.GetAllScans(qrID)
	if err != nil {
		utils.InternalError(c, "Failed to retrieve scans")
		return
	}

	// Set headers
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=qr_analytics_%s.csv", qrID.String()))

	writer := csv.NewWriter(c.Writer)

	// Write CSV Header
	writer.Write([]string{"Time", "IP Address", "Country", "City", "Device", "OS", "Browser", "Referrer"})

	// Write Data
	for _, scan := range scans {
		writer.Write([]string{
			scan.ScannedAt.Format(time.RFC3339),
			scan.IPAddress,
			scan.CountryName,
			scan.City,
			scan.DeviceType,
			scan.OS,
			scan.Browser,
			scan.Referrer,
		})
	}
	writer.Flush()
}
