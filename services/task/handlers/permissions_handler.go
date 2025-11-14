package handlers

import (
	"context"
	"net/http"
	"strconv"

	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// GetTaskPermissions returns permissions for the current user on a specific task
// GET /api/v1/tasks/:id/permissions
func (h *TaskHandler) GetTaskPermissions(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
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

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    c.Param("id"),
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Get permissions
	ctx := context.Background()
	permissions, err := h.taskUsecase.GetTaskPermissions(ctx, uint(taskID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to get task permissions")

		statusCode := http.StatusInternalServerError
		if containsKeyword(err.Error(), "not found") {
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
		"user_id":    userID,
		"task_id":    taskID,
	}).Info("Task permissions retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"permissions": permissions,
		"request_id":  requestID,
	})
}

// EmergencyCompleteTask completes a task in emergency mode
// POST /api/v1/tasks/:id/emergency-complete
func (h *TaskHandler) EmergencyCompleteTask(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context
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

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    c.Param("id"),
			"error":      err.Error(),
		}).Warn("Invalid task ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid task ID",
			"request_id": requestID,
		})
		return
	}

	// Emergency complete task
	ctx := context.Background()
	err = h.taskUsecase.EmergencyCompleteTask(ctx, uint(taskID), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to emergency complete task")

		statusCode := http.StatusInternalServerError
		if containsKeyword(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if containsKeyword(err.Error(), "only allowed") || containsKeyword(err.Error(), "overdue") {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
	}).Info("Task emergency completed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Task completed (emergency)",
		"request_id": requestID,
	})
}
