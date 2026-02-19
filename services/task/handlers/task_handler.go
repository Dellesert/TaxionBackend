package handlers

import (
	"net/http"
	"strconv"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/usecase"
	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// TaskHandler handles HTTP requests for task operations
type TaskHandler struct {
	taskUsecase     usecase.TaskUsecase
	analyticsClient *analytics.Client
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(taskUsecase usecase.TaskUsecase, analyticsClient *analytics.Client) *TaskHandler {
	return &TaskHandler{
		taskUsecase:     taskUsecase,
		analyticsClient: analyticsClient,
	}
}

// CreateTask handles task creation requests
// POST /api/v1/tasks
func (h *TaskHandler) CreateTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user role from JWT token
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user department from JWT claims (if available) or user-service
	userDepartment := ""

	// For department_head role, we need to know their department
	if userRole == "department_head" {
		// TODO: Get department_id from user-service or JWT
		// For now, we'll need to query user-service
		// This is handled in the usecase layer via userClient
	}

	var req models.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create task")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.CreateTask(userID, userRole, userDepartment, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"title":      req.Title,
			"error":      err.Error(),
		}).Error("Failed to create task")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create task",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    task.ID,
		"title":      task.Title,
	}).Info("Task created successfully")

	// Send analytics event
	h.analyticsClient.SendEvent(
		analytics.EventTaskCreated,
		analytics.CategoryTask,
		uint64(userID),
		map[string]interface{}{
			"task_id":  task.ID,
			"title":    task.Title,
			"priority": task.Priority,
			"status":   task.Status,
		},
	)

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Task created successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// GetTask handles getting a single task by ID
// GET /api/v1/tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user role from JWT token
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.GetTaskByID(userID, userRole, uint(taskID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to get task")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task":       task,
		"request_id": requestID,
	})
}

// UpdateTask handles task update requests
// PUT /api/v1/tasks/:id
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user role from JWT token
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update task")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.UpdateTask(userID, userRole, uint(taskID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to update task")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update task",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
	}).Info("Task updated successfully")

	// Send analytics event
	h.analyticsClient.SendEvent(
		analytics.EventTaskUpdated,
		analytics.CategoryTask,
		uint64(userID),
		map[string]interface{}{
			"task_id": taskID,
			"status":  task.Status,
		},
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task updated successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// AssignTask handles task assignment requests
// POST /api/v1/tasks/:id/assign
func (h *TaskHandler) AssignTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AssignTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Warn("Invalid request body for assign task")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.AssignTask(userID, uint(taskID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"assignee":   req.AssignedTo,
			"error":      err.Error(),
		}).Error("Failed to assign task")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to assign task",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
		"assignee":   req.AssignedTo,
	}).Info("Task assigned successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task assigned successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// UnassignTask handles task unassignment requests
// DELETE /api/v1/tasks/:id/assign
func (h *TaskHandler) UnassignTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.UnassignTask(userID, uint(taskID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to unassign task")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to unassign task",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
	}).Info("Task unassigned successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task unassigned successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// GetTasks handles getting tasks with filtering and pagination
// GET /api/v1/tasks
// Supports incremental sync with updated_since parameter
func (h *TaskHandler) GetTasks(c *gin.Context) {
	requestID := requestid.Get(c)
	serverTime := time.Now().UTC()

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user role from JWT token
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse filter parameters
	var filter models.TaskFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid filter parameters")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid filter parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set default pagination if not provided
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	tasks, total, err := h.taskUsecase.GetUserTasks(userID, userRole, &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get tasks")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get tasks",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// If updated_since is provided, return sync-aware response format
	if filter.UpdatedSince != nil {
		// Get deleted task IDs since the timestamp
		deletedIDs, err := h.taskUsecase.GetDeletedTaskIDsSince(*filter.UpdatedSince)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":    requestID,
				"user_id":       userID,
				"updated_since": filter.UpdatedSince,
				"error":         err.Error(),
			}).Warn("Failed to get deleted task IDs, continuing without them")
			deletedIDs = []uint{}
		}

		c.JSON(http.StatusOK, models.TaskSyncListResponse{
			Tasks:      tasks,
			Total:      total,
			DeletedIDs: deletedIDs,
			ServerTime: serverTime,
			Limit:      filter.Limit,
			Offset:     filter.Offset,
		})
		return
	}

	// Default response format (backward compatible)
	c.JSON(http.StatusOK, gin.H{
		"tasks":       tasks,
		"total":       total,
		"limit":       filter.Limit,
		"offset":      filter.Offset,
		"server_time": serverTime,
		"request_id":  requestID,
	})
}

