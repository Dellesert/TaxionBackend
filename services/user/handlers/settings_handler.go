package handlers

import (
	"net/http"

	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	"tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SettingsHandler handles system settings operations
type SettingsHandler struct{}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// GetAuthMode returns current authentication mode
func (h *SettingsHandler) GetAuthMode(c *gin.Context) {
	requestID := requestid.Get(c)

	authMode := middleware.GetAuthMode()

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"auth_mode":  authMode,
	}).Info("Auth mode retrieved")

	c.JSON(http.StatusOK, gin.H{
		"auth_mode":  authMode,
		"request_id": requestID,
	})
}

// SetAuthMode updates authentication mode (admin only)
func (h *SettingsHandler) SetAuthMode(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get admin user info from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get admin ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Admin not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req struct {
		AuthMode string `json:"auth_mode" binding:"required,oneof=jwt session"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for set auth mode")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body. auth_mode must be 'jwt' or 'session'",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Convert string to AuthMode
	var authMode models.AuthMode
	switch req.AuthMode {
	case "jwt":
		authMode = models.AuthModeJWT
	case "session":
		authMode = models.AuthModeSession
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid auth mode. Must be 'jwt' or 'session'",
			"request_id": requestID,
		})
		return
	}

	// Update auth mode
	err = middleware.SetAuthMode(authMode)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"auth_mode":  authMode,
			"error":      err.Error(),
		}).Error("Failed to set auth mode")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to update authentication mode",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"auth_mode":  authMode,
	}).Info("Auth mode updated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Authentication mode updated successfully",
		"auth_mode":  authMode,
		"request_id": requestID,
	})
}

// GetAuthSettings returns all authentication-related settings
func (h *SettingsHandler) GetAuthSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	authMode := middleware.GetAuthMode()
	authConfig := middleware.GetAuthConfig()

	settings := gin.H{
		"auth_mode": authMode,
	}

	if authConfig != nil {
		settings["session_duration_hours"] = int(authConfig.SessionDuration.Hours())
		settings["jwt_access_token_duration_minutes"] = int(authConfig.JWTConfig.AccessTokenDuration.Minutes())
		settings["jwt_refresh_token_duration_hours"] = int(authConfig.JWTConfig.RefreshTokenDuration.Hours())
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"settings":   settings,
	}).Info("Auth settings retrieved")

	c.JSON(http.StatusOK, gin.H{
		"settings":   settings,
		"request_id": requestID,
	})
}
