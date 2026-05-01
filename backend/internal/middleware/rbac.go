package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/qrapp/backend/internal/models"
)

// RBACConfig defines which roles are allowed for an action
type RBACConfig struct {
	AllowedRoles []string
}

// WorkspaceRBAC middleware checks if the authenticated user has the required role
// in the workspace specified by :id param
func WorkspaceRBAC(db *gorm.DB, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from auth context
		userIDVal, exists := c.Get("userID")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authentication required",
			})
			return
		}
		userID := userIDVal.(uuid.UUID)

		// Get workspace ID from URL param
		workspaceIDStr := c.Param("id")
		if workspaceIDStr == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Workspace ID required",
			})
			return
		}

		workspaceID, err := uuid.Parse(workspaceIDStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid workspace ID",
			})
			return
		}

		// Look up membership
		var member models.WorkspaceMember
		err = db.Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&member).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "You are not a member of this workspace",
			})
			return
		}

		// Check role permission
		if !isRoleAllowed(member.Role, allowedRoles) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "Insufficient permissions. Required role: " + strings.Join(allowedRoles, " or "),
			})
			return
		}

		// Inject workspace member info into context for downstream handlers
		c.Set("workspaceMember", member)
		c.Set("workspaceID", workspaceID)
		c.Set("memberRole", member.Role)

		c.Next()
	}
}

func isRoleAllowed(role string, allowed []string) bool {
	for _, r := range allowed {
		if role == r {
			return true
		}
	}
	return false
}
