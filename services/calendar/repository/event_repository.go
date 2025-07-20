package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// EventRepository defines the interface for event data operations
type EventRepository interface {
	CreateEvent(event *models.Event) error
	GetEventByID(id uint) (*models.Event, error)
	GetUserEvents(userID uint, filter *models.EventFilterRequest) ([]*models.Event, int64, error)
	UpdateEvent(event *models.Event) error
	DeleteEvent(id uint) error
	GetEventsByDateRange(userID uint, startDate, endDate time.Time) ([]*models.Event, error)
	CheckTimeConflict(userID uint, startTime, endTime time.Time, excludeEventID *uint) (bool, error)
	GetEventWithParticipants(id uint) (*models.Event, error)
	GetEventWithReminders(id uint) (*models.Event, error)
	GetEventWithAll(id uint) (*models.Event, error)
	GetUpcomingEvents(userID uint, limit int) ([]*models.Event, error)
	GetOverdueEvents(userID uint) ([]*models.Event, error)
	GetEventStats(userID uint) (*models.EventStatsResponse, error)
	SearchEvents(userID uint, searchQuery string, filter *models.EventFilterRequest) ([]*models.Event, int64, error)
	GetRecurringEvents(userID uint) ([]*models.Event, error)
}

// ParticipantRepository defines the interface for participant data operations
type ParticipantRepository interface {
	AddParticipant(participant *models.EventParticipant) error
	RemoveParticipant(eventID, userID uint) error
	UpdateParticipantStatus(eventID, userID uint, status models.ParticipantStatus) error
	GetEventParticipants(eventID uint) ([]*models.EventParticipant, error)
	GetUserParticipations(userID uint) ([]*models.EventParticipant, error)
	IsParticipant(eventID, userID uint) (bool, error)
	GetParticipantStatus(eventID, userID uint) (models.ParticipantStatus, error)
}

// ReminderRepository defines the interface for reminder data operations
type ReminderRepository interface {
	CreateReminder(reminder *models.EventReminder) error
	GetEventReminders(eventID uint) ([]*models.EventReminder, error)
	GetUserReminders(userID uint) ([]*models.EventReminder, error)
	GetPendingReminders(before time.Time) ([]*models.EventReminder, error)
	MarkReminderSent(id uint) error
	DeleteReminder(id uint) error
	UpdateReminder(reminder *models.EventReminder) error
}

// eventRepository implements EventRepository interface
type eventRepository struct {
	db *database.DB
}

// participantRepository implements ParticipantRepository interface
type participantRepository struct {
	db *database.DB
}

// reminderRepository implements ReminderRepository interface
type reminderRepository struct {
	db *database.DB
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *database.DB) EventRepository {
	return &eventRepository{
		db: db,
	}
}

// NewParticipantRepository creates a new participant repository
func NewParticipantRepository(db *database.DB) ParticipantRepository {
	return &participantRepository{
		db: db,
	}
}

// NewReminderRepository creates a new reminder repository
func NewReminderRepository(db *database.DB) ReminderRepository {
	return &reminderRepository{
		db: db,
	}
}

// AddParticipant adds a participant to an event
func (r *participantRepository) AddParticipant(participant *models.EventParticipant) error {
	if participant == nil {
		return errors.New("participant cannot be nil")
	}
	return r.db.Create(participant).Error
}

func (r *participantRepository) RemoveParticipant(eventID, userID uint) error {
	return r.db.Where("event_id = ? AND user_id = ?", eventID, userID).Delete(&models.EventParticipant{}).Error
}

func (r *participantRepository) UpdateParticipantStatus(eventID, userID uint, status models.ParticipantStatus) error {
	result := r.db.Model(&models.EventParticipant{}).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("participant not found")
	}
	return nil
}

// Event Repository Methods

// CreateEvent creates a new event
func (r *eventRepository) CreateEvent(event *models.Event) error {
	if err := r.db.Create(event).Error; err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}
	return nil
}

