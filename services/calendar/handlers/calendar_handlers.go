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

// CalendarHandler handles HTTP requests for calendar operations
type CalendarHandler struct {
	calendarUsecase usecase.CalendarUsecase
}

// NewCalendarHandler creates a new calendar handler
func NewCalendarHandler(calendarUsecase usecase.CalendarUsecase) *CalendarHandler {
	return &CalendarHandler{
		calendarUsecase: calendarUsecase,
	}
}

// CreateEvent handles event creation requests
// POST /api/v1/events
func (h *CalendarHandler) CreateEvent(c *gin.Context) {
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

	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create event")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	event, err := h.calendarUsecase.CreateEvent(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"title":      req.Title,
			"error":      err.Error(),
		}).Error("Failed to create event")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		} else if containsConflictError(err.Error()) {
			statusCode = http.StatusConflict
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to create event",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"event_id":   event.ID,
		"title":      event.Title,
	}).Info("Event created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Event created successfully",
		"event":      event,
		"request_id": requestID,
	})
}

// GetEvent handles getting a single event by ID
// GET /api/v1/events/:id
func (h *CalendarHandler) GetEvent(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	event, err := h.calendarUsecase.GetEventByID(userID, uint(eventID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Error("Failed to get event")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"event":      event,
		"request_id": requestID,
	})
}

// UpdateEvent handles event update requests
// PUT /api/v1/events/:id
func (h *CalendarHandler) UpdateEvent(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update event")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	event, err := h.calendarUsecase.UpdateEvent(userID, uint(eventID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Error("Failed to update event")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		} else if containsConflictError(err.Error()) {
			statusCode = http.StatusConflict
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update event",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"event_id":   eventID,
	}).Info("Event updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Event updated successfully",
		"event":      event,
		"request_id": requestID,
	})
}

// DeleteEvent handles event deletion requests
// DELETE /api/v1/events/:id
func (h *CalendarHandler) DeleteEvent(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	err = h.calendarUsecase.DeleteEvent(userID, uint(eventID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Error("Failed to delete event")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to delete event",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"event_id":   eventID,
	}).Info("Event deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Event deleted successfully",
		"request_id": requestID,
	})
}

// GetUserEvents handles getting user's events with filtering
// GET /api/v1/events
func (h *CalendarHandler) GetUserEvents(c *gin.Context) {
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
	var filter models.EventFilterRequest
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

	// Set default pagination if not provided
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	eventList, err := h.calendarUsecase.GetUserEvents(userID, &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user events")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get events",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events":     eventList.Events,
		"total":      eventList.Total,
		"limit":      eventList.Limit,
		"offset":     eventList.Offset,
		"filters":    eventList.Filters,
		"request_id": requestID,
	})
}

// GetUserCalendar handles getting user's calendar for a date range
// GET /api/v1/calendar
func (h *CalendarHandler) GetUserCalendar(c *gin.Context) {
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

	// Parse calendar view parameters
	var calendarReq models.CalendarViewRequest
	if err := c.ShouldBindQuery(&calendarReq); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid calendar parameters")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid calendar parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	eventList, err := h.calendarUsecase.GetUserCalendar(userID, calendarReq.StartDate, calendarReq.EndDate)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"start_date": calendarReq.StartDate,
			"end_date":   calendarReq.EndDate,
			"error":      err.Error(),
		}).Error("Failed to get user calendar")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to get calendar",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events":     eventList.Events,
		"total":      eventList.Total,
		"start_date": calendarReq.StartDate,
		"end_date":   calendarReq.EndDate,
		"view_type":  calendarReq.ViewType,
		"request_id": requestID,
	})
}

