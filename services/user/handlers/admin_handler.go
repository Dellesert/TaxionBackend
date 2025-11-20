package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/metrics"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// AdminHandler handles HTTP requests for admin operations
type AdminHandler struct {
	adminUsecase usecase.AdminUsecase
	userUsecase  usecase.UserUsecase
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(adminUsecase usecase.AdminUsecase, userUsecase usecase.UserUsecase) *AdminHandler {
	return &AdminHandler{
		adminUsecase: adminUsecase,
		userUsecase:  userUsecase,
	}
}

// GetUsers handles getting all users (admin only)
func (h *AdminHandler) GetUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100 // Maximum limit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get filter parameters
	status := c.Query("status")
	role := c.Query("role")
	departmentIDStr := c.Query("department_id")
	isActiveStr := c.Query("is_active")

	// Parse filters
	var departmentID *uint
	if departmentIDStr != "" {
		if id, err := strconv.ParseUint(departmentIDStr, 10, 32); err == nil {
			temp := uint(id)
			departmentID = &temp
		}
	}

	var isActive *bool
	if isActiveStr != "" {
		if isActiveStr == "true" {
			temp := true
			isActive = &temp
		} else if isActiveStr == "false" {
			temp := false
			isActive = &temp
		}
	}

	var roleFilter *string
	if role != "" {
		roleFilter = &role
	}

	// Get current user role from context
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")
		userRole = "admin" // Default to admin for admin endpoints
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"limit":         limit,
		"offset":        offset,
		"status":        status,
		"role":          role,
		"department_id": departmentID,
		"is_active":     isActive,
	}).Info("Admin getting users list with filters")

	// Use GetUsersWithFilters to support filtering
	users, total, err := h.userUsecase.GetUsersWithFilters(limit, offset, departmentID, isActive, roleFilter, string(userRole))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get users")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"count":      len(users),
		"total":      total,
	}).Info("Users retrieved successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"users":      users,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"request_id": requestID,
	})
}

