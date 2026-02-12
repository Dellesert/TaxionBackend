package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"
)

// NotificationClient handles communication with notification-service
type NotificationClient struct {
	baseURL     string
	httpClient  *http.Client
	redisClient *redis.Client
	log         *logger.Logger
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
func NewNotificationClient(redisClient *redis.Client) *NotificationClient {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://notification-service:8084"
	}

	return &NotificationClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		redisClient: redisClient,
		log: logger.New(&logger.Config{
			Level:  "info",
			Format: "json",
		}),
	}
}

// SendNotification sends a single notification with duplicate prevention
func (c *NotificationClient) SendNotification(req *NotificationRequest) error {
	// Check for duplicate notification using Redis
	if c.shouldSkipNotification(req) {
		c.log.WithFields(map[string]interface{}{
			"user_id":      req.UserID,
			"type":         req.Type,
			"related_id":   req.RelatedID,
			"related_type": req.RelatedType,
		}).Debug("Skipping duplicate notification")
		return nil
	}

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

	// Mark notification as sent in Redis
	c.markNotificationSent(req)

	return nil
}

// BulkNotificationRequest represents multiple notifications to be sent
type BulkNotificationRequest struct {
	Notifications []*NotificationRequest `json:"notifications"`
}

// SendBulkNotification sends multiple notifications at once
func (c *NotificationClient) SendBulkNotification(notifications []*NotificationRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/notifications/task", c.baseURL)

	// Filter out duplicate notifications
	filteredNotifications := make([]*NotificationRequest, 0, len(notifications))
	for _, notif := range notifications {
		if !c.shouldSkipNotification(notif) {
			filteredNotifications = append(filteredNotifications, notif)
		}
	}

	if len(filteredNotifications) == 0 {
		c.log.Debug("All bulk notifications were duplicates, skipping")
		return nil
	}

	// Wrap in bulk task payload
	taskPayload := map[string]interface{}{
		"type":          "bulk",
		"notifications": filteredNotifications,
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

	// Mark all sent notifications in Redis
	for _, notif := range filteredNotifications {
		c.markNotificationSent(notif)
	}

	return nil
}

// shouldSkipNotification checks if we recently sent this notification
func (c *NotificationClient) shouldSkipNotification(req *NotificationRequest) bool {
	if c.redisClient == nil {
		return false // No Redis, don't skip
	}

	key := c.getNotificationKey(req)
	exists, err := c.redisClient.Exists(key)
	if err != nil {
		c.log.WithFields(map[string]interface{}{
			"key":   key,
			"error": err.Error(),
		}).Warn("Failed to check notification status in Redis")
		return false // On error, send notification anyway
	}

	return exists
}

// markNotificationSent marks a notification as sent in Redis
func (c *NotificationClient) markNotificationSent(req *NotificationRequest) {
	if c.redisClient == nil {
		return
	}

	key := c.getNotificationKey(req)

	// Set TTL based on notification type
	ttl := c.getNotificationTTL(req.Type)

	err := c.redisClient.Set(key, "1", ttl)
	if err != nil {
		c.log.WithFields(map[string]interface{}{
			"key":   key,
			"ttl":   ttl,
			"error": err.Error(),
		}).Warn("Failed to mark notification as sent in Redis")
	}
}

// getNotificationKey generates a unique Redis key for the notification
func (c *NotificationClient) getNotificationKey(req *NotificationRequest) string {
	if req.RelatedID != nil {
		// For message notifications: chat:message_notification:messageID:userID
		return fmt.Sprintf("chat:%s_notification:%d:%d", req.Type, *req.RelatedID, req.UserID)
	}
	// For notifications without related_id (chat events, etc.)
	return fmt.Sprintf("chat:%s_notification:%d:%s", req.Type, req.UserID, req.RelatedType)
}

// getNotificationTTL returns appropriate TTL based on notification type
func (c *NotificationClient) getNotificationTTL(notificationType string) time.Duration {
	switch notificationType {
	case "message":
		return 5 * time.Minute // Message notifications: 5 min to prevent spam
	case "member_added", "member_removed", "role_changed":
		return 1 * time.Hour // Member event notifications: 1 hour
	case "chat_created", "chat_deleted":
		return 24 * time.Hour // Chat lifecycle events: 24 hours
	default:
		return 10 * time.Minute // Default: 10 minutes
	}
}

// MarkNotificationsReadByChatID marks all message notifications for a chat as read
func (c *NotificationClient) MarkNotificationsReadByChatID(userID, chatID uint) error {
	url := fmt.Sprintf("%s/api/v1/internal/notifications/mark-read-by-chat", c.baseURL)

	payload, err := json.Marshal(map[string]uint{
		"user_id": userID,
		"chat_id": chatID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification-service returned status %d", resp.StatusCode)
	}

	return nil
}
