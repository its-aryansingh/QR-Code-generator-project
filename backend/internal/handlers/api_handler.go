package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/qrapp/backend/internal/middleware"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
)

// APIHandler handles API endpoints for programmatic access
type APIHandler struct {
	qrService *services.QRService
	userRepo  repository.UserRepository
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(qrService *services.QRService, userRepo repository.UserRepository) *APIHandler {
	return &APIHandler{qrService: qrService, userRepo: userRepo}
}

// SingleGenerateRequest for API single QR generation
type SingleGenerateRequest struct {
	Content       string                  `json:"content" binding:"required"`
	QRType        string                  `json:"qr_type"`
	Size          int                     `json:"size"`
	Title         string                  `json:"title"`
	IsDynamic     bool                    `json:"is_dynamic"`
	Metadata      map[string]interface{}  `json:"metadata,omitempty"`
	Customization *models.QRCustomization `json:"customization,omitempty"`
	Format        string                  `json:"format"` // "png" or "svg"
}

// Generate creates a single QR code via API
// POST /api/v1/api/generate
func (h *APIHandler) Generate(c *gin.Context) {
	var req SingleGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		utils.Unauthorized(c, "User not found")
		return
	}

	// Default values
	if req.Size == 0 {
		req.Size = 512
	}
	if req.QRType == "" {
		req.QRType = "url"
	}
	if req.Format == "" {
		req.Format = "png"
	}

	// Generate QR
	result, err := h.qrService.Generate(services.GenerateRequest{
		UserID:        userID,
		Title:         req.Title,
		Content:       req.Content,
		QRType:        req.QRType,
		Size:          req.Size,
		IsDynamic:     req.IsDynamic,
		Metadata:      req.Metadata,
		Customization: req.Customization,
	})

	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Generate SVG/PDF if requested
	var qrData string
	if req.Format == "svg" {
		svg, err := h.qrService.GenerateQRSVG(result.Record.Content, req.Size, req.Customization)
		if err != nil {
			utils.InternalError(c, "Failed to generate SVG")
			return
		}
		qrData = svg
	} else if req.Format == "pdf" {
		pdfBase64, err := h.qrService.GenerateQRPDF(result.Record.Content, req.Size, req.Customization, nil)
		if err != nil {
			utils.InternalError(c, "Failed to generate PDF")
			return
		}
		// Strip data URI prefix for PDF blob creation on frontend
		if len(pdfBase64) > 28 && pdfBase64[:28] == "data:application/pdf;base64," {
			qrData = pdfBase64[28:]
		} else {
			qrData = pdfBase64
		}
	} else {
		qrData = result.QRBase64
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         result.Record.ID,
			"content":    result.Record.Content,
			"qr_type":    result.Record.QRType,
			"short_code": result.Record.ShortCode,
			"is_dynamic": result.Record.IsDynamic,
			"format":     req.Format,
			"qr_data":    qrData,
			"created_at": result.Record.CreatedAt,
		},
	})
}

// BulkGenerateRequest for multiple QR codes
type BulkGenerateRequest struct {
	Items []SingleGenerateRequest `json:"items" binding:"required,min=1,max=100"`
}

