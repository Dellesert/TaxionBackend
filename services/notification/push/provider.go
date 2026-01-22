// File: services/notification/push/provider.go
package push

import (
	"context"
	"tachyon-messenger/services/notification/models"
)

// PushProvider defines the interface for push notification providers
type PushProvider interface {
	// SendPush sends a push notification to a single device
	SendPush(ctx context.Context, notification *PushNotification) error

	// SendBatchPush sends push notifications to multiple devices
	SendBatchPush(ctx context.Context, notifications []*PushNotification) error

	// SendToTopic sends a push notification to a topic/channel
	SendToTopic(ctx context.Context, topic string, notification *PushNotification) error

	// SubscribeToTopic subscribes tokens to a topic
	SubscribeToTopic(ctx context.Context, tokens []string, topic string) error

	// UnsubscribeFromTopic unsubscribes tokens from a topic
	UnsubscribeFromTopic(ctx context.Context, tokens []string, topic string) error

	// ValidateToken validates if a token is valid
	ValidateToken(ctx context.Context, token string) error

	// Close closes the provider connection
	Close() error
}

// PushNotification represents a push notification to be sent
type PushNotification struct {
	// Target
	Token  string   `json:"token,omitempty"`  // Device token
	Tokens []string `json:"tokens,omitempty"` // Multiple device tokens
	Topic  string   `json:"topic,omitempty"`  // Topic/channel name

	// Notification content
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Body     string `json:"body,omitempty" validate:"omitempty,max=1000"`
	ImageURL string `json:"image_url,omitempty" validate:"omitempty,url"`

	// Data payload (custom data)
	Data map[string]interface{} `json:"data,omitempty"`

	// iOS specific
	Badge          *int   `json:"badge,omitempty"`           // Badge count
	Sound          string `json:"sound,omitempty"`           // Sound file name
	Category       string `json:"category,omitempty"`        // Notification category
	ThreadID       string `json:"thread_id,omitempty"`       // Thread identifier for grouping
	ContentAvailable bool `json:"content_available,omitempty"` // Silent push for background updates

	// Android specific
	ChannelID      string                 `json:"channel_id,omitempty"`      // Android notification channel
	Color          string                 `json:"color,omitempty"`           // Notification color (hex)
	ClickAction    string                 `json:"click_action,omitempty"`    // Action on notification click
	AndroidPriority string                `json:"android_priority,omitempty"` // "high" or "normal"
	TTL            *int                   `json:"ttl,omitempty"`             // Time to live in seconds
	CollapseKey    string                 `json:"collapse_key,omitempty"`    // Collapse key for grouping

	// Priority
	Priority models.NotificationPriority `json:"priority,omitempty" validate:"omitempty,oneof=low medium high critical"`

	// Additional metadata
	NotificationID uint                     `json:"notification_id,omitempty"` // Internal notification ID
	Type           models.NotificationType  `json:"type,omitempty"`            // Notification type
	RelatedID      *uint                    `json:"related_id,omitempty"`      // Related object ID
	RelatedType    string                   `json:"related_type,omitempty"`    // Related object type
	ActionURL      string                   `json:"action_url,omitempty"`      // Deep link URL
}

// PushResponse represents the response from sending a push notification
type PushResponse struct {
	MessageID    string `json:"message_id"`              // FCM message ID
	Success      bool   `json:"success"`                 // Whether send was successful
	Error        string `json:"error,omitempty"`         // Error message if failed
	FailedTokens []string `json:"failed_tokens,omitempty"` // Tokens that failed
}

// PushConfig represents configuration for push notification provider
type PushConfig struct {
	// FCM specific
	CredentialsFile string `json:"credentials_file,omitempty"` // Path to FCM service account JSON
	ProjectID       string `json:"project_id,omitempty"`       // Firebase project ID

	// Provider type
	Provider string `json:"provider"` // "fcm", "apns", "expo", etc.

	// Common settings
	DryRun  bool `json:"dry_run"`  // Test mode without actually sending
	Timeout int  `json:"timeout"`  // Request timeout in seconds
}

// NotificationDataBuilder helps build data payload for different notification types
type NotificationDataBuilder struct {
	data map[string]interface{}
}

// NewDataBuilder creates a new NotificationDataBuilder
func NewDataBuilder() *NotificationDataBuilder {
	return &NotificationDataBuilder{
		data: make(map[string]interface{}),
	}
}

// SetType sets the notification type
func (b *NotificationDataBuilder) SetType(notifType models.NotificationType) *NotificationDataBuilder {
	b.data["type"] = string(notifType)
	return b
}

// SetChatID sets the chat ID for message notifications
func (b *NotificationDataBuilder) SetChatID(chatID uint) *NotificationDataBuilder {
	b.data["chat_id"] = chatID
	return b
}

// SetMessageID sets the message ID
func (b *NotificationDataBuilder) SetMessageID(messageID uint) *NotificationDataBuilder {
	b.data["message_id"] = messageID
	return b
}

// SetTaskID sets the task ID for task notifications
func (b *NotificationDataBuilder) SetTaskID(taskID uint) *NotificationDataBuilder {
	b.data["task_id"] = taskID
	return b
}

// SetEventID sets the event ID for calendar notifications
func (b *NotificationDataBuilder) SetEventID(eventID uint) *NotificationDataBuilder {
	b.data["event_id"] = eventID
	return b
}

// SetPollID sets the poll ID for poll notifications
func (b *NotificationDataBuilder) SetPollID(pollID uint) *NotificationDataBuilder {
	b.data["poll_id"] = pollID
	return b
}

// SetScheduleID sets the schedule ID for schedule notifications
func (b *NotificationDataBuilder) SetScheduleID(scheduleID uint) *NotificationDataBuilder {
	b.data["schedule_id"] = scheduleID
	return b
}

// SetAction sets the action to perform when notification is tapped
func (b *NotificationDataBuilder) SetAction(action string) *NotificationDataBuilder {
	b.data["action"] = action
	return b
}

// SetActionURL sets the deep link URL
func (b *NotificationDataBuilder) SetActionURL(url string) *NotificationDataBuilder {
	b.data["action_url"] = url
	return b
}

// SetCustomField sets a custom field
func (b *NotificationDataBuilder) SetCustomField(key string, value interface{}) *NotificationDataBuilder {
	b.data[key] = value
	return b
}

// Build returns the constructed data map
func (b *NotificationDataBuilder) Build() map[string]interface{} {
	return b.data
}
