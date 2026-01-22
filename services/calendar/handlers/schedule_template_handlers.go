package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ScheduleTemplateHandler handles HTTP requests for template operations
type ScheduleTemplateHandler struct {
	templateUsecase usecase.ScheduleTemplateUsecase
}

// NewScheduleTemplateHandler creates a new template handler
func NewScheduleTemplateHandler(templateUsecase usecase.ScheduleTemplateUsecase) *ScheduleTemplateHandler {
	return &ScheduleTemplateHandler{
		templateUsecase: templateUsecase,
	}
}

// CreateTemplate handles template creation requests
// POST /api/v1/schedule-templates
func (h *ScheduleTemplateHandler) CreateTemplate(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateScheduleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	template, err := h.templateUsecase.CreateTemplate(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to create template")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create template",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Template created successfully",
		"template":   template,
		"request_id": requestID,
	})
}

// GetTemplates handles retrieving templates with filters
// GET /api/v1/schedule-templates
func (h *ScheduleTemplateHandler) GetTemplates(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	var filter usecase.TemplateFilterParams

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

	// Pagination
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.Limit = limit
	filter.Offset = offset

	response, err := h.templateUsecase.GetTemplates(userID, filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get templates")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get templates",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetTemplate handles retrieving a single template
// GET /api/v1/schedule-templates/:id
func (h *ScheduleTemplateHandler) GetTemplate(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	template, err := h.templateUsecase.GetTemplate(userID, uint(templateID))
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get template",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"template":   template,
		"request_id": requestID,
	})
}

// UpdateTemplate handles updating a template
// PUT /api/v1/schedule-templates/:id
func (h *ScheduleTemplateHandler) UpdateTemplate(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateScheduleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	template, err := h.templateUsecase.UpdateTemplate(userID, uint(templateID), &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update template",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template updated successfully",
		"template":   template,
		"request_id": requestID,
	})
}

// DeleteTemplate handles deleting a template
// DELETE /api/v1/schedule-templates/:id
func (h *ScheduleTemplateHandler) DeleteTemplate(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	if err := h.templateUsecase.DeleteTemplate(userID, uint(templateID)); err != nil {
		statusCode := http.StatusInternalServerError
		if containsNotFoundError(err.Error()) {
			statusCode = http.StatusNotFound
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete template",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template deleted successfully",
		"request_id": requestID,
	})
}

// AddTemplateEntry handles adding an entry or batch of entries to a template
// POST /api/v1/schedule-templates/:id/entries
// Supports both single entry and batch requests:
// - Single: { "day_of_week": 1, "start_time": "08:00", "end_time": "14:00", ... }
// - Batch: { "entries": [{ "day_of_week": 1, ... }, { "day_of_week": 3, ... }] }
func (h *ScheduleTemplateHandler) AddTemplateEntry(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	// Read body once
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Failed to read request body",
			"request_id": requestID,
		})
		return
	}

	// Try to parse as batch request first
	var batchReq models.CreateBatchTemplateEntriesRequest
	if err := json.Unmarshal(bodyBytes, &batchReq); err == nil && len(batchReq.Entries) > 0 {
		// Batch request
		var entries []*models.ScheduleTemplateEntryResponse
		for _, entryReq := range batchReq.Entries {
			entry, err := h.templateUsecase.AddTemplateEntry(userID, uint(templateID), &entryReq)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      "Failed to add template entry",
					"details":    err.Error(),
					"request_id": requestID,
				})
				return
			}
			entries = append(entries, entry)
		}

		c.JSON(http.StatusCreated, gin.H{
			"message":    "Template entries added successfully",
			"entries":    entries,
			"count":      len(entries),
			"request_id": requestID,
		})
		return
	}

	// Fallback to single entry request
	var req models.CreateTemplateEntryRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	entry, err := h.templateUsecase.AddTemplateEntry(userID, uint(templateID), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to add template entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Template entry added successfully",
		"entry":      entry,
		"request_id": requestID,
	})
}

// GetTemplateEntries handles retrieving template entries
// GET /api/v1/schedule-templates/:id/entries
func (h *ScheduleTemplateHandler) GetTemplateEntries(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	entries, err := h.templateUsecase.GetTemplateEntries(userID, uint(templateID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get template entries",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"entries":    entries,
		"request_id": requestID,
	})
}

// DeleteTemplateEntry handles deleting a template entry
// DELETE /api/v1/schedule-templates/:id/entries/:entry_id
func (h *ScheduleTemplateHandler) DeleteTemplateEntry(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
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

	if err := h.templateUsecase.DeleteTemplateEntry(userID, uint(templateID), uint(entryID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete template entry",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Template entry deleted successfully",
		"request_id": requestID,
	})
}

// ApplyTemplate handles applying a template to a schedule
// POST /api/v1/schedule-templates/:id/apply
func (h *ScheduleTemplateHandler) ApplyTemplate(c *gin.Context) {
	requestID := requestid.Get(c)

	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid template ID",
			"request_id": requestID,
		})
		return
	}

	// Get schedule ID from query or body
	scheduleIDStr := c.Query("schedule_id")
	if scheduleIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "schedule_id is required",
			"request_id": requestID,
		})
		return
	}

	scheduleID, err := strconv.ParseUint(scheduleIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid schedule_id",
			"request_id": requestID,
		})
		return
	}

	var req models.ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	count, err := h.templateUsecase.ApplyTemplate(userID, uint(templateID), uint(scheduleID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"template_id": templateID,
			"schedule_id": scheduleID,
			"error":       err.Error(),
		}).Error("Failed to apply template")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to apply template",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Template applied successfully",
		"entries_created": count,
		"request_id":      requestID,
	})
}
