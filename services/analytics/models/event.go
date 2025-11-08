package models

import (
	"time"

	"gorm.io/gorm"
)

// AnalyticsEvent represents a raw analytics event
type AnalyticsEvent struct {
	ID            uint64                 `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType     string                 `gorm:"type:varchar(100);not null;index:idx_events_type_timestamp" json:"event_type"`
	EventCategory string                 `gorm:"type:varchar(50);not null;index:idx_events_category_timestamp" json:"event_category"`
	UserID        *uint64                `gorm:"index:idx_events_user_timestamp" json:"user_id,omitempty"`
	EntityID      *uint64                `json:"entity_id,omitempty"`
	DepartmentID  *uint64                `gorm:"index:idx_events_department_timestamp" json:"department_id,omitempty"`
	Metadata      map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
	Timestamp     time.Time              `gorm:"not null;index:idx_events_type_timestamp,idx_events_user_timestamp,idx_events_category_timestamp,idx_events_department_timestamp" json:"timestamp"`
	CreatedAt     time.Time              `gorm:"not null" json:"created_at"`
}

// TableName specifies the table name for AnalyticsEvent
func (AnalyticsEvent) TableName() string {
	return "analytics_events"
}

// BeforeCreate hook
func (e *AnalyticsEvent) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if e.Timestamp.IsZero() {
		e.Timestamp = now
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	return nil
}

// Event types constants
const (
	// User events
	EventUserLogin        = "user_login"
	EventUserLogout       = "user_logout"
	EventUserRegistration = "user_registration"
	EventUserUpdate       = "user_update"
	EventUserDelete       = "user_delete"

	// Message events
	EventMessageSent     = "message_sent"
	EventMessageRead     = "message_read"
	EventMessageDeleted  = "message_deleted"
	EventMessageEdited   = "message_edited"
	EventChannelCreated  = "channel_created"
	EventChannelJoined   = "channel_joined"
	EventChannelLeft     = "channel_left"

	// Task events
	EventTaskCreated    = "task_created"
	EventTaskUpdated    = "task_updated"
	EventTaskCompleted  = "task_completed"
	EventTaskDeleted    = "task_deleted"
	EventTaskAssigned   = "task_assigned"
	EventTaskCommented  = "task_commented"

	// Calendar events
	EventEventCreated   = "event_created"
	EventEventUpdated   = "event_updated"
	EventEventDeleted   = "event_deleted"
	EventEventAttended  = "event_attended"
	EventEventDeclined  = "event_declined"

	// Poll events
	EventPollCreated    = "poll_created"
	EventPollVoted      = "poll_voted"
	EventPollCompleted  = "poll_completed"
	EventPollDeleted    = "poll_deleted"

	// File events
	EventFileUploaded   = "file_uploaded"
	EventFileDownloaded = "file_downloaded"
	EventFileDeleted    = "file_deleted"
	EventFileShared     = "file_shared"

	// System events
	EventAPIRequest     = "api_request"
	EventAPIError       = "api_error"
	EventSystemError    = "system_error"
)

// Event categories
const (
	CategoryUser     = "user"
	CategoryMessage  = "message"
	CategoryTask     = "task"
	CategoryCalendar = "calendar"
	CategoryPoll     = "poll"
	CategoryFile     = "file"
	CategorySystem   = "system"
)

// CreateEventRequest represents the request to create an event
type CreateEventRequest struct {
	EventType     string                 `json:"event_type" binding:"required"`
	EventCategory string                 `json:"event_category" binding:"required"`
	UserID        *uint64                `json:"user_id,omitempty"`
	EntityID      *uint64                `json:"entity_id,omitempty"`
	DepartmentID  *uint64                `json:"department_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     *time.Time             `json:"timestamp,omitempty"`
}

// ToModel converts CreateEventRequest to AnalyticsEvent model
func (r *CreateEventRequest) ToModel() *AnalyticsEvent {
	event := &AnalyticsEvent{
		EventType:     r.EventType,
		EventCategory: r.EventCategory,
		UserID:        r.UserID,
		EntityID:      r.EntityID,
		DepartmentID:  r.DepartmentID,
		Metadata:      r.Metadata,
	}

	if r.Timestamp != nil {
		event.Timestamp = *r.Timestamp
	} else {
		event.Timestamp = time.Now()
	}

	return event
}
