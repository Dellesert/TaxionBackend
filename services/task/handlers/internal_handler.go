package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"

	"github.com/gin-gonic/gin"
)

// InternalHandler handles internal API requests (for inter-service communication)
type InternalHandler struct {
	taskUsecase usecase.TaskUsecase
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(taskUsecase usecase.TaskUsecase) *InternalHandler {
	return &InternalHandler{
		taskUsecase: taskUsecase,
	}
}

// GetTaskForChat returns task information for chat service (no auth required)
// @Summary Get task for chat service
// @Description Gets basic task information for chat service (internal use only)
// @Tags internal
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} TaskForChatResponse
// @Failure 400 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /internal/tasks/{id} [get]
func (h *InternalHandler) GetTaskForChat(c *gin.Context) {
	// Parse task ID
	taskIDStr := c.Param("id")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Get task using internal method (no auth check)
	task, err := h.taskUsecase.GetTaskByIDInternal(uint(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Build response with only necessary fields
	response := TaskForChatResponse{
		ID:        task.ID,
		Title:     task.Title,
		CreatedBy: task.CreatedBy,
		Assignees: make([]TaskAssigneeInfo, 0),
	}

	// Add assignees if present
	if task.Assignees != nil && len(task.Assignees) > 0 {
		for _, assignee := range task.Assignees {
			response.Assignees = append(response.Assignees, TaskAssigneeInfo{
				UserID: assignee.ID,
			})
		}
	}

	c.JSON(http.StatusOK, response)
}

// TaskForChatResponse represents minimal task info for chat service
type TaskForChatResponse struct {
	ID        uint               `json:"id"`
	Title     string             `json:"title"`
	CreatedBy uint               `json:"created_by"`
	Assignees []TaskAssigneeInfo `json:"assignees"`
}

// TaskAssigneeInfo represents minimal assignee info
type TaskAssigneeInfo struct {
	UserID uint `json:"user_id"`
}
