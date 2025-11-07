// File: services/chat/handlers/metrics_handler.go
package handlers

import (
	"net/http"

	"tachyon-messenger/services/chat/websocket"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles internal metrics endpoints
type MetricsHandler struct {
	hub *websocket.Hub
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(hub *websocket.Hub) *MetricsHandler {
	return &MetricsHandler{
		hub: hub,
	}
}

// GetWebSocketMetrics returns WebSocket hub metrics
func (h *MetricsHandler) GetWebSocketMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching websocket metrics")

	metrics := h.hub.GetMetrics()

	response := gin.H{
		"status":            "healthy",
		"connected_clients": metrics.ConnectedClients,
		"active_chat_rooms": metrics.ActiveChatRooms,
		"messages_sent":     metrics.MessagesSent,
		"messages_received": metrics.MessagesReceived,
		"uptime":            metrics.Uptime,
	}

	c.JSON(http.StatusOK, response)
}
