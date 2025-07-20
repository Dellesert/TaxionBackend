package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// EventType represents the type of event
type EventType string

const (
	EventTypePersonal EventType = "personal"
	EventTypeMeeting  EventType = "meeting"
	EventTypeDeadline EventType = "deadline"
)

// ParticipantStatus represents the participation status
type ParticipantStatus string

const (
	ParticipantStatusPending  ParticipantStatus = "pending"
	ParticipantStatusAccepted ParticipantStatus = "accepted"
	ParticipantStatusDeclined ParticipantStatus = "declined"
	ParticipantStatusMaybe    ParticipantStatus = "maybe"
)

// ReminderType represents the type of reminder
type ReminderType string

const (
	ReminderTypeEmail        ReminderType = "email"
	ReminderTypeNotification ReminderType = "notification"
	ReminderTypeSMS          ReminderType = "sms"
)

// Event represents a calendar event
type Event struct {
	models.BaseModel
	Title       string    `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Description string    `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=2000"`
	StartTime   time.Time `gorm:"not null;index" json:"start_time" validate:"required"`
	EndTime     time.Time `gorm:"not null;index" json:"end_time" validate:"required"`
	AllDay      bool      `gorm:"not null;default:false" json:"all_day"`
	Location    string    `gorm:"size:500" json:"location,omitempty" validate:"omitempty,max=500"`
	Type        EventType `gorm:"not null;default:'personal';size:20" json:"type" validate:"required,oneof=personal meeting deadline"`
	CreatedBy   uint      `gorm:"not null;index" json:"created_by" validate:"required,min=1"`

	// Calendar organization
	Color       string `gorm:"size:7;default:'#3788d8'" json:"color" validate:"omitempty,len=7"`
	IsPrivate   bool   `gorm:"not null;default:false" json:"is_private"`
	IsRecurring bool   `gorm:"not null;default:false" json:"is_recurring"`

	// Recurrence settings (JSON stored as string)
	RecurrenceRule string `gorm:"type:text" json:"recurrence_rule,omitempty"`

	// Task integration
	TaskID *uint `gorm:"index" json:"task_id,omitempty" validate:"omitempty,min=1"`

	// Associations
	Participants []EventParticipant `gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE" json:"participants,omitempty"`
	Reminders    []EventReminder    `gorm:"foreignKey:EventID;constraint:OnDelete:CASCADE" json:"reminders,omitempty"`

	// Computed fields (not stored in DB)
	ParticipantCount int               `gorm:"-" json:"participant_count,omitempty"`
	UserStatus       ParticipantStatus `gorm:"-" json:"user_status,omitempty"`
}

// TableName returns the table name for Event model
func (Event) TableName() string {
	return "events"
}

// BeforeCreate hook is called before creating an event
func (e *Event) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if e.Type == "" {
		e.Type = EventTypePersonal
	}
	if e.Color == "" {
		e.Color = "#3788d8"
	}

	// Validate time logic
	if e.EndTime.Before(e.StartTime) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// BeforeUpdate hook is called before updating an event
