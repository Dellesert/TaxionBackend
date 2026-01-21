package handlers

import (
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

// AbsenceHandler handles HTTP requests for absence operations
type AbsenceHandler struct {
	absenceUsecase usecase.AbsenceUsecase
}

// NewAbsenceHandler creates a new absence handler
func NewAbsenceHandler(absenceUsecase usecase.AbsenceUsecase) *AbsenceHandler {
	return &AbsenceHandler{
		absenceUsecase: absenceUsecase,
	}
}

// CreateAbsence handles absence creation requests
// POST /api/v1/absences
func (h *AbsenceHandler) CreateAbsence(c *gin.Context) {
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

	var req models.CreateAbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create absence")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	absence, err := h.absenceUsecase.CreateAbsence(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"target_user": req.UserID,
			"error":       err.Error(),
		}).Error("Failed to create absence")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create absence",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"absence_id":  absence.ID,
		"target_user": absence.UserID,
		"type":        absence.Type,
	}).Info("Absence created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Absence created successfully",
		"absence":    absence,
		"request_id": requestID,
	})
}

// GetAbsence handles get absence by ID requests
// GET /api/v1/absences/:id
func (h *AbsenceHandler) GetAbsence(c *gin.Context) {
	requestID := requestid.Get(c)

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	absence, err := h.absenceUsecase.GetAbsence(uint(absenceID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"absence_id": absenceID,
			"error":      err.Error(),
		}).Error("Failed to get absence")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get absence",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"absence":    absence,
		"request_id": requestID,
	})
}

// GetAbsences handles list absences requests
// GET /api/v1/absences
func (h *AbsenceHandler) GetAbsences(c *gin.Context) {
	requestID := requestid.Get(c)

	// Parse query parameters
	filter := usecase.AbsenceFilterParams{
		Limit:  20,
		Offset: 0,
	}

	// User ID filter
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err == nil {
			uid := uint(userID)
			filter.UserID = &uid
		}
	}

	// Type filter
	if typeStr := c.Query("type"); typeStr != "" {
		absenceType := models.AbsenceType(typeStr)
		filter.Type = &absenceType
	}

	// Date range filters
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		startDate, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			filter.EndDate = &endDate
		}
	}

	// Pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	result, err := h.absenceUsecase.GetAbsences(filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get absences")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get absences",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"absences":   result.Absences,
		"total":      result.Total,
		"limit":      result.Limit,
		"offset":     result.Offset,
		"request_id": requestID,
	})
}

// GetUserAbsences handles get absences for a specific user
// GET /api/v1/users/:id/absences
func (h *AbsenceHandler) GetUserAbsences(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	// Parse date range (default to current month)
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, -1)

	if startDateStr := c.Query("start_date"); startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}

	if endDateStr := c.Query("end_date"); endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	absences, err := h.absenceUsecase.GetUserAbsences(uint(userID), startDate, endDate)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user absences")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get user absences",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"absences":   absences,
		"user_id":    userID,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
		"request_id": requestID,
	})
}

// UpdateAbsence handles absence update requests
// PUT /api/v1/absences/:id
func (h *AbsenceHandler) UpdateAbsence(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateAbsenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	absence, err := h.absenceUsecase.UpdateAbsence(userID, uint(absenceID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"absence_id": absenceID,
			"error":      err.Error(),
		}).Error("Failed to update absence")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" {
			statusCode = http.StatusNotFound
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update absence",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"absence_id": absenceID,
	}).Info("Absence updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Absence updated successfully",
		"absence":    absence,
		"request_id": requestID,
	})
}

// DeleteAbsence handles absence deletion requests
// DELETE /api/v1/absences/:id
func (h *AbsenceHandler) DeleteAbsence(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	absenceID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid absence ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.absenceUsecase.DeleteAbsence(userID, uint(absenceID)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"absence_id": absenceID,
			"error":      err.Error(),
		}).Error("Failed to delete absence")

		statusCode := http.StatusInternalServerError
		if err.Error() == "absence not found" {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete absence",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"absence_id": absenceID,
	}).Info("Absence deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Absence deleted successfully",
		"request_id": requestID,
	})
}
