package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// DashboardHandler handles HTTP requests for dashboard-related operations
type DashboardHandler struct {
	dashboardUsecase usecase.DashboardUsecase
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(dashboardUsecase usecase.DashboardUsecase) *DashboardHandler {
	return &DashboardHandler{
		dashboardUsecase: dashboardUsecase,
	}
}

// GetDashboard handles GET /api/v1/dashboard
// Returns aggregated data for user's dashboard
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"error":      "Не авторизован",
			"request_id": requestID,
		})
		return
	}

	// Parse limit parameter with default value
	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Get dashboard data
	dashboard, err := h.dashboardUsecase.GetDashboard(userID, limit)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get dashboard")

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":    false,
			"error":      "Не удалось получить дашборд",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":          requestID,
		"user_id":             userID,
		"new_tasks_count":     len(dashboard.NewTasks),
		"active_tasks_count":  len(dashboard.ActiveTasks),
		"overdue_tasks_count": len(dashboard.OverdueTasks),
		"pending_polls_count": len(dashboard.PendingPolls),
	}).Info("Successfully retrieved dashboard")

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"data":       dashboard,
		"request_id": requestID,
	})
}
