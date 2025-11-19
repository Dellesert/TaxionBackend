// File: services/notification/push/fcm_provider.go
package push

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/shared/logger"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// FCMProvider implements PushProvider for Firebase Cloud Messaging
type FCMProvider struct {
	client *messaging.Client
	config *PushConfig
}

// NewFCMProvider creates a new FCM push notification provider
func NewFCMProvider(config *PushConfig) (PushProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if config.CredentialsFile == "" {
		return nil, fmt.Errorf("credentials_file is required for FCM")
	}

	// Initialize Firebase app
	opt := option.WithCredentialsFile(config.CredentialsFile)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Get messaging client
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get messaging client: %w", err)
	}

	logger.Info("FCM provider initialized successfully")

	return &FCMProvider{
		client: client,
		config: config,
	}, nil
}

// SendPush sends a push notification to a single device
func (f *FCMProvider) SendPush(ctx context.Context, notification *PushNotification) error {
	if notification == nil {
		return fmt.Errorf("notification is required")
	}

	if notification.Token == "" {
		return fmt.Errorf("token is required")
	}

	// Build FCM message
	message := f.buildMessage(notification)

	// Send message
	response, err := f.client.Send(ctx, message)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"token": maskToken(notification.Token),
			"error": err.Error(),
		}).Error("Failed to send FCM push notification")
		return fmt.Errorf("failed to send push notification: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"message_id": response,
		"token":      maskToken(notification.Token),
		"title":      notification.Title,
	}).Info("FCM push notification sent successfully")

	return nil
}

// SendBatchPush sends push notifications to multiple devices
func (f *FCMProvider) SendBatchPush(ctx context.Context, notifications []*PushNotification) error {
	if len(notifications) == 0 {
		return nil
	}

	// FCM supports up to 500 messages per batch
	batchSize := 500
	var allErrors []string

	for i := 0; i < len(notifications); i += batchSize {
		end := i + batchSize
		if end > len(notifications) {
			end = len(notifications)
		}

		batch := notifications[i:end]
		messages := make([]*messaging.Message, len(batch))

		for j, notif := range batch {
			messages[j] = f.buildMessage(notif)
		}

		// Send batch
		response, err := f.client.SendEach(ctx, messages)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("batch %d: %v", i/batchSize, err))
			continue
		}

		// Log results
		logger.WithFields(map[string]interface{}{
			"success_count": response.SuccessCount,
			"failure_count": response.FailureCount,
			"batch_size":    len(batch),
		}).Info("FCM batch push sent")

		// Log individual failures
		if response.FailureCount > 0 {
			for idx, resp := range response.Responses {
				if !resp.Success {
					logger.WithFields(map[string]interface{}{
						"token": maskToken(batch[idx].Token),
						"error": resp.Error,
					}).Warn("FCM push failed for token")
				}
			}
		}
	}

	if len(allErrors) > 0 {
		return fmt.Errorf("batch send errors: %s", strings.Join(allErrors, "; "))
	}

	return nil
}

// SendToTopic sends a push notification to a topic/channel
func (f *FCMProvider) SendToTopic(ctx context.Context, topic string, notification *PushNotification) error {
	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	if notification == nil {
		return fmt.Errorf("notification is required")
	}

	// Build message for topic
	message := f.buildMessage(notification)
	message.Topic = topic
	message.Token = "" // Clear token when sending to topic

	// Send message
	response, err := f.client.Send(ctx, message)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"topic": topic,
			"error": err.Error(),
		}).Error("Failed to send FCM topic notification")
		return fmt.Errorf("failed to send topic notification: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"message_id": response,
		"topic":      topic,
		"title":      notification.Title,
	}).Info("FCM topic notification sent successfully")

	return nil
}

// SubscribeToTopic subscribes tokens to a topic
func (f *FCMProvider) SubscribeToTopic(ctx context.Context, tokens []string, topic string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("at least one token is required")
	}

	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	response, err := f.client.SubscribeToTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"topic":         topic,
		"success_count": response.SuccessCount,
		"failure_count": response.FailureCount,
	}).Info("Subscribed tokens to FCM topic")

	return nil
}

// UnsubscribeFromTopic unsubscribes tokens from a topic
func (f *FCMProvider) UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error {
	if len(tokens) == 0 {
		return fmt.Errorf("at least one token is required")
	}

	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	response, err := f.client.UnsubscribeFromTopic(ctx, tokens, topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"topic":         topic,
		"success_count": response.SuccessCount,
		"failure_count": response.FailureCount,
	}).Info("Unsubscribed tokens from FCM topic")

	return nil
}

// ValidateToken validates if a token is valid
func (f *FCMProvider) ValidateToken(ctx context.Context, token string) error {
	if token == "" {
		return fmt.Errorf("token is required")
	}

	// Try sending a dry-run message
	message := &messaging.Message{
		Token: token,
		Data: map[string]string{
			"validation": "true",
		},
	}

	// Send with dry-run flag
	_, err := f.client.SendDryRun(ctx, message)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	return nil
}