// GetEventByID retrieves an event by ID
func (r *eventRepository) GetEventByID(id uint) (*models.Event, error) {
	var event models.Event
	err := r.db.First(&event, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Load participant count
	var participantCount int64
	r.db.Model(&models.EventParticipant{}).Where("event_id = ?", id).Count(&participantCount)
	event.ParticipantCount = int(participantCount)

	return &event, nil
}

// GetUserEvents retrieves events for a user with filtering
func (r *eventRepository) GetUserEvents(userID uint, filter *models.EventFilterRequest) ([]*models.Event, int64, error) {
	query := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')", userID, userID).
		Group("events.id")

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count user events: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var events []*models.Event
	if err := query.Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get user events: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, total, nil
}

// UpdateEvent updates an existing event
func (r *eventRepository) UpdateEvent(event *models.Event) error {
	result := r.db.Save(event)
	if result.Error != nil {
		return fmt.Errorf("failed to update event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("event not found")
	}
	return nil
}

// DeleteEvent soft deletes an event by ID
func (r *eventRepository) DeleteEvent(id uint) error {
	result := r.db.Delete(&models.Event{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("event not found")
	}
	return nil
}

// GetEventsByDateRange retrieves events within a date range for a user
func (r *eventRepository) GetEventsByDateRange(userID uint, startDate, endDate time.Time) ([]*models.Event, error) {
	var events []*models.Event

	err := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.start_time >= ? AND events.start_time <= ?",
			userID, userID, startDate, endDate).
		Group("events.id").
		Order("events.start_time ASC").
		Find(&events).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get events by date range: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, nil
}

// CheckTimeConflict checks if there's a time conflict for a user
func (r *eventRepository) CheckTimeConflict(userID uint, startTime, endTime time.Time, excludeEventID *uint) (bool, error) {
	query := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status = 'accepted'))", userID, userID).
		Where("NOT (events.end_time <= ? OR events.start_time >= ?)", startTime, endTime)

	// Exclude specific event if provided (for updates)
	if excludeEventID != nil {
		query = query.Where("events.id != ?", *excludeEventID)
	}

	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check time conflict: %w", err)
	}

	return count > 0, nil
}

// GetEventWithParticipants retrieves an event with its participants
func (r *eventRepository) GetEventWithParticipants(id uint) (*models.Event, error) {
	var event models.Event
	err := r.db.Preload("Participants").First(&event, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event with participants: %w", err)
	}

	event.ParticipantCount = len(event.Participants)
	return &event, nil
}

// GetEventWithReminders retrieves an event with its reminders
func (r *eventRepository) GetEventWithReminders(id uint) (*models.Event, error) {
	var event models.Event
	err := r.db.Preload("Reminders").First(&event, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event with reminders: %w", err)
	}

	// Load participant count
	var participantCount int64
	r.db.Model(&models.EventParticipant{}).Where("event_id = ?", id).Count(&participantCount)
	event.ParticipantCount = int(participantCount)

	return &event, nil
}

// GetEventWithAll retrieves an event with all related data
func (r *eventRepository) GetEventWithAll(id uint) (*models.Event, error) {
	var event models.Event
	err := r.db.Preload("Participants").Preload("Reminders").First(&event, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event with all data: %w", err)
	}

	event.ParticipantCount = len(event.Participants)
	return &event, nil
}

// GetUpcomingEvents retrieves upcoming events for a user
func (r *eventRepository) GetUpcomingEvents(userID uint, limit int) ([]*models.Event, error) {
	if limit <= 0 {
		limit = 10
	}

	var events []*models.Event
	now := time.Now()

	err := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.start_time > ?",
			userID, userID, now).
		Group("events.id").
		Order("events.start_time ASC").
		Limit(limit).
		Find(&events).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming events: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, nil
}

// GetOverdueEvents retrieves overdue events for a user
func (r *eventRepository) GetOverdueEvents(userID uint) ([]*models.Event, error) {
	var events []*models.Event
	now := time.Now()

	err := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.end_time < ? AND events.type = 'deadline'",
			userID, userID, now).
		Group("events.id").
		Order("events.end_time DESC").
		Find(&events).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get overdue events: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, nil
}

// GetEventStats retrieves event statistics for a user
func (r *eventRepository) GetEventStats(userID uint) (*models.EventStatsResponse, error) {
	stats := &models.EventStatsResponse{}
	now := time.Now()

	// Base query for user's events
	baseQuery := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')", userID, userID).
		Group("events.id")

	// Total events
	var totalCount int64
	if err := baseQuery.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total events: %w", err)
	}
	stats.TotalEvents = int(totalCount)

	// Events by type
	typeCounts := []struct {
		Type  models.EventType
		Count *int
	}{
		{models.EventTypePersonal, &stats.PersonalEvents},
		{models.EventTypeMeeting, &stats.MeetingEvents},
		{models.EventTypeDeadline, &stats.DeadlineEvents},
	}

	for _, tc := range typeCounts {
		var count int64
		query := r.db.Model(&models.Event{}).
			Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
			Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.type = ?",
				userID, userID, tc.Type).
			Group("events.id")
		if err := query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count events by type %s: %w", tc.Type, err)
		}
		*tc.Count = int(count)
	}

	// Upcoming events
	var upcomingCount int64
	upcomingQuery := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.start_time > ?",
			userID, userID, now).
		Group("events.id")
	if err := upcomingQuery.Count(&upcomingCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count upcoming events: %w", err)
	}
	stats.UpcomingEvents = int(upcomingCount)

	// Overdue events
	var overdueCount int64
	overdueQuery := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.end_time < ? AND events.type = 'deadline'",
			userID, userID, now).
		Group("events.id")
	if err := overdueQuery.Count(&overdueCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count overdue events: %w", err)
	}
	stats.OverdueEvents = int(overdueCount)

	// Events this week
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekEnd := weekStart.AddDate(0, 0, 7)
	var weekCount int64
	weekQuery := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.start_time >= ? AND events.start_time < ?",
			userID, userID, weekStart, weekEnd).
		Group("events.id")
	if err := weekQuery.Count(&weekCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count events this week: %w", err)
	}
	stats.EventsThisWeek = int(weekCount)

	// Events this month
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	var monthCount int64
	monthQuery := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.start_time >= ? AND events.start_time < ?",
			userID, userID, monthStart, monthEnd).
		Group("events.id")
	if err := monthQuery.Count(&monthCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count events this month: %w", err)
	}
	stats.EventsThisMonth = int(monthCount)

	return stats, nil
}