func (e *Event) BeforeUpdate(tx *gorm.DB) error {
	// Validate time logic
	if e.EndTime.Before(e.StartTime) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// EventParticipant represents a participant in an event
type EventParticipant struct {
	models.BaseModel
	EventID     uint              `gorm:"not null;index" json:"event_id" validate:"required"`
	UserID      uint              `gorm:"not null;index" json:"user_id" validate:"required"`
	Status      ParticipantStatus `gorm:"not null;default:'pending';size:20" json:"status" validate:"required,oneof=pending accepted declined maybe"`
	IsOrganizer bool              `gorm:"not null;default:false" json:"is_organizer"`

	// Optional role/title for the participant
	Role string `gorm:"size:100" json:"role,omitempty" validate:"omitempty,max=100"`

	// Response tracking
	RespondedAt *time.Time `json:"responded_at,omitempty"`

	// Associations
	Event *Event `gorm:"foreignKey:EventID" json:"event,omitempty"`
}

// TableName returns the table name for EventParticipant model
func (EventParticipant) TableName() string {
	return "event_participants"
}

// BeforeCreate hook is called before creating an event participant
func (ep *EventParticipant) BeforeCreate(tx *gorm.DB) error {
	// Set default status if not provided
	if ep.Status == "" {
		ep.Status = ParticipantStatusPending
	}
	return nil
}

// BeforeUpdate hook is called before updating an event participant
func (ep *EventParticipant) BeforeUpdate(tx *gorm.DB) error {
	// Set responded time when status changes from pending
	if ep.Status != ParticipantStatusPending && ep.RespondedAt == nil {
		now := time.Now()
		ep.RespondedAt = &now
	}
	return nil
}

// EventReminder represents a reminder for an event
type EventReminder struct {
	models.BaseModel
	EventID     uint         `gorm:"not null;index" json:"event_id" validate:"required"`
	UserID      uint         `gorm:"not null;index" json:"user_id" validate:"required"`
	Type        ReminderType `gorm:"not null;size:20" json:"type" validate:"required,oneof=email notification sms"`
	TriggerTime time.Time    `gorm:"not null;index" json:"trigger_time" validate:"required"`
	IsSent      bool         `gorm:"not null;default:false" json:"is_sent"`
	SentAt      *time.Time   `json:"sent_at,omitempty"`

	// Reminder settings
	Message string `gorm:"type:text" json:"message,omitempty" validate:"omitempty,max=500"`

	// Relative reminder (e.g., "15 minutes before")
	MinutesBefore *int `gorm:"index" json:"minutes_before,omitempty" validate:"omitempty,min=0,max=43200"` // Max 30 days

	// Associations
	Event *Event `gorm:"foreignKey:EventID" json:"event,omitempty"`
}

// TableName returns the table name for EventReminder model
func (EventReminder) TableName() string {
	return "event_reminders"
}

// BeforeCreate hook is called before creating an event reminder
func (er *EventReminder) BeforeCreate(tx *gorm.DB) error {
	// Calculate trigger time based on minutes before if provided
	if er.MinutesBefore != nil && er.EventID != 0 {
		var event Event
		if err := tx.First(&event, er.EventID).Error; err == nil {
			er.TriggerTime = event.StartTime.Add(-time.Duration(*er.MinutesBefore) * time.Minute)
		}
	}
	return nil
}

// Request/Response Models

// CreateEventRequest represents request for creating an event
type CreateEventRequest struct {
	Title          string    `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description    string    `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	StartTime      time.Time `json:"start_time" binding:"required" validate:"required"`
	EndTime        time.Time `json:"end_time" binding:"required" validate:"required"`
	AllDay         bool      `json:"all_day"`
	Location       string    `json:"location,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	Type           EventType `json:"type" binding:"omitempty,oneof=personal meeting deadline" validate:"omitempty,oneof=personal meeting deadline"`
	Color          string    `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	IsPrivate      bool      `json:"is_private"`
	IsRecurring    bool      `json:"is_recurring"`
	RecurrenceRule string    `json:"recurrence_rule,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
	TaskID         *uint     `json:"task_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`

	// Participants to invite
	ParticipantIDs []uint `json:"participant_ids,omitempty" validate:"omitempty,dive,min=1"`

	// Reminders to create
	Reminders []CreateReminderRequest `json:"reminders,omitempty" validate:"omitempty,dive"`
}

// UpdateEventRequest represents request for updating an event
type UpdateEventRequest struct {
	Title          *string    `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description    *string    `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	StartTime      *time.Time `json:"start_time,omitempty"`
	EndTime        *time.Time `json:"end_time,omitempty"`
	AllDay         *bool      `json:"all_day,omitempty"`
	Location       *string    `json:"location,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	Type           *EventType `json:"type,omitempty" binding:"omitempty,oneof=personal meeting deadline" validate:"omitempty,oneof=personal meeting deadline"`
	Color          *string    `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	IsPrivate      *bool      `json:"is_private,omitempty"`
	IsRecurring    *bool      `json:"is_recurring,omitempty"`
	RecurrenceRule *string    `json:"recurrence_rule,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
}

// CreateReminderRequest represents request for creating a reminder
type CreateReminderRequest struct {
	Type          ReminderType `json:"type" binding:"required,oneof=email notification sms" validate:"required,oneof=email notification sms"`
	MinutesBefore int          `json:"minutes_before" binding:"required,min=0,max=43200" validate:"required,min=0,max=43200"`
	Message       string       `json:"message,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// UpdateParticipantStatusRequest represents request for updating participant status
type UpdateParticipantStatusRequest struct {
	Status ParticipantStatus `json:"status" binding:"required,oneof=pending accepted declined maybe" validate:"required,oneof=pending accepted declined maybe"`
}

// AddParticipantsRequest represents request for adding participants to an event
type AddParticipantsRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required,min=1,dive,min=1" validate:"required,min=1,dive,min=1"`
}

// Response Models

// EventResponse represents an event in API responses
type EventResponse struct {
	ID               uint                        `json:"id"`
	Title            string                      `json:"title"`
	Description      string                      `json:"description,omitempty"`
	StartTime        time.Time                   `json:"start_time"`
	EndTime          time.Time                   `json:"end_time"`
	AllDay           bool                        `json:"all_day"`
	Location         string                      `json:"location,omitempty"`
	Type             EventType                   `json:"type"`
	CreatedBy        uint                        `json:"created_by"`
	Color            string                      `json:"color"`
	IsPrivate        bool                        `json:"is_private"`
	IsRecurring      bool                        `json:"is_recurring"`
	RecurrenceRule   string                      `json:"recurrence_rule,omitempty"`
	TaskID           *uint                       `json:"task_id,omitempty"`
	ParticipantCount int                         `json:"participant_count"`
	UserStatus       ParticipantStatus           `json:"user_status,omitempty"`
	Participants     []*EventParticipantResponse `json:"participants,omitempty"`
	Reminders        []*EventReminderResponse    `json:"reminders,omitempty"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

// ToResponse converts Event model to EventResponse
func (e *Event) ToResponse() *EventResponse {
	response := &EventResponse{
		ID:               e.ID,
		Title:            e.Title,
		Description:      e.Description,
		StartTime:        e.StartTime,
		EndTime:          e.EndTime,
		AllDay:           e.AllDay,
		Location:         e.Location,
		Type:             e.Type,
		CreatedBy:        e.CreatedBy,
		Color:            e.Color,
		IsPrivate:        e.IsPrivate,
		IsRecurring:      e.IsRecurring,
		RecurrenceRule:   e.RecurrenceRule,
		TaskID:           e.TaskID,
		ParticipantCount: e.ParticipantCount,
		UserStatus:       e.UserStatus,
		CreatedAt:        e.CreatedAt,
		UpdatedAt:        e.UpdatedAt,
	}

	// Convert participants if they exist
	if len(e.Participants) > 0 {
		response.Participants = make([]*EventParticipantResponse, len(e.Participants))
		for i, participant := range e.Participants {
			response.Participants[i] = participant.ToResponse()
		}
	}

	// Convert reminders if they exist
	if len(e.Reminders) > 0 {
		response.Reminders = make([]*EventReminderResponse, len(e.Reminders))
		for i, reminder := range e.Reminders {
			response.Reminders[i] = reminder.ToResponse()
		}
	}

	return response
}

// EventParticipantResponse represents a participant in API responses
type EventParticipantResponse struct {
	ID          uint              `json:"id"`
	EventID     uint              `json:"event_id"`
	UserID      uint              `json:"user_id"`
	Status      ParticipantStatus `json:"status"`
	IsOrganizer bool              `json:"is_organizer"`
	Role        string            `json:"role,omitempty"`
	RespondedAt *time.Time        `json:"responded_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ToResponse converts EventParticipant model to EventParticipantResponse
func (ep *EventParticipant) ToResponse() *EventParticipantResponse {
	return &EventParticipantResponse{
		ID:          ep.ID,
		EventID:     ep.EventID,
		UserID:      ep.UserID,
		Status:      ep.Status,
		IsOrganizer: ep.IsOrganizer,
		Role:        ep.Role,
		RespondedAt: ep.RespondedAt,
		CreatedAt:   ep.CreatedAt,
		UpdatedAt:   ep.UpdatedAt,
	}
}

// EventReminderResponse represents a reminder in API responses
type EventReminderResponse struct {
	ID            uint         `json:"id"`
	EventID       uint         `json:"event_id"`
	UserID        uint         `json:"user_id"`
	Type          ReminderType `json:"type"`
	TriggerTime   time.Time    `json:"trigger_time"`
	IsSent        bool         `json:"is_sent"`
	SentAt        *time.Time   `json:"sent_at,omitempty"`
	Message       string       `json:"message,omitempty"`
	MinutesBefore *int         `json:"minutes_before,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// ToResponse converts EventReminder model to EventReminderResponse
func (er *EventReminder) ToResponse() *EventReminderResponse {
	return &EventReminderResponse{
		ID:            er.ID,
		EventID:       er.EventID,
		UserID:        er.UserID,
		Type:          er.Type,
		TriggerTime:   er.TriggerTime,
		IsSent:        er.IsSent,
		SentAt:        er.SentAt,
		Message:       er.Message,
		MinutesBefore: er.MinutesBefore,
		CreatedAt:     er.CreatedAt,
		UpdatedAt:     er.UpdatedAt,
	}
}

// Filtering and Pagination Models

// EventFilterRequest represents filtering parameters for events
type EventFilterRequest struct {
	Type        *EventType `form:"type" binding:"omitempty,oneof=personal meeting deadline"`
	StartAfter  *time.Time `form:"start_after" time_format:"2006-01-02T15:04:05Z07:00"`
	StartBefore *time.Time `form:"start_before" time_format:"2006-01-02T15:04:05Z07:00"`
	EndAfter    *time.Time `form:"end_after" time_format:"2006-01-02T15:04:05Z07:00"`
	EndBefore   *time.Time `form:"end_before" time_format:"2006-01-02T15:04:05Z07:00"`
	AllDay      *bool      `form:"all_day"`
	IsPrivate   *bool      `form:"is_private"`
	IsRecurring *bool      `form:"is_recurring"`
	CreatedBy   *uint      `form:"created_by" binding:"omitempty,min=1"`
	TaskID      *uint      `form:"task_id" binding:"omitempty,min=1"`
	Search      string     `form:"search" binding:"omitempty,max=100"`
	Limit       int        `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset      int        `form:"offset" binding:"omitempty,min=0"`
	SortBy      string     `form:"sort_by" binding:"omitempty,oneof=start_time end_time created_at updated_at title"`
	SortOrder   string     `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}

// EventListResponse represents a paginated list of events
type EventListResponse struct {
	Events  []*EventResponse    `json:"events"`
	Total   int64               `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
	Filters *EventFilterRequest `json:"filters,omitempty"`
}

// CalendarViewRequest represents request for calendar view
type CalendarViewRequest struct {
	StartDate time.Time `form:"start_date" binding:"required" time_format:"2006-01-02"`
	EndDate   time.Time `form:"end_date" binding:"required" time_format:"2006-01-02"`
	ViewType  string    `form:"view_type" binding:"omitempty,oneof=day week month year" validate:"omitempty,oneof=day week month year"`
}

// EventStatsResponse represents event statistics
type EventStatsResponse struct {
	TotalEvents     int `json:"total_events"`
	PersonalEvents  int `json:"personal_events"`
	MeetingEvents   int `json:"meeting_events"`
	DeadlineEvents  int `json:"deadline_events"`
	UpcomingEvents  int `json:"upcoming_events"`
	OverdueEvents   int `json:"overdue_events"`
	EventsThisWeek  int `json:"events_this_week"`
	EventsThisMonth int `json:"events_this_month"`
}
