package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	searchclient "tachyon-messenger/services/search/client"
	"tachyon-messenger/shared/logger"

	"gorm.io/gorm"
)

// CalendarUsecase defines the interface for calendar business logic
type CalendarUsecase interface {
	CreateEvent(userID uint, req *models.CreateEventRequest) (*models.EventResponse, error)
	GetEventByID(userID, eventID uint) (*models.EventResponse, error)
	UpdateEvent(userID, eventID uint, req *models.UpdateEventRequest) (*models.EventResponse, error)
	DeleteEvent(userID, eventID uint) error
	GetUserCalendar(userID uint, startDate, endDate time.Time) (*models.EventListResponse, error)
	GetUserEvents(userID uint, filter *models.EventFilterRequest) (*models.EventListResponse, error)

	// Participant management
	InviteParticipants(userID, eventID uint, req *models.AddParticipantsRequest) error
	RemoveParticipant(userID, eventID, participantID uint) error
	UpdateParticipantStatus(userID, eventID uint, req *models.UpdateParticipantStatusRequest) error

	// Reminder management
	SetReminder(userID, eventID uint, req *models.CreateReminderRequest) (*models.EventReminderResponse, error)
	RemoveReminder(userID, eventID, reminderID uint) error

	// Additional features
	GetEventStats(userID uint) (*models.EventStatsResponse, error)
	SearchEvents(userID uint, searchQuery string, filter *models.EventFilterRequest) (*models.EventListResponse, error)
	CheckTimeConflict(userID uint, startTime, endTime time.Time, excludeEventID *uint) (bool, error)

	// Sync methods
	GetDeletedEventIDsSince(since time.Time) ([]uint, error)

	// Background processing methods
	ProcessEventReminders() error

	// Dashboard methods
	GetTodayEvents(userID uint, startTime, endTime time.Time, limit int) ([]*models.TodayEventResponse, int64, error)
}

// calendarUsecase implements CalendarUsecase interface
type calendarUsecase struct {
	eventRepo          repository.EventRepository
	participantRepo    repository.ParticipantRepository
	reminderRepo       repository.ReminderRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
	searchClient       *searchclient.SearchClient
}

// NewCalendarUsecase creates a new calendar usecase
func NewCalendarUsecase(
	eventRepo repository.EventRepository,
	participantRepo repository.ParticipantRepository,
	reminderRepo repository.ReminderRepository,
) CalendarUsecase {
	return &calendarUsecase{
		eventRepo:          eventRepo,
		participantRepo:    participantRepo,
		reminderRepo:       reminderRepo,
		notificationClient: clients.NewNotificationClient(),
		userClient:         clients.NewUserClient(),
		searchClient:       searchclient.NewSearchClient(),
	}
}

