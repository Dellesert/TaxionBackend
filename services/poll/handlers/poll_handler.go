// File: services/poll/handlers/poll_handler.go
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/services/poll/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PollHandler handles HTTP requests for poll-related operations
type PollHandler struct {
	pollUsecase usecase.PollUsecase
}

// NewPollHandler creates a new poll handler
func NewPollHandler(pollUsecase usecase.PollUsecase) *PollHandler {
	return &PollHandler{
		pollUsecase: pollUsecase,
	}
}

// CreatePoll handles poll creation requests
// POST /api/v1/polls
func (h *PollHandler) CreatePoll(c *gin.Context) {
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

	var req models.CreatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create poll")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	poll, err := h.pollUsecase.CreatePoll(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to create poll")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create poll",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    poll.ID,
		"poll_title": poll.Title,
	}).Info("Poll created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Poll created successfully",
		"poll":       poll,
		"request_id": requestID,
	})
}

// GetPoll handles getting a single poll by ID
// GET /api/v1/polls/:id
func (h *PollHandler) GetPoll(c *gin.Context) {
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

	poll, err := h.pollUsecase.GetPoll(userID, uint(pollID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to get poll")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" || errors.Is(err, gorm.ErrRecordNotFound) {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get poll",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"poll":       poll,
		"request_id": requestID,
	})
}

// UpdatePoll handles poll update requests
// PUT /api/v1/polls/:id
func (h *PollHandler) UpdatePoll(c *gin.Context) {
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

	var req models.UpdatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update poll")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	poll, err := h.pollUsecase.UpdatePoll(userID, uint(pollID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to update poll")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update poll",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    pollID,
	}).Info("Poll updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Poll updated successfully",
		"poll":       poll,
		"request_id": requestID,
	})
}

// DeletePoll handles poll deletion requests
// DELETE /api/v1/polls/:id
func (h *PollHandler) DeletePoll(c *gin.Context) {
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

	err = h.pollUsecase.DeletePoll(userID, uint(pollID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to delete poll")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete poll",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    pollID,
	}).Info("Poll deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Poll deleted successfully",
		"request_id": requestID,
	})
}

// GetPolls handles getting polls with filtering and pagination
// GET /api/v1/polls
func (h *PollHandler) GetPolls(c *gin.Context) {
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
	var filter models.PollFilterRequest
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

	// Set default values
	if filter.Limit <= 0 {
		filter.Limit = models.DefaultLimit
	}
	if filter.Limit > models.MaxLimit {
		filter.Limit = models.MaxLimit
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	pollList, err := h.pollUsecase.GetPolls(userID, &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get polls")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get polls",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"polls":      pollList.Polls,
		"total":      pollList.Total,
		"limit":      pollList.Limit,
		"offset":     pollList.Offset,
		"filters":    pollList.Filters,
		"request_id": requestID,
	})
}

// SearchPolls handles poll search requests
// GET /api/v1/polls/search
func (h *PollHandler) SearchPolls(c *gin.Context) {
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

	// Get search query
	searchQuery := strings.TrimSpace(c.Query("q"))
	if searchQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Search query is required",
			"request_id": requestID,
		})
		return
	}

	// Parse filter parameters
	var filter models.PollFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"query":      searchQuery,
			"error":      err.Error(),
		}).Warn("Invalid filter parameters")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid filter parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	pollList, err := h.pollUsecase.SearchPolls(userID, searchQuery, &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"query":      searchQuery,
			"error":      err.Error(),
		}).Error("Failed to search polls")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to search polls",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"polls":      pollList.Polls,
		"total":      pollList.Total,
		"limit":      pollList.Limit,
		"offset":     pollList.Offset,
		"query":      searchQuery,
		"request_id": requestID,
	})
}

// VotePoll handles poll voting requests
// POST /api/v1/polls/:id/vote
func (h *PollHandler) VotePoll(c *gin.Context) {
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

	var req models.VotePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Warn("Invalid request body for vote poll")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	votes, err := h.pollUsecase.VotePoll(userID, uint(pollID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to vote on poll")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) || strings.Contains(err.Error(), "already voted") {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to vote on poll",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"poll_id":    pollID,
		"vote_count": len(votes),
	}).Info("Vote submitted successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Vote submitted successfully",
		"votes":      votes,
		"request_id": requestID,
	})
}

// GetPollResults handles getting poll results
// GET /api/v1/polls/:id/results
func (h *PollHandler) GetPollResults(c *gin.Context) {
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

	results, err := h.pollUsecase.GetPollResults(userID, uint(pollID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"poll_id":    pollID,
			"error":      err.Error(),
		}).Error("Failed to get poll results")

		statusCode := http.StatusInternalServerError
		if err.Error() == "poll not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get poll results",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":    results,
		"request_id": requestID,
	})
}

// Helper functions

// containsValidationError checks if error message contains validation-related keywords
func containsValidationError(errMsg string) bool {
	validationKeywords := []string{
		"validation failed",
		"invalid",
		"required",
		"too long",
		"too short",
		"not allowed",
		"already exists",
		"duplicate",
	}

	errMsgLower := strings.ToLower(errMsg)
	for _, keyword := range validationKeywords {
		if strings.Contains(errMsgLower, keyword) {
			return true
		}
	}
	return false
}

// containsAccessDeniedError checks if error message contains access-related keywords
func containsAccessDeniedError(errMsg string) bool {
	accessKeywords := []string{
		"access denied",
		"permission",
		"forbidden",
		"unauthorized",
		"not allowed",
		"insufficient",
	}

	errMsgLower := strings.ToLower(errMsg)
	for _, keyword := range accessKeywords {
		if strings.Contains(errMsgLower, keyword) {
			return true
		}
	}
	return false
}
