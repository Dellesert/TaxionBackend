package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ProfileHandler handles HTTP requests for profile operations
type ProfileHandler struct {
	profileUsecase usecase.ProfileUsecase
}

// NewProfileHandler creates a new profile handler
func NewProfileHandler(profileUsecase usecase.ProfileUsecase) *ProfileHandler {
	return &ProfileHandler{
		profileUsecase: profileUsecase,
	}
}

// GetProfile handles getting user profile by ID
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"profile_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid profile ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid profile ID",
			"request_id": requestID,
		})
		return
	}

	profile, err := h.profileUsecase.GetProfile(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"profile_id": id,
			"error":      err.Error(),
		}).Error("Failed to get profile")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get profile"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"profile_id": id,
		"email":      profile.Email,
	}).Info("Profile retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"profile":    profile,
		"request_id": requestID,
	})
}

// GetMyProfile handles getting current user's profile
func (h *ProfileHandler) GetMyProfile(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	profile, err := h.profileUsecase.GetProfile(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get my profile")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get profile"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"email":      profile.Email,
	}).Info("My profile retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"profile":    profile,
		"request_id": requestID,
	})
}

// UpdateMyProfile handles updating current user's profile
func (h *ProfileHandler) UpdateMyProfile(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update profile")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	profile, err := h.profileUsecase.UpdateProfile(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to update profile")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update profile"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		} else if strings.Contains(err.Error(), "department not found") {
			statusCode = http.StatusBadRequest
			errorMessage = "Department not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"email":      profile.Email,
		"birth_date": profile.BirthDate,
	}).Info("Profile updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Profile updated successfully",
		"profile":    profile,
		"request_id": requestID,
	})
}

// ChangePassword handles changing current user's password
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for change password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.profileUsecase.ChangePassword(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to change password")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to change password"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "current password is incorrect") {
			statusCode = http.StatusBadRequest
			errorMessage = "Current password is incorrect"
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("Password changed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Password changed successfully",
		"request_id": requestID,
	})
}

// ChangeSuperAdminPassword handles changing super admin password (first login)
func (h *ProfileHandler) ChangeSuperAdminPassword(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for change super admin password")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.profileUsecase.ChangeSuperAdminPassword(userID, req.NewPassword)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to change super admin password")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to change password"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
			errorMessage = "Unauthorized: only super admin can use this endpoint"
		} else if strings.Contains(err.Error(), "validation") || strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "must be") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("Super admin password changed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Password changed successfully. You can now access all features.",
		"request_id": requestID,
	})
}

// UpdateSuperAdmin2FAStatus handles enabling/disabling 2FA for super admin
func (h *ProfileHandler) UpdateSuperAdmin2FAStatus(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token or session
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req struct {
		TwoFactorEnabled bool `json:"two_factor_enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update super admin 2FA status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Update 2FA status for the user
	user, err := h.profileUsecase.UpdateUser2FAStatus(userID, req.TwoFactorEnabled)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to update super admin 2FA status")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update 2FA status"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "forbidden") {
			statusCode = http.StatusForbidden
			errorMessage = "Unauthorized to change 2FA status"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":         requestID,
		"user_id":            userID,
		"two_factor_enabled": req.TwoFactorEnabled,
	}).Info("Super admin 2FA status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "2FA status updated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// UpdateStatus handles updating current user's status
func (h *ProfileHandler) UpdateStatus(c *gin.Context) {
	requestID := requestid.Get(c)

	// Extract user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	profile, err := h.profileUsecase.UpdateStatus(userID, req.Status)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"status":     req.Status,
			"error":      err.Error(),
		}).Error("Failed to update status")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update status"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Profile not found"
		} else if strings.Contains(err.Error(), "invalid status") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Profile is deactivated"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"new_status": req.Status,
	}).Info("Status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Status updated successfully",
		"profile":    profile,
		"request_id": requestID,
	})
}