// CreateEvent creates a new event with conflict checking
func (u *calendarUsecase) CreateEvent(userID uint, req *models.CreateEventRequest) (*models.EventResponse, error) {
	// Validate request
	if err := u.validateCreateEventRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for time conflicts - DISABLED to allow overlapping events
	// hasConflict, err := u.eventRepo.CheckTimeConflict(userID, req.StartTime, req.EndTime, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to check time conflict: %w", err)
	// }
	//
	// if hasConflict {
	// 	return nil, fmt.Errorf("time conflict detected: you have another event scheduled at this time")
	// }

	// Create event model
	event := &models.Event{
		Title:          strings.TrimSpace(req.Title),
		Description:    strings.TrimSpace(req.Description),
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		AllDay:         req.AllDay,
		Location:       strings.TrimSpace(req.Location),
		CreatedBy:      userID,
		IsPrivate:      req.IsPrivate,
		IsRecurring:    req.IsRecurring,
		RecurrenceRule: strings.TrimSpace(req.RecurrenceRule),
		TaskID:         req.TaskID,
	}

	// Set type (default to personal if not provided)
	if req.Type != "" {
		event.Type = req.Type
	} else {
		event.Type = models.EventTypePersonal
	}

	// Set color (default if not provided)
	if req.Color != "" {
		event.Color = req.Color
	} else {
		event.Color = "#3788d8"
	}

	// Save event
	if err := u.eventRepo.CreateEvent(event); err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	// Add creator as organizer participant
	creatorParticipant := &models.EventParticipant{
		EventID:     event.ID,
		UserID:      userID,
		Status:      models.ParticipantStatusAccepted,
		IsOrganizer: true,
	}

	if err := u.participantRepo.AddParticipant(creatorParticipant); err != nil {
		return nil, fmt.Errorf("failed to add creator as participant: %w", err)
	}

	// Invite additional participants if provided
	if len(req.ParticipantIDs) > 0 {
		for _, participantID := range req.ParticipantIDs {
			if participantID != userID { // Skip creator
				participant := &models.EventParticipant{
					EventID: event.ID,
					UserID:  participantID,
					Status:  models.ParticipantStatusPending,
				}

				if err := u.participantRepo.AddParticipant(participant); err != nil {
					// Log error but don't fail the entire operation
					continue
				}
			}
		}
	}

	// Create custom reminders if provided
	customMinutes := make(map[int]bool)
	if len(req.Reminders) > 0 {
		for _, reminderReq := range req.Reminders {
			customMinutes[reminderReq.MinutesBefore] = true
			reminder := &models.EventReminder{
				EventID:       event.ID,
				UserID:        userID,
				Type:          reminderReq.Type,
				MinutesBefore: &reminderReq.MinutesBefore,
				Message:       strings.TrimSpace(reminderReq.Message),
			}

			// Calculate trigger time
			reminder.TriggerTime = event.StartTime.Add(-time.Duration(reminderReq.MinutesBefore) * time.Minute)

			if err := u.reminderRepo.CreateReminder(reminder); err != nil {
				// Log error but don't fail the entire operation
				continue
			}
		}
	}

	// Create default reminders (1 hour and 24 hours before) if not already set
	for _, mb := range []int{60, 1440} {
		if customMinutes[mb] {
			continue
		}
		minutesBefore := mb
		reminder := &models.EventReminder{
			EventID:       event.ID,
			UserID:        userID,
			Type:          models.ReminderTypeNotification,
			MinutesBefore: &minutesBefore,
			TriggerTime:   event.StartTime.Add(-time.Duration(minutesBefore) * time.Minute),
		}
		if err := u.reminderRepo.CreateReminder(reminder); err != nil {
			continue
		}
	}

	// Get the created event with all details
	createdEvent, err := u.eventRepo.GetEventWithAll(event.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created event: %w", err)
	}

	// Send notifications to invited participants (async, don't block)
	if len(req.ParticipantIDs) > 0 {
		go u.sendEventCreatedNotification(event, userID, req.ParticipantIDs)
	}

	// Index event in search service
	u.indexEventInSearch(createdEvent)

	return createdEvent.ToResponse(), nil
}

