// File: services/poll/handlers/poll_management.go
package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// AddParticipants handles adding participants to a poll
// POST /api/v1/polls/:id/participants
func (h *PollHandler) AddParticipants(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	idStr := c.Param("id")
	pollID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AddParticipantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Warn("Invalid request body for add participants")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.pollUsecase.AddParticipants(userID, uint(pollID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"poll_id":         pollID,
			"participant_ids": req.UserIDs,
			"error":           err.Error(),
		}).Error("Failed to add participants")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to add participants",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"poll_id":         pollID,
		"participant_ids": req.UserIDs,
	}).Info("Participants added successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Participants added successfully",
		"request_id": requestID,
	})
}

// RemoveParticipant handles removing a participant from a poll
// DELETE /api/v1/polls/:id/participants/:user_id
func (h *PollHandler) RemoveParticipant(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	pollIDStr := c.Param("id")
	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollIDStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	// Parse participant user ID from URL parameter
	participantIDStr := c.Param("user_id")
	participantID, err := strconv.ParseUint(participantIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"user_id":        userID,
			"poll_id":        pollID,
			"participant_id": participantIDStr,
			"error":          err.Error(),
		}).Warn("Invalid participant ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid participant ID",
			"request_id": requestID,
		})
		return
	}

	err = h.pollUsecase.RemoveParticipant(userID, uint(pollID), uint(participantID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"user_id":        userID,
			"poll_id":        pollID,
			"participant_id": participantID,
			"error":          err.Error(),
		}).Error("Failed to remove participant")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to remove participant",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"user_id":        userID,
		"poll_id":        pollID,
		"participant_id": participantID,
	}).Info("Participant removed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Participant removed successfully",
		"request_id": requestID,
	})
}

// CreateComment handles creating a comment on a poll
// POST /api/v1/polls/:id/comments
func (h *PollHandler) CreateComment(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	idStr := c.Param("id")
	pollID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create comment")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	comment, err := h.pollUsecase.CreateComment(userID, uint(pollID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to create comment")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create comment",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    pollID,
		"comment_id": comment.ID,
	}).Info("Comment created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Comment created successfully",
		"comment":    comment,
		"request_id": requestID,
	})
}

// GetComments handles getting comments for a poll
// GET /api/v1/polls/:id/comments
func (h *PollHandler) GetComments(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	idStr := c.Param("id")
	pollID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 || limit > models.MaxLimit {
		limit = models.DefaultLimit
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	comments, total, err := h.pollUsecase.GetComments(userID, uint(pollID), limit, offset)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to get comments")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get comments",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments":   comments,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"request_id": requestID,
	})
}

// DeleteComment handles deleting a comment
// DELETE /api/v1/polls/:id/comments/:comment_id
func (h *PollHandler) DeleteComment(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	pollIDStr := c.Param("id")
	pollID, err := strconv.ParseUint(pollIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollIDStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	// Parse comment ID from URL parameter
	commentIDStr := c.Param("comment_id")
	commentID, err := strconv.ParseUint(commentIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"comment_id": commentIDStr,
			"error":      err.Error(),
		}).Warn("Invalid comment ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid comment ID",
			"request_id": requestID,
		})
		return
	}

	err = h.pollUsecase.DeleteComment(userID, uint(pollID), uint(commentID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
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
		"poll_id":    pollID,
		"comment_id": commentID,
	}).Info("Comment deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Comment deleted successfully",
		"request_id": requestID,
	})
}

// GetPollStats handles getting poll statistics
// GET /api/v1/polls/stats
func (h *PollHandler) GetPollStats(c *gin.Context) {
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

	stats, err := h.pollUsecase.GetPollStats(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get poll stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get poll stats",
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

// UpdatePollStatus handles updating poll status
// PATCH /api/v1/polls/:id/status
func (h *PollHandler) UpdatePollStatus(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	idStr := c.Param("id")
	pollID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	var req struct {
		Status models.PollStatus `json:"status" binding:"required,oneof=draft active closed archived cancelled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update poll status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.pollUsecase.UpdatePollStatus(userID, uint(pollID), req.Status)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"status":     req.Status,
			"error":      err.Error(),
		}).Error("Failed to update poll status")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update poll status",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    pollID,
		"status":     req.Status,
	}).Info("Poll status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Poll status updated successfully",
		"request_id": requestID,
	})
}

// GetMyVotes handles getting user's votes for a poll
// GET /api/v1/polls/:id/my-votes
func (h *PollHandler) GetMyVotes(c *gin.Context) {
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

	// Parse poll ID from URL parameter
	idStr := c.Param("id")
	pollID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid poll ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid poll ID",
			"request_id": requestID,
		})
		return
	}

	votes, err := h.pollUsecase.GetUserVotes(userID, uint(pollID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to get user votes")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get user votes",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"votes":      votes,
		"request_id": requestID,
	})
}
