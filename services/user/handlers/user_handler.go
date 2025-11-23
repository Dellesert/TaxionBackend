package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// UserHandler handles HTTP requests for user operations
type UserHandler struct {
	userUsecase usecase.UserUsecase
}

// NewUserHandler creates a new user handler
func NewUserHandler(userUsecase usecase.UserUsecase) *UserHandler {
	return &UserHandler{
		userUsecase: userUsecase,
	}
}

// CreateUser handles user creation requests
func (h *UserHandler) CreateUser(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.CreateUser(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to create user")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create user",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("User created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// GetUser handles getting a single user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.GetUser(uint(id))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to get user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// GetUsers handles getting all users with pagination
func (h *UserHandler) GetUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get current user role and department from context
	userRole, exists := c.Get("user_role")
	var currentUserRole string
	if exists {
		// Try to convert to string first
		if role, ok := userRole.(string); ok {
			currentUserRole = role
		} else if role, ok := userRole.(sharedmodels.Role); ok {
			// If it's sharedmodels.Role type, convert to string
			currentUserRole = string(role)
		}
	}

	// Get current user's department ID for filtering
	var currentUserDeptID *uint
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			// Get user to find their department
			user, err := h.userUsecase.GetUser(id)
			if err == nil && user.DepartmentID != nil {
				currentUserDeptID = user.DepartmentID
			}
		}
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Parse filter parameters
	var departmentID *uint
	if deptIDStr := c.Query("department_id"); deptIDStr != "" {
		if deptID, err := strconv.ParseUint(deptIDStr, 10, 32); err == nil {
			dept := uint(deptID)
			departmentID = &dept
		}
	}

	var isActive *bool
	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		if isActiveStr == "true" || isActiveStr == "1" {
			active := true
			isActive = &active
		} else if isActiveStr == "false" || isActiveStr == "0" {
			inactive := false
			isActive = &inactive
		}
	}

	var roleFilter *string
	if role := c.Query("role"); role != "" {
		roleFilter = &role
	}

	// Check if this is for task assignment
	forTaskAssignment := c.Query("for_task_assignment") == "true"
	if forTaskAssignment {
		departmentID = nil // Clear explicit department filter for task assignment
	}

	users, total, err := h.userUsecase.GetUsersWithFilters(limit, offset, departmentID, isActive, roleFilter, currentUserRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"limit":         limit,
			"offset":        offset,
			"department_id": departmentID,
			"error":         err.Error(),
		}).Error("Failed to get users")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users",
			"request_id": requestID,
		})
		return
	}

	// Apply task assignment filtering for department heads
	if forTaskAssignment && currentUserRole == "department_head" && currentUserDeptID != nil {
		filteredUsers := make([]*models.UserResponse, 0)
		for _, user := range users {
			// Include users from own department
			if user.DepartmentID != nil && *user.DepartmentID == *currentUserDeptID {
				filteredUsers = append(filteredUsers, user)
				continue
			}
			// Include only department heads from other departments (not admin/super_admin)
			if user.Role == "department_head" {
				filteredUsers = append(filteredUsers, user)
				continue
			}
		}
		users = filteredUsers
		total = int64(len(filteredUsers))
	}

	c.JSON(http.StatusOK, gin.H{
		"users":      users,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"request_id": requestID,
	})
}

// UpdateUser handles user update requests
func (h *UserHandler) UpdateUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
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
			"user_id":    id,
			"error":      err.Error(),
		}).Warn("Invalid request body for update user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	user, err := h.userUsecase.UpdateUser(uint(id), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to update user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    id,
	}).Info("User updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"user":       user,
		"request_id": requestID,
	})
}

// DeleteUser handles user deletion requests
func (h *UserHandler) DeleteUser(c *gin.Context) {
	requestID := requestid.Get(c)

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.userUsecase.DeleteUser(uint(id)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    id,
			"error":      err.Error(),
		}).Error("Failed to delete user")

		statusCode := http.StatusInternalServerError
		if err.Error() == "user not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    id,
	}).Info("User deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"request_id": requestID,
	})
}

// GetUsersByIDs handles getting multiple users by their IDs (internal endpoint)
func (h *UserHandler) GetUsersByIDs(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get current user role from context
	userRole, exists := c.Get("user_role")
	var currentUserRole string
	if exists {
		if role, ok := userRole.(string); ok {
			currentUserRole = role
		}
	}

	// Parse IDs from query parameter (comma-separated)
	idsStr := c.Query("ids")
	if idsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "IDs parameter is required",
			"request_id": requestID,
		})
		return
	}

	// Parse comma-separated IDs
	var ids []uint
	for _, idStr := range splitAndTrim(idsStr, ",") {
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			continue // Skip invalid IDs
		}
		ids = append(ids, uint(id))
	}

	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "No valid IDs provided",
			"request_id": requestID,
		})
		return
	}

	users, err := h.userUsecase.GetUsersByIDs(ids, currentUserRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"ids":        ids,
			"error":      err.Error(),
		}).Error("Failed to get users by IDs")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":      users,
		"request_id": requestID,
	})
}

// GetUsersByDepartment retrieves all user IDs in a department (internal endpoint)
// GET /internal/users/department/:department_id
func (h *UserHandler) GetUsersByDepartment(c *gin.Context) {
	requestID := requestid.Get(c)

	// Parse department ID from URL parameter
	departmentIDStr := c.Param("department_id")
	departmentID, err := strconv.ParseUint(departmentIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": departmentIDStr,
			"error":         err.Error(),
		}).Warn("Invalid department ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid department ID",
			"request_id": requestID,
		})
		return
	}

	// Get all users in the department
	users, err := h.userUsecase.GetUsersByDepartment(uint(departmentID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"department_id": departmentID,
			"error":         err.Error(),
		}).Error("Failed to get users by department")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get department users",
			"request_id": requestID,
		})
		return
	}

	// Extract user IDs
	userIDs := make([]uint, len(users))
	for i, user := range users {
		userIDs[i] = user.ID
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"department_id": departmentID,
		"user_count":    len(userIDs),
	}).Info("Department users retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"user_ids":   userIDs,
		"request_id": requestID,
	})
}

// splitAndTrim splits a string by separator and trims whitespace
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := []string{}
	for _, part := range splitString(s, sep) {
		trimmed := trimString(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	result := []string{}
	current := ""
	for _, ch := range s {
		if string(ch) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