// SearchEvents searches events by title and description
func (r *eventRepository) SearchEvents(userID uint, searchQuery string, filter *models.EventFilterRequest) ([]*models.Event, int64, error) {
	query := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')", userID, userID).
		Group("events.id")

	// Add search conditions
	if searchQuery != "" {
		searchPattern := "%" + searchQuery + "%"
		query = query.Where("events.title ILIKE ? OR events.description ILIKE ?", searchPattern, searchPattern)
	}

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var events []*models.Event
	if err := query.Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search events: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, total, nil
}

// GetRecurringEvents retrieves all recurring events for a user
func (r *eventRepository) GetRecurringEvents(userID uint) ([]*models.Event, error) {
	var events []*models.Event

	err := r.db.Model(&models.Event{}).
		Joins("LEFT JOIN event_participants ON events.id = event_participants.event_id").
		Where("(events.created_by = ? OR (event_participants.user_id = ? AND event_participants.status != 'declined')) AND events.is_recurring = true",
			userID, userID).
		Group("events.id").
		Order("events.start_time ASC").
		Find(&events).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get recurring events: %w", err)
	}

	// Load participant counts and user status
	r.loadEventDetails(events, userID)

	return events, nil
}

// Helper methods

// applyFilters applies filtering conditions to the query
func (r *eventRepository) applyFilters(query *gorm.DB, filter *models.EventFilterRequest) *gorm.DB {
	if filter == nil {
		return query
	}

	if filter.Type != nil {
		query = query.Where("events.type = ?", *filter.Type)
	}

	if filter.StartAfter != nil {
		query = query.Where("events.start_time > ?", *filter.StartAfter)
	}

	if filter.StartBefore != nil {
		query = query.Where("events.start_time < ?", *filter.StartBefore)
	}

	if filter.EndAfter != nil {
		query = query.Where("events.end_time > ?", *filter.EndAfter)
	}

	if filter.EndBefore != nil {
		query = query.Where("events.end_time < ?", *filter.EndBefore)
	}

	if filter.AllDay != nil {
		query = query.Where("events.all_day = ?", *filter.AllDay)
	}

	if filter.IsPrivate != nil {
		query = query.Where("events.is_private = ?", *filter.IsPrivate)
	}

	if filter.IsRecurring != nil {
		query = query.Where("events.is_recurring = ?", *filter.IsRecurring)
	}

	if filter.CreatedBy != nil {
		query = query.Where("events.created_by = ?", *filter.CreatedBy)
	}

	if filter.TaskID != nil {
		query = query.Where("events.task_id = ?", *filter.TaskID)
	}

	// Search filter
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where("events.title ILIKE ? OR events.description ILIKE ?", searchPattern, searchPattern)
	}

	return query
}

// applySortingAndPagination applies sorting and pagination to the query
func (r *eventRepository) applySortingAndPagination(query *gorm.DB, filter *models.EventFilterRequest) *gorm.DB {
	if filter == nil {
		return query.Order("events.start_time ASC").Limit(20)
	}

	// Apply sorting
	sortBy := "events.start_time"
	sortOrder := "ASC"

	if filter.SortBy != "" {
		sortBy = "events." + filter.SortBy
	}

	if filter.SortOrder != "" {
		sortOrder = filter.SortOrder
	}

	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	return query.Limit(limit).Offset(offset)
}

