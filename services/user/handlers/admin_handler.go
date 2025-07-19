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
	departmentID := c.Query("department_id")
	isActive := c.Query("is_active")

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"limit":         limit,
		"offset":        offset,
		"status":        status,
		"role":          role,
		"department_id": departmentID,
		"is_active":     isActive,
	}).Info("Admin getting users list")

	users, total, err := h.userUsecase.GetUsers(limit, offset)
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

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

	user, err := h.adminUsecase.UpdateUserRole(uint(id), &req)
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

	user, err := h.adminUsecase.UpdateUserStatus(uint(id), &req)
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

	user, err := h.adminUsecase.ActivateUser(uint(id))
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

	user, err := h.adminUsecase.DeactivateUser(uint(id))
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
