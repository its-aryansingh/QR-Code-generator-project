package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
)

// LeadCaptureHandler handles lead capture page and submission endpoints
type LeadCaptureHandler struct {
	repo *repository.WorkspaceRepository
}

func NewLeadCaptureHandler(repo *repository.WorkspaceRepository) *LeadCaptureHandler {
	return &LeadCaptureHandler{repo: repo}
}

// CreatePage creates a new lead capture landing page
func (h *LeadCaptureHandler) CreatePage(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	var req struct {
		Name            string `json:"name" binding:"required"`
		Headline        string `json:"headline"`
		Subheadline     string `json:"subheadline"`
		HeroImage       string `json:"hero_image"`
		ButtonText      string `json:"button_text"`
		ButtonColor     string `json:"button_color"`
		BackgroundColor string `json:"background_color"`
		TextColor       string `json:"text_color"`
		ThankYouMessage string `json:"thank_you_message"`
		RedirectURL     string `json:"redirect_url"`
		FormFields      string `json:"form_fields"` // JSON array
		RequiresOptIn   bool   `json:"requires_opt_in"`
		QRRecordID      string `json:"qr_record_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	slug := generateLeadSlug(req.Name)

	page := &models.LeadCapturePage{
		WorkspaceID:     wsID,
		Name:            req.Name,
		Slug:            slug,
		Headline:        req.Headline,
		Subheadline:     req.Subheadline,
		HeroImage:       req.HeroImage,
		ButtonText:      req.ButtonText,
		ButtonColor:     req.ButtonColor,
		BackgroundColor: req.BackgroundColor,
		TextColor:       req.TextColor,
		ThankYouMessage: req.ThankYouMessage,
		RedirectURL:     req.RedirectURL,
		FormFields:      req.FormFields,
		RequiresOptIn:   req.RequiresOptIn,
		IsActive:        true,
	}

	if req.QRRecordID != "" {
		qrID, _ := uuid.Parse(req.QRRecordID)
		page.QRRecordID = &qrID
	}

	if page.ButtonText == "" {
		page.ButtonText = "Submit"
	}
	if page.ButtonColor == "" {
		page.ButtonColor = "#8B5CF6"
	}
	if page.BackgroundColor == "" {
		page.BackgroundColor = "#09090B"
	}
	if page.TextColor == "" {
		page.TextColor = "#FAFAFA"
	}
	if page.FormFields == "" {
		page.FormFields = `[{"name":"email","type":"email","required":true,"label":"Email Address"}]`
	}

	if err := h.repo.CreateLeadCapturePage(page); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create page"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "data": page})
}

// GetPages returns all lead capture pages for a workspace
func (h *LeadCaptureHandler) GetPages(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	pages, err := h.repo.GetLeadCapturePages(wsID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch pages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": pages})
}

// GetPage returns a single lead capture page (public, for rendering)
func (h *LeadCaptureHandler) GetPageBySlug(c *gin.Context) {
	slug := c.Param("slug")

	page, err := h.repo.GetLeadCapturePageBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Page not found"})
		return
	}

	// Increment view count
	h.repo.IncrementPageViews(page.ID)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": page})
}

// SubmitLead captures a lead submission
func (h *LeadCaptureHandler) SubmitLead(c *gin.Context) {
	slug := c.Param("slug")

	page, err := h.repo.GetLeadCapturePageBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Page not found"})
		return
	}

	if !page.IsActive {
		c.JSON(http.StatusGone, gin.H{"success": false, "error": "This page is no longer active"})
		return
	}

	var formData map[string]interface{}
	if err := c.ShouldBindJSON(&formData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	dataJSON, _ := json.Marshal(formData)

	email := ""
	if e, ok := formData["email"].(string); ok {
		email = e
	}

	lead := &models.Lead{
		PageID:    page.ID,
		Data:      string(dataJSON),
		Email:     email,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		OptedIn:   formData["opt_in"] == true,
	}

	if err := h.repo.CreateLead(lead); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to submit"})
		return
	}

	// Increment submission count
	h.repo.IncrementPageSubmissions(page.ID)

	response := gin.H{
		"success": true,
		"message": page.ThankYouMessage,
	}
	if page.RedirectURL != "" {
		response["redirect_url"] = page.RedirectURL
	}

	c.JSON(http.StatusCreated, response)
}

// GetLeads returns captured leads for a workspace
func (h *LeadCaptureHandler) GetLeads(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	var pageID *uuid.UUID
	if pid := c.Query("page_id"); pid != "" {
		id, _ := uuid.Parse(pid)
		pageID = &id
	}

	leads, total, err := h.repo.GetLeads(wsID, pageID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to fetch leads"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": leads, "total": total})
}

// DeletePage deletes a lead capture page
func (h *LeadCaptureHandler) DeletePage(c *gin.Context) {
	pageID, _ := uuid.Parse(c.Param("pageID"))

	if err := h.repo.DeleteLeadCapturePage(pageID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Page deleted"})
}

func generateLeadSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.TrimSpace(slug)
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return slug + "-" + uuid.New().String()[:6]
}
