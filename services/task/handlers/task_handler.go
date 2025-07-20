package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// TaskHandler handles HTTP requests for task operations
type TaskHandler struct {
	taskUsecase usecase.TaskUsecase
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(taskUsecase usecase.TaskUsecase) *TaskHandler {
	return &TaskHandler{
		taskUsecase: taskUsecase,
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

	task, err := h.taskUsecase.CreateTask(userID, &req)
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

	task, err := h.taskUsecase.GetTaskByID(userID, uint(taskID))
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

	task, err := h.taskUsecase.UpdateTask(userID, uint(taskID), &req)
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
func (h *TaskHandler) GetTasks(c *gin.Context) {
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

	tasks, total, err := h.taskUsecase.GetUserTasks(userID, &filter)
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

	c.JSON(http.StatusOK, gin.H{
		"tasks":      tasks,
		"total":      total,
		"limit":      filter.Limit,
		"offset":     filter.Offset,
		"request_id": requestID,
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

	err = h.taskUsecase.DeleteTask(userID, uint(taskID))
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
