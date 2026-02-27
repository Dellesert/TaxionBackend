package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// HolidayHandler handles HTTP requests for holiday operations
type HolidayHandler struct {
	holidayUsecase usecase.HolidayUsecase
}

// NewHolidayHandler creates a new holiday handler
func NewHolidayHandler(holidayUsecase usecase.HolidayUsecase) *HolidayHandler {
	return &HolidayHandler{
		holidayUsecase: holidayUsecase,
	}
}

// GetHolidays handles GET /api/v1/calendar/holidays?year=2026
func (h *HolidayHandler) GetHolidays(c *gin.Context) {
	requestID := requestid.Get(c)

	yearStr := c.Query("year")
	if yearStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Параметр year обязателен",
			"request_id": requestID,
		})
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверный параметр year",
			"request_id": requestID,
		})
		return
	}

	result, err := h.holidayUsecase.GetHolidays(year)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"year":       year,
			"error":      err.Error(),
		}).Error("Failed to get holidays")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Не удалось получить праздники",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"year":       result.Year,
		"holidays":   result.Holidays,
		"request_id": requestID,
	})
}
