// File: services/notification/models/notification.go
package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotificationTypeMessage  NotificationType = "message"  // Новое сообщение в чате
	NotificationTypeTask     NotificationType = "task"     // Уведомления о задачах
	NotificationTypeCalendar NotificationType = "calendar" // События календаря
	NotificationTypeSystem   NotificationType = "system"   // Системные уведомления
	NotificationTypeMention  NotificationType = "mention"  // Упоминания в сообщениях
	NotificationTypePoll     NotificationType = "poll"     // Уведомления об опросах
	NotificationTypeReminder NotificationType = "reminder" // Напоминания
	NotificationTypeAnnounce NotificationType = "announce" // Объявления
)

// NotificationPriority represents the priority level of notification
type NotificationPriority string

const (
	NotificationPriorityLow      NotificationPriority = "low"      // Низкий приоритет
	NotificationPriorityMedium   NotificationPriority = "medium"   // Средний приоритет
	NotificationPriorityHigh     NotificationPriority = "high"     // Высокий приоритет
	NotificationPriorityCritical NotificationPriority = "critical" // Критический приоритет
)

// NotificationStatus represents the status of notification delivery
type NotificationStatus string

const (
	NotificationStatusPending   NotificationStatus = "pending"   // Ожидает отправки
	NotificationStatusDelivered NotificationStatus = "delivered" // Доставлено
	NotificationStatusRead      NotificationStatus = "read"      // Прочитано
	NotificationStatusFailed    NotificationStatus = "failed"    // Ошибка доставки
)

// DeliveryChannel represents the delivery channel for notifications
type DeliveryChannel string

const (
	DeliveryChannelInApp   DeliveryChannel = "in_app"  // Внутри приложения
	DeliveryChannelEmail   DeliveryChannel = "email"   // Email уведомления
	DeliveryChannelPush    DeliveryChannel = "push"    // Push уведомления
	DeliveryChannelSMS     DeliveryChannel = "sms"     // SMS уведомления
	DeliveryChannelSlack   DeliveryChannel = "slack"   // Slack интеграция
	DeliveryChannelWebhook DeliveryChannel = "webhook" // Webhook уведомления
)

// Notification represents a notification in the system
type Notification struct {
	models.BaseModel
	UserID   uint                 `gorm:"not null;index" json:"user_id" validate:"required,min=1"`
	Type     NotificationType     `gorm:"not null;size:20;index" json:"type" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	Title    string               `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Message  string               `gorm:"type:text" json:"message,omitempty" validate:"omitempty,max=2000"`
	Priority NotificationPriority `gorm:"not null;default:'medium';size:20" json:"priority" validate:"required,oneof=low medium high critical"`
	Status   NotificationStatus   `gorm:"not null;default:'pending';size:20;index" json:"status" validate:"required,oneof=pending delivered read failed"`
	IsRead   bool                 `gorm:"not null;default:false;index" json:"is_read"`
	ReadAt   *time.Time           `json:"read_at,omitempty"`

	// Metadata for related objects
	RelatedID   *uint  `gorm:"index" json:"related_id,omitempty"`     // ID связанного объекта (задача, сообщение и т.д.)
	RelatedType string `gorm:"size:50" json:"related_type,omitempty"` // Тип связанного объекта
	ActionURL   string `gorm:"size:500" json:"action_url,omitempty"`  // URL для действия
	ImageURL    string `gorm:"size:500" json:"image_url,omitempty"`   // URL изображения

	// Delivery tracking
	DeliveryChannels []NotificationDelivery `gorm:"foreignKey:NotificationID;constraint:OnDelete:CASCADE" json:"delivery_channels,omitempty"`

	// Scheduling
	ScheduledAt *time.Time `gorm:"index" json:"scheduled_at,omitempty"` // Время запланированной отправки
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at,omitempty"`   // Время истечения актуальности
}

// NotificationDelivery represents delivery attempt for specific channel
type NotificationDelivery struct {
	models.BaseModel
	NotificationID uint               `gorm:"not null;index" json:"notification_id"`
	Channel        DeliveryChannel    `gorm:"not null;size:20" json:"channel" validate:"required,oneof=in_app email push sms slack webhook"`
	Status         NotificationStatus `gorm:"not null;default:'pending';size:20" json:"status" validate:"required,oneof=pending delivered read failed"`
	AttemptCount   int                `gorm:"not null;default:0" json:"attempt_count"`
	LastAttemptAt  *time.Time         `json:"last_attempt_at,omitempty"`
	DeliveredAt    *time.Time         `json:"delivered_at,omitempty"`
	ErrorMessage   string             `gorm:"type:text" json:"error_message,omitempty"`

	// Channel-specific metadata
	ExternalID  string `gorm:"size:255" json:"external_id,omitempty"`    // ID во внешней системе
	ChannelData string `gorm:"type:jsonb" json:"channel_data,omitempty"` // Дополнительные данные канала
}

