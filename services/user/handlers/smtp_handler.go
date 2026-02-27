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

// SMTPHandler handles HTTP requests for SMTP settings operations
type SMTPHandler struct {
	smtpUsecase usecase.SMTPUsecase
}

// NewSMTPHandler creates a new SMTP handler
func NewSMTPHandler(smtpUsecase usecase.SMTPUsecase) *SMTPHandler {
	return &SMTPHandler{
		smtpUsecase: smtpUsecase,
	}
}

// GetSettings handles getting SMTP settings (admin only)
// @Summary Get SMTP settings
// @Description Get current SMTP configuration (password is hidden)
// @Tags SMTP
// @Security Bearer
// @Produce json
// @Success 200 {object} models.SMTPSettingsResponse
// @Success 204 "No SMTP settings configured yet"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/admin/smtp-settings [get]
func (h *SMTPHandler) GetSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
	}).Info("Getting SMTP settings")

	settings, err := h.smtpUsecase.GetSettings()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get SMTP settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось получить настройки SMTP",
			"details": err.Error(),
		})
		return
	}

	if settings == nil {
		// No settings configured yet
		c.Status(http.StatusNoContent)
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"host":       settings.Host,
	}).Info("SMTP settings retrieved successfully")

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings handles updating SMTP settings (admin only)
// @Summary Update SMTP settings
// @Description Update SMTP configuration
// @Tags SMTP
// @Security Bearer
// @Accept json
// @Produce json
// @Param request body models.UpdateSMTPSettingsRequest true "SMTP settings update request"
// @Success 200 {object} models.SMTPSettingsResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/admin/smtp-settings [put]
func (h *SMTPHandler) UpdateSettings(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.UpdateSMTPSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Invalid request body for updating SMTP settings")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверное тело запроса",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Не авторизован",
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"host":       req.Host,
		"port":       req.Port,
	}).Info("Updating SMTP settings")

	settings, err := h.smtpUsecase.UpdateSettings(&req, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to update SMTP settings")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось обновить настройки SMTP",
			"details": err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"host":       settings.Host,
	}).Info("SMTP settings updated successfully")

	c.JSON(http.StatusOK, settings)
}

// TestConnection handles testing SMTP connection (admin only)
// @Summary Test SMTP connection
// @Description Test SMTP connection with provided settings
// @Tags SMTP
// @Security Bearer
// @Accept json
// @Produce json
// @Param request body models.TestSMTPConnectionRequest true "SMTP connection test request"
// @Success 200 {object} models.TestSMTPConnectionResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/admin/smtp-settings/test [post]
func (h *SMTPHandler) TestConnection(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.TestSMTPConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Invalid request body for testing SMTP connection")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверное тело запроса",
			"details": err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"host":       req.Host,
		"port":       req.Port,
		"test_email": req.TestEmail,
	}).Info("Testing SMTP connection")

	result, err := h.smtpUsecase.TestConnection(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to test SMTP connection")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Не удалось протестировать SMTP-соединение",
			"details": err.Error(),
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"success":    result.Success,
		"message":    result.Message,
	}).Info("SMTP connection test completed")

	c.JSON(http.StatusOK, result)
}
