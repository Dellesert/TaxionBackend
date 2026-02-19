// File: services/chat/handlers/internal_handler.go
package handlers

import (
	"net/http"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/websocket"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// InternalHandler handles internal service-to-service requests
type InternalHandler struct {
	hub *websocket.Hub
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(hub *websocket.Hub) *InternalHandler {
	return &InternalHandler{
		hub: hub,
	}
}

// BroadcastEventRequest represents a request to broadcast a WebSocket event
type BroadcastEventRequest struct {
	Type   string      `json:"type" binding:"required"`
	UserID uint        `json:"user_id" binding:"required"`
	Data   interface{} `json:"data"`
}

// BroadcastToUser broadcasts a WebSocket event to a specific user
// POST /api/v1/internal/ws/broadcast/user
func (h *InternalHandler) BroadcastToUser(c *gin.Context) {
	requestID := requestid.Get(c)

	var req BroadcastEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request for broadcast to user")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    req.UserID,
		"type":       req.Type,
	}).Info("Broadcasting WebSocket event to user")

	// Broadcast to user via WebSocket hub
	if h.hub != nil {
		msgType := models.WSMessageType(req.Type)
		h.hub.SendToUser(req.UserID, req.Data, msgType)
	} else {
		logger.WithField("request_id", requestID).Warn("WebSocket hub is nil")
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Event broadcast to user",
		"user_id":    req.UserID,
		"type":       req.Type,
		"request_id": requestID,
	})
}

// DisconnectSessionRequest represents a request to disconnect a WebSocket by session ID
type DisconnectSessionRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Reason    string `json:"reason,omitempty"` // e.g. "session_deleted", "session_limit_exceeded"
}

// DisconnectSession disconnects the WebSocket connection associated with a session
// POST /api/v1/internal/ws/disconnect-session
func (h *InternalHandler) DisconnectSession(c *gin.Context) {
	requestID := requestid.Get(c)

	var req DisconnectSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request for disconnect session")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Default reason if not provided
	reason := req.Reason
	if reason == "" {
		reason = "session_deleted"
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"session_id": req.SessionID,
		"reason":     reason,
	}).Info("Disconnecting WebSocket for revoked session")

	found := false
	if h.hub != nil {
		found = h.hub.DisconnectBySessionID(req.SessionID, reason)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Session disconnect processed",
		"session_id":    req.SessionID,
		"reason":        reason,
		"was_connected": found,
		"request_id":    requestID,
	})
}
