package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SessionHandler handles HTTP requests for session management
type SessionHandler struct {
	analyticsClient *analytics.Client
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(analyticsClient *analytics.Client) *SessionHandler {
	return &SessionHandler{
		analyticsClient: analyticsClient,
	}
}

// notifyWebSocketDisconnect sends an async request to chat-service to disconnect
// the WebSocket connection associated with the given sessionID.
func notifyWebSocketDisconnect(sessionID string) {
	go func() {
		chatServiceURL := os.Getenv("CHAT_SERVICE_URL")
		if chatServiceURL == "" {
			chatServiceURL = "http://chat-service:8082"
		}

		payload, _ := json.Marshal(map[string]string{
			"session_id": sessionID,
		})

		url := fmt.Sprintf("%s/api/v1/internal/ws/disconnect-session", chatServiceURL)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			}).Warn("Failed to create WebSocket disconnect request")
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			}).Warn("Failed to notify chat-service about session disconnect")
			return
		}
		defer resp.Body.Close()

		logger.WithFields(map[string]interface{}{
			"session_id":  sessionID,
			"status_code": resp.StatusCode,
		}).Info("Notified chat-service to disconnect WebSocket for session")
	}()
}

// GetActiveSessions returns all active sessions for the current user
// GET /api/v1/sessions
func (h *SessionHandler) GetActiveSessions(c *gin.Context) {
	requestID := requestid.Get(c)
	userID := c.GetUint("user_id")

	authConfig := middleware.GetAuthConfig()
	if authConfig == nil || authConfig.SessionStore == nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
		}).Error("Session store not configured")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Session management not available",
			"request_id": requestID,
		})
		return
	}

	sessions, err := authConfig.SessionStore.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user sessions")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get active sessions",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"session_count": len(sessions),
	}).Info("Retrieved active sessions")

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}

// DeleteSession terminates a specific session
// DELETE /api/v1/sessions/:session_id
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	requestID := requestid.Get(c)
	userID := c.GetUint("user_id")
	sessionID := c.Param("session_id")

	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Session ID is required",
			"request_id": requestID,
		})
		return
	}

	authConfig := middleware.GetAuthConfig()
	if authConfig == nil || authConfig.SessionStore == nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
		}).Error("Session store not configured")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Session management not available",
			"request_id": requestID,
		})
		return
	}

	// Verify the session belongs to the user before deleting
	session, err := authConfig.SessionStore.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"session_id": sessionID,
			"error":      err.Error(),
		}).Warn("Session not found or already deleted")

		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Session not found",
			"request_id": requestID,
		})
		return
	}

	// Check if the session belongs to the current user
	if session.UserID != userID {
		logger.WithFields(map[string]interface{}{
			"request_id":       requestID,
			"user_id":          userID,
			"session_id":       sessionID,
			"session_owner_id": session.UserID,
		}).Warn("User attempted to delete another user's session")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "You can only delete your own sessions",
			"request_id": requestID,
		})
		return
	}

	// Delete the session
	if err := authConfig.SessionStore.DeleteSession(c.Request.Context(), sessionID); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"session_id": sessionID,
			"error":      err.Error(),
		}).Error("Failed to delete session")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete session",
			"request_id": requestID,
		})
		return
	}

	// Disconnect WebSocket connection for this session
	notifyWebSocketDisconnect(sessionID)

	// Deactivate session in analytics
	if h.analyticsClient != nil {
		h.analyticsClient.DeactivateSessionAsync(sessionID)
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"session_id": sessionID,
	}).Info("Session deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session deleted successfully",
		"request_id": requestID,
	})
}

// DeleteAllSessions terminates all sessions except the current one
// DELETE /api/v1/sessions
func (h *SessionHandler) DeleteAllSessions(c *gin.Context) {
	requestID := requestid.Get(c)
	userID := c.GetUint("user_id")

	// Get current session ID from header
	currentSessionID := c.GetHeader("X-Session-ID")
	if currentSessionID == "" {
		// Try to get from cookie
		currentSessionID, _ = c.Cookie("session_id")
	}

	authConfig := middleware.GetAuthConfig()
	if authConfig == nil || authConfig.SessionStore == nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
		}).Error("Session store not configured")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Session management not available",
			"request_id": requestID,
		})
		return
	}

	// Get all user sessions
	sessions, err := authConfig.SessionStore.GetUserSessions(c.Request.Context(), userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user sessions")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete sessions",
			"request_id": requestID,
		})
		return
	}

	// Delete all sessions except the current one
	deletedCount := 0
	for _, session := range sessions {
		if session.SessionID != currentSessionID {
			if err := authConfig.SessionStore.DeleteSession(c.Request.Context(), session.SessionID); err != nil {
				logger.WithFields(map[string]interface{}{
					"request_id": requestID,
					"user_id":    userID,
					"session_id": session.SessionID,
					"error":      err.Error(),
				}).Warn("Failed to delete session")
				continue
			}

			// Disconnect WebSocket connection for this session
			notifyWebSocketDisconnect(session.SessionID)

			// Deactivate session in analytics
			if h.analyticsClient != nil {
				h.analyticsClient.DeactivateSessionAsync(session.SessionID)
			}

			deletedCount++
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"deleted_count": deletedCount,
	}).Info("Deleted other sessions")

	c.JSON(http.StatusOK, gin.H{
		"message":       "All other sessions deleted successfully",
		"deleted_count": deletedCount,
		"request_id":    requestID,
	})
}

// TerminateSessionInternal terminates any session (for internal/admin use)
// DELETE /internal/sessions/:session_id
func (h *SessionHandler) TerminateSessionInternal(c *gin.Context) {
	requestID := requestid.Get(c)
	sessionID := c.Param("session_id")

	if sessionID == "" {
		logger.WithField("request_id", requestID).Warn("Session ID is required")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Session ID is required",
			"request_id": requestID,
		})
		return
	}

	authConfig := middleware.GetAuthConfig()
	if authConfig == nil || authConfig.SessionStore == nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"session_id": sessionID,
		}).Error("Session store not configured")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Session management not available",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"request_id": requestID,
	}).Info("Terminating session (internal)")

	// Delete the session from Redis
	if err := authConfig.SessionStore.DeleteSession(c.Request.Context(), sessionID); err != nil {
		logger.WithFields(map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
			"request_id": requestID,
		}).Error("Failed to delete session from Redis")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to terminate session",
			"request_id": requestID,
		})
		return
	}

	// Disconnect WebSocket connection for this session
	notifyWebSocketDisconnect(sessionID)

	// Deactivate session in analytics (async)
	go func() {
		if err := h.analyticsClient.DeactivateSession(sessionID); err != nil {
			logger.WithFields(map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			}).Warn("Failed to deactivate session in analytics")
		}
	}()

	logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"request_id": requestID,
	}).Info("Session terminated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session terminated successfully",
		"request_id": requestID,
	})
}