// GetEventByID retrieves an event by ID with access control
func (u *calendarUsecase) GetEventByID(userID, eventID uint) (*models.EventResponse, error) {
	event, err := u.eventRepo.GetEventWithAll(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Check access rights
	if !u.hasEventAccess(userID, event) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Set user status for response
	status, err := u.participantRepo.GetParticipantStatus(eventID, userID)
	if err == nil {
		event.UserStatus = status
	}

	return event.ToResponse(), nil
}

// UpdateEvent updates an existing event
func (u *calendarUsecase) UpdateEvent(userID, eventID uint, req *models.UpdateEventRequest) (*models.EventResponse, error) {
	// Validate request
	if err := u.validateUpdateEventRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing event
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Check permissions: only creator can update
	if event.CreatedBy != userID {
		return nil, fmt.Errorf("access denied: only event creator can update the event")
	}

	// Update fields if provided
	if req.Title != nil {
		event.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		event.Description = strings.TrimSpace(*req.Description)
	}
	if req.Location != nil {
		event.Location = strings.TrimSpace(*req.Location)
	}
	if req.Type != nil {
		event.Type = *req.Type
	}
	if req.Color != nil {
		event.Color = *req.Color
	}
	if req.IsPrivate != nil {
		event.IsPrivate = *req.IsPrivate
	}
	if req.IsRecurring != nil {
		event.IsRecurring = *req.IsRecurring
	}
	if req.RecurrenceRule != nil {
		event.RecurrenceRule = strings.TrimSpace(*req.RecurrenceRule)
	}
	if req.AllDay != nil {
		event.AllDay = *req.AllDay
	}

	// Track if time changed for notification
	timeChanged := req.StartTime != nil || req.EndTime != nil

	// Handle time changes - conflict checking DISABLED to allow overlapping events
	if timeChanged {
		startTime := event.StartTime
		endTime := event.EndTime

		if req.StartTime != nil {
			startTime = *req.StartTime
		}
		if req.EndTime != nil {
			endTime = *req.EndTime
		}

		// Check for time conflicts (excluding current event) - DISABLED
		// hasConflict, err := u.eventRepo.CheckTimeConflict(userID, startTime, endTime, &eventID)
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to check time conflict: %w", err)
		// }
		//
		// if hasConflict {
		// 	return nil, fmt.Errorf("time conflict detected: you have another event scheduled at this time")
		// }

		event.StartTime = startTime
		event.EndTime = endTime
	}

	// Save updated event
	if err := u.eventRepo.UpdateEvent(event); err != nil {
		return nil, fmt.Errorf("failed to update event: %w", err)
	}

	// Get updated event with all details
	updatedEvent, err := u.eventRepo.GetEventWithAll(eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated event: %w", err)
	}

	// Send update notifications to participants (async, don't block)
	go u.sendEventUpdatedNotification(event, userID, timeChanged)

	// Re-index event in search service
	u.indexEventInSearch(updatedEvent)

	return updatedEvent.ToResponse(), nil
}

// DeleteEvent deletes an event
func (u *calendarUsecase) DeleteEvent(userID, eventID uint) error {
	// Get existing event
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("event not found")
		}
		return fmt.Errorf("failed to get event: %w", err)
	}

	// Check permissions: only creator can delete
	if event.CreatedBy != userID {
		return fmt.Errorf("access denied: only event creator can delete the event")
	}

	// Send cancellation notifications before deleting (async)
	go u.sendEventCancelledNotification(event, userID)

	// Delete event (cascades to participants and reminders)
	if err := u.eventRepo.DeleteEvent(eventID); err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	// Record deletion for sync tracking
	if err := u.eventRepo.RecordDeletion(eventID, &userID); err != nil {
		// Log error but don't fail the operation - deletion was successful
		logger.WithFields(map[string]interface{}{
			"event_id": eventID,
			"user_id":  userID,
			"error":    err.Error(),
		}).Warn("Failed to record event deletion for sync")
	}

	// Remove event from search index
	u.searchClient.DeleteDocument("event", eventID)

	return nil
}

// GetUserCalendar retrieves user's calendar for a date range
func (u *calendarUsecase) GetUserCalendar(userID uint, startDate, endDate time.Time) (*models.EventListResponse, error) {
	// Validate date range
	if endDate.Before(startDate) {
		return nil, fmt.Errorf("end date cannot be before start date")
	}

	// Check for reasonable date range (e.g., max 1 year)
	if endDate.Sub(startDate) > 365*24*time.Hour {
		return nil, fmt.Errorf("date range too large: maximum 1 year allowed")
	}

	// Get events in date range
	events, err := u.eventRepo.GetEventsByDateRange(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get calendar events: %w", err)
	}

	// Convert to response format
	responses := make([]*models.EventResponse, len(events))
	for i, event := range events {
		responses[i] = event.ToResponse()
	}

	// Append birthday events
	birthdayEvents, err := u.generateBirthdayEvents(startDate, endDate)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("Failed to generate birthday events, continuing without them")
	} else {
		responses = append(responses, birthdayEvents...)
	}

	return &models.EventListResponse{
		Events: responses,
		Total:  int64(len(responses)),
		Limit:  len(responses),
		Offset: 0,
	}, nil
}

