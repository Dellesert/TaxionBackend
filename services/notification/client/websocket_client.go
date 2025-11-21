// File: services/notification/client/websocket_client.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// WebSocketClient handles sending WebSocket events via chat-service
type WebSocketClient struct {
	baseURL    string
	httpClient *http.Client
}

// WebSocketEventRequest represents a request to broadcast a WebSocket event
type WebSocketEventRequest struct {
	Type   string      `json:"type"`
	UserID uint        `json:"user_id"`
	Data   interface{} `json:"data"`
}

// NewWebSocketClient creates a new WebSocket client
func NewWebSocketClient() *WebSocketClient {
	chatServiceURL := os.Getenv("CHAT_SERVICE_URL")
	if chatServiceURL == "" {
		chatServiceURL = "http://chat-service:8082"
	}

	return &WebSocketClient{
		baseURL: chatServiceURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// BroadcastToUser sends a WebSocket event to a specific user
func (c *WebSocketClient) BroadcastToUser(userID uint, eventType string, data interface{}) error {
	event := WebSocketEventRequest{
		Type:   eventType,
		UserID: userID,
		Data:   data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal WebSocket event: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/internal/ws/broadcast/user", c.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send WebSocket event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