// CreateUser handles user creation by admin
func (h *AdminHandler) CreateUser(c *gin.Context) {
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

	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for admin create user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Additional validation for required fields
	if strings.TrimSpace(req.Email) == "" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
		}).Warn("Email is required for user creation")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Email is required",
			"request_id": requestID,
		})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
		}).Warn("Name is required for user creation")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Name is required",
			"request_id": requestID,
		})
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
		}).Warn("Password is required for user creation")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Password is required",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.CreateUser(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to create user by admin")

		// Determine appropriate HTTP status code based on error
		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to create user"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = "User with this email already exists"
		} else if strings.Contains(err.Error(), "invalid email") ||
			strings.Contains(err.Error(), "invalid password") ||
			strings.Contains(err.Error(), "invalid role") ||
			strings.Contains(err.Error(), "invalid department") {
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
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("User created successfully by admin")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User created successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// UpdateUser handles user update by admin
func (h *AdminHandler) UpdateUser(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// First, check for protected fields in raw JSON
	var rawBody map[string]interface{}
	if err := c.ShouldBindJSON(&rawBody); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for admin update user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Check for protected fields that should use dedicated endpoints
	protectedFields := []string{"is_active", "status", "role", "email", "password", "hashed_password"}
	var foundProtectedFields []string

	for _, field := range protectedFields {
		if _, exists := rawBody[field]; exists {
			foundProtectedFields = append(foundProtectedFields, field)
		}
	}

	if len(foundProtectedFields) > 0 {
		var errorMsg string
		for _, field := range foundProtectedFields {
			switch field {
			case "is_active":
				errorMsg = "Cannot change 'is_active' through this endpoint. Use /users/:id/activate or /users/:id/deactivate instead"
			case "status":
				errorMsg = "Cannot change 'status' through this endpoint. Use /users/:id/status instead"
			case "role":
				errorMsg = "Cannot change 'role' through this endpoint. Use /users/:id/role instead"
			case "email":
				errorMsg = "Cannot change 'email' - it is used as the user identifier"
			case "password", "hashed_password":
				errorMsg = "Cannot change password through this endpoint. Use the password reset endpoint instead"
			}
		}

		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"admin_id":         adminID,
			"user_id":          id,
			"protected_fields": foundProtectedFields,
		}).Warn("Attempt to modify protected fields through update endpoint")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "Cannot modify protected fields",
			"message":          errorMsg,
			"protected_fields": foundProtectedFields,
			"request_id":       requestID,
		})
		return
	}

	// Parse into proper struct
	var req models.UpdateUserRequest
	bodyBytes, _ := json.Marshal(rawBody)
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request format",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.UpdateUser(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to update user by admin")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update user"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "validation failed") {
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
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
	}).Info("User updated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User updated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// GetUserStats handles getting user statistics (admin only)
func (h *AdminHandler) GetUserStats(c *gin.Context) {
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

	stats, err := h.adminUsecase.GetUserStats()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to get user stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get user statistics",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"stats":      stats,
	}).Info("User stats retrieved successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// UpdateUserRole handles updating user role (admin only)
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Get admin role from context
	adminRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to get admin role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Admin role not found",
			"request_id": requestID,
		})
		return
	}

	var req models.AdminUpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for update user role")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	user, err := h.adminUsecase.UpdateUserRole(uint(id), &req, adminID, adminRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"new_role":   req.Role,
			"error":      err.Error(),
		}).Error("Failed to update user role")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update user role"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "invalid role") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "cannot modify your own role") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot modify your own role"
		} else if strings.Contains(err.Error(), "cannot remove the last super admin") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot remove the last super admin from the system"
		} else if strings.Contains(err.Error(), "only super admin") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
		"new_role":   req.Role,
	}).Info("User role updated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User role updated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// UpdateUserStatus handles updating user status (admin only)
func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AdminUpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for update user status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	user, err := h.adminUsecase.UpdateUserStatus(uint(id), &req, adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"new_status": req.Status,
			"error":      err.Error(),
		}).Error("Failed to update user status")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update user status"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "invalid status") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "cannot modify your own status") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot modify your own status"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
		"new_status": req.Status,
	}).Info("User status updated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User status updated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// ActivateUser handles user activation (admin only)
func (h *AdminHandler) ActivateUser(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	user, err := h.adminUsecase.ActivateUser(uint(id), adminID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to activate user")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to activate user"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "cannot modify your own activation status") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot modify your own activation status"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
	}).Info("User activated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User activated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// DeactivateUser handles user deactivation (admin only)
func (h *AdminHandler) DeactivateUser(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Get admin role from context
	adminRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to get admin role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Admin role not found",
			"request_id": requestID,
		})
		return
	}

	user, err := h.adminUsecase.DeactivateUser(uint(id), adminID, adminRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to deactivate user")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to deactivate user"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "cannot deactivate your own account") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot deactivate your own account"
		} else if strings.Contains(err.Error(), "cannot deactivate the last active super admin") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot deactivate the last active super admin in the system"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
	}).Info("User deactivated successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User deactivated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// DeleteUser handles user deletion (admin only)
func (h *AdminHandler) DeleteUser(c *gin.Context) {
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

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	err = h.userUsecase.DeleteUser(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to delete user")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete user"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
	}).Info("User deleted successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User deleted successfully",
		"request_id": requestID,
	})
}