// GetUserEvents retrieves user's events with filtering
func (u *calendarUsecase) GetUserEvents(userID uint, filter *models.EventFilterRequest) (*models.EventListResponse, error) {
	// Set default pagination if not provided
	if filter == nil {
		filter = &models.EventFilterRequest{
			Limit:  20,
			Offset: 0,
		}
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Get events from repository
	events, total, err := u.eventRepo.GetUserEvents(userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get user events: %w", err)
	}

	// Convert to response format
	responses := make([]*models.EventResponse, len(events))
	for i, event := range events {
		responses[i] = event.ToResponse()
	}

	// Append birthday events if date range is specified and type filter is not set or is birthday
	if filter.Start != nil && filter.End != nil && (filter.Type == nil || *filter.Type == models.EventTypeBirthday) {
		birthdayEvents, err := u.generateBirthdayEvents(*filter.Start, *filter.End)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Failed to generate birthday events, continuing without them")
		} else {
			responses = append(responses, birthdayEvents...)
			total += int64(len(birthdayEvents))
		}
	}

	return &models.EventListResponse{
		Events:  responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		Filters: filter,
	}, nil
}

// InviteParticipants adds participants to an event
func (u *calendarUsecase) InviteParticipants(userID, eventID uint, req *models.AddParticipantsRequest) error {
	// Validate request
	if err := u.validateAddParticipantsRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get event and check permissions
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("event not found")
		}
		return fmt.Errorf("failed to get event: %w", err)
	}

	// Check permissions: only creator can invite participants
	if event.CreatedBy != userID {
		return fmt.Errorf("access denied: only event creator can invite participants")
	}

	// Track successfully added participants
	var addedParticipants []uint

	// Add participants
	for _, participantID := range req.UserIDs {
		// Check if user is already a participant
		isParticipant, err := u.participantRepo.IsParticipant(eventID, participantID)
		if err != nil {
			continue // Skip on error
		}
		if isParticipant {
			continue // Skip if already participant
		}

		participant := &models.EventParticipant{
			EventID: eventID,
			UserID:  participantID,
			Status:  models.ParticipantStatusPending,
		}

		if err := u.participantRepo.AddParticipant(participant); err != nil {
			// Log error but continue with other participants
			continue
		}

		// Track successfully added participant
		addedParticipants = append(addedParticipants, participantID)
	}

	// Send notifications to newly added participants (async)
	if len(addedParticipants) > 0 {
		go u.sendParticipantAddedNotification(event, userID, addedParticipants)
	}

	return nil
}

// RemoveParticipant removes a participant from an event
func (u *calendarUsecase) RemoveParticipant(userID, eventID, participantID uint) error {
	// Get event and check permissions
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("event not found")
		}
		return fmt.Errorf("failed to get event: %w", err)
	}

	// Check permissions: creator can remove anyone, participants can remove themselves
	if event.CreatedBy != userID && participantID != userID {
		return fmt.Errorf("access denied: insufficient permissions")
	}

	// Cannot remove the creator/organizer
	if participantID == event.CreatedBy {
		return fmt.Errorf("cannot remove event organizer")
	}

	// Remove participant
	if err := u.participantRepo.RemoveParticipant(eventID, participantID); err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	// Send notification to removed participant (async)
	// Only if they were removed by organizer (not self-removal)
	if event.CreatedBy == userID && participantID != userID {
		go u.sendParticipantRemovedNotification(event, participantID)
	}

	return nil
}

// UpdateParticipantStatus updates a participant's status for an event
func (u *calendarUsecase) UpdateParticipantStatus(userID, eventID uint, req *models.UpdateParticipantStatusRequest) error {
	// Validate request
	if err := u.validateUpdateParticipantStatusRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get event for notification
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		return fmt.Errorf("failed to get event: %w", err)
	}

	// Check if user is a participant
	isParticipant, err := u.participantRepo.IsParticipant(eventID, userID)
	if err != nil {
		return fmt.Errorf("failed to check participant status: %w", err)
	}
	if !isParticipant {
		return fmt.Errorf("user is not a participant of this event")
	}

	// Update participant status
	if err := u.participantRepo.UpdateParticipantStatus(eventID, userID, req.Status); err != nil {
		return fmt.Errorf("failed to update participant status: %w", err)
	}

	// Send notification to organizer about status change (async)
	go u.sendParticipantStatusNotification(event, userID, req.Status)

	return nil
}

