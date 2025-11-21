package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// NotificationClient handles communication with notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NotificationRequest represents a notification request to notification-service
type NotificationRequest struct {
	UserID      uint                   `json:"user_id"`
	Type        string                 `json:"type"` // "message", "chat", etc.
	Title       string                 `json:"title"`
	Message     string                 `json:"message,omitempty"`
	Priority    *string                `json:"priority,omitempty"`
	RelatedID   *uint                  `json:"related_id,omitempty"`
	RelatedType string                 `json:"related_type,omitempty"`
	ActionURL   string                 `json:"action_url,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Channels    []string               `json:"channels,omitempty"`
}

// notificationTaskPayload wraps notification in task format expected by notification worker
type notificationTaskPayload struct {
	Type         string               `json:"type"` // "single", "bulk", etc.
	Notification *NotificationRequest `json:"notification"`
	Priority     string               `json:"priority"`
}

// NewNotificationClient creates a new notification service client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service:8084"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendNotification sends a single notification
func (c *NotificationClient) SendNotification(req *NotificationRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/notifications/task", c.baseURL)

	// Default priority
	priority := "medium"
	if req.Priority != nil {
		priority = *req.Priority
	}

	// Wrap in task payload format
	taskPayload := notificationTaskPayload{
		Type:         "single",
		Notification: req,
		Priority:     priority,
	}

	payload, err := json.Marshal(taskPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call notification-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	return nil
}

// BulkNotificationRequest represents multiple notifications to be sent
type BulkNotificationRequest struct {
	Notifications []*NotificationRequest `json:"notifications"`
}

// SendBulkNotification sends multiple notifications at once
func (c *NotificationClient) SendBulkNotification(notifications []*NotificationRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/notifications/task", c.baseURL)

	// Wrap in bulk task payload
	taskPayload := map[string]interface{}{
		"type":          "bulk",
		"notifications": notifications,
		"priority":      "medium",
	}

	payload, err := json.Marshal(taskPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal notifications: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to call notification-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	return nil
}