// UpdateUser2FA handles enabling/disabling 2FA for a user (super admin only)
func (h *AdminHandler) UpdateUser2FA(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get admin info from context
	adminID, adminIDExists := c.Get("user_id")
	adminRole, adminRoleExists := c.Get("user_role")

	if !adminIDExists || !adminRoleExists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Only super_admin can manage 2FA settings
	role, ok := adminRole.(sharedmodels.Role)
	if !ok || role != sharedmodels.RoleSuperAdmin {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"admin_role": adminRole,
		}).Warn("Non-super admin attempted to manage 2FA")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super administrators can manage 2FA settings",
			"request_id": requestID,
		})
		return
	}

	// Get user ID from path
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Bind request
	var req models.AdminUpdate2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	// Update 2FA status
	user, err := h.adminUsecase.UpdateUser2FAStatus(uint(id), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errorMessage := err.Error()

		if strings.Contains(errorMessage, "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":         requestID,
		"admin_id":           adminID,
		"admin_role":         adminRole,
		"user_id":            id,
		"two_factor_enabled": req.TwoFactorEnabled,
	}).Info("User 2FA status updated by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "2FA status updated successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// ResetUserPassword handles resetting a user's password (super admin only)
func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get admin info from context
	adminID, adminIDExists := c.Get("user_id")
	adminRole, adminRoleExists := c.Get("user_role")

	if !adminIDExists || !adminRoleExists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Only super_admin can reset passwords
	role, ok := adminRole.(sharedmodels.Role)
	if !ok || role != sharedmodels.RoleSuperAdmin {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"admin_role": adminRole,
		}).Warn("Non-super admin attempted to reset password")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Only super administrators can reset passwords",
			"request_id": requestID,
		})
		return
	}

	// Get user ID from URL parameter
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID for password reset")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for password reset")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Validate new password is not empty
	if strings.TrimSpace(req.NewPassword) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "New password is required",
			"request_id": requestID,
		})
		return
	}

	// Call usecase to reset password
	err = h.adminUsecase.ResetUserPassword(uint(id), req.NewPassword)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to reset user password")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to reset password"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "User not found"
		} else if strings.Contains(err.Error(), "password") && strings.Contains(err.Error(), "weak") {
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
		"request_id": requestID,
		"admin_id":   adminID,
		"user_id":    id,
	}).Info("User password reset successfully by admin")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Password reset successfully",
		"request_id": requestID,
	})
}

// parseCSVFile parses a CSV file and returns an array of CSVUserRow
func parseCSVFile(file interface{ Read([]byte) (int, error) }) ([]models.CSVUserRow, error) {
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least a header row and one data row")
	}

	// Parse header to get column indices
	header := records[0]
	colIndices := make(map[string]int)
	for i, col := range header {
		// Remove BOM (Byte Order Mark) and other invisible characters
		cleanCol := strings.TrimSpace(col)
		cleanCol = strings.TrimPrefix(cleanCol, "\uFEFF")       // Remove UTF-8 BOM
		cleanCol = strings.TrimPrefix(cleanCol, "\xEF\xBB\xBF") // Remove UTF-8 BOM bytes
		colIndices[strings.ToLower(cleanCol)] = i
	}

	// Validate required columns (password is now optional since users will set it via invitation)
	requiredCols := []string{"email", "name", "first_name", "last_name"}
	for _, reqCol := range requiredCols {
		if _, exists := colIndices[reqCol]; !exists {
			return nil, fmt.Errorf("missing required column: %s", reqCol)
		}
	}

	// Parse data rows
	var csvRows []models.CSVUserRow
	for i := 1; i < len(records); i++ {
		record := records[i]

		// Skip empty rows
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		row := models.CSVUserRow{}

		// Get values by column name
		if idx, ok := colIndices["email"]; ok && idx < len(record) {
			row.Email = record[idx]
		}
		if idx, ok := colIndices["name"]; ok && idx < len(record) {
			row.Name = record[idx]
		}
		if idx, ok := colIndices["first_name"]; ok && idx < len(record) {
			row.FirstName = record[idx]
		}
		if idx, ok := colIndices["last_name"]; ok && idx < len(record) {
			row.LastName = record[idx]
		}
		if idx, ok := colIndices["middle_name"]; ok && idx < len(record) {
			row.MiddleName = record[idx]
		}
		if idx, ok := colIndices["birth_date"]; ok && idx < len(record) {
			row.BirthDate = record[idx]
		}
		if idx, ok := colIndices["password"]; ok && idx < len(record) {
			row.Password = record[idx]
		}
		if idx, ok := colIndices["role"]; ok && idx < len(record) {
			row.Role = record[idx]
		}
		if idx, ok := colIndices["department_id"]; ok && idx < len(record) {
			row.DepartmentID = record[idx]
		}
		if idx, ok := colIndices["phone"]; ok && idx < len(record) {
			row.Phone = record[idx]
		}
		if idx, ok := colIndices["position"]; ok && idx < len(record) {
			row.Position = record[idx]
		}

		csvRows = append(csvRows, row)
	}

	if len(csvRows) == 0 {
		return nil, fmt.Errorf("no valid data rows found in CSV")
	}

	return csvRows, nil
}

