package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/qrapp/backend/internal/models"
	"github.com/qrapp/backend/pkg/utils"
)

// APIKeyAuth middleware validates API key authentication
func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check X-API-Key header first
		apiKey := c.GetHeader("X-API-Key")

		// Fall back to Authorization: Bearer <key>
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			utils.Unauthorized(c, "API key required. Provide via X-API-Key header.")
			c.Abort()
			return
		}

		// Look up user by API key
		var user models.User
		if err := db.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
			utils.Unauthorized(c, "Invalid API key")
			c.Abort()
			return
		}

		// Check if user can use API
		if !user.CanUseAPI() {
			utils.Forbidden(c, "API access requires Pro or Enterprise plan")
			c.Abort()
			return
		}

		// Check API rate limit
		if user.APICallsToday >= user.GetAPICallsLimit() {
			c.JSON(429, gin.H{
				"success": false,
				"error":   "API rate limit exceeded",
				"limit":   user.GetAPICallsLimit(),
				"reset":   "midnight UTC",
			})
			c.Abort()
			return
		}

		// Increment API call count
		db.Model(&user).UpdateColumn("api_calls_today", gorm.Expr("api_calls_today + 1"))

		// Store user in context
		c.Set("userID", user.ID.String())
		c.Set("user", &user)
		c.Set("apiAccess", true)

		c.Next()
	}
}

// GetUserFromContext retrieves the user from context (set by auth middleware)
func GetUserFromContext(c *gin.Context) (*models.User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}
	return user.(*models.User), true
}

// GetUserIDFromContext retrieves just the user ID from context
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	userIDStr, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}

	userID, err := uuid.Parse(userIDStr.(string))
	if err != nil {
		return uuid.Nil, false
	}

	return userID, true
}
