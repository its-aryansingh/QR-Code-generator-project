package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
	"gorm.io/datatypes"
)

type QRHandler struct {
	qrService *services.QRService
}

func NewQRHandler(qrService *services.QRService) *QRHandler {
	return &QRHandler{qrService: qrService}
}

type GenerateRequest struct {
	Title         string                  `json:"title"`
	Content       string                  `json:"content"`
	QRType        string                  `json:"qr_type"`
	Size          int                     `json:"size"`
	IsDynamic     bool                    `json:"is_dynamic"`
	Metadata      map[string]interface{}  `json:"metadata,omitempty"`
	Customization *models.QRCustomization `json:"customization,omitempty"`
}

func (h *QRHandler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Get user ID from context
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	// Convert to service request
	serviceReq := services.GenerateRequest{
		UserID:        userID,
		Title:         req.Title,
		Content:       req.Content,
		QRType:        req.QRType,
		Size:          req.Size,
		IsDynamic:     req.IsDynamic,
		Metadata:      req.Metadata,
		Customization: req.Customization,
	}

	result, err := h.qrService.Generate(serviceReq)
	if err != nil {
		switch err {
		case services.ErrInvalidSize:
			utils.BadRequest(c, "Size must be between 64 and 2048 pixels")
		case services.ErrInvalidContent:
			utils.BadRequest(c, "Content cannot be empty")
		case services.ErrDynamicQRLimit:
			utils.Forbidden(c, "Dynamic QR limit reached. Upgrade your plan.")
		default:
			utils.BadRequest(c, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":         result.Record.ID,
			"title":      result.Record.Title,
			"content":    result.Record.Content,
			"qr_type":    result.Record.QRType,
			"size":       result.Record.Size,
			"is_dynamic": result.Record.IsDynamic,
			"short_code": result.Record.ShortCode,
			"qr_base64":  result.QRBase64,
			"created_at": result.Record.CreatedAt,
		},
	})
}

func (h *QRHandler) History(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	result, err := h.qrService.GetHistory(userID, page, pageSize)
	if err != nil {
		utils.InternalError(c, "Failed to fetch history")
		return
	}

	utils.Success(c, result)
}

func (h *QRHandler) GetByID(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	record, err := h.qrService.GetByID(id, userID)
	if err != nil {
		if err == services.ErrQRNotFound {
			utils.NotFound(c, "QR record not found")
			return
		}
		utils.InternalError(c, "Failed to fetch QR record")
		return
	}

	utils.Success(c, record)
}

type UpdateRequest struct {
	Title         *string                `json:"title,omitempty"`
	RedirectURL   *string                `json:"redirect_url,omitempty"`
	IsActive      *bool                  `json:"is_active,omitempty"`
	Customization map[string]interface{} `json:"customization,omitempty"`
}

func (h *QRHandler) Update(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Call UpdateQR service
	updateReq := services.UpdateQRRequest{
		Title:       req.Title,
		RedirectURL: req.RedirectURL,
		IsActive:    req.IsActive,
	}

	if req.Customization != nil {
		jsonBytes, err := json.Marshal(req.Customization)
		if err == nil {
			jsonJSON := datatypes.JSON(jsonBytes)
			updateReq.Customization = &jsonJSON
		}
	}

	if err := h.qrService.UpdateQR(id, userID, updateReq); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	// Get updated record
	record, err := h.qrService.GetByID(id, userID)
	if err != nil {
		utils.NotFound(c, "QR record not found")
		return
	}

	utils.Success(c, record)
}

func (h *QRHandler) Delete(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID := userIDStr.(uuid.UUID)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.BadRequest(c, "Invalid QR ID")
		return
	}

	if err := h.qrService.DeleteQR(id, userID); err != nil {
		// Assuming error means not found or forbidden (since service checks ID)
		// But service currently just returns error from repo request if not found
		utils.InternalError(c, "Failed to delete QR code")
		return
	}

	c.Status(http.StatusNoContent)
}