// SearchEvents handles searching events
// GET /api/v1/events/search
func (h *CalendarHandler) SearchEvents(c *gin.Context) {
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
	searchQuery := c.Query("q")
	if searchQuery == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Search query parameter 'q' is required",
			"request_id": requestID,
		})
		return
	}

	// Parse filter parameters
	var filter models.EventFilterRequest
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

	eventList, err := h.calendarUsecase.SearchEvents(userID, searchQuery, &filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"query":      searchQuery,
			"error":      err.Error(),
		}).Error("Failed to search events")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to search events",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events":     eventList.Events,
		"total":      eventList.Total,
		"limit":      eventList.Limit,
		"offset":     eventList.Offset,
		"query":      searchQuery,
		"request_id": requestID,
	})
}

// InviteParticipants handles inviting participants to an event
// POST /api/v1/events/:id/participants
func (h *CalendarHandler) InviteParticipants(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AddParticipantsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Warn("Invalid request body for invite participants")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.calendarUsecase.InviteParticipants(userID, uint(eventID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"event_id":        eventID,
			"participant_ids": req.UserIDs,
			"error":           err.Error(),
		}).Error("Failed to invite participants")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to invite participants",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"event_id":        eventID,
		"participant_ids": req.UserIDs,
	}).Info("Participants invited successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Participants invited successfully",
		"request_id": requestID,
	})
}

// RemoveParticipant handles removing a participant from an event
// DELETE /api/v1/events/:id/participants/:user_id
func (h *CalendarHandler) RemoveParticipant(c *gin.Context) {
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

	// Parse event ID from URL parameter
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventIDStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	// Parse participant user ID from URL parameter
	participantIDStr := c.Param("user_id")
	participantID, err := strconv.ParseUint(participantIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"user_id":        userID,
			"event_id":       eventID,
			"participant_id": participantIDStr,
			"error":          err.Error(),
		}).Warn("Invalid participant ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid participant ID",
			"request_id": requestID,
		})
		return
	}

	err = h.calendarUsecase.RemoveParticipant(userID, uint(eventID), uint(participantID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"user_id":        userID,
			"event_id":       eventID,
			"participant_id": participantID,
			"error":          err.Error(),
		}).Error("Failed to remove participant")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to remove participant",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"user_id":        userID,
		"event_id":       eventID,
		"participant_id": participantID,
	}).Info("Participant removed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Participant removed successfully",
		"request_id": requestID,
	})
}

// UpdateParticipantStatus handles updating user's participation status
// PUT /api/v1/events/:id/status
func (h *CalendarHandler) UpdateParticipantStatus(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateParticipantStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update participant status")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.calendarUsecase.UpdateParticipantStatus(userID, uint(eventID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"status":     req.Status,
			"error":      err.Error(),
		}).Error("Failed to update participant status")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to update participant status",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"event_id":   eventID,
		"status":     req.Status,
	}).Info("Participant status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Participant status updated successfully",
		"request_id": requestID,
	})
}

// SetReminder handles setting a reminder for an event
// POST /api/v1/events/:id/reminders
func (h *CalendarHandler) SetReminder(c *gin.Context) {
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

	// Parse event ID from URL parameter
	idStr := c.Param("id")
	eventID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   idStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateReminderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Warn("Invalid request body for set reminder")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	reminder, err := h.calendarUsecase.SetReminder(userID, uint(eventID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventID,
			"error":      err.Error(),
		}).Error("Failed to set reminder")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		} else if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to set reminder",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"event_id":    eventID,
		"reminder_id": reminder.ID,
		"type":        reminder.Type,
	}).Info("Reminder set successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Reminder set successfully",
		"reminder":   reminder,
		"request_id": requestID,
	})
}

