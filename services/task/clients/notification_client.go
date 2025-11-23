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

// NotificationClient is HTTP client for notification-service
type NotificationClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewNotificationClient creates a new notification service client
func NewNotificationClient() *NotificationClient {
	notificationServiceURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationServiceURL == "" {
		notificationServiceURL = "http://notification-service:8087"
	}

	return &NotificationClient{
		baseURL: notificationServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// NotificationRequest represents a notification to be sent (matches models.CreateNotificationRequest)
type NotificationRequest struct {
	UserID      uint                   `json:"user_id"`
	Type        string                 `json:"type"` // NotificationType: "message", "task", "calendar", etc.
	Title       string                 `json:"title"`
	Message     string                 `json:"message,omitempty"`
	Priority    *string                `json:"priority,omitempty"`    // Pointer to priority string
	RelatedID   *uint                  `json:"related_id,omitempty"`
	RelatedType string                 `json:"related_type,omitempty"`
	ActionURL   string                 `json:"action_url,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"` // Дополнительные данные (task_id, chat_id, и т.д.)
	Channels    []string               `json:"channels,omitempty"`

	// Grouping fields (for task notifications)
	GroupKey  string `json:"group_key,omitempty"`  // Ключ группировки
	TaskCount int    `json:"task_count,omitempty"` // Количество задач в группе
}

// notificationTaskPayload represents the worker.NotificationTask format
type notificationTaskPayload struct {
	Type         string               `json:"type"` // TaskType: "single", "bulk", etc.
	Notification *NotificationRequest `json:"notification"`
	Priority     string               `json:"priority"` // NotificationPriority for the task
}

// SendNotification sends a notification to a user
func (c *NotificationClient) SendNotification(req *NotificationRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/notifications/task", c.baseURL)

	// Wrap the notification request in the worker task format
	priority := "medium"
	if req.Priority != nil {
		priority = *req.Priority
	}

	taskPayload := notificationTaskPayload{
		Type:         "single", // Single notification task type
		Notification: req,
		Priority:     priority,
	}

	// Marshal task payload to JSON
	payload, err := json.Marshal(taskPayload)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to marshal notification request")
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"url":     url,
		"user_id": req.UserID,
		"type":    req.Type,
		"title":   req.Title,
		"payload": string(payload), // Log the full JSON payload for debugging
	}).Info("Sending notification to notification-service")

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to create HTTP request")
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Error("Failed to send notification to notification-service")
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		logger.WithFields(map[string]interface{}{
			"status_code": resp.StatusCode,
			"user_id":     req.UserID,
		}).Warn("Notification service returned non-OK status")
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	logger.WithFields(map[string]interface{}{
		"user_id": req.UserID,
		"type":    req.Type,
	}).Info("Notification sent successfully")

	return nil
}

// SendBulkNotification sends a notification to multiple users
func (c *NotificationClient) SendBulkNotification(userIDs []uint, notificationType, title, message, priority string, relatedID *uint, relatedType, actionURL string) error {
	for _, userID := range userIDs {
		req := &NotificationRequest{
			UserID:      userID,
			Type:        notificationType,
			Title:       title,
			Message:     message,
			Priority:    &priority, // Pointer to priority
			RelatedID:   relatedID,
			RelatedType: relatedType,
			ActionURL:   actionURL,
			Channels:    []string{"in_app", "email", "push"},
		}

		// Send notification (non-blocking, continue on error)
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
