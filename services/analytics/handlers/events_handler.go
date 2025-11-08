package handlers

import (
	"net/http"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/repository"

	"github.com/gin-gonic/gin"
)

// EventsHandler handles event-related requests
type EventsHandler struct {
	eventsRepo *repository.EventsRepository
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(eventsRepo *repository.EventsRepository) *EventsHandler {
	return &EventsHandler{
		eventsRepo: eventsRepo,
	}
}

// CreateEvent creates a new analytics event
// @Summary Create analytics event
// @Tags Analytics
// @Accept json
// @Param event body models.CreateEventRequest true "Event data"
// @Success 201 {object} models.AnalyticsEvent
// @Router /api/v1/analytics/events [post]
func (h *EventsHandler) CreateEvent(c *gin.Context) {
	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	event := req.ToModel()
	if err := h.eventsRepo.CreateEvent(event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, event)
}

// CreateEventsBatch creates multiple events at once
// @Summary Create multiple analytics events
// @Tags Analytics
// @Accept json
// @Param events body []models.CreateEventRequest true "Array of events"
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/analytics/events/batch [post]
func (h *EventsHandler) CreateEventsBatch(c *gin.Context) {
	var requests []models.CreateEventRequest
	if err := c.ShouldBindJSON(&requests); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	events := make([]*models.AnalyticsEvent, len(requests))
	for i, req := range requests {
		events[i] = req.ToModel()
	}

	if err := h.eventsRepo.CreateEventsBatch(events); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"created": len(events),
		"message": "Events created successfully",
	})
}