// BulkGenerateResult for each item in bulk generation
type BulkGenerateResult struct {
	Index   int         `json:"index"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// BulkGenerate creates multiple QR codes in one request
// POST /api/v1/api/bulk
func (h *APIHandler) BulkGenerate(c *gin.Context) {
	var req BulkGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		utils.Unauthorized(c, "User not found")
		return
	}

	// Get user to check plan limits
	user, ok := middleware.GetUserFromContext(c)
	if !ok {
		utils.Unauthorized(c, "User not found")
		return
	}

	// Check if enterprise (required for bulk)
	if user.Plan != models.PlanEnterprise && user.Plan != models.PlanPro {
		utils.Forbidden(c, "Bulk generation requires Pro or Enterprise plan")
		return
	}

	// Limit based on plan
	maxBulk := 20
	if user.Plan == models.PlanEnterprise {
		maxBulk = 100
	}
	if len(req.Items) > maxBulk {
		utils.BadRequest(c, "Exceeds maximum bulk size for your plan")
		return
	}

	// Process items concurrently with worker pool
	results := make([]BulkGenerateResult, len(req.Items))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent generations

	for i, item := range req.Items {
		wg.Add(1)
		go func(index int, req SingleGenerateRequest) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Default values
			if req.Size == 0 {
				req.Size = 512
			}
			if req.QRType == "" {
				req.QRType = "url"
			}
			if req.Format == "" {
				req.Format = "png"
			}

			result, err := h.qrService.Generate(services.GenerateRequest{
				UserID:        userID,
				Title:         req.Title,
				Content:       req.Content,
				QRType:        req.QRType,
				Size:          req.Size,
				IsDynamic:     req.IsDynamic,
				Metadata:      req.Metadata,
				Customization: req.Customization,
			})

			if err != nil {
				results[index] = BulkGenerateResult{
					Index:   index,
					Success: false,
					Error:   err.Error(),
				}
				return
			}

			// Generate SVG/PDF if requested
			var qrData string
			if req.Format == "svg" {
				svg, err := h.qrService.GenerateQRSVG(result.Record.Content, req.Size, req.Customization)
				if err != nil {
					results[index] = BulkGenerateResult{
						Index:   index,
						Success: false,
						Error:   "Failed to generate SVG",
					}
					return
				}
				qrData = svg
			} else if req.Format == "pdf" {
				pdfBase64, err := h.qrService.GenerateQRPDF(result.Record.Content, req.Size, req.Customization, nil)
				if err != nil {
					results[index] = BulkGenerateResult{
						Index:   index,
						Success: false,
						Error:   "Failed to generate PDF",
					}
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

			results[index] = BulkGenerateResult{
				Index:   index,
				Success: true,
				Data: gin.H{
					"id":         result.Record.ID,
					"content":    result.Record.Content,
					"short_code": result.Record.ShortCode,
					"format":     req.Format,
					"qr_data":    qrData,
				},
			}
		}(i, item)
	}

	wg.Wait()

	// Count successes
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"total":      len(req.Items),
		"successful": successCount,
		"failed":     len(req.Items) - successCount,
		"results":    results,
	})
}

// GetAPIKey retrieves or regenerates user's API key
// GET /api/v1/api/key
func (h *APIHandler) GetAPIKey(c *gin.Context) {
	user, ok := middleware.GetUserFromContext(c)
	if !ok {
		// Try getting from JWT auth
		userIDStr, exists := c.Get("userID")
		if !exists {
			utils.Unauthorized(c, "User not found")
			return
		}
		// Return placeholder - actual implementation would fetch from DB
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"user_id":    userIDStr,
				"api_key":    "****",
				"can_access": false,
				"message":    "API access requires Pro or Enterprise plan",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"api_key":     user.APIKey,
			"plan":        user.Plan,
			"calls_today": user.APICallsToday,
			"calls_limit": user.GetAPICallsLimit(),
			"can_access":  user.CanUseAPI(),
		},
	})
}

// RegenerateAPIKey creates a new API key for the user
// POST /api/v1/api/key/regenerate
func (h *APIHandler) RegenerateAPIKey(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	userID := userIDStr.(uuid.UUID)

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		utils.NotFound(c, "User not found")
		return
	}

	if !user.CanUseAPI() {
		utils.Forbidden(c, "API access requires Pro or Enterprise plan")
		return
	}

	if err := user.GenerateAPIKey(); err != nil {
		utils.InternalError(c, "Failed to generate API key")
		return
	}

	if err := h.userRepo.Update(user); err != nil {
		utils.InternalError(c, "Failed to save API key")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"api_key": user.APIKey,
		},
	})
}
