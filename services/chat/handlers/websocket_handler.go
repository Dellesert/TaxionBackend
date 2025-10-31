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

	// Authenticate user via session_id (session mode) or JWT token (JWT mode)
	var userID uint
	var err error

	// Try session_id first (session mode)
	sessionID := c.Query("session_id")
	if sessionID != "" {
		// Session mode authentication
		authConfig := middleware.GetAuthConfig()
		if authConfig == nil || authConfig.SessionStore == nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
			}).Error("Session store not configured for WebSocket")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Session authentication not configured",
				"request_id": requestID,
			})
			return
		}

		session, err := authConfig.SessionStore.GetSession(c.Request.Context(), sessionID)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
			}).Error("Failed to validate session for WebSocket")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Invalid or expired session",
				"request_id": requestID,
			})
			return
		}

		userID = session.UserID
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"auth_mode":  "session",
		}).Info("WebSocket authenticated via session")
	} else {
		// Fallback to JWT mode
		var tokenString string

		// Try to get token from query parameter (for WebSocket)
		if token := c.Query("token"); token != "" {
			tokenString = token
		} else {
			// If not in query, try Authorization header
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
			}).Error("No authentication provided for WebSocket connection")

			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authentication required - provide session_id or token in query parameter",
				"request_id": requestID,
			})
			return
		}

		// Create temporary JWT config for validation
		jwtConfig := middleware.DefaultJWTConfig(os.Getenv("JWT_SECRET"))

		// Validate token
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

		userID = claims.UserID
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"auth_mode":  "jwt",
		}).Info("WebSocket authenticated via JWT")
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