// EmailTemplate represents email notification template
type EmailTemplate struct {
	models.BaseModel
	Name         string           `gorm:"uniqueIndex;not null;size:100" json:"name" validate:"required,min=1,max=100"`
	Type         NotificationType `gorm:"not null;size:20;index" json:"type" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	Subject      string           `gorm:"not null;size:255" json:"subject" validate:"required,min=1,max=255"`
	HTMLTemplate string           `gorm:"type:text;not null" json:"html_template" validate:"required"`
	TextTemplate string           `gorm:"type:text" json:"text_template,omitempty"`
	IsActive     bool             `gorm:"not null;default:true" json:"is_active"`

	// Template variables documentation
	Variables   string `gorm:"type:jsonb" json:"variables,omitempty"`  // JSON массив доступных переменных
	Description string `gorm:"type:text" json:"description,omitempty"` // Описание шаблона
}

// UserNotificationPreference represents user's notification preferences
type UserNotificationPreference struct {
	models.BaseModel
	UserID           uint             `gorm:"not null;index" json:"user_id" validate:"required,min=1"`
	NotificationType NotificationType `gorm:"not null;size:20" json:"notification_type" validate:"required,oneof=message task calendar system mention poll reminder announce"`

	// Channel preferences
	InAppEnabled bool `gorm:"not null;default:true" json:"in_app_enabled"`
	EmailEnabled bool `gorm:"not null;default:true" json:"email_enabled"`
	PushEnabled  bool `gorm:"not null;default:true" json:"push_enabled"`
	SMSEnabled   bool `gorm:"not null;default:false" json:"sms_enabled"`

	// Priority filters
	MinPriority NotificationPriority `gorm:"not null;default:'low';size:20" json:"min_priority" validate:"required,oneof=low medium high critical"`

	// Time preferences
	QuietHoursStart *int `json:"quiet_hours_start,omitempty" validate:"omitempty,min=0,max=23"` // Час начала тихого времени (0-23)
	QuietHoursEnd   *int `json:"quiet_hours_end,omitempty" validate:"omitempty,min=0,max=23"`   // Час окончания тихого времени (0-23)
	WeekendEnabled  bool `gorm:"not null;default:true" json:"weekend_enabled"`

	// Frequency limits
	DigestEnabled   bool `gorm:"not null;default:false" json:"digest_enabled"`                    // Группировка уведомлений
	DigestFrequency *int `json:"digest_frequency,omitempty" validate:"omitempty,min=15,max=1440"` // Частота дайджеста в минутах
}

// NotificationTemplate represents a reusable notification template
type NotificationTemplate struct {
	models.BaseModel
	Name            string               `gorm:"uniqueIndex;not null;size:100" json:"name" validate:"required,min=1,max=100"`
	Type            NotificationType     `gorm:"not null;size:20;index" json:"type" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	TitleTemplate   string               `gorm:"not null;size:255" json:"title_template" validate:"required,min=1,max=255"`
	MessageTemplate string               `gorm:"type:text" json:"message_template,omitempty" validate:"omitempty,max=2000"`
	Priority        NotificationPriority `gorm:"not null;default:'medium';size:20" json:"priority" validate:"required,oneof=low medium high critical"`
	IsActive        bool                 `gorm:"not null;default:true" json:"is_active"`

	// Template metadata
	Variables       string `gorm:"type:jsonb" json:"variables,omitempty"`        // JSON массив доступных переменных
	Description     string `gorm:"type:text" json:"description,omitempty"`       // Описание шаблона
	DefaultChannels string `gorm:"type:jsonb" json:"default_channels,omitempty"` // JSON массив каналов по умолчанию
}

// Table names
func (Notification) TableName() string {
	return "notifications"
}

func (NotificationDelivery) TableName() string {
	return "notification_deliveries"
}

func (EmailTemplate) TableName() string {
	return "email_templates"
}

func (UserNotificationPreference) TableName() string {
	return "user_notification_preferences"
}

func (NotificationTemplate) TableName() string {
	return "notification_templates"
}

// GORM hooks

// BeforeCreate hook for Notification
func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if n.Priority == "" {
		n.Priority = NotificationPriorityMedium
	}
	if n.Status == "" {
		n.Status = NotificationStatusPending
	}
	return nil
}

