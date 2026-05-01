package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/internal/repository"
)

// WhiteLabelHandler handles workspace white-labeling configuration
type WhiteLabelHandler struct {
	repo *repository.WorkspaceRepository
}

func NewWhiteLabelHandler(repo *repository.WorkspaceRepository) *WhiteLabelHandler {
	return &WhiteLabelHandler{repo: repo}
}

// GetBranding returns the white-label configuration for a workspace
func (h *WhiteLabelHandler) GetBranding(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"custom_domain":   workspace.CustomDomain,
			"brand_color":     workspace.BrandColor,
			"brand_logo":      workspace.BrandLogo,
			"favicon_url":     workspace.FaviconURL,
			"custom_css":      workspace.CustomCSS,
			"remove_branding": workspace.RemoveBranding,
			"custom_footer":   workspace.CustomFooter,
		},
	})
}

// UpdateBranding updates the white-label configuration
func (h *WhiteLabelHandler) UpdateBranding(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	// Check plan allows white-labeling
	if workspace.Plan != models.PlanEnterprise && workspace.Plan != "pro" {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "White-labeling requires a Pro or Enterprise plan",
		})
		return
	}

	var req struct {
		CustomDomain   *string `json:"custom_domain"`
		BrandColor     *string `json:"brand_color"`
		BrandLogo      *string `json:"brand_logo"`
		FaviconURL     *string `json:"favicon_url"`
		CustomCSS      *string `json:"custom_css"`
		RemoveBranding *bool   `json:"remove_branding"`
		CustomFooter   *string `json:"custom_footer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.CustomDomain != nil {
		workspace.CustomDomain = *req.CustomDomain
	}
	if req.BrandColor != nil {
		workspace.BrandColor = *req.BrandColor
	}
	if req.BrandLogo != nil {
		workspace.BrandLogo = *req.BrandLogo
	}
	if req.FaviconURL != nil {
		workspace.FaviconURL = *req.FaviconURL
	}
	if req.CustomCSS != nil {
		workspace.CustomCSS = *req.CustomCSS
	}
	if req.RemoveBranding != nil {
		workspace.RemoveBranding = *req.RemoveBranding
	}
	if req.CustomFooter != nil {
		workspace.CustomFooter = *req.CustomFooter
	}

	if err := h.repo.Update(workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update branding"})
		return
	}

	userID := c.MustGet("userID").(uuid.UUID)
	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionUpdate, Resource: "branding",
		Details: "Updated white-label configuration", IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "data": workspace})
}

// GetSSOConfig returns SSO configuration for a workspace
func (h *WhiteLabelHandler) GetSSOConfig(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	var ssoConfig map[string]interface{}
	if workspace.SSOConfig != "" {
		json.Unmarshal([]byte(workspace.SSOConfig), &ssoConfig)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sso_enabled":  workspace.SSOEnabled,
			"sso_provider": workspace.SSOProvider,
			"sso_config":   ssoConfig,
		},
	})
}

// UpdateSSOConfig updates SSO configuration
func (h *WhiteLabelHandler) UpdateSSOConfig(c *gin.Context) {
	wsID, _ := uuid.Parse(c.Param("id"))

	workspace, err := h.repo.GetByID(wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Workspace not found"})
		return
	}

	if workspace.Plan != models.PlanEnterprise {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "SSO requires an Enterprise plan",
		})
		return
	}

	var req struct {
		Enabled  bool                   `json:"enabled"`
		Provider string                 `json:"provider"` // "saml" or "oidc"
		Config   map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	if req.Provider != "saml" && req.Provider != "oidc" && req.Provider != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Provider must be 'saml' or 'oidc'"})
		return
	}

	workspace.SSOEnabled = req.Enabled
	workspace.SSOProvider = req.Provider

	if req.Config != nil {
		configJSON, _ := json.Marshal(req.Config)
		workspace.SSOConfig = string(configJSON)
	}

	if err := h.repo.Update(workspace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update SSO config"})
		return
	}

	userID := c.MustGet("userID").(uuid.UUID)
	h.repo.CreateAuditLog(&models.AuditLog{
		WorkspaceID: wsID, UserID: userID,
		Action: models.AuditActionUpdate, Resource: "sso",
		Details: "Updated SSO configuration: " + req.Provider, IPAddress: c.ClientIP(),
	})

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "SSO configuration updated"})
}
