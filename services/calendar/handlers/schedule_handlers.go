package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ScheduleHandler handles HTTP requests for schedule operations
type ScheduleHandler struct {
	scheduleUsecase usecase.ScheduleUsecase
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(scheduleUsecase usecase.ScheduleUsecase) *ScheduleHandler {
	return &ScheduleHandler{
		scheduleUsecase: scheduleUsecase,
	}
}

// CreateSchedule handles schedule creation requests
// POST /api/v1/schedules
func (h *ScheduleHandler) CreateSchedule(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID and role from JWT token
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

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Check if user can create schedules (only admin, super_admin, department_head)
	if userRole != "super_admin" && userRole != "admin" && userRole != "department_head" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_role":  userRole,
		}).Warn("User attempted to create schedule without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на создание графиков",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create schedule")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	schedule, err := h.scheduleUsecase.CreateSchedule(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"title":      req.Title,
			"error":      err.Error(),
		}).Error("Failed to create schedule")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"schedule_id": schedule.ID,
		"title":       schedule.Title,
	}).Info("Schedule created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Schedule created successfully",
		"schedule":   schedule,
		"request_id": requestID,
	})
}

// GetSchedules handles retrieving schedules with filters
// GET /api/v1/schedules
func (h *ScheduleHandler) GetSchedules(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	var filter usecase.ScheduleFilterParams

	if typeStr := c.Query("type"); typeStr != "" {
		scheduleType := models.ScheduleType(typeStr)
		filter.Type = &scheduleType
	}

	if isActiveStr := c.Query("is_active"); isActiveStr != "" {
		isActive := isActiveStr == "true"
		filter.IsActive = &isActive
	}

	if deptIDStr := c.Query("department_id"); deptIDStr != "" {
		if deptID, err := strconv.ParseUint(deptIDStr, 10, 32); err == nil {
			deptIDUint := uint(deptID)
			filter.DepartmentID = &deptIDUint
		}
	}

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.Parse("2006-01-02", endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	response, err := h.scheduleUsecase.GetSchedules(userID, userRole, filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get schedules")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get schedules",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetSchedule handles retrieving a single schedule
// GET /api/v1/schedules/:id
func (h *ScheduleHandler) GetSchedule(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check visibility permissions
	canView, err := h.scheduleUsecase.CanViewSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule visibility")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to check schedule visibility",
			"request_id": requestID,
		})
		return
	}
	if !canView {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет доступа к этому графику",
			"request_id": requestID,
		})
		return
	}

	schedule, err := h.scheduleUsecase.GetScheduleByID(userID, uint(scheduleID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to get schedule")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"schedule":   schedule,
		"request_id": requestID,
	})
}

// UpdateSchedule handles updating a schedule
// PUT /api/v1/schedules/:id
func (h *ScheduleHandler) UpdateSchedule(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check edit permission
	canEdit, err := h.scheduleUsecase.CanEditSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule edit permission")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !canEdit {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"user_role":   userRole,
			"schedule_id": scheduleID,
		}).Warn("User attempted to update schedule without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на редактирование этого графика",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	schedule, err := h.scheduleUsecase.UpdateSchedule(userID, uint(scheduleID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to update schedule")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"schedule_id": scheduleID,
	}).Info("Schedule updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Schedule updated successfully",
		"schedule":   schedule,
		"request_id": requestID,
	})
}

// DeleteSchedule handles deleting a schedule
// DELETE /api/v1/schedules/:id
func (h *ScheduleHandler) DeleteSchedule(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check edit permission (same as delete permission)
	canEdit, err := h.scheduleUsecase.CanEditSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule delete permission")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !canEdit {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"user_role":   userRole,
			"schedule_id": scheduleID,
		}).Warn("User attempted to delete schedule without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на удаление этого графика",
			"request_id": requestID,
		})
		return
	}

	if err := h.scheduleUsecase.DeleteSchedule(userID, uint(scheduleID)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to delete schedule")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"schedule_id": scheduleID,
	}).Info("Schedule deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Schedule deleted successfully",
		"request_id": requestID,
	})
}