// RemoveReminder handles removing a reminder from an event
// DELETE /api/v1/events/:id/reminders/:reminder_id
func (h *CalendarHandler) RemoveReminder(c *gin.Context) {
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

	// Parse event ID from URL parameter
	eventIDStr := c.Param("id")
	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"event_id":   eventIDStr,
			"error":      err.Error(),
		}).Warn("Invalid event ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid event ID",
			"request_id": requestID,
		})
		return
	}

	// Parse reminder ID from URL parameter
	reminderIDStr := c.Param("reminder_id")
	reminderID, err := strconv.ParseUint(reminderIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"event_id":    eventID,
			"reminder_id": reminderIDStr,
			"error":       err.Error(),
		}).Warn("Invalid reminder ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid reminder ID",
			"request_id": requestID,
		})
		return
	}

	err = h.calendarUsecase.RemoveReminder(userID, uint(eventID), uint(reminderID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"event_id":    eventID,
			"reminder_id": reminderID,
			"error":       err.Error(),
		}).Error("Failed to remove reminder")

		statusCode := http.StatusInternalServerError
		if err.Error() == "event not found" {
			statusCode = http.StatusNotFound
		} else if containsAccessDeniedError(err.Error()) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to remove reminder",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"event_id":    eventID,
		"reminder_id": reminderID,
	}).Info("Reminder removed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Reminder removed successfully",
		"request_id": requestID,
	})
}

// GetEventStats handles getting event statistics
// GET /api/v1/events/stats
func (h *CalendarHandler) GetEventStats(c *gin.Context) {
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

	stats, err := h.calendarUsecase.GetEventStats(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get event stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get event stats",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// CheckTimeConflict handles checking for time conflicts
// POST /api/v1/events/check-conflict
func (h *CalendarHandler) CheckTimeConflict(c *gin.Context) {
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

	var req struct {
		StartTime      time.Time `json:"start_time" binding:"required"`
		EndTime        time.Time `json:"end_time" binding:"required"`
		ExcludeEventID *uint     `json:"exclude_event_id,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for check conflict")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	hasConflict, err := h.calendarUsecase.CheckTimeConflict(userID, req.StartTime, req.EndTime, req.ExcludeEventID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"start_time": req.StartTime,
			"end_time":   req.EndTime,
			"error":      err.Error(),
		}).Error("Failed to check time conflict")

		statusCode := http.StatusInternalServerError
		if containsValidationError(err.Error()) {
			statusCode = http.StatusBadRequest
		}

		c.JSON(statusCode, gin.H{
			"error":      "Failed to check time conflict",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_conflict": hasConflict,
		"start_time":   req.StartTime,
		"end_time":     req.EndTime,
		"request_id":   requestID,
	})
}

// Helper functions

// containsValidationError checks if the error message contains validation-related keywords
func containsValidationError(errMsg string) bool {
	validationKeywords := []string{
		"validation failed",
		"invalid",
		"required",
		"must be",
		"cannot be empty",
		"too long",
		"too short",
		"cannot be in the past",
		"must be after",
		"date range too large",
	}

	for _, keyword := range validationKeywords {
		if containsKeyword(errMsg, keyword) {
			return true
		}
	}
	return false
}

// containsAccessDeniedError checks if the error message contains access denied keywords
func containsAccessDeniedError(errMsg string) bool {
	accessKeywords := []string{
		"access denied",
		"insufficient permissions",
		"unauthorized",
		"forbidden",
		"only event creator",
		"only organizer",
	}

	for _, keyword := range accessKeywords {
		if containsKeyword(errMsg, keyword) {
			return true
		}
	}
	return false
}

// containsConflictError checks if the error message contains conflict-related keywords
func containsConflictError(errMsg string) bool {
	conflictKeywords := []string{
		"time conflict",
		"conflict detected",
		"already scheduled",
		"time overlap",
	}

	for _, keyword := range conflictKeywords {
		if containsKeyword(errMsg, keyword) {
			return true
		}
	}
	return false
}

// containsKeyword checks if a string contains a keyword (case-insensitive)
func containsKeyword(text, keyword string) bool {
	return len(text) >= len(keyword) &&
		text[:len(keyword)] == keyword[:len(keyword)] ||
		len(text) > len(keyword) &&
			text[len(text)-len(keyword):] == keyword ||
		containsSubstring(text, keyword)
}

func containsSubstring(text, substring string) bool {
	for i := 0; i <= len(text)-len(substring); i++ {
		if text[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}
