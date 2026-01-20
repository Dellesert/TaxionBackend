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

	// Get current user's ID and department ID for filtering
	var currentUserID uint
	var currentUserDeptID *uint
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(uint); ok {
			currentUserID = id
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

	// Parse advanced filter parameters
	excludeRolesStr := c.Query("exclude_roles")     // e.g., "admin,super_admin"
	includeRolesStr := c.Query("include_roles")     // e.g., "employee,department_head"
	onlyHeads := c.Query("only_heads") == "true"    // Show only department heads
	includeOtherDeptHeads := c.Query("include_other_dept_heads") == "true"

	// Parse sorting parameters
	sortBy := c.DefaultQuery("sort_by", "created_at")           // name, email, created_at, department
	sortOrder := c.DefaultQuery("sort_order", "desc")           // asc, desc
	prioritizeMyDept := c.Query("prioritize_my_dept") == "true" // My department first
	excludeMe := c.Query("exclude_me") == "true"                // Exclude current user from results
	deptHeadFirst := c.Query("dept_head_first") == "true"       // Department head first within each department

	// Parse search parameter
	searchQuery := c.Query("search") // Search by name, email, phone, position

	// Check if this is for task assignment
	forTaskAssignment := c.Query("for_task_assignment") == "true"
	if forTaskAssignment {
		departmentID = nil // Clear explicit department filter for task assignment
	}

	// Use advanced filtering if any advanced parameters are provided
	useAdvancedFiltering := excludeRolesStr != "" || includeRolesStr != "" || onlyHeads || includeOtherDeptHeads

	var users []*models.UserResponse
	var total int64

	if useAdvancedFiltering {
		// Parse roles lists
		var excludeRoles []string
		if excludeRolesStr != "" {
			excludeRoles = splitByComma(excludeRolesStr)
		}

		var includeRoles []string
		if includeRolesStr != "" {
			includeRoles = splitByComma(includeRolesStr)
		} else if onlyHeads {
			// Only department heads
			includeRoles = []string{string(sharedmodels.RoleDepartmentHead)}
		}

		// Handle include_other_dept_heads logic
		if includeOtherDeptHeads && currentUserRole == "department_head" && currentUserDeptID != nil {
			// This logic will be handled after fetching: include own department + other dept heads
			departmentID = nil // Clear department filter to get all users
		}

		var filterErr error
		users, total, filterErr = h.userUsecase.GetUsersWithFiltersAdvanced(limit, offset, departmentID, isActive, includeRoles, excludeRoles, currentUserRole, currentUserDeptID, sortBy, sortOrder, searchQuery)
		if filterErr != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"limit":         limit,
				"offset":        offset,
				"department_id": departmentID,
				"error":         filterErr.Error(),
			}).Error("Failed to get users with advanced filters")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to get users",
				"request_id": requestID,
			})
			return
		}
	} else {
		var filterErr error
		users, total, filterErr = h.userUsecase.GetUsersWithFilters(limit, offset, departmentID, isActive, roleFilter, currentUserRole)
		if filterErr != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"limit":         limit,
				"offset":        offset,
				"department_id": departmentID,
				"error":         filterErr.Error(),
			}).Error("Failed to get users")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to get users",
				"request_id": requestID,
			})
			return
		}
	}

	// Step 1: Apply prioritize_my_dept sorting FIRST (before any filtering that could disrupt order)
	if prioritizeMyDept && currentUserDeptID != nil {
		myDeptUsers := make([]*models.UserResponse, 0)
		otherUsers := make([]*models.UserResponse, 0)

		for _, user := range users {
			if user.DepartmentID != nil && *user.DepartmentID == *currentUserDeptID {
				myDeptUsers = append(myDeptUsers, user)
			} else {
				otherUsers = append(otherUsers, user)
			}
		}

		// Concatenate: my department first, then others
		users = append(myDeptUsers, otherUsers...)
	}

	// Step 2: Apply dept_head_first sorting (maintains department grouping from step 1)
	if deptHeadFirst {
		users = sortByDepartmentHeadFirst(users)
	}

	// Step 3: Apply filtering (preserves the order established above)
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

	// Apply include_other_dept_heads filtering
	if includeOtherDeptHeads && currentUserRole == "department_head" && currentUserDeptID != nil {
		filteredUsers := make([]*models.UserResponse, 0)
		for _, user := range users {
			// Include users from own department
			if user.DepartmentID != nil && *user.DepartmentID == *currentUserDeptID {
				filteredUsers = append(filteredUsers, user)
				continue
			}
			// Include only department heads from other departments
			if user.Role == "department_head" && (user.DepartmentID == nil || *user.DepartmentID != *currentUserDeptID) {
				filteredUsers = append(filteredUsers, user)
				continue
			}
		}
		users = filteredUsers
		total = int64(len(filteredUsers))
	}

	// Step 4: Exclude current user (should be last to preserve all sorting)
	if excludeMe && currentUserID > 0 {
		filteredUsers := make([]*models.UserResponse, 0)
		for _, user := range users {
			if user.ID != currentUserID {
				filteredUsers = append(filteredUsers, user)
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

// splitByComma splits a comma-separated string and trims whitespace
func splitByComma(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := splitAndTrim(s, ",")
	return parts
}

// sortByDepartmentHeadFirst sorts users so department heads appear first within each department
// This function preserves the original order of departments (important for prioritize_my_dept)
func sortByDepartmentHeadFirst(users []*models.UserResponse) []*models.UserResponse {
	// Track department order as they appear in the input
	deptOrder := make([]uint, 0)
	deptSeen := make(map[uint]bool)
	deptMap := make(map[uint][]*models.UserResponse)
	noDeptUsers := make([]*models.UserResponse, 0)

	// Group users by department while preserving department order
	for _, user := range users {
		if user.DepartmentID == nil {
			noDeptUsers = append(noDeptUsers, user)
		} else {
			deptID := *user.DepartmentID
			if !deptSeen[deptID] {
				deptOrder = append(deptOrder, deptID)
				deptSeen[deptID] = true
			}
			deptMap[deptID] = append(deptMap[deptID], user)
		}
	}

	// Build result maintaining department order
	result := make([]*models.UserResponse, 0, len(users))

	// Process departments in the order they appeared (preserves prioritize_my_dept)
	for _, deptID := range deptOrder {
		deptUsers := deptMap[deptID]
		heads := make([]*models.UserResponse, 0)
		others := make([]*models.UserResponse, 0)

		// Separate heads and others
		for _, user := range deptUsers {
			if user.Role == "department_head" {
				heads = append(heads, user)
			} else {
				others = append(others, user)
			}
		}

		// Add heads first, then others
		result = append(result, heads...)
		result = append(result, others...)
	}

	// Add users without department at the end
	result = append(result, noDeptUsers...)

	return result
}

// GetAllUsers retrieves all users for internal service-to-service communication
// GET /internal/users/all
func (h *UserHandler) GetAllUsers(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get all users with a large limit
	users, _, err := h.userUsecase.GetUsers(10000, 0)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get all users")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"count":      len(users),
	}).Info("All users retrieved successfully (internal)")

	c.JSON(http.StatusOK, gin.H{
		"users":      users,
		"request_id": requestID,
	})
}

// ResetOnlineStatuses resets all online user statuses to offline (internal endpoint)
// POST /internal/users/reset-online-statuses
func (h *UserHandler) ResetOnlineStatuses(c *gin.Context) {
	requestID := requestid.Get(c)

	count, err := h.userUsecase.ResetAllOnlineStatuses()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to reset online statuses")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to reset online statuses",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"count":      count,
	}).Info("Successfully reset all online statuses to offline")

	c.JSON(http.StatusOK, gin.H{
		"message":     "Online statuses reset successfully",
		"users_reset": count,
		"request_id":  requestID,
	})
}

// CleanupStatuses marks users as offline if they are not in the connected users list (internal endpoint)
// POST /internal/users/cleanup-statuses
func (h *UserHandler) CleanupStatuses(c *gin.Context) {
	requestID := requestid.Get(c)

	var req struct {
		ConnectedUserIDs []uint `json:"connected_user_ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for cleanup statuses")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	count, err := h.userUsecase.CleanupDisconnectedStatuses(req.ConnectedUserIDs)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":         requestID,
			"connected_users":    len(req.ConnectedUserIDs),
			"error":              err.Error(),
		}).Error("Failed to cleanup statuses")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to cleanup statuses",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":         requestID,
		"connected_users":    len(req.ConnectedUserIDs),
		"users_set_offline":  count,
	}).Info("Successfully cleaned up disconnected user statuses")

	c.JSON(http.StatusOK, gin.H{
		"message":           "Statuses cleaned up successfully",
		"users_set_offline": count,
		"request_id":        requestID,
	})
}
