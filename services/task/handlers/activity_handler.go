package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"

	"github.com/gin-gonic/gin"
)

// ActivityHandler handles HTTP requests for task activities
type ActivityHandler struct {
	activityUsecase usecase.ActivityUsecase
}

// NewActivityHandler creates a new activity handler
func NewActivityHandler(activityUsecase usecase.ActivityUsecase) *ActivityHandler {
	return &ActivityHandler{
		activityUsecase: activityUsecase,
	}
}

// GetTaskActivities retrieves activities for a task
// GET /api/v1/tasks/:id/activities
func (h *ActivityHandler) GetTaskActivities(c *gin.Context) {
	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Get pagination parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Get activities
	activities, total, err := h.activityUsecase.GetTaskActivities(uint(taskID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"activities": activities,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}