// BulkActivateUsers handles bulk user activation (admin only)
func (h *AdminHandler) BulkActivateUsers(c *gin.Context) {
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

	// Parse request body
	var req struct {
		UserIDs []uint `json:"user_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for bulk activate users")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "No user IDs provided",
			"request_id": requestID,
		})
		return
	}

	// Activate users one by one
	var successCount, errorCount int
	var updatedUsers []*models.UserResponse
	var errors []map[string]interface{}

	for _, userID := range req.UserIDs {
		user, err := h.adminUsecase.ActivateUser(userID, adminID)
		if err != nil {
			errorCount++
			errors = append(errors, map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			})
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"admin_id":   adminID,
				"user_id":    userID,
				"error":      err.Error(),
			}).Warn("Failed to activate user in bulk operation")
		} else {
			successCount++
			updatedUsers = append(updatedUsers, user)
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"admin_id":      adminID,
		"total":         len(req.UserIDs),
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("Bulk user activation completed")

	response := gin.H{
		"success_count": successCount,
		"error_count":   errorCount,
		"updated_users": updatedUsers,
		"request_id":    requestID,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}

// BulkDeactivateUsers handles bulk user deactivation (admin only)
func (h *AdminHandler) BulkDeactivateUsers(c *gin.Context) {
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

	// Get admin role from context
	adminRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to get admin role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Admin role not found",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req struct {
		UserIDs []uint `json:"user_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for bulk deactivate users")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "No user IDs provided",
			"request_id": requestID,
		})
		return
	}

	// Deactivate users one by one
	var successCount, errorCount int
	var updatedUsers []*models.UserResponse
	var errors []map[string]interface{}

	for _, userID := range req.UserIDs {
		user, err := h.adminUsecase.DeactivateUser(userID, adminID, adminRole)
		if err != nil {
			errorCount++
			errors = append(errors, map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			})
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"admin_id":   adminID,
				"user_id":    userID,
				"error":      err.Error(),
			}).Warn("Failed to deactivate user in bulk operation")
		} else {
			successCount++
			updatedUsers = append(updatedUsers, user)
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"admin_id":      adminID,
		"total":         len(req.UserIDs),
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("Bulk user deactivation completed")

	response := gin.H{
		"success_count": successCount,
		"error_count":   errorCount,
		"updated_users": updatedUsers,
		"request_id":    requestID,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}

// BulkAssignDepartment handles bulk department assignment to users (admin only)
func (h *AdminHandler) BulkAssignDepartment(c *gin.Context) {
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

	// Parse request body
	var req struct {
		UserIDs      []uint `json:"user_ids" binding:"required"`
		DepartmentID *uint  `json:"department_id"` // Can be nil to remove department assignment
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Warn("Invalid request body for bulk assign department")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "No user IDs provided",
			"request_id": requestID,
		})
		return
	}

	// Assign department to users one by one
	var successCount, errorCount int
	var updatedUsers []*models.UserResponse
	var errors []map[string]interface{}

	for _, userID := range req.UserIDs {
		user, err := h.adminUsecase.AssignDepartmentToUser(userID, req.DepartmentID)
		if err != nil {
			errorCount++
			errors = append(errors, map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			})
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"admin_id":      adminID,
				"user_id":       userID,
				"department_id": req.DepartmentID,
				"error":         err.Error(),
			}).Warn("Failed to assign department to user in bulk operation")
		} else {
			successCount++
			updatedUsers = append(updatedUsers, user)
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"admin_id":      adminID,
		"total":         len(req.UserIDs),
		"success_count": successCount,
		"error_count":   errorCount,
		"department_id": req.DepartmentID,
	}).Info("Bulk department assignment completed")

	response := gin.H{
		"success_count": successCount,
		"error_count":   errorCount,
		"updated_users": updatedUsers,
		"request_id":    requestID,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}