// SetReminder creates a reminder for an event
func (u *calendarUsecase) SetReminder(userID, eventID uint, req *models.CreateReminderRequest) (*models.EventReminderResponse, error) {
	// Validate request
	if err := u.validateCreateReminderRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user has access to the event
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Check access rights
	if !u.hasEventAccess(userID, event) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Create reminder
	reminder := &models.EventReminder{
		EventID:       eventID,
		UserID:        userID,
		Type:          req.Type,
		MinutesBefore: &req.MinutesBefore,
		Message:       strings.TrimSpace(req.Message),
	}

	// Calculate trigger time
	reminder.TriggerTime = event.StartTime.Add(-time.Duration(req.MinutesBefore) * time.Minute)

	if err := u.reminderRepo.CreateReminder(reminder); err != nil {
		return nil, fmt.Errorf("failed to create reminder: %w", err)
	}

	return reminder.ToResponse(), nil
}

// RemoveReminder removes a reminder from an event
func (u *calendarUsecase) RemoveReminder(userID, eventID, reminderID uint) error {
	// Check if user has access to the event
	event, err := u.eventRepo.GetEventByID(eventID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("event not found")
		}
		return fmt.Errorf("failed to get event: %w", err)
	}

	// Check access rights
	if !u.hasEventAccess(userID, event) {
		return fmt.Errorf("access denied: insufficient permissions")
	}

	// TODO: Check if reminder belongs to user or if user is event creator

	// Delete reminder
	if err := u.reminderRepo.DeleteReminder(reminderID); err != nil {
		return fmt.Errorf("failed to delete reminder: %w", err)
	}

	return nil
}

// GetEventStats retrieves event statistics for a user
func (u *calendarUsecase) GetEventStats(userID uint) (*models.EventStatsResponse, error) {
	stats, err := u.eventRepo.GetEventStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get event stats: %w", err)
	}

	return stats, nil
}

