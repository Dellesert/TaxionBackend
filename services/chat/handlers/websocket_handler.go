package handlers

import (
	"net/http"
	"os"
	"strings"

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
	// WebSocket может получать токен из query параметра или заголовка
	var tokenString string

	// Сначала пробуем получить из query параметра (для WebSocket)
	if token := c.Query("token"); token != "" {
		tokenString = token
	} else {
		// Если нет в query, пробуем Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenParts := strings.Split(authHeader, " ")
			if len(tokenParts) == 2 && tokenParts[0] == "Bearer" {
				tokenString = tokenParts[1]
			}
		}
	}

	if tokenString == "" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
		}).Error("No JWT token provided for WebSocket connection")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Authentication required - provide token in query parameter or Authorization header",
			"request_id": requestID,
		})
		return
	}

	// Создаем временный JWT config для валидации
	jwtConfig := middleware.DefaultJWTConfig(os.Getenv("JWT_SECRET"))

	// Валидируем токен
	claims, err := middleware.ValidateToken(tokenString, jwtConfig)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to validate JWT token for WebSocket")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Invalid or expired token",
			"request_id": requestID,
		})
		return
	}

	userID := claims.UserID

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