// Close closes the FCM provider
func (f *FCMProvider) Close() error {
	// FCM client doesn't need explicit closing
	logger.Info("FCM provider closed")
	return nil
}

// buildMessage builds an FCM message from PushNotification
func (f *FCMProvider) buildMessage(notification *PushNotification) *messaging.Message {
	message := &messaging.Message{
		Token: notification.Token,
	}

	// Build notification payload
	if notification.Title != "" {
		message.Notification = &messaging.Notification{
			Title:    notification.Title,
			Body:     notification.Body,
			ImageURL: notification.ImageURL,
		}
	}

	// Build data payload
	data := make(map[string]string)
	for key, value := range notification.Data {
		data[key] = fmt.Sprintf("%v", value)
	}

	// Add metadata
	if notification.NotificationID > 0 {
		data["notification_id"] = fmt.Sprintf("%d", notification.NotificationID)
	}
	if notification.Type != "" {
		data["type"] = string(notification.Type)
	}
	if notification.RelatedID != nil {
		data["related_id"] = fmt.Sprintf("%d", *notification.RelatedID)
	}
	if notification.RelatedType != "" {
		data["related_type"] = notification.RelatedType
	}
	if notification.ActionURL != "" {
		data["action_url"] = notification.ActionURL
	}

	message.Data = data

	// Platform-specific configurations
	message.Android = f.buildAndroidConfig(notification)
	message.APNS = f.buildAPNSConfig(notification)
	message.Webpush = f.buildWebpushConfig(notification)

	return message
}

// buildAndroidConfig builds Android-specific configuration
func (f *FCMProvider) buildAndroidConfig(notification *PushNotification) *messaging.AndroidConfig {
	config := &messaging.AndroidConfig{
		Priority: "high", // Default to high priority
	}

	// Set priority based on notification priority
	switch notification.Priority {
	case models.NotificationPriorityCritical, models.NotificationPriorityHigh:
		config.Priority = "high"
	default:
		config.Priority = "normal"
	}

	// Set TTL
	if notification.TTL != nil {
		ttl := time.Duration(*notification.TTL) * time.Second
		config.TTL = &ttl
	}

	// Set collapse key
	if notification.CollapseKey != "" {
		config.CollapseKey = notification.CollapseKey
	}

	// Android notification configuration
	androidNotification := &messaging.AndroidNotification{
		ChannelID:   notification.ChannelID,
		Color:       notification.Color,
		ClickAction: notification.ClickAction,
		Sound:       notification.Sound,
	}

	// Set priority for notification
	if notification.Priority == models.NotificationPriorityCritical {
		androidNotification.Priority = messaging.PriorityMax
	} else if notification.Priority == models.NotificationPriorityHigh {
		androidNotification.Priority = messaging.PriorityHigh
	}

	config.Notification = androidNotification

	return config
}

// buildAPNSConfig builds Apple Push Notification Service configuration
func (f *FCMProvider) buildAPNSConfig(notification *PushNotification) *messaging.APNSConfig {
	config := &messaging.APNSConfig{
		Headers: make(map[string]string),
		Payload: &messaging.APNSPayload{},
	}

	// Set priority
	if notification.Priority == models.NotificationPriorityCritical || notification.Priority == models.NotificationPriorityHigh {
		config.Headers["apns-priority"] = "10" // High priority
	} else {
		config.Headers["apns-priority"] = "5" // Normal priority
	}

	// Build APS payload
	aps := &messaging.Aps{
		Sound: notification.Sound,
	}

	if notification.Sound == "" {
		aps.Sound = "default"
	}

	// Set badge
	if notification.Badge != nil {
		aps.Badge = notification.Badge
	}

	// Set content-available for silent push
	if notification.ContentAvailable {
		aps.ContentAvailable = true
	}

	// Set category
	if notification.Category != "" {
		aps.Category = notification.Category
	}

	// Set thread ID for grouping
	if notification.ThreadID != "" {
		aps.ThreadID = notification.ThreadID
	}

	config.Payload.Aps = aps

	return config
}

// buildWebpushConfig builds Web Push configuration
func (f *FCMProvider) buildWebpushConfig(notification *PushNotification) *messaging.WebpushConfig {
	config := &messaging.WebpushConfig{}

	// Set notification options
	if notification.Title != "" || notification.Body != "" {
		config.Notification = &messaging.WebpushNotification{
			Title: notification.Title,
			Body:  notification.Body,
			Icon:  notification.ImageURL,
		}
	}

	return config
}

// maskToken masks the token for logging (show first 10 and last 10 characters)
func maskToken(token string) string {
	if len(token) <= 20 {
		return "***"
	}
	return token[:10] + "..." + token[len(token)-10:]
}
