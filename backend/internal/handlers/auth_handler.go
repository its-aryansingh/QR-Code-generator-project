package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/qrapp/backend/internal/services"
	"github.com/qrapp/backend/pkg/utils"
)

type AuthHandler struct {
	authService services.AuthService
}

func NewAuthHandler(authService services.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

// googleTokenInfo represents the response from Google's tokeninfo endpoint
type googleTokenInfo struct {
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Sub           string `json:"sub"`
	Error         string `json:"error_description"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	user, err := h.authService.Register(req.Email, req.Password)
	if err != nil {
		if err == services.ErrEmailExists {
			utils.ErrorResponse(c, http.StatusConflict, "Email already registered")
			return
		}
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create user")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"created_at": user.CreatedAt,
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, ok := userIDStr.(uuid.UUID)
	if !ok {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Invalid user ID format")
		return
	}

	user, err := h.authService.GetProfile(userID)
	if err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "User not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, user)
}

type UpdateProfileRequest struct {
	Name     string `json:"name"`
	Password string `json:"password" binding:"omitempty,min=8"`
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	userIDStr, exists := c.Get("userID")
	if !exists {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	userID, ok := userIDStr.(uuid.UUID)
	if !ok {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Invalid user ID format")
		return
	}

	if err := h.authService.UpdateProfile(userID, req.Name, req.Password); err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	tokens, err := h.authService.Login(req.Email, req.Password)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, tokens)
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Verify the ID token with Google's tokeninfo endpoint
	resp, err := http.Get(fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", req.IDToken))
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to verify Google token")
		return
	}
	defer resp.Body.Close()

	var tokenInfo googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to parse Google response")
		return
	}

	if tokenInfo.Error != "" || tokenInfo.Email == "" {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Invalid Google token")
		return
	}

	if tokenInfo.EmailVerified != "true" {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Google email not verified")
		return
	}

	// Create or find user and generate tokens
	tokens, err := h.authService.GoogleLogin(tokenInfo.Email, tokenInfo.Name, tokenInfo.Picture)
	if err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to authenticate with Google")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	tokens, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		utils.ErrorResponse(c, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, tokens)
}
