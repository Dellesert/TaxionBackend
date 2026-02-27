package handlers

import (
	"net/http"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SettingsHandler handles system settings operations
type SettingsHandler struct {
	settingsUsecase usecase.SettingsUsecase
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(settingsUsecase usecase.SettingsUsecase) *SettingsHandler {
	return &SettingsHandler{
		settingsUsecase: settingsUsecase,
	}
}

// GetSettings retrieves current system settings
// GET /admin/settings/auth
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	settings, err := h.settingsUsecase.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get system settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить системные настройки",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings":   settings,
		"request_id": requestID,
	})
}

// GetPresets retrieves all available security presets
// GET /admin/settings/auth/presets
func (h *SettingsHandler) GetPresets(c *gin.Context) {
	requestID := requestid.Get(c)

	presets := h.settingsUsecase.GetPresets()

	c.JSON(http.StatusOK, gin.H{
		"presets":    presets,
		"request_id": requestID,
	})
}

// ApplyPreset applies a security preset
// PUT /admin/settings/auth/preset
func (h *SettingsHandler) ApplyPreset(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get admin ID from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get admin ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Администратор не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	var req models.ApplyPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for apply preset")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса. Пресет должен быть 'minimal', 'medium' или 'maximum'",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Apply preset
	settings, err := h.settingsUsecase.ApplyPreset(req.Preset, adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"preset":     req.Preset,
			"error":      err.Error(),
		}).Error("Failed to apply security preset")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось применить пресет безопасности",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"preset":     req.Preset,
	}).Info("Security preset applied successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Security preset applied successfully",
		"settings":   settings,
		"request_id": requestID,
	})
}

// UpdateCustomSettings updates custom security settings
// PUT /admin/settings/auth/custom
func (h *SettingsHandler) UpdateCustomSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get admin ID from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get admin ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Администратор не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateCustomSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update custom settings")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Update custom settings
	settings, err := h.settingsUsecase.UpdateCustomSettings(&req, adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to update custom settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось обновить пользовательские настройки",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
	}).Info("Custom security settings updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Custom settings updated successfully",
		"settings":   settings,
		"request_id": requestID,
	})
}

// GetSummary retrieves current security configuration summary with statistics
// GET /admin/settings/auth/summary
func (h *SettingsHandler) GetSummary(c *gin.Context) {
	requestID := requestid.Get(c)

	summary, err := h.settingsUsecase.GetSummary()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get security summary")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить сводку безопасности",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":    summary,
		"request_id": requestID,
	})
}

// Legacy endpoints for backward compatibility

// GetAuthMode returns current authentication mode (deprecated)
func (h *SettingsHandler) GetAuthMode(c *gin.Context) {
	requestID := requestid.Get(c)

	authMode := middleware.GetAuthMode()

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"auth_mode":  authMode,
	}).Info("Auth mode retrieved (legacy endpoint)")

	c.JSON(http.StatusOK, gin.H{
		"auth_mode":  authMode,
		"deprecated": true,
		"message":    "This endpoint is deprecated. Use GET /admin/settings/auth instead",
		"request_id": requestID,
	})
}

// SetAuthMode updates authentication mode (deprecated)
func (h *SettingsHandler) SetAuthMode(c *gin.Context) {
	requestID := requestid.Get(c)

	c.JSON(http.StatusBadRequest, gin.H{
		"error":      "Этот эндпоинт устарел",
		"message":    "Please use PUT /admin/settings/auth/preset or PUT /admin/settings/auth/custom instead",
		"request_id": requestID,
	})
}

// GetAuthSettings returns all authentication-related settings (deprecated)
func (h *SettingsHandler) GetAuthSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	settings, err := h.settingsUsecase.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get auth settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить настройки аутентификации",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings":   settings,
		"deprecated": true,
		"message":    "This endpoint is deprecated. Use GET /admin/settings/auth instead",
		"request_id": requestID,
	})
}

// GetUserSettings retrieves user-specific settings
// GET /user/settings
func (h *SettingsHandler) GetUserSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	settings, err := h.settingsUsecase.GetUserSettings(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить настройки пользователя",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// GetPasswordPolicy returns public password policy for frontend validation
// GET /api/v1/password-policy (public, no auth required)
func (h *SettingsHandler) GetPasswordPolicy(c *gin.Context) {
	requestID := requestid.Get(c)

	settings, err := h.settingsUsecase.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get password policy")

		// Return default policy on error
		c.JSON(http.StatusOK, gin.H{
			"policy": &models.PublicPasswordPolicy{
				MinLength:         8,
				RequireComplexity: true,
				ComplexityRules: []string{
					"Минимум одна буква (a-z, A-Z)",
					"Минимум одна цифра или спецсимвол (!@#$%^&*)",
				},
			},
			"request_id": requestID,
		})
		return
	}

	// Convert to public policy (only password-related fields)
	policy := &models.PublicPasswordPolicy{
		MinLength:         settings.MinPasswordLength,
		RequireComplexity: settings.RequirePasswordComplexity,
	}

	if settings.RequirePasswordComplexity {
		policy.ComplexityRules = []string{
			"Минимум одна буква (a-z, A-Z)",
			"Минимум одна цифра или спецсимвол (!@#$%^&*)",
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"policy":     policy,
		"request_id": requestID,
	})
}

// UpdateUserSettings updates user-specific settings
// PUT /user/settings
func (h *SettingsHandler) UpdateUserSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Пользователь не аутентифицирован",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update user settings")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	settings, err := h.settingsUsecase.UpdateUserSettings(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to update user settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось обновить настройки пользователя",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("User settings updated successfully")

	c.JSON(http.StatusOK, settings)
}