// ImportUsers handles bulk user import from CSV file
func (h *AdminHandler) ImportUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	// Log request details for debugging
	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"content_type": c.ContentType(),
		"method":       c.Request.Method,
		"form_values":  c.Request.Form,
	}).Info("Import users request received")

	// Get admin ID from context
	adminID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to get admin ID from context for user import")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse multipart form
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("No file uploaded for user import")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "No file uploaded",
			"request_id": requestID,
		})
		return
	}
	defer file.Close()

	// Parse CSV
	csvRows, err := parseCSVFile(file)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to parse CSV file")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Failed to parse CSV file: " + err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Import users
	result, err := h.adminUsecase.ImportUsersFromCSV(csvRows)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"admin_id":   adminID,
			"error":      err.Error(),
		}).Error("Failed to import users from CSV")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to import users",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"admin_id":      adminID,
		"total_rows":    result.TotalRows,
		"success_count": result.SuccessCount,
		"error_count":   result.ErrorCount,
	}).Info("Users imported from CSV")

	c.JSON(http.StatusOK, result)
}

// GetAllServiceMetrics aggregates metrics from all microservices
func (h *AdminHandler) GetAllServiceMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Info("Fetching aggregated service metrics")

	// Define services to collect metrics from
	services := map[string]string{
		"user-service":         os.Getenv("USER_SERVICE_URL"),
		"analytics-service":    os.Getenv("ANALYTICS_SERVICE_URL"),
		"chat-service":         os.Getenv("CHAT_SERVICE_URL"),
		"task-service":         os.Getenv("TASK_SERVICE_URL"),
		"file-service":         os.Getenv("FILE_SERVICE_URL"),
		"calendar-service":     os.Getenv("CALENDAR_SERVICE_URL"),
		"notification-service": os.Getenv("NOTIFICATION_SERVICE_URL"),
		"poll-service":         os.Getenv("POLL_SERVICE_URL"),
	}

	// Set default URLs if not in environment
	if services["user-service"] == "" {
		services["user-service"] = "http://user-service:8081"
	}
	if services["analytics-service"] == "" {
		services["analytics-service"] = "http://analytics-service:8086"
	}
	if services["chat-service"] == "" {
		services["chat-service"] = "http://chat-service:8082"
	}
	if services["task-service"] == "" {
		services["task-service"] = "http://task-service:8083"
	}
	if services["file-service"] == "" {
		services["file-service"] = "http://file-service:8088"
	}
	if services["calendar-service"] == "" {
		services["calendar-service"] = "http://calendar-service:8084"
	}
	if services["notification-service"] == "" {
		services["notification-service"] = "http://notification-service:8085"
	}
	if services["poll-service"] == "" {
		services["poll-service"] = "http://poll-service:8087"
	}

	// Collect metrics from each service
	allMetrics := make(map[string]interface{})
	errors := make(map[string]string)

	for serviceName, baseURL := range services {
		metricsURL := fmt.Sprintf("%s/internal/metrics/runtime", baseURL)

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		// Make request
		resp, err := client.Get(metricsURL)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"url":        metricsURL,
				"error":      err.Error(),
			}).Warn("Failed to fetch metrics from service")
			errors[serviceName] = err.Error()
			continue
		}
		defer resp.Body.Close()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
			}).Warn("Failed to read metrics response")
			errors[serviceName] = err.Error()
			continue
		}

		// Parse JSON
		var serviceMetrics metrics.ServiceMetrics
		if err := json.Unmarshal(body, &serviceMetrics); err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"service":    serviceName,
				"error":      err.Error(),
			}).Warn("Failed to parse metrics JSON")
			errors[serviceName] = err.Error()
			continue
		}

		allMetrics[serviceName] = serviceMetrics
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"services_count": len(allMetrics),
		"errors_count":   len(errors),
	}).Info("Aggregated service metrics collected")

	response := gin.H{
		"services":   allMetrics,
		"timestamp":  time.Now(),
		"request_id": requestID,
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}
