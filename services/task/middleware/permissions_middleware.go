package middleware

import (
	"context"
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// PermissionsMiddleware creates a middleware that checks user permissions for a task
type PermissionsMiddleware struct {
	taskUsecase usecase.TaskUsecase
}

// NewPermissionsMiddleware creates a new permissions middleware
func NewPermissionsMiddleware(taskUsecase usecase.TaskUsecase) *PermissionsMiddleware {
	return &PermissionsMiddleware{
		taskUsecase: taskUsecase,
	}
}

// RequirePermission returns a middleware that checks if user has a specific permission for a task
func (m *PermissionsMiddleware) RequirePermission(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
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
			c.Abort()
			return
		}

		// Get task ID from URL parameter
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
			c.Abort()
			return
		}

		// Check permission
		ctx := context.Background()
		hasPermission, err := m.taskUsecase.CheckPermission(ctx, uint(taskID), userID, action)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"task_id":    taskID,
				"action":     action,
				"error":      err.Error(),
			}).Error("Failed to check permission")

			statusCode := http.StatusInternalServerError
			if err.Error() == "task not found" {
				statusCode = http.StatusNotFound
			}

			c.JSON(statusCode, gin.H{
				"error":      err.Error(),
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		if !hasPermission {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"task_id":    taskID,
				"action":     action,
			}).Warn("Permission denied")

			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Permission denied",
				"details":    "You don't have permission to " + action + " this task",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// Permission granted, continue
		c.Next()
	}
}
