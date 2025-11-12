package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/analytics/usecase"

	"github.com/gin-gonic/gin"
)

// TasksHandler handles task analytics requests
type TasksHandler struct {
	analyticsUsecase *usecase.AnalyticsUsecase
}

// NewTasksHandler creates a new tasks handler
func NewTasksHandler(analyticsUsecase *usecase.AnalyticsUsecase) *TasksHandler {
	return &TasksHandler{analyticsUsecase: analyticsUsecase}
}

// GetStats returns task statistics
func (h *TasksHandler) GetStats(c *gin.Context) {
	// Get query parameters
	period := c.DefaultQuery("period", "week")
	departmentIDStr := c.Query("department_id")

	var departmentID *uint
	if departmentIDStr != "" {
		id, err := strconv.ParseUint(departmentIDStr, 10, 32)
		if err == nil {
			uid := uint(id)
			departmentID = &uid
		}
	}

	stats, err := h.analyticsUsecase.GetTaskStats(period, departmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetCompletionRate returns task completion rate
func (h *TasksHandler) GetCompletionRate(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	departmentIDStr := c.Query("department_id")

	var departmentID *uint
	if departmentIDStr != "" {
		id, err := strconv.ParseUint(departmentIDStr, 10, 32)
		if err == nil {
			uid := uint(id)
			departmentID = &uid
		}
	}

	rate, err := h.analyticsUsecase.GetCompletionRate(period, departmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"completion_rate": rate})
}

// GetTopPerformers returns top task performers
func (h *TasksHandler) GetTopPerformers(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	period := c.DefaultQuery("period", "week")
	departmentIDStr := c.Query("department_id")

	var departmentID *uint
	if departmentIDStr != "" {
		id, err := strconv.ParseUint(departmentIDStr, 10, 32)
		if err == nil {
			uid := uint(id)
			departmentID = &uid
		}
	}

	performers, err := h.analyticsUsecase.GetTopPerformers(limit, period, departmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": performers})
}

// GetDepartmentStats returns task statistics by department
func (h *TasksHandler) GetDepartmentStats(c *gin.Context) {
	period := c.DefaultQuery("period", "week")

	stats, err := h.analyticsUsecase.GetDepartmentTaskStats(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// GetTaskTrends returns task completion trends
func (h *TasksHandler) GetTaskTrends(c *gin.Context) {
	period := c.DefaultQuery("period", "week")
	interval := c.DefaultQuery("interval", "day")

	trends, err := h.analyticsUsecase.GetTaskTrends(period, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": trends})
}

// GetPriorityDistribution returns task distribution by priority
func (h *TasksHandler) GetPriorityDistribution(c *gin.Context) {
	period := c.DefaultQuery("period", "week")

	distribution, err := h.analyticsUsecase.GetPriorityDistribution(period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": distribution})
}
