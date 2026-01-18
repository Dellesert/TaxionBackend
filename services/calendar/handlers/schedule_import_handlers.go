package handlers

import (
	"net/http"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ScheduleImportHandler handles HTTP requests for schedule import
type ScheduleImportHandler struct {
	importUsecase usecase.ScheduleImportUsecase
	userClient    *clients.UserClient
}

// NewScheduleImportHandler creates a new schedule import handler
func NewScheduleImportHandler(importUsecase usecase.ScheduleImportUsecase, userClient *clients.UserClient) *ScheduleImportHandler {
	return &ScheduleImportHandler{
		importUsecase: importUsecase,
		userClient:    userClient,
	}
}

// ImportSchedule handles schedule import from Word document
// POST /api/v1/schedules/import
func (h *ScheduleImportHandler) ImportSchedule(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	var req models.ImportScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Get all users for fuzzy matching
	allUsers, err := h.userClient.GetAllUsers()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get users for matching")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get users for matching",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Check if preview mode
	if req.Preview {
		preview, err := h.importUsecase.PreviewImport(userID, &req, allUsers)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"file_id":    req.FileID,
				"error":      err.Error(),
			}).Error("Failed to preview import")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to preview import",
				"details":    err.Error(),
				"request_id": requestID,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"preview":    preview,
			"request_id": requestID,
		})
		return
	}

	// Perform actual import
	result, err := h.importUsecase.ImportSchedule(userID, &req, allUsers)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"file_id":    req.FileID,
			"error":      err.Error(),
		}).Error("Failed to import schedule")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to import schedule",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Schedule imported successfully",
		"result":     result,
		"request_id": requestID,
	})
}

// GetSupportedFormats returns information about supported import formats
// GET /api/v1/schedules/import/formats
func (h *ScheduleImportHandler) GetSupportedFormats(c *gin.Context) {
	requestID := requestid.Get(c)

	formats := []gin.H{
		{
			"name":        "Time Slots Format",
			"description": "Table with dates in first column and time slots (e.g., 10:00-14:00) as column headers. Names are listed in cells.",
			"example":     "декабрь 2025 format",
		},
		{
			"name":        "У/В Designation Format",
			"description": "Table with names in first column and dates as column headers. Cells contain 'У' (morning) and/or 'В' (evening) markers.",
			"example":     "январь 2026 format",
		},
		{
			"name":        "Calendar Grid Format",
			"description": "Calendar-style grid with dates 1-31 as columns and names as rows.",
			"example":     "Standard calendar layout",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"formats":    formats,
		"file_types": []string{".docx", ".doc"},
		"request_id": requestID,
	})
}