// loadEventDetails loads participant counts and user status for events
func (r *eventRepository) loadEventDetails(events []*models.Event, userID uint) {
	if len(events) == 0 {
		return
	}

	eventIDs := make([]uint, len(events))
	for i, event := range events {
		eventIDs[i] = event.ID
	}

	// Get participant counts for all events in one query
	type participantCount struct {
		EventID uint
		Count   int
	}

	var counts []participantCount
	r.db.Model(&models.EventParticipant{}).
		Select("event_id, COUNT(*) as count").
		Where("event_id IN ?", eventIDs).
		Group("event_id").
		Scan(&counts)

	// Get user status for all events
	type userStatus struct {
		EventID uint
		Status  models.ParticipantStatus
	}

	var statuses []userStatus
	r.db.Model(&models.EventParticipant{}).
		Select("event_id, status").
		Where("event_id IN ? AND user_id = ?", eventIDs, userID).
		Scan(&statuses)

	// Map counts and statuses to events
	countMap := make(map[uint]int)
	for _, count := range counts {
		countMap[count.EventID] = count.Count
	}

	statusMap := make(map[uint]models.ParticipantStatus)
	for _, status := range statuses {
		statusMap[status.EventID] = status.Status
	}

	for _, event := range events {
		event.ParticipantCount = countMap[event.ID]
		if status, exists := statusMap[event.ID]; exists {
			event.UserStatus = status
		}
	}
}

func (r *participantRepository) GetEventParticipants(eventID uint) ([]*models.EventParticipant, error) {
	var participants []*models.EventParticipant
	err := r.db.Where("event_id = ?", eventID).Find(&participants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get event participants: %w", err)
	}
	return participants, nil
}

// GetUserParticipations retrieves all events where user is a participant
func (r *participantRepository) GetUserParticipations(userID uint) ([]*models.EventParticipant, error) {
	var participations []*models.EventParticipant
	err := r.db.Where("user_id = ?", userID).Find(&participations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user participations: %w", err)
	}
	return participations, nil
}

// IsParticipant checks if a user is a participant in an event
func (r *participantRepository) IsParticipant(eventID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.EventParticipant{}).
		Where("event_id = ? AND user_id = ?", eventID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if user is participant: %w", err)
	}
	return count > 0, nil
}

// GetParticipantStatus retrieves participant status for a user in an event
func (r *participantRepository) GetParticipantStatus(eventID, userID uint) (models.ParticipantStatus, error) {
	var participant models.EventParticipant
	err := r.db.Where("event_id = ? AND user_id = ?", eventID, userID).
		First(&participant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("participant not found")
		}
		return "", fmt.Errorf("failed to get participant status: %w", err)
	}
	return participant.Status, nil
}

// ReminderRepository методы (добавить недостающие)

// CreateReminder creates a new reminder
func (r *reminderRepository) CreateReminder(reminder *models.EventReminder) error {
	if reminder == nil {
		return errors.New("reminder cannot be nil")
	}
	if err := r.db.Create(reminder).Error; err != nil {
		return fmt.Errorf("failed to create reminder: %w", err)
	}
	return nil
}

// GetEventReminders retrieves all reminders for an event
func (r *reminderRepository) GetEventReminders(eventID uint) ([]*models.EventReminder, error) {
	var reminders []*models.EventReminder
	err := r.db.Where("event_id = ?", eventID).Find(&reminders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get event reminders: %w", err)
	}
	return reminders, nil
}

// GetUserReminders retrieves all reminders for a user
func (r *reminderRepository) GetUserReminders(userID uint) ([]*models.EventReminder, error) {
	var reminders []*models.EventReminder
	err := r.db.Where("user_id = ?", userID).Find(&reminders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user reminders: %w", err)
	}
	return reminders, nil
}

// GetPendingReminders retrieves reminders that need to be sent
func (r *reminderRepository) GetPendingReminders(before time.Time) ([]*models.EventReminder, error) {
	var reminders []*models.EventReminder
	err := r.db.Where("trigger_time <= ? AND is_sent = ?", before, false).
		Find(&reminders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get pending reminders: %w", err)
	}
	return reminders, nil
}

// MarkReminderSent marks a reminder as sent
func (r *reminderRepository) MarkReminderSent(id uint) error {
	now := time.Now()
	result := r.db.Model(&models.EventReminder{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_sent": true,
			"sent_at": &now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to mark reminder as sent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("reminder not found")
	}
	return nil
}

// DeleteReminder deletes a reminder
func (r *reminderRepository) DeleteReminder(id uint) error {
	result := r.db.Delete(&models.EventReminder{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete reminder: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("reminder not found")
	}
	return nil
}

// UpdateReminder updates an existing reminder
func (r *reminderRepository) UpdateReminder(reminder *models.EventReminder) error {
	if reminder == nil {
		return errors.New("reminder cannot be nil")
	}
	result := r.db.Save(reminder)
	if result.Error != nil {
		return fmt.Errorf("failed to update reminder: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("reminder not found")
	}
	return nil
}