// BeforeUpdate hook for Notification
func (n *Notification) BeforeUpdate(tx *gorm.DB) error {
	// Set read timestamp when marking as read
	if n.IsRead && n.ReadAt == nil {
		now := time.Now()
		n.ReadAt = &now
	}
	return nil
}

// BeforeCreate hook for NotificationDelivery
func (nd *NotificationDelivery) BeforeCreate(tx *gorm.DB) error {
	if nd.Status == "" {
		nd.Status = NotificationStatusPending
	}
	return nil
}

// BeforeCreate hook for UserNotificationPreference
func (p *UserNotificationPreference) BeforeCreate(tx *gorm.DB) error {
	if p.MinPriority == "" {
		p.MinPriority = NotificationPriorityLow
	}
	return nil
}

// BeforeCreate hook for NotificationTemplate
func (nt *NotificationTemplate) BeforeCreate(tx *gorm.DB) error {
	if nt.Priority == "" {
		nt.Priority = NotificationPriorityMedium
	}
	return nil
}

// Request/Response Models

// CreateNotificationRequest represents request for creating a notification
type CreateNotificationRequest struct {
	UserID      uint                  `json:"user_id" binding:"required,min=1" validate:"required,min=1"`
	Type        NotificationType      `json:"type" binding:"required,oneof=message task calendar system mention poll reminder announce" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	Title       string                `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Message     string                `json:"message,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Priority    *NotificationPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	RelatedID   *uint                 `json:"related_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	RelatedType string                `json:"related_type,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	ActionURL   string                `json:"action_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ImageURL    string                `json:"image_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ScheduledAt *time.Time            `json:"scheduled_at,omitempty"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
	Channels    []DeliveryChannel     `json:"channels,omitempty" validate:"omitempty,dive,oneof=in_app email push sms slack webhook"`
}

// BulkCreateNotificationRequest represents request for creating multiple notifications
type BulkCreateNotificationRequest struct {
	UserIDs     []uint                `json:"user_ids" binding:"required,min=1,dive,min=1" validate:"required,min=1,dive,min=1"`
	Type        NotificationType      `json:"type" binding:"required,oneof=message task calendar system mention poll reminder announce" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	Title       string                `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Message     string                `json:"message,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Priority    *NotificationPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	RelatedID   *uint                 `json:"related_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	RelatedType string                `json:"related_type,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	ActionURL   string                `json:"action_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ImageURL    string                `json:"image_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ScheduledAt *time.Time            `json:"scheduled_at,omitempty"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
	Channels    []DeliveryChannel     `json:"channels,omitempty" validate:"omitempty,dive,oneof=in_app email push sms slack webhook"`
}

// UpdateNotificationRequest represents request for updating a notification
type UpdateNotificationRequest struct {
	Title       *string               `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Message     *string               `json:"message,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Priority    *NotificationPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	Status      *NotificationStatus   `json:"status,omitempty" binding:"omitempty,oneof=pending delivered read failed" validate:"omitempty,oneof=pending delivered read failed"`
	ActionURL   *string               `json:"action_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ImageURL    *string               `json:"image_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	ScheduledAt *time.Time            `json:"scheduled_at,omitempty"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
}

// MarkAsReadRequest represents request for marking notifications as read
type MarkAsReadRequest struct {
	NotificationIDs []uint `json:"notification_ids" binding:"required,min=1,dive,min=1" validate:"required,min=1,dive,min=1"`
}

