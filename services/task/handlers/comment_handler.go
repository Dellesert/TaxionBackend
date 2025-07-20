package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// Comment handlers

// AddComment handles adding a comment to a task
// POST /api/v1/tasks/:id/comments
func (h *TaskHandler) AddComment(c *gin.Context) {
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

	var req models.CreateTaskCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Warn("Invalid request body for add comment")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	comment, err := h.taskUsecase.AddComment(userID, uint(taskID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to add comment")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to add comment",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"task_id":    taskID,
		"comment_id": comment.ID,
	}).Info("Comment added successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Comment added successfully",
		"comment":    comment,
		"request_id": requestID,
	})
}

// GetTaskComments handles getting comments for a task
// GET /api/v1/tasks/:id/comments
func (h *TaskHandler) GetTaskComments(c *gin.Context) {
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

	// Parse filter parameters
	var filter models.CommentFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
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

	comments, err := h.taskUsecase.GetTaskComments(userID, uint(taskID), &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"task_id":    taskID,
			"error":      err.Error(),
		}).Error("Failed to get task comments")

		statusCode := http.StatusInternalServerError
		if err.Error() == "task not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get task comments",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments":   comments.Comments,
		"total":      comments.Total,
		"limit":      comments.Limit,
		"offset":     comments.Offset,
		"request_id": requestID,
	})
}

// UpdateComment handles updating a task comment
// PUT /api/v1/comments/:id
func (h *TaskHandler) UpdateComment(c *gin.Context) {
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

	// Parse comment ID from URL parameter
	idStr := c.Param("id")
	commentID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"comment_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid comment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid comment ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateTaskCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"comment_id": commentID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update comment")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	comment, err := h.taskUsecase.UpdateComment(userID, uint(commentID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"comment_id": commentID,
			"error":      err.Error(),
		}).Error("Failed to update comment")

		statusCode := http.StatusInternalServerError
		if err.Error() == "comment not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update comment",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"comment_id": commentID,
	}).Info("Comment updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Comment updated successfully",
		"comment":    comment,
		"request_id": requestID,
	})
}

// DeleteComment handles deleting a task comment
// DELETE /api/v1/comments/:id
func (h *TaskHandler) DeleteComment(c *gin.Context) {
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

	// Parse comment ID from URL parameter
	idStr := c.Param("id")
	commentID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"comment_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid comment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid comment ID",
			"request_id": requestID,
		})
		return
	}

	err = h.taskUsecase.DeleteComment(userID, uint(commentID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"comment_id": commentID,
			"error":      err.Error(),
		}).Error("Failed to delete comment")

		statusCode := http.StatusInternalServerError
		if err.Error() == "comment not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete comment",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"comment_id": commentID,
	}).Info("Comment deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Comment deleted successfully",
		"request_id": requestID,
	})
}
