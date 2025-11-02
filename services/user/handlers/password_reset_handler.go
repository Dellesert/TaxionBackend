package handlers

import (
	"net/http"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PasswordResetHandler handles password reset related HTTP requests
type PasswordResetHandler struct {
	passwordResetUsecase usecase.PasswordResetUsecase
}

// NewPasswordResetHandler creates a new password reset handler
func NewPasswordResetHandler(passwordResetUsecase usecase.PasswordResetUsecase) *PasswordResetHandler {
	return &PasswordResetHandler{
		passwordResetUsecase: passwordResetUsecase,
	}
}

// InitiatePasswordReset handles admin request to initiate password reset (admin only)
func (h *PasswordResetHandler) InitiatePasswordReset(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get admin ID from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Check if user is admin or super_admin
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || (userRole != sharedmodels.RoleAdmin && userRole != sharedmodels.RoleSuperAdmin) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only admins can initiate password reset",
			"request_id": requestID,
		})
		return
	}

	var req models.InitiatePasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for initiate password reset")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Initiate password reset
	passwordReset, err := h.passwordResetUsecase.InitiatePasswordReset(req.UserID, &adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    req.UserID,
			"error":      err.Error(),
		}).Error("Failed to initiate password reset")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to initiate password reset",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":        requestID,
		"admin_id":          adminID,
		"user_id":           req.UserID,
		"password_reset_id": passwordReset.ID,
	}).Info("Password reset initiated")

	c.JSON(http.StatusOK, gin.H{
		"message":        "Password reset initiated successfully",
		"password_reset": passwordReset,
		"request_id":     requestID,
	})
}

// ValidateResetToken handles validation of password reset token (public endpoint)
func (h *PasswordResetHandler) ValidateResetToken(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Reset token is required",
			"request_id": requestID,
		})
		return
	}

	// Validate token
	passwordReset, err := h.passwordResetUsecase.ValidateResetToken(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid or expired reset token")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":        true,
		"reset_info":   passwordReset,
		"request_id":   requestID,
	})
}

// ResetPassword handles password reset using token (public endpoint)
func (h *PasswordResetHandler) ResetPassword(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Reset token is required",
			"request_id": requestID,
		})
		return
	}

	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for reset password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Reset password
	if err := h.passwordResetUsecase.ResetPassword(token, &req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to reset password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
	}).Info("Password reset successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Password reset successfully",
		"request_id": requestID,
	})
}

// RequestPasswordReset handles self-service password reset request (public endpoint)
func (h *PasswordResetHandler) RequestPasswordReset(c *gin.Context) {
	requestID := c.GetString("request_id")
	if requestID == "" {
		requestID = uuid.New().String()
	}

	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for password reset request")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"email":      req.Email,
	}).Info("Password reset request received")

	// Request password reset by email (always returns success to prevent email enumeration)
	_ = h.passwordResetUsecase.RequestPasswordResetByEmail(req.Email)

	// Always return the same generic success message
	c.JSON(http.StatusOK, gin.H{
		"message":    "If an account with that email exists, you will receive password reset instructions",
		"request_id": requestID,
	})
}
