// File: services/poll/clients/notification_client.go
package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/logger"
)

// NotificationClient handles communication with notification service
type NotificationClient struct {
	baseURL string
	client  *http.Client
}

// NewNotificationClient creates a new notification client
func NewNotificationClient() *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tachyon-notification-service:8087"
	}

	return &NotificationClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotificationRequest represents a request to create a notification
type NotificationRequest struct {
	UserID      uint                   `json:"user_id"`
	Type        string                 `json:"type"`        // "poll", "reminder", etc.
	Title       string                 `json:"title"`
	Message     string                 `json:"message,omitempty"`
	Priority    *string                `json:"priority,omitempty"`    // Pointer to priority string
	RelatedID   *uint                  `json:"related_id,omitempty"`
	RelatedType string                 `json:"related_type,omitempty"`
	ActionURL   string                 `json:"action_url,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"` // Additional data (poll_id, etc.)
	Channels    []string               `json:"channels,omitempty"`

	// Grouping fields (for grouped notifications)
	GroupKey  string `json:"group_key,omitempty"`  // Grouping key
	TaskCount int    `json:"task_count,omitempty"` // Number of items in group
}

// notificationTaskPayload represents the worker.NotificationTask format
type notificationTaskPayload struct {
	Type         string               `json:"type"` // TaskType: "single", "bulk", etc.
	Notification *NotificationRequest `json:"notification"`
	Priority     string               `json:"priority"` // NotificationPriority for the task
}

// SendNotification sends a notification to a single user
func (c *NotificationClient) SendNotification(req *NotificationRequest) error {
	// Prepare the payload in the format expected by notification service
	payload := notificationTaskPayload{
		Type:         "single",
		Notification: req,
		Priority:     "medium", // default priority
	}

	// Override priority if specified
	if req.Priority != nil {
		payload.Priority = *req.Priority
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	// Log the notification being sent
	logger.WithFields(map[string]interface{}{
		"user_id": req.UserID,
		"type":    req.Type,
		"title":   req.Title,
		"url":     c.baseURL + "/api/v1/internal/notifications/poll",
		"payload": string(jsonData),
	}).Info("Sending notification to notification-service")

	// Send HTTP request to notification service
	url := c.baseURL + "/api/v1/internal/notifications/poll"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send notification request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	logger.WithFields(map[string]interface{}{
		"user_id": req.UserID,
		"type":    req.Type,
	}).Info("Notification sent successfully")

	return nil
}

// SendBulkNotification sends notifications to multiple users
func (c *NotificationClient) SendBulkNotification(userIDs []uint, notificationType, title, message string, priority *string, relatedID *uint, relatedType string, data map[string]interface{}, channels []string) error {
	// Send individual notifications for each user
	for _, userID := range userIDs {
		req := &NotificationRequest{
			UserID:      userID,
			Type:        notificationType,
			Title:       title,
			Message:     message,
			Priority:    priority,
			RelatedID:   relatedID,
			RelatedType: relatedType,
			Data:        data,
			Channels:    channels,
		}

		if err := c.SendNotification(req); err != nil {
			logger.WithFields(map[string]interface{}{
				"error":   err.Error(),
				"user_id": userID,
			}).Warn("Failed to send notification to user")
			// Continue sending to other users
		}
	}

	return nil
}