// CreateScheduleEntry handles creating a schedule entry
// POST /api/v1/schedules/:id/entries
func (h *ScheduleHandler) CreateScheduleEntry(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check edit permission for the schedule
	canEdit, err := h.scheduleUsecase.CanEditSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule edit permission")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !canEdit {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"user_role":   userRole,
			"schedule_id": scheduleID,
		}).Warn("User attempted to create schedule entry without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на добавление записей в этот график",
			"request_id": requestID,
		})
		return
	}

	// Read raw body once to avoid EOF issue with multiple ShouldBindJSON calls
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Failed to read request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Try to parse as batch request first
	var batchReq models.BatchCreateScheduleEntriesRequest
	if err := json.Unmarshal(rawBody, &batchReq); err == nil && len(batchReq.Entries) > 0 {
		// Batch creation
		result, err := h.scheduleUsecase.CreateScheduleEntries(userID, uint(scheduleID), &batchReq)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id":  requestID,
				"user_id":     userID,
				"schedule_id": scheduleID,
				"error":       err.Error(),
			}).Error("Failed to create schedule entries")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to create schedule entries",
				"details":    err.Error(),
				"request_id": requestID,
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":    "Schedule entries created successfully",
			"entries":    result.Entries,
			"warnings":   result.Warnings,
			"skipped":    result.Skipped,
			"request_id": requestID,
		})
		return
	}

	// Single entry creation
	var req models.CreateScheduleEntryRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	result, err := h.scheduleUsecase.CreateScheduleEntry(userID, uint(scheduleID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to create schedule entry")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) || containsConflictError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !result.Created {
		// Warnings present, entry not created — return 200 with warnings for confirmation
		c.JSON(http.StatusOK, gin.H{
			"entry":      nil,
			"warnings":   result.Warnings,
			"created":    false,
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Schedule entry created successfully",
		"entry":      result.Entry,
		"warnings":   result.Warnings,
		"created":    true,
		"request_id": requestID,
	})
}

// GetScheduleEntries handles retrieving schedule entries
// GET /api/v1/schedules/:id/entries
func (h *ScheduleHandler) GetScheduleEntries(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check visibility permissions
	canView, err := h.scheduleUsecase.CanViewSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule visibility")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to check schedule visibility",
			"request_id": requestID,
		})
		return
	}
	if !canView {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет доступа к этому графику",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	var filter usecase.EntryFilterParams

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uidUint := uint(uid)
			filter.UserID = &uidUint
		}
	}

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if startDate, err := time.Parse("2006-01-02", startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if endDate, err := time.Parse("2006-01-02", endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	if shiftTypeStr := c.Query("shift_type"); shiftTypeStr != "" {
		shiftType := models.ShiftType(shiftTypeStr)
		filter.ShiftType = &shiftType
	}

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	response, err := h.scheduleUsecase.GetScheduleEntries(userID, uint(scheduleID), filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to get schedule entries")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get schedule entries",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateScheduleEntry handles updating a schedule entry
// PUT /api/v1/schedules/:id/entries/:entry_id
func (h *ScheduleHandler) UpdateScheduleEntry(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check edit permission for the schedule
	canEdit, err := h.scheduleUsecase.CanEditSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule edit permission")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !canEdit {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"user_role":   userRole,
			"schedule_id": scheduleID,
		}).Warn("User attempted to update schedule entry without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на редактирование записей этого графика",
			"request_id": requestID,
		})
		return
	}

	entryID, err := strconv.ParseUint(c.Param("entry_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid entry ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateScheduleEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	result, err := h.scheduleUsecase.UpdateScheduleEntry(userID, uint(scheduleID), uint(entryID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"entry_id":    entryID,
			"error":       err.Error(),
		}).Error("Failed to update schedule entry")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !result.Created {
		// Warnings present, entry not updated — return 200 with warnings for confirmation
		c.JSON(http.StatusOK, gin.H{
			"entry":      result.Entry,
			"warnings":   result.Warnings,
			"created":    false,
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Schedule entry updated successfully",
		"entry":      result.Entry,
		"warnings":   result.Warnings,
		"created":    true,
		"request_id": requestID,
	})
}

// DeleteScheduleEntry handles deleting a schedule entry
// DELETE /api/v1/schedules/:id/entries/:entry_id
func (h *ScheduleHandler) DeleteScheduleEntry(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	// Check edit permission for the schedule
	canEdit, err := h.scheduleUsecase.CanEditSchedule(userID, uint(scheduleID), userRole)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to check schedule edit permission")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if !canEdit {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"user_role":   userRole,
			"schedule_id": scheduleID,
		}).Warn("User attempted to delete schedule entry without permission")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "У вас нет прав на удаление записей этого графика",
			"request_id": requestID,
		})
		return
	}

	entryID, err := strconv.ParseUint(c.Param("entry_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid entry ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.scheduleUsecase.DeleteScheduleEntry(userID, uint(scheduleID), uint(entryID)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"schedule_id": scheduleID,
			"entry_id":    entryID,
			"error":       err.Error(),
		}).Error("Failed to delete schedule entry")

		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete schedule entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Schedule entry deleted successfully",
		"request_id": requestID,
	})
}

// GetMyScheduleEntries handles retrieving user's schedule entries
// GET /api/v1/schedules/my-entries
func (h *ScheduleHandler) GetMyScheduleEntries(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse date range
	startDateStr := c.DefaultQuery("start_date", time.Now().Format("2006-01-02"))
	endDateStr := c.DefaultQuery("end_date", time.Now().AddDate(0, 1, 0).Format("2006-01-02"))

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid start_date format (expected YYYY-MM-DD)",
			"request_id": requestID,
		})
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid end_date format (expected YYYY-MM-DD)",
			"request_id": requestID,
		})
		return
	}

	entries, err := h.scheduleUsecase.GetMyScheduleEntries(userID, startDate, endDate)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user schedule entries")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get schedule entries",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":    entries,
		"start_date": startDate,
		"end_date":   endDate,
		"request_id": requestID,
	})
}

// GetDailySummary handles retrieving daily schedule summary
// GET /api/v1/schedules/daily-summary?date=2024-01-15
func (h *ScheduleHandler) GetDailySummary(c *gin.Context) {
	requestID := requestid.Get(c)

	_, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse date parameter (default to today)
	dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный формат даты (ожидается YYYY-MM-DD)",
			"request_id": requestID,
		})
		return
	}

	summary, err := h.scheduleUsecase.GetDailySummary(date)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"date":       dateStr,
			"error":      err.Error(),
		}).Error("Failed to get daily summary")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить сводку за день",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":    summary,
		"request_id": requestID,
	})
}

// GetScheduleGroupMembers handles getting user group members for a schedule
func (h *ScheduleHandler) GetScheduleGroupMembers(c *gin.Context) {
	requestID := requestid.Get(c)

	scheduleIDStr := c.Param("id")
	scheduleID, err := strconv.ParseUint(scheduleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule ID",
			"request_id": requestID,
		})
		return
	}

	members, err := h.scheduleUsecase.GetScheduleGroupMembers(uint(scheduleID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to get schedule group members")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get group members",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"members":    members,
		"count":      len(members),
		"request_id": requestID,
	})
}

// Helper functions from calendar_handlers.go
func containsNotFoundError(errMsg string) bool {
	return contains(errMsg, "not found")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}
