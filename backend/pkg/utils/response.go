package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func SuccessResponse(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, APIResponse{
		Success: true,
		Data:    data,
	})
}

func ErrorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}

// Convenience functions for common responses

// Success sends a 200 OK with data
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// BadRequest sends a 400 error
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, APIResponse{
		Success: false,
		Error:   message,
	})
}

// Unauthorized sends a 401 error
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, APIResponse{
		Success: false,
		Error:   message,
	})
}

// Forbidden sends a 403 error
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, APIResponse{
		Success: false,
		Error:   message,
	})
}

// NotFound sends a 404 error
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, APIResponse{
		Success: false,
		Error:   message,
	})
}

// InternalError sends a 500 error
func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, APIResponse{
		Success: false,
		Error:   message,
	})
}
