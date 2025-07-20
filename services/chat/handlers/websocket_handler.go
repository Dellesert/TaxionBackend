package handlers

import (
	"net/http"

	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/services/chat/websocket"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	gorilla_websocket "github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket HTTP connections
type WebSocketHandler struct {
	hub            *websocket.Hub
	messageUsecase usecase.MessageUsecase
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *websocket.Hub, messageUsecase usecase.MessageUsecase) *WebSocketHandler {
	return &WebSocketHandler{
		hub:            hub,
		messageUsecase: messageUsecase,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
// Route: /ws
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	requestID := requestid.Get(c)

	// Authenticate user via JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to authenticate WebSocket user")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Authentication required",
			"request_id": requestID,
		})
		return
	}

	// Configure WebSocket upgrader
	upgrader := gorilla_websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: In production, implement proper origin checking
			// For now, allow all origins for development
			return true
		},
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to upgrade WebSocket connection")
		return
	}

	// Create new WebSocket client
	client := websocket.NewClient(conn, h.hub, userID)

	// Add client to hub
	h.hub.RegisterClient(client)

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("WebSocket client connected and registered")

	// Start client message pumps in separate goroutines
	// WritePump handles sending messages to client
	go client.WritePump()

	// ReadPump handles receiving messages from client
	// This will block until connection is closed
	go client.ReadPump()
}
