package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// InvitationHandler handles HTTP requests for invitations
type InvitationHandler struct {
	invitationUsecase usecase.InvitationUsecase
}

// NewInvitationHandler creates a new invitation handler
func NewInvitationHandler(invitationUsecase usecase.InvitationUsecase) *InvitationHandler {
	return &InvitationHandler{
		invitationUsecase: invitationUsecase,
	}
}

// CreateInvitation handles invitation creation (super_admin only)
func (h *InvitationHandler) CreateInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to get user ID for invitation creation")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_role":  userRole,
		}).Warn("Unauthorized invitation creation attempt")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can create invitations",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for invitation creation")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Create invitation
	invitation, err := h.invitationUsecase.CreateInvitation(&req, userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to create invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to create invitation"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "pending invitation") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitation.ID,
		"email":         invitation.Email,
	}).Info("Invitation created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Invitation created successfully",
		"invitation": invitation,
		"request_id": requestID,
	})
}

// ListInvitations handles listing invitations (super_admin only)
func (h *InvitationHandler) ListInvitations(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can list invitations",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	filters := make(map[string]interface{})

	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}

	if email := c.Query("email"); email != "" {
		filters["email"] = email
	}

	if role := c.Query("role"); role != "" {
		filters["role"] = role
	}

	if departmentIDStr := c.Query("department_id"); departmentIDStr != "" {
		if departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32); err == nil {
			filters["department_id"] = uint(departmentID)
		}
	}

	if isValidStr := c.Query("is_valid"); isValidStr == "true" {
		filters["is_valid"] = true
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// List invitations
	response, err := h.invitationUsecase.ListInvitations(filters, page, pageSize)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to list invitations")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to list invitations",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       response,
		"request_id": requestID,
	})
}

// GetInvitation handles getting a single invitation (super_admin only)
func (h *InvitationHandler) GetInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can view invitations",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid invitation ID",
			"request_id": requestID,
		})
		return
	}

	// Get invitation
	invitation, err := h.invitationUsecase.GetInvitation(uint(invitationID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to get invitation")

		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invitation": invitation,
		"request_id": requestID,
	})
}

// ResendInvitation handles resending an invitation (super_admin only)
func (h *InvitationHandler) ResendInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can resend invitations",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid invitation ID",
			"request_id": requestID,
		})
		return
	}

	// Resend invitation
	invitation, err := h.invitationUsecase.ResendInvitation(uint(invitationID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to resend invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to resend invitation"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "can only resend") ||
			strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitationID,
	}).Info("Invitation resent successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Invitation resent successfully",
		"invitation": invitation,
		"request_id": requestID,
	})
}

// CancelInvitation handles canceling an invitation (super_admin only)
func (h *InvitationHandler) CancelInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can cancel invitations",
			"request_id": requestID,
		})
		return
	}

	// Get invitation ID from URL
	invitationID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid invitation ID",
			"request_id": requestID,
		})
		return
	}

	// Cancel invitation
	err = h.invitationUsecase.CancelInvitation(uint(invitationID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"invitation_id": invitationID,
			"error":         err.Error(),
		}).Error("Failed to cancel invitation")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to cancel invitation"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "can only cancel") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"invitation_id": invitationID,
	}).Info("Invitation cancelled successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Invitation cancelled successfully",
		"request_id": requestID,
	})
}

// GetStats handles getting invitation statistics (super_admin only)
func (h *InvitationHandler) GetStats(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super admin can view invitation statistics",
			"request_id": requestID,
		})
		return
	}

	// Get stats
	stats, err := h.invitationUsecase.GetStats()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get invitation statistics")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get statistics",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// ValidateInvitation handles invitation validation (public endpoint)
func (h *InvitationHandler) ValidateInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Token is required",
			"request_id": requestID,
		})
		return
	}

	// Validate invitation
	invitation, err := h.invitationUsecase.ValidateInvitation(token)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid invitation validation attempt")

		statusCode := http.StatusBadRequest
		if strings.Contains(err.Error(), "expired") {
			statusCode = http.StatusGone
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"invitation": invitation,
		"request_id": requestID,
	})
}

// AcceptInvitation handles invitation acceptance (public endpoint)
func (h *InvitationHandler) AcceptInvitation(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get token from URL
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Token is required",
			"request_id": requestID,
		})
		return
	}

	var req models.AcceptInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for invitation acceptance")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// Accept invitation
	loginResponse, err := h.invitationUsecase.AcceptInvitation(token, &req, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"token":      token,
			"error":      err.Error(),
		}).Error("Failed to accept invitation")

		statusCode := http.StatusBadRequest
		errorMessage := err.Error()

		if strings.Contains(err.Error(), "expired") {
			statusCode = http.StatusGone
		} else if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "no longer valid") {
			statusCode = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
		} else {
			statusCode = http.StatusInternalServerError
			errorMessage = "Failed to accept invitation"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			int(loginResponse.Session.ExpiresAt),
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			true,  // httpOnly
		)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
	}).Info("Invitation accepted successfully, user created and logged in")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Account activated successfully",
		"user":       loginResponse.User,
		"auth_mode":  loginResponse.AuthMode,
		"session":    loginResponse.Session,
		"request_id": requestID,
	})
}