// SearchEvents searches events by query
func (u *calendarUsecase) SearchEvents(userID uint, searchQuery string, filter *models.EventFilterRequest) (*models.EventListResponse, error) {
	// Validate search query
	searchQuery = strings.TrimSpace(searchQuery)
	if searchQuery == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if len(searchQuery) > 100 {
		return nil, fmt.Errorf("search query too long (max 100 characters)")
	}

	// Set default pagination
	if filter == nil {
		filter = &models.EventFilterRequest{
			Limit:  20,
			Offset: 0,
		}
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Search events
	events, total, err := u.eventRepo.SearchEvents(userID, searchQuery, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search events: %w", err)
	}

	// Convert to response format
	responses := make([]*models.EventResponse, len(events))
	for i, event := range events {
		responses[i] = event.ToResponse()
	}

	return &models.EventListResponse{
		Events:  responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		Filters: filter,
	}, nil
}

// CheckTimeConflict checks if there's a time conflict for a user
func (u *calendarUsecase) CheckTimeConflict(userID uint, startTime, endTime time.Time, excludeEventID *uint) (bool, error) {
	// Validate time range
	if endTime.Before(startTime) || endTime.Equal(startTime) {
		return false, fmt.Errorf("end time must be after start time")
	}

	return u.eventRepo.CheckTimeConflict(userID, startTime, endTime, excludeEventID)
}

// generateBirthdayEvents generates virtual birthday events for users whose birthdays
// fall within the given date range. Events are not stored in DB.
func (u *calendarUsecase) generateBirthdayEvents(startDate, endDate time.Time) ([]*models.EventResponse, error) {
	users, err := u.userClient.GetUsersWithBirthdays()
	if err != nil {
		return nil, fmt.Errorf("failed to get users with birthdays: %w", err)
	}

	var events []*models.EventResponse

	for _, user := range users {
		birthDate, err := time.Parse("2006-01-02", user.BirthDate)
		if err != nil {
			continue
		}

		birthMonth := birthDate.Month()
		birthDay := birthDate.Day()

		// Check each year in the range
		for year := startDate.Year(); year <= endDate.Year(); year++ {
			birthdayThisYear := time.Date(year, birthMonth, birthDay, 0, 0, 0, 0, startDate.Location())

			// Check if this birthday falls within the requested range
			if birthdayThisYear.Before(startDate) || birthdayThisYear.After(endDate) {
				continue
			}

			// Build display name
			displayName := user.Name
			if user.FirstName != "" && user.LastName != "" {
				displayName = user.FirstName + " " + user.LastName
			}

			birthdayEnd := time.Date(year, birthMonth, birthDay, 23, 59, 59, 0, startDate.Location())

			event := &models.EventResponse{
				ID:        uint(user.ID*100000) + uint(year),
				Title:     "День рождения — " + displayName,
				StartTime: birthdayThisYear,
				EndTime:   birthdayEnd,
				AllDay:    true,
				Type:      models.EventTypeBirthday,
				Color:     "#E91E63",
				CreatedBy: user.ID,
				CreatedAt: birthdayThisYear,
				UpdatedAt: birthdayThisYear,
			}

			events = append(events, event)
		}
	}

	return events, nil
}

// Helper methods

// hasEventAccess checks if user has access to the event
func (u *calendarUsecase) hasEventAccess(userID uint, event *models.Event) bool {
	// User is creator
	if event.CreatedBy == userID {
		return true
	}

	// Check if user is a participant (non-declined)
	isParticipant, err := u.participantRepo.IsParticipant(event.ID, userID)
	if err != nil {
		return false
	}

	// If event is private, only participants can access
	if event.IsPrivate {
		return isParticipant
	}

	// Public events can be viewed by anyone (for now)
	return true
}

// Validation methods

// validateCreateEventRequest validates event creation request
func (u *calendarUsecase) validateCreateEventRequest(req *models.CreateEventRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate title
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return fmt.Errorf("event title is required")
	}
	if len(title) > 255 {
		return fmt.Errorf("event title must be less than 255 characters")
	}

	// Validate description if provided
	if req.Description != "" {
		description := strings.TrimSpace(req.Description)
		if len(description) > 2000 {
			return fmt.Errorf("event description must be less than 2000 characters")
		}
	}

	// Validate time logic
	if req.EndTime.Before(req.StartTime) || req.EndTime.Equal(req.StartTime) {
		return fmt.Errorf("end time must be after start time")
	}

	// Validate location if provided
	if req.Location != "" {
		location := strings.TrimSpace(req.Location)
		if len(location) > 500 {
			return fmt.Errorf("location must be less than 500 characters")
		}
	}

	// Validate type if provided
	if req.Type != "" {
		if !u.isValidEventType(req.Type) {
			return fmt.Errorf("invalid event type")
		}
	}

	// Validate color if provided
	if req.Color != "" {
		if len(req.Color) != 7 || req.Color[0] != '#' {
			return fmt.Errorf("color must be a valid hex color code (e.g., #3788d8)")
		}
	}

	// Validate that start time is not in the past (except for all-day events)
	if !req.AllDay && req.StartTime.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("start time cannot be in the past")
	}

	return nil
}

// validateUpdateEventRequest validates event update request
func (u *calendarUsecase) validateUpdateEventRequest(req *models.UpdateEventRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate title if provided
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return fmt.Errorf("event title cannot be empty")
		}
		if len(title) > 255 {
			return fmt.Errorf("event title must be less than 255 characters")
		}
	}

	// Validate time logic if both times are provided
	if req.StartTime != nil && req.EndTime != nil {
		if req.EndTime.Before(*req.StartTime) || req.EndTime.Equal(*req.StartTime) {
			return fmt.Errorf("end time must be after start time")
		}
	}

	return nil
}

// validateAddParticipantsRequest validates add participants request
func (u *calendarUsecase) validateAddParticipantsRequest(req *models.AddParticipantsRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if len(req.UserIDs) == 0 {
		return fmt.Errorf("at least one user ID is required")
	}

	for _, userID := range req.UserIDs {
		if userID == 0 {
			return fmt.Errorf("invalid user ID")
		}
	}

	return nil
}

// validateUpdateParticipantStatusRequest validates participant status update request
func (u *calendarUsecase) validateUpdateParticipantStatusRequest(req *models.UpdateParticipantStatusRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if !u.isValidParticipantStatus(req.Status) {
		return fmt.Errorf("invalid participant status")
	}

	return nil
}