// NotificationFilterRequest represents filtering parameters for notifications
type NotificationFilterRequest struct {
	Type          *NotificationType     `form:"type" binding:"omitempty,oneof=message task calendar system mention poll reminder announce"`
	Priority      *NotificationPriority `form:"priority" binding:"omitempty,oneof=low medium high critical"`
	Status        *NotificationStatus   `form:"status" binding:"omitempty,oneof=pending delivered read failed"`
	IsRead        *bool                 `form:"is_read"`
	RelatedType   string                `form:"related_type" binding:"omitempty,max=50"`
	CreatedAfter  *time.Time            `form:"created_after" time_format:"2006-01-02T15:04:05Z07:00"`
	CreatedBefore *time.Time            `form:"created_before" time_format:"2006-01-02T15:04:05Z07:00"`
	Limit         int                   `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset        int                   `form:"offset" binding:"omitempty,min=0"`
	SortBy        string                `form:"sort_by" binding:"omitempty,oneof=created_at updated_at priority type"`
	SortOrder     string                `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

// UserPreferenceRequest represents request for updating user notification preferences
type UserPreferenceRequest struct {
	NotificationType NotificationType      `json:"notification_type" binding:"required,oneof=message task calendar system mention poll reminder announce" validate:"required,oneof=message task calendar system mention poll reminder announce"`
	InAppEnabled     *bool                 `json:"in_app_enabled,omitempty"`
	EmailEnabled     *bool                 `json:"email_enabled,omitempty"`
	PushEnabled      *bool                 `json:"push_enabled,omitempty"`
	SMSEnabled       *bool                 `json:"sms_enabled,omitempty"`
	MinPriority      *NotificationPriority `json:"min_priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	QuietHoursStart  *int                  `json:"quiet_hours_start,omitempty" binding:"omitempty,min=0,max=23" validate:"omitempty,min=0,max=23"`
	QuietHoursEnd    *int                  `json:"quiet_hours_end,omitempty" binding:"omitempty,min=0,max=23" validate:"omitempty,min=0,max=23"`
	WeekendEnabled   *bool                 `json:"weekend_enabled,omitempty"`
	DigestEnabled    *bool                 `json:"digest_enabled,omitempty"`
	DigestFrequency  *int                  `json:"digest_frequency,omitempty" binding:"omitempty,min=15,max=1440" validate:"omitempty,min=15,max=1440"`
}

// Response Models

// NotificationResponse represents a notification in API responses
type NotificationResponse struct {
	ID               uint                           `json:"id"`
	UserID           uint                           `json:"user_id"`
	Type             NotificationType               `json:"type"`
	Title            string                         `json:"title"`
	Message          string                         `json:"message,omitempty"`
	Priority         NotificationPriority           `json:"priority"`
	Status           NotificationStatus             `json:"status"`
	IsRead           bool                           `json:"is_read"`
	ReadAt           *time.Time                     `json:"read_at,omitempty"`
	RelatedID        *uint                          `json:"related_id,omitempty"`
	RelatedType      string                         `json:"related_type,omitempty"`
	ActionURL        string                         `json:"action_url,omitempty"`
	ImageURL         string                         `json:"image_url,omitempty"`
	ScheduledAt      *time.Time                     `json:"scheduled_at,omitempty"`
	ExpiresAt        *time.Time                     `json:"expires_at,omitempty"`
	CreatedAt        time.Time                      `json:"created_at"`
	UpdatedAt        time.Time                      `json:"updated_at"`
	DeliveryChannels []NotificationDeliveryResponse `json:"delivery_channels,omitempty"`
}

// NotificationDeliveryResponse represents delivery status in API responses
type NotificationDeliveryResponse struct {
	ID            uint               `json:"id"`
	Channel       DeliveryChannel    `json:"channel"`
	Status        NotificationStatus `json:"status"`
	AttemptCount  int                `json:"attempt_count"`
	LastAttemptAt *time.Time         `json:"last_attempt_at,omitempty"`
	DeliveredAt   *time.Time         `json:"delivered_at,omitempty"`
	ErrorMessage  string             `json:"error_message,omitempty"`
}

// NotificationStatsResponse represents notification statistics
type NotificationStatsResponse struct {
	TotalNotifications   int `json:"total_notifications"`
	UnreadNotifications  int `json:"unread_notifications"`
	PendingNotifications int `json:"pending_notifications"`
	FailedNotifications  int `json:"failed_notifications"`
	TodayNotifications   int `json:"today_notifications"`
	WeekNotifications    int `json:"week_notifications"`
}

// ToResponse converts Notification model to NotificationResponse
func (n *Notification) ToResponse() *NotificationResponse {
	response := &NotificationResponse{
		ID:          n.ID,
		UserID:      n.UserID,
		Type:        n.Type,
		Title:       n.Title,
		Message:     n.Message,
		Priority:    n.Priority,
		Status:      n.Status,
		IsRead:      n.IsRead,
		ReadAt:      n.ReadAt,
		RelatedID:   n.RelatedID,
		RelatedType: n.RelatedType,
		ActionURL:   n.ActionURL,
		ImageURL:    n.ImageURL,
		ScheduledAt: n.ScheduledAt,
		ExpiresAt:   n.ExpiresAt,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}

	// Convert delivery channels if loaded
	if len(n.DeliveryChannels) > 0 {
		response.DeliveryChannels = make([]NotificationDeliveryResponse, len(n.DeliveryChannels))
		for i, delivery := range n.DeliveryChannels {
			response.DeliveryChannels[i] = NotificationDeliveryResponse{
				ID:            delivery.ID,
				Channel:       delivery.Channel,
				Status:        delivery.Status,
				AttemptCount:  delivery.AttemptCount,
				LastAttemptAt: delivery.LastAttemptAt,
				DeliveredAt:   delivery.DeliveredAt,
				ErrorMessage:  delivery.ErrorMessage,
			}
		}
	}

	return response
}