// UpdateTaskStatus handles task status update requests
// PATCH /api/v1/tasks/:id/status
func (h *TaskHandler) UpdateTaskStatus(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateTaskStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update task status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.UpdateTaskStatus(userID, uint(taskID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"status":     req.Status,
			"error":      err.Error(),
		}).Error("Failed to update task status")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update task status",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
		"status":     req.Status,
	}).Info("Task status updated successfully")

	// Send analytics event (task completed if status is "done")
	eventType := analytics.EventTaskUpdated
	if req.Status == "done" {
		eventType = analytics.EventTaskCompleted
	}
	h.analyticsClient.SendEvent(
		eventType,
		analytics.CategoryTask,
		uint64(userID),
		map[string]interface{}{
			"task_id": taskID,
			"status":  req.Status,
		},
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task status updated successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// DeleteTask handles task deletion requests
// DELETE /api/v1/tasks/:id
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get user role from JWT token
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user role from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	err = h.taskUsecase.DeleteTask(userID, userRole, uint(taskID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to delete task")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete task",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
	}).Info("Task deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task deleted successfully",
		"request_id": requestID,
	})
}

// GetTaskStats handles getting task statistics
// GET /api/v1/tasks/stats
func (h *TaskHandler) GetTaskStats(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	stats, err := h.taskUsecase.GetTaskStats(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get task stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get task stats",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// Helper functions

// containsValidationError checks if the error message contains validation-related keywords
func containsValidationError(errMsg string) bool {
	validationKeywords := []string{
		"validation failed",
		"invalid",
		"required",
		"must be",
		"cannot be empty",
		"too long",
		"too short",
	}

	for _, keyword := range validationKeywords {
		if containsKeyword(errMsg, keyword) {
			return true
		}
	}
	return false
}

// containsAccessDeniedError checks if the error message contains access denied keywords
func containsAccessDeniedError(errMsg string) bool {
	accessKeywords := []string{
		"access denied",
		"insufficient permissions",
		"unauthorized",
		"forbidden",
	}

	for _, keyword := range accessKeywords {
		if containsKeyword(errMsg, keyword) {
			return true
		}
	}
	return false
}

// containsKeyword checks if a string contains a keyword (case-insensitive)
func containsKeyword(text, keyword string) bool {
	return len(text) >= len(keyword) &&
		text[:len(keyword)] == keyword[:len(keyword)] ||
		len(text) > len(keyword) &&
			text[len(text)-len(keyword):] == keyword ||
		containsSubstring(text, keyword)
}

func containsSubstring(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}

// UpdateAssigneeStatus handles updating a group task assignee's own status
// PATCH /api/v1/tasks/:id/assignee-status
func (h *TaskHandler) UpdateAssigneeStatus(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse task ID from URL parameter
	idStr := c.Param("id")
	taskID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateAssigneeStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	task, err := h.taskUsecase.UpdateAssigneeStatus(userID, uint(taskID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"status":     req.Status,
			"error":      err.Error(),
		}).Error("Failed to update assignee status")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update assignee status",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
		"status":     req.Status,
	}).Info("Assignee status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Assignee status updated successfully",
		"task":       task,
		"request_id": requestID,
	})
}

// --- NEW HANDLERS FOR HIERARCHY, DELEGATION, AND TRACKING ---

// CreateSubtask creates a subtask under a parent task
// POST /api/v1/tasks/:id/subtasks
func (h *TaskHandler) CreateSubtask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get parent task ID from path
	parentTaskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req models.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Create subtask
	subtask, err := h.taskUsecase.CreateSubtask(userID, uint(parentTaskID), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, subtask)
}

// GetSubtasks retrieves all subtasks for a parent task
// GET /api/v1/tasks/:id/subtasks
func (h *TaskHandler) GetSubtasks(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Get subtasks
	subtasks, err := h.taskUsecase.GetSubtasks(userID, uint(taskID))
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, subtasks)
}

// GetTaskHierarchy retrieves task with full hierarchy
// GET /api/v1/tasks/:id/hierarchy
func (h *TaskHandler) GetTaskHierarchy(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Get task hierarchy
	task, err := h.taskUsecase.GetTaskHierarchy(userID, uint(taskID))
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

// DelegateTask delegates a task to another user
// POST /api/v1/tasks/:id/delegate
func (h *TaskHandler) DelegateTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req struct {
		ToUserID uint `json:"to_user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Delegate task
	task, err := h.taskUsecase.DelegateTask(userID, uint(taskID), req.ToUserID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetDelegationChain retrieves the delegation chain for a task
// GET /api/v1/tasks/:id/delegation-chain
func (h *TaskHandler) GetDelegationChain(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Get delegation chain
	chain, err := h.taskUsecase.GetDelegationChain(userID, uint(taskID))
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"delegation_chain": chain})
}

// MarkTaskAsViewed marks a task as viewed
// POST /api/v1/tasks/:id/view
func (h *TaskHandler) MarkTaskAsViewed(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Mark as viewed
	task, err := h.taskUsecase.MarkTaskAsViewed(userID, uint(taskID))
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

// UpdateTaskProgress updates task progress manually
// PATCH /api/v1/tasks/:id/progress
func (h *TaskHandler) UpdateTaskProgress(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req struct {
		Progress int `json:"progress" binding:"required,min=0,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Update progress
	task, err := h.taskUsecase.UpdateTaskProgress(userID, uint(taskID), req.Progress)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, task)
}