// validateCreateReminderRequest validates reminder creation request
func (u *calendarUsecase) validateCreateReminderRequest(req *models.CreateReminderRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate reminder type
	if !u.isValidReminderType(req.Type) {
		return fmt.Errorf("invalid reminder type")
	}

	// Validate minutes before
	if req.MinutesBefore < 0 {
		return fmt.Errorf("minutes before cannot be negative")
	}
	if req.MinutesBefore > 43200 { // 30 days
		return fmt.Errorf("minutes before cannot exceed 30 days (43200 minutes)")
	}

	return nil
}

// Helper validation methods

// isValidEventType checks if the event type is valid
func (u *calendarUsecase) isValidEventType(eventType models.EventType) bool {
	switch eventType {
	case models.EventTypePersonal, models.EventTypeMeeting, models.EventTypeDeadline, models.EventTypeSchedule, models.EventTypeAbsence, models.EventTypeBirthday:
		return true
	default:
		return false
	}
}

// isValidParticipantStatus checks if the participant status is valid
func (u *calendarUsecase) isValidParticipantStatus(status models.ParticipantStatus) bool {
	switch status {
	case models.ParticipantStatusPending, models.ParticipantStatusAccepted,
		models.ParticipantStatusDeclined, models.ParticipantStatusMaybe:
		return true
	default:
		return false
	}
}

// isValidReminderType checks if the reminder type is valid
func (u *calendarUsecase) isValidReminderType(reminderType models.ReminderType) bool {
	switch reminderType {
	case models.ReminderTypeEmail, models.ReminderTypeNotification, models.ReminderTypeSMS:
		return true
	default:
		return false
	}
}

// GetDeletedEventIDsSince returns IDs of events deleted since the given timestamp
func (u *calendarUsecase) GetDeletedEventIDsSince(since time.Time) ([]uint, error) {
	return u.eventRepo.GetDeletedEventIDsSince(since)
}

// GetTodayEvents retrieves events for a user within a date range (for dashboard)
func (u *calendarUsecase) GetTodayEvents(userID uint, startTime, endTime time.Time, limit int) ([]*models.TodayEventResponse, int64, error) {
	events, err := u.eventRepo.GetEventsByDateRange(userID, startTime, endTime)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get today's events: %w", err)
	}

	total := int64(len(events))

	// Apply limit
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}

	// Convert to TodayEventResponse
	responses := make([]*models.TodayEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, &models.TodayEventResponse{
			ID:          event.ID,
			Title:       event.Title,
			Description: event.Description,
			StartTime:   event.StartTime.Format(time.RFC3339),
			EndTime:     event.EndTime.Format(time.RFC3339),
			AllDay:      event.AllDay,
			Location:    event.Location,
			Type:        string(event.Type),
			Color:       event.Color,
			IsPrivate:   event.IsPrivate,
		})
	}

	return responses, total, nil
}

// indexEventInSearch indexes or updates an event in the search service
func (u *calendarUsecase) indexEventInSearch(event *models.Event) {
	accessibleBy := []uint{event.CreatedBy}

	// Add participant user IDs
	for _, p := range event.Participants {
		if p.UserID != event.CreatedBy {
			accessibleBy = append(accessibleBy, p.UserID)
		}
	}

	metadata := map[string]interface{}{
		"type":     string(event.Type),
		"location": event.Location,
	}
	if event.AllDay {
		metadata["all_day"] = true
	}
	if creatorInfo, err := u.userClient.GetUserByID(event.CreatedBy); err == nil {
		metadata["creator_name"] = creatorInfo.Name
		metadata["creator_avatar"] = creatorInfo.Avatar
	}

	content := event.Description
	if event.Location != "" {
		content += " " + event.Location
	}

	u.searchClient.IndexDocument(&searchclient.IndexRequest{
		EntityType:   "event",
		EntityID:     event.ID,
		Title:        event.Title,
		Content:      content,
		Metadata:     metadata,
		AccessibleBy: accessibleBy,
		IsPublic:     !event.IsPrivate,
		CreatorID:    event.CreatedBy,
	})
}
