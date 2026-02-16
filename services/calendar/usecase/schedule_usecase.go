package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	sharedmodels "tachyon-messenger/shared/models"
)

// ScheduleUsecase defines the interface for schedule business logic
type ScheduleUsecase interface {
	// Schedule management
	CreateSchedule(userID uint, req *models.CreateScheduleRequest) (*models.ScheduleResponse, error)
	GetScheduleByID(userID, scheduleID uint) (*models.ScheduleResponse, error)
	GetSchedules(userID uint, userRole sharedmodels.Role, filter ScheduleFilterParams) (*models.ScheduleListResponse, error)
	UpdateSchedule(userID, scheduleID uint, req *models.UpdateScheduleRequest) (*models.ScheduleResponse, error)
	DeleteSchedule(userID, scheduleID uint) error

	// Schedule entry management
	CreateScheduleEntry(userID, scheduleID uint, req *models.CreateScheduleEntryRequest) (*models.ScheduleEntryResponse, error)
	CreateScheduleEntries(userID, scheduleID uint, req *models.BatchCreateScheduleEntriesRequest) ([]*models.ScheduleEntryResponse, error)
	GetScheduleEntries(userID, scheduleID uint, filter EntryFilterParams) (*models.ScheduleEntryListResponse, error)
	UpdateScheduleEntry(userID, scheduleID, entryID uint, req *models.UpdateScheduleEntryRequest) (*models.ScheduleEntryResponse, error)
	DeleteScheduleEntry(userID, scheduleID, entryID uint) error
	GetMyScheduleEntries(userID uint, startDate, endDate time.Time) ([]*models.ScheduleEntryResponse, error)

	// Daily summary
	GetDailySummary(date time.Time) (*models.DailySummaryResponse, error)

	// Group members
	GetScheduleGroupMembers(scheduleID uint) ([]*clients.UserGroupMember, error)

	// Permission check
	CanViewSchedule(userID, scheduleID uint, userRole sharedmodels.Role) (bool, error)
	CanEditSchedule(userID, scheduleID uint, userRole sharedmodels.Role) (bool, error)
}

// ScheduleFilterParams represents filtering parameters
type ScheduleFilterParams struct {
	Type         *models.ScheduleType
	IsActive     *bool
	DepartmentID *uint
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}

// EntryFilterParams represents entry filtering parameters
type EntryFilterParams struct {
	UserID    *uint
	StartDate *time.Time
	EndDate   *time.Time
	ShiftType *models.ShiftType
	Limit     int
	Offset    int
}

// scheduleUsecase implements ScheduleUsecase interface
type scheduleUsecase struct {
	scheduleRepo       repository.ScheduleRepository
	eventRepo          repository.EventRepository
	absenceRepo        repository.AbsenceRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
}

// NewScheduleUsecase creates a new schedule usecase
func NewScheduleUsecase(
	scheduleRepo repository.ScheduleRepository,
	eventRepo repository.EventRepository,
	absenceRepo repository.AbsenceRepository,
) ScheduleUsecase {
	return &scheduleUsecase{
		scheduleRepo:       scheduleRepo,
		eventRepo:          eventRepo,
		absenceRepo:        absenceRepo,
		notificationClient: clients.NewNotificationClient(),
		userClient:         clients.NewUserClient(),
	}
}

// CreateSchedule creates a new schedule
func (u *scheduleUsecase) CreateSchedule(userID uint, req *models.CreateScheduleRequest) (*models.ScheduleResponse, error) {
	// Validate request
	if err := u.validateCreateScheduleRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create schedule model
	schedule := &models.Schedule{
		Title:         strings.TrimSpace(req.Title),
		Description:   strings.TrimSpace(req.Description),
		Type:          req.Type,
		CreatedBy:     userID,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		IsForAllUsers: req.IsForAllUsers,
		DepartmentID:  req.DepartmentID,
		UserGroupID:   req.UserGroupID,
		TemplateID:    req.TemplateID,
		IsActive:      true,
	}

	// Set visibility (default to management)
	if req.Visibility != "" {
		schedule.Visibility = req.Visibility
	} else {
		schedule.Visibility = models.VisibilityManagement
	}

	// Set edit permission (default to creator_only)
	if req.EditPermission != "" {
		schedule.EditPermission = req.EditPermission
	} else {
		schedule.EditPermission = models.EditPermissionCreatorOnly
	}

	// Set color (default if not provided)
	if req.Color != "" {
		schedule.Color = req.Color
	} else {
		schedule.Color = "#4CAF50"
	}

	// Set shift times
	if req.MorningStart != "" {
		schedule.MorningStart = req.MorningStart
	}
	if req.MorningEnd != "" {
		schedule.MorningEnd = req.MorningEnd
	}
	if req.EveningStart != "" {
		schedule.EveningStart = req.EveningStart
	}
	if req.EveningEnd != "" {
		schedule.EveningEnd = req.EveningEnd
	}

	// Set mode if provided
	if req.Mode != nil {
		schedule.Mode = *req.Mode
	}

	// Save schedule
	if err := u.scheduleRepo.CreateSchedule(schedule); err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	// Set viewers if provided (for specific_users visibility)
	if len(req.ViewerIDs) > 0 {
		if err := u.scheduleRepo.SetScheduleViewers(schedule.ID, req.ViewerIDs); err != nil {
			return nil, fmt.Errorf("failed to set schedule viewers: %w", err)
		}
	}

	// Set editors if provided (for specific_users edit_permission)
	if len(req.EditorIDs) > 0 {
		if err := u.scheduleRepo.SetScheduleEditors(schedule.ID, req.EditorIDs); err != nil {
			return nil, fmt.Errorf("failed to set schedule editors: %w", err)
		}
	}

	// For recurring schedules, auto-create a template if not provided
	if schedule.Mode == models.ScheduleModeRecurring && schedule.TemplateID == nil {
		template := &models.ScheduleTemplate{
			Title:        schedule.Title + " (шаблон)",
			Description:  "Автоматически созданный шаблон для повторяющегося графика",
			Type:         schedule.Type,
			CreatedBy:    userID,
			DepartmentID: schedule.DepartmentID,
			IsActive:     true,
		}
		if err := u.scheduleRepo.CreateScheduleTemplate(template); err != nil {
			return nil, fmt.Errorf("failed to create template for recurring schedule: %w", err)
		}
		// Link template to schedule
		schedule.TemplateID = &template.ID
		if err := u.scheduleRepo.UpdateSchedule(schedule); err != nil {
			return nil, fmt.Errorf("failed to link template to schedule: %w", err)
		}
	}

	// Get schedule with creator info
	createdSchedule, err := u.scheduleRepo.GetScheduleByID(schedule.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created schedule: %w", err)
	}

	// Send notifications to participants asynchronously
	go u.sendScheduleCreatedNotification(createdSchedule, userID)

	return createdSchedule.ToResponse(), nil
}

// GetScheduleByID retrieves a schedule by ID with permission check
func (u *scheduleUsecase) GetScheduleByID(userID, scheduleID uint) (*models.ScheduleResponse, error) {
	schedule, err := u.scheduleRepo.GetScheduleWithPermissions(scheduleID)
	if err != nil {
		return nil, err
	}

	// Auto-create template for existing recurring schedules without one
	if schedule.Mode == models.ScheduleModeRecurring && schedule.TemplateID == nil {
		template := &models.ScheduleTemplate{
			Title:        schedule.Title + " (шаблон)",
			Description:  "Автоматически созданный шаблон для повторяющегося графика",
			Type:         schedule.Type,
			CreatedBy:    schedule.CreatedBy,
			DepartmentID: schedule.DepartmentID,
			IsActive:     true,
		}
		if err := u.scheduleRepo.CreateScheduleTemplate(template); err != nil {
			return nil, fmt.Errorf("failed to create template for recurring schedule: %w", err)
		}
		schedule.TemplateID = &template.ID
		schedule.Template = template
		if err := u.scheduleRepo.UpdateSchedule(schedule); err != nil {
			return nil, fmt.Errorf("failed to link template to schedule: %w", err)
		}
	}

	return schedule.ToResponse(), nil
}

// GetSchedules retrieves schedules based on user permissions
func (u *scheduleUsecase) GetSchedules(userID uint, userRole sharedmodels.Role, filter ScheduleFilterParams) (*models.ScheduleListResponse, error) {
	// Convert to repository filter
	repoFilter := repository.ScheduleFilter{
		Type:         filter.Type,
		IsActive:     filter.IsActive,
		DepartmentID: filter.DepartmentID,
		StartDate:    filter.StartDate,
		EndDate:      filter.EndDate,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
	}

	schedules, _, err := u.scheduleRepo.GetSchedules(repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	// Filter schedules by visibility
	var filteredSchedules []*models.Schedule
	for _, schedule := range schedules {
		canView, err := u.canViewScheduleObj(userID, schedule, userRole)
		if err != nil {
			continue
		}
		if canView {
			filteredSchedules = append(filteredSchedules, schedule)
		}
	}

	// Convert to responses
	responses := make([]*models.ScheduleResponse, len(filteredSchedules))
	for i, schedule := range filteredSchedules {
		responses[i] = schedule.ToResponse()
	}

	return &models.ScheduleListResponse{
		Schedules: responses,
		Total:     int64(len(filteredSchedules)),
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}, nil
}

// UpdateSchedule updates an existing schedule
func (u *scheduleUsecase) UpdateSchedule(userID, scheduleID uint, req *models.UpdateScheduleRequest) (*models.ScheduleResponse, error) {
	// Get existing schedule
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Title != nil {
		schedule.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		schedule.Description = strings.TrimSpace(*req.Description)
	}
	if req.Type != nil {
		schedule.Type = *req.Type
	}
	if req.Visibility != nil {
		schedule.Visibility = *req.Visibility
	}
	if req.EditPermission != nil {
		schedule.EditPermission = *req.EditPermission
	}
	if req.StartDate != nil {
		schedule.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		schedule.EndDate = *req.EndDate
	}
	if req.IsForAllUsers != nil {
		schedule.IsForAllUsers = *req.IsForAllUsers
	}
	if req.DepartmentID != nil {
		schedule.DepartmentID = req.DepartmentID
	}
	if req.UserGroupID != nil {
		schedule.UserGroupID = req.UserGroupID
	}
	if req.Color != nil {
		schedule.Color = *req.Color
	}
	if req.IsActive != nil {
		schedule.IsActive = *req.IsActive
	}
	if req.MorningStart != nil {
		schedule.MorningStart = *req.MorningStart
	}
	if req.MorningEnd != nil {
		schedule.MorningEnd = *req.MorningEnd
	}
	if req.EveningStart != nil {
		schedule.EveningStart = *req.EveningStart
	}
	if req.EveningEnd != nil {
		schedule.EveningEnd = *req.EveningEnd
	}
	if req.Mode != nil {
		schedule.Mode = *req.Mode
	}

	// Save updated schedule
	if err := u.scheduleRepo.UpdateSchedule(schedule); err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	// Update viewers if provided
	if req.ViewerIDs != nil {
		if err := u.scheduleRepo.SetScheduleViewers(scheduleID, *req.ViewerIDs); err != nil {
			return nil, fmt.Errorf("failed to update schedule viewers: %w", err)
		}
	}

	// Update editors if provided
	if req.EditorIDs != nil {
		if err := u.scheduleRepo.SetScheduleEditors(scheduleID, *req.EditorIDs); err != nil {
			return nil, fmt.Errorf("failed to update schedule editors: %w", err)
		}
	}

	// Get updated schedule with creator info and permissions
	updatedSchedule, err := u.scheduleRepo.GetScheduleWithPermissions(schedule.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated schedule: %w", err)
	}

	return updatedSchedule.ToResponse(), nil
}

// DeleteSchedule deletes a schedule
func (u *scheduleUsecase) DeleteSchedule(userID, scheduleID uint) error {
	// Get schedule to check permissions
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return err
	}

	// Get all entries for this schedule to delete associated events
	entries, _, err := u.scheduleRepo.GetScheduleEntries(scheduleID, repository.EntryFilter{})
	if err != nil {
		return fmt.Errorf("failed to get schedule entries: %w", err)
	}

	// Delete associated events
	for _, entry := range entries {
		if entry.EventID != nil {
			if err := u.eventRepo.DeleteEvent(*entry.EventID); err != nil {
				// Log error but continue
				continue
			}
		}
	}

	// Delete schedule
	if err := u.scheduleRepo.DeleteSchedule(schedule.ID); err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	return nil
}

// CreateScheduleEntry creates a new schedule entry and associated calendar event
func (u *scheduleUsecase) CreateScheduleEntry(userID, scheduleID uint, req *models.CreateScheduleEntryRequest) (*models.ScheduleEntryResponse, error) {
	// Get schedule
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return nil, err
	}

	// Check if user is absent on this date
	isAbsent, absence, err := u.absenceRepo.IsUserAbsent(req.UserID, req.Date)
	if err != nil {
		return nil, fmt.Errorf("failed to check absence: %w", err)
	}
	if isAbsent {
		return nil, fmt.Errorf("пользователь в отсутствии (%s) с %s по %s",
			GetAbsenceTypeName(absence.Type),
			absence.StartDate.Format("02.01"),
			absence.EndDate.Format("02.01"))
	}

	// Calculate start and end times based on shift type
	startTime, endTime, err := u.calculateShiftTimes(schedule, req.Date, req.ShiftType, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}

	// Check for schedule conflicts with incompatible schedule types
	conflictingEntries, err := u.scheduleRepo.GetConflictingEntries(req.UserID, req.Date, startTime, endTime, schedule.Type, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check schedule conflict: %w", err)
	}
	if len(conflictingEntries) > 0 {
		conflict := conflictingEntries[0]
		return nil, fmt.Errorf("пользователь уже стоит в графике \"%s\" с %s до %s",
			conflict.Schedule.Title,
			conflict.StartTime.Format("15:04"),
			conflict.EndTime.Format("15:04"))
	}

	// Create schedule entry
	entry := &models.ScheduleEntry{
		ScheduleID:  scheduleID,
		UserID:      req.UserID,
		Date:        req.Date,
		ShiftType:   req.ShiftType,
		StartTime:   startTime,
		EndTime:     endTime,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Location:    strings.TrimSpace(req.Location),
		CreatedBy:   userID,
	}

	// Save schedule entry
	if err := u.scheduleRepo.CreateScheduleEntry(entry); err != nil {
		return nil, fmt.Errorf("failed to create schedule entry: %w", err)
	}

	// Create associated calendar event (skip for recurring schedules)
	if schedule.Mode != models.ScheduleModeRecurring {
		event, err := u.createEventForScheduleEntry(schedule, entry)
		if err != nil {
			// Log error but don't fail the entry creation
			// The entry can exist without an event
		} else {
			entry.EventID = &event.ID
			if err := u.scheduleRepo.UpdateScheduleEntry(entry); err != nil {
				// Log error
			}
		}
	}

	// Get created entry with relations
	createdEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created entry: %w", err)
	}

	// Send notification to the user asynchronously
	go u.sendScheduleEntryNotification(schedule, createdEntry, userID)

	return createdEntry.ToResponse(), nil
}

// CreateScheduleEntries creates multiple schedule entries in batch
func (u *scheduleUsecase) CreateScheduleEntries(userID, scheduleID uint, req *models.BatchCreateScheduleEntriesRequest) ([]*models.ScheduleEntryResponse, error) {
	// Get schedule
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return nil, err
	}

	entries := make([]*models.ScheduleEntry, 0, len(req.Entries))
	responses := make([]*models.ScheduleEntryResponse, 0, len(req.Entries))

	// Collect all user IDs and date range
	var userIDs []uint
	var minDate, maxDate time.Time
	userIDSet := make(map[uint]bool)

	for _, entryReq := range req.Entries {
		if !userIDSet[entryReq.UserID] {
			userIDs = append(userIDs, entryReq.UserID)
			userIDSet[entryReq.UserID] = true
		}
		if minDate.IsZero() || entryReq.Date.Before(minDate) {
			minDate = entryReq.Date
		}
		if maxDate.IsZero() || entryReq.Date.After(maxDate) {
			maxDate = entryReq.Date
		}
	}

	// Get absences for all users in the date range
	absenceMap, err := u.absenceRepo.GetAbsentUsersForPeriod(userIDs, minDate, maxDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	for _, entryReq := range req.Entries {
		// Check if user is absent on this date
		if userAbsences, ok := absenceMap[entryReq.UserID]; ok {
			isAbsent := false
			for _, absence := range userAbsences {
				if !entryReq.Date.Before(absence.StartDate) && !entryReq.Date.After(absence.EndDate) {
					isAbsent = true
					break
				}
			}
			if isAbsent {
				continue // Skip entries for absent users
			}
		}

		// Calculate shift times
		startTime, endTime, err := u.calculateShiftTimes(schedule, entryReq.Date, entryReq.ShiftType, entryReq.StartTime, entryReq.EndTime)
		if err != nil {
			continue // Skip invalid entries
		}

		// Check for schedule conflicts with incompatible schedule types
		conflictingEntries, err := u.scheduleRepo.GetConflictingEntries(entryReq.UserID, entryReq.Date, startTime, endTime, schedule.Type, nil)
		if err != nil {
			continue // Skip on error
		}
		if len(conflictingEntries) > 0 {
			continue // Skip entries with conflicts
		}

		// Create entry
		entry := &models.ScheduleEntry{
			ScheduleID:  scheduleID,
			UserID:      entryReq.UserID,
			Date:        entryReq.Date,
			ShiftType:   entryReq.ShiftType,
			StartTime:   startTime,
			EndTime:     endTime,
			Title:       strings.TrimSpace(entryReq.Title),
			Description: strings.TrimSpace(entryReq.Description),
			Location:    strings.TrimSpace(entryReq.Location),
			CreatedBy:   userID,
		}

		entries = append(entries, entry)
	}

	// Save all entries
	if err := u.scheduleRepo.CreateScheduleEntries(entries); err != nil {
		return nil, fmt.Errorf("failed to create schedule entries: %w", err)
	}

	// Create calendar events for each entry (skip for recurring schedules)
	for _, entry := range entries {
		if schedule.Mode != models.ScheduleModeRecurring {
			event, err := u.createEventForScheduleEntry(schedule, entry)
			if err == nil {
				entry.EventID = &event.ID
				u.scheduleRepo.UpdateScheduleEntry(entry)
			}
		}

		// Get entry with relations
		createdEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
		if err == nil {
			responses = append(responses, createdEntry.ToResponse())
		}
	}

	// Send notifications to all affected users asynchronously
	go u.sendBatchScheduleEntryNotifications(schedule, entries, userID)

	return responses, nil
}

// GetScheduleEntries retrieves entries for a schedule
func (u *scheduleUsecase) GetScheduleEntries(userID, scheduleID uint, filter EntryFilterParams) (*models.ScheduleEntryListResponse, error) {
	// Convert to repository filter
	repoFilter := repository.EntryFilter{
		UserID:    filter.UserID,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		ShiftType: filter.ShiftType,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}

	entries, total, err := u.scheduleRepo.GetScheduleEntries(scheduleID, repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule entries: %w", err)
	}

	// Convert to responses
	responses := make([]*models.ScheduleEntryResponse, len(entries))
	for i, entry := range entries {
		responses[i] = entry.ToResponse()
	}

	return &models.ScheduleEntryListResponse{
		Entries: responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}, nil
}

// UpdateScheduleEntry updates an existing schedule entry
func (u *scheduleUsecase) UpdateScheduleEntry(userID, scheduleID, entryID uint, req *models.UpdateScheduleEntryRequest) (*models.ScheduleEntryResponse, error) {
	// Get existing entry
	entry, err := u.scheduleRepo.GetScheduleEntry(entryID)
	if err != nil {
		return nil, err
	}

	// Verify entry belongs to the schedule
	if entry.ScheduleID != scheduleID {
		return nil, errors.New("entry does not belong to this schedule")
	}

	// Get schedule for time calculations
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return nil, err
	}

	// Save old values for notification
	oldEntry := &models.ScheduleEntry{
		UserID:    entry.UserID,
		Date:      entry.Date,
		StartTime: entry.StartTime,
		EndTime:   entry.EndTime,
		ShiftType: entry.ShiftType,
	}

	// Track if user changed for event update
	userChanged := false
	oldUserID := entry.UserID

	// Update fields if provided
	if req.UserID != nil && *req.UserID != entry.UserID {
		entry.UserID = *req.UserID
		userChanged = true
		// Clear the User relation so it will be reloaded after save
		entry.User = nil
	}
	if req.Date != nil {
		entry.Date = *req.Date
	}
	if req.ShiftType != nil {
		entry.ShiftType = *req.ShiftType
	}
	if req.Title != nil {
		entry.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		entry.Description = strings.TrimSpace(*req.Description)
	}
	if req.Location != nil {
		entry.Location = strings.TrimSpace(*req.Location)
	}

	// Recalculate times if shift type, date, or custom times changed
	if req.ShiftType != nil || req.Date != nil || req.StartTime != nil || req.EndTime != nil {
		startTime, endTime, err := u.calculateShiftTimes(schedule, entry.Date, entry.ShiftType, req.StartTime, req.EndTime)
		if err != nil {
			return nil, err
		}
		entry.StartTime = startTime
		entry.EndTime = endTime
	}

	// Save updated entry - use Select to explicitly update user_id
	if err := u.scheduleRepo.UpdateScheduleEntryFields(entry, userChanged); err != nil {
		return nil, fmt.Errorf("failed to update schedule entry: %w", err)
	}

	// If user changed, recreate the event for the new user
	if userChanged && entry.EventID != nil {
		// Delete old event (it belonged to the old user)
		if err := u.eventRepo.DeleteEvent(*entry.EventID); err != nil {
			// Log error but continue
		}
		entry.EventID = nil

		// Create new event for the new user
		newEvent, err := u.createEventForScheduleEntry(schedule, entry)
		if err == nil && newEvent != nil {
			entry.EventID = &newEvent.ID
			// Update entry with new event ID
			if err := u.scheduleRepo.UpdateScheduleEntryFields(entry, false); err != nil {
				// Log error but continue
			}
		}
	} else if entry.EventID != nil {
		// Regular update - just update existing event
		event, err := u.eventRepo.GetEventByID(*entry.EventID)
		if err == nil {
			event.StartTime = entry.StartTime
			event.EndTime = entry.EndTime
			event.Title = u.generateEventTitle(schedule, entry)
			event.Description = entry.Description
			event.Location = entry.Location
			u.eventRepo.UpdateEvent(event)
		}
	} else if userChanged {
		// User changed but there was no event - create one for the new user
		newEvent, err := u.createEventForScheduleEntry(schedule, entry)
		if err == nil && newEvent != nil {
			entry.EventID = &newEvent.ID
			// Update entry with new event ID
			if err := u.scheduleRepo.UpdateScheduleEntryFields(entry, false); err != nil {
				// Log error but continue
			}
		}
	}

	_ = oldUserID // Used in notification

	// Get updated entry with relations
	updatedEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated entry: %w", err)
	}

	// Send notification about the change asynchronously
	go u.sendScheduleEntryUpdatedNotification(schedule, oldEntry, updatedEntry, userID)

	return updatedEntry.ToResponse(), nil
}

// DeleteScheduleEntry deletes a schedule entry
func (u *scheduleUsecase) DeleteScheduleEntry(userID, scheduleID, entryID uint) error {
	// Get entry
	entry, err := u.scheduleRepo.GetScheduleEntry(entryID)
	if err != nil {
		return err
	}

	// Verify entry belongs to the schedule
	if entry.ScheduleID != scheduleID {
		return errors.New("entry does not belong to this schedule")
	}

	// Get schedule for notification
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return err
	}

	// Delete associated event if exists
	if entry.EventID != nil {
		if err := u.eventRepo.DeleteEvent(*entry.EventID); err != nil {
			// Log error but continue
		}
	}

	// Send notification about cancellation asynchronously (before deletion)
	go u.sendScheduleEntryCancelledNotification(schedule, entry, userID)

	// Delete entry
	if err := u.scheduleRepo.DeleteScheduleEntry(entryID); err != nil {
		return fmt.Errorf("failed to delete schedule entry: %w", err)
	}

	return nil
}

// GetMyScheduleEntries retrieves schedule entries for a user
func (u *scheduleUsecase) GetMyScheduleEntries(userID uint, startDate, endDate time.Time) ([]*models.ScheduleEntryResponse, error) {
	// Get recurring schedules for this user and ensure entries are generated
	recurringSchedules, err := u.scheduleRepo.GetRecurringSchedulesForUser(userID)
	if err != nil {
		// Log but don't fail - continue with regular entries
	} else {
		for _, schedule := range recurringSchedules {
			if schedule.TemplateID != nil {
				if err := u.ensureEntriesGenerated(schedule, userID, startDate, endDate); err != nil {
					// Log but continue
				}
			}
		}
	}

	// Get all entries for the user in the date range
	entries, err := u.scheduleRepo.GetUserScheduleEntries(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user schedule entries: %w", err)
	}

	// Get user absences for the period
	absences, err := u.absenceRepo.GetUserAbsences(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user absences: %w", err)
	}

	// Filter out entries on absence days
	filteredEntries := u.filterEntriesByAbsences(entries, absences)

	// Convert to responses
	responses := make([]*models.ScheduleEntryResponse, len(filteredEntries))
	for i, entry := range filteredEntries {
		responses[i] = entry.ToResponse()
	}

	return responses, nil
}

// GetDailySummary returns a summary of all schedule entries and absences for a specific date
func (u *scheduleUsecase) GetDailySummary(date time.Time) (*models.DailySummaryResponse, error) {
	// Get all schedule entries for the date
	entries, err := u.scheduleRepo.GetAllEntriesForDate(date)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule entries for date: %w", err)
	}

	// Get all absences for the date
	absences, err := u.absenceRepo.GetAbsencesForDate(date)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences for date: %w", err)
	}

	// Build set of absent user IDs to filter them out from schedule entries
	absentUserIDs := make(map[uint]bool)
	for _, absence := range absences {
		absentUserIDs[absence.UserID] = true
	}

	// Group entries by schedule, filtering out absent users
	scheduleGroups := make(map[uint]*models.DailySummaryScheduleGroup)
	scheduleOrder := make([]uint, 0)

	for _, entry := range entries {
		if absentUserIDs[entry.UserID] {
			continue
		}

		group, exists := scheduleGroups[entry.ScheduleID]
		if !exists {
			group = &models.DailySummaryScheduleGroup{
				ScheduleID:    entry.ScheduleID,
				ScheduleTitle: "",
				ScheduleType:  "",
				Color:         "",
				Users:         make([]*models.DailySummaryUserEntry, 0),
			}
			if entry.Schedule != nil {
				group.ScheduleTitle = entry.Schedule.Title
				group.ScheduleType = entry.Schedule.Type
				group.Color = entry.Schedule.Color
			}
			scheduleGroups[entry.ScheduleID] = group
			scheduleOrder = append(scheduleOrder, entry.ScheduleID)
		}

		userEntry := &models.DailySummaryUserEntry{
			UserID:    entry.UserID,
			User:      entry.User,
			ShiftType: entry.ShiftType,
			StartTime: entry.StartTime,
			EndTime:   entry.EndTime,
			Title:     entry.Title,
			Location:  entry.Location,
		}
		group.Users = append(group.Users, userEntry)
	}

	// Build ordered schedule groups list
	scheduleGroupsList := make([]*models.DailySummaryScheduleGroup, 0, len(scheduleOrder))
	for _, scheduleID := range scheduleOrder {
		scheduleGroupsList = append(scheduleGroupsList, scheduleGroups[scheduleID])
	}

	// Build absences list
	absencesList := make([]*models.DailySummaryAbsence, 0, len(absences))
	for _, absence := range absences {
		absencesList = append(absencesList, &models.DailySummaryAbsence{
			UserID: absence.UserID,
			User:   absence.User,
			Type:   absence.Type,
			Reason: absence.Reason,
		})
	}

	return &models.DailySummaryResponse{
		Date:      date,
		Schedules: scheduleGroupsList,
		Absences:  absencesList,
	}, nil
}

// ensureEntriesGenerated ensures that entries exist for a recurring schedule in the given period
func (u *scheduleUsecase) ensureEntriesGenerated(schedule *models.Schedule, userID uint, startDate, endDate time.Time) error {
	if schedule.TemplateID == nil {
		return errors.New("recurring schedule must have a template")
	}

	// Get template with entries
	template, err := u.scheduleRepo.GetTemplateWithEntries(*schedule.TemplateID)
	if err != nil {
		return err
	}

	// Get months that need generation
	months := u.getMonthsInRange(startDate, endDate)

	for _, month := range months {
		// Check if entries already exist for this month
		hasEntries, err := u.scheduleRepo.HasEntriesForMonth(schedule.ID, month.Year, month.Month)
		if err != nil {
			continue
		}

		if !hasEntries {
			// Generate entries from template for this month
			if err := u.generateMonthEntries(schedule, template, userID, month.Year, month.Month); err != nil {
				// Log but continue
			}
		}
	}

	return nil
}

// YearMonth represents a year and month combination
type YearMonth struct {
	Year  int
	Month time.Month
}

// getMonthsInRange returns all months in the given date range
func (u *scheduleUsecase) getMonthsInRange(startDate, endDate time.Time) []YearMonth {
	var months []YearMonth

	current := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, time.Local)
	end := time.Date(endDate.Year(), endDate.Month(), 1, 0, 0, 0, 0, time.Local)

	for !current.After(end) {
		months = append(months, YearMonth{Year: current.Year(), Month: current.Month()})
		current = current.AddDate(0, 1, 0)
	}

	return months
}

// generateMonthEntries generates entries from template for a specific month
func (u *scheduleUsecase) generateMonthEntries(schedule *models.Schedule, template *models.ScheduleTemplate, userID uint, year int, month time.Month) error {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	// Get absences for this user in this month
	absences, _ := u.absenceRepo.GetUserAbsences(userID, startOfMonth, endOfMonth)
	absenceSet := make(map[string]bool)
	for _, absence := range absences {
		for d := absence.StartDate; !d.After(absence.EndDate); d = d.AddDate(0, 0, 1) {
			absenceSet[d.Format("2006-01-02")] = true
		}
	}

	var entries []*models.ScheduleEntry

	// Iterate through each day of the month
	currentDate := startOfMonth
	for !currentDate.After(endOfMonth) {
		// Skip if user is absent
		if absenceSet[currentDate.Format("2006-01-02")] {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}

		dayOfWeek := int(currentDate.Weekday())

		// Find matching template entries for this day of week
		for _, templateEntry := range template.Entries {
			if templateEntry.DayOfWeek == dayOfWeek {
				// Check if this template entry applies to this user
				if templateEntry.UserID != nil && *templateEntry.UserID != userID {
					continue
				}

				startTime := u.parseTimeOnDate(currentDate, templateEntry.StartTime)
				endTime := u.parseTimeOnDate(currentDate, templateEntry.EndTime)

				entry := &models.ScheduleEntry{
					ScheduleID: schedule.ID,
					UserID:     userID,
					Date:       currentDate,
					ShiftType:  models.ShiftCustom,
					StartTime:  startTime,
					EndTime:    endTime,
					Title:      templateEntry.Title,
					Location:   templateEntry.Location,
					CreatedBy:  schedule.CreatedBy,
				}

				entries = append(entries, entry)
			}
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	// Save all entries
	if len(entries) > 0 {
		if err := u.scheduleRepo.CreateScheduleEntries(entries); err != nil {
			return err
		}

		// Create events for entries (skip for recurring schedules)
		if schedule.Mode != models.ScheduleModeRecurring {
			for _, entry := range entries {
				event, err := u.createEventForScheduleEntry(schedule, entry)
				if err == nil {
					entry.EventID = &event.ID
					u.scheduleRepo.UpdateScheduleEntry(entry)
				}
			}
		}
	}

	return nil
}

// filterEntriesByAbsences filters out entries that fall on absence days
func (u *scheduleUsecase) filterEntriesByAbsences(entries []*models.ScheduleEntry, absences []*models.Absence) []*models.ScheduleEntry {
	if len(absences) == 0 {
		return entries
	}

	// Build a set of absence dates
	absenceSet := make(map[string]bool)
	for _, absence := range absences {
		for d := absence.StartDate; !d.After(absence.EndDate); d = d.AddDate(0, 0, 1) {
			absenceSet[d.Format("2006-01-02")] = true
		}
	}

	// Filter entries
	var filtered []*models.ScheduleEntry
	for _, entry := range entries {
		if !absenceSet[entry.Date.Format("2006-01-02")] {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// Helper functions

// calculateShiftTimes calculates start and end times based on shift type
func (u *scheduleUsecase) calculateShiftTimes(schedule *models.Schedule, date time.Time, shiftType models.ShiftType, customStart, customEnd *string) (time.Time, time.Time, error) {
	var startTime, endTime time.Time

	switch shiftType {
	case models.ShiftMorning:
		startTime = u.parseTimeOnDate(date, schedule.MorningStart)
		endTime = u.parseTimeOnDate(date, schedule.MorningEnd)
	case models.ShiftEvening:
		startTime = u.parseTimeOnDate(date, schedule.EveningStart)
		endTime = u.parseTimeOnDate(date, schedule.EveningEnd)
	case models.ShiftFullDay:
		startTime = u.parseTimeOnDate(date, schedule.MorningStart)
		endTime = u.parseTimeOnDate(date, schedule.EveningEnd)
	case models.ShiftCustom:
		if customStart == nil || customEnd == nil {
			return time.Time{}, time.Time{}, errors.New("custom shift requires start_time and end_time")
		}
		startTime = u.parseTimeOnDate(date, *customStart)
		endTime = u.parseTimeOnDate(date, *customEnd)
	default:
		return time.Time{}, time.Time{}, errors.New("invalid shift type")
	}

	if endTime.Before(startTime) || endTime.Equal(startTime) {
		return time.Time{}, time.Time{}, errors.New("end time must be after start time")
	}

	return startTime, endTime, nil
}

// parseTimeOnDate parses a time string (HH:MM) and applies it to a date
func (u *scheduleUsecase) parseTimeOnDate(date time.Time, timeStr string) time.Time {
	// Parse time string "HH:MM"
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return date
	}

	var hour, minute int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)

	// Use local timezone to ensure consistent time display
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, time.Local)
}

// createEventForScheduleEntry creates a calendar event for a schedule entry
func (u *scheduleUsecase) createEventForScheduleEntry(schedule *models.Schedule, entry *models.ScheduleEntry) (*models.Event, error) {
	event := &models.Event{
		Title:           u.generateEventTitle(schedule, entry),
		Description:     entry.Description,
		StartTime:       entry.StartTime,
		EndTime:         entry.EndTime,
		Location:        entry.Location,
		Type:            models.EventTypeSchedule,
		CreatedBy:       entry.UserID, // Event should belong to the user assigned to the shift, not the creator
		Color:           schedule.Color,
		IsPrivate:       true, // Schedule events are private by default
		ScheduleEntryID: &entry.ID,
	}

	if err := u.eventRepo.CreateEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

// generateEventTitle generates a title for a schedule event
func (u *scheduleUsecase) generateEventTitle(schedule *models.Schedule, entry *models.ScheduleEntry) string {
	if entry.Title != "" {
		return entry.Title
	}

	// Just use the schedule title - shift time is already shown in the event
	return schedule.Title
}

// validateCreateScheduleRequest validates create schedule request
func (u *scheduleUsecase) validateCreateScheduleRequest(req *models.CreateScheduleRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return errors.New("title is required")
	}

	if req.EndDate.Before(req.StartDate) {
		return errors.New("end date must be after start date")
	}

	return nil
}

// Permission checking methods

// isManagement checks if user role is DepartmentHead or higher
func isManagement(role sharedmodels.Role) bool {
	return role == sharedmodels.RoleSuperAdmin ||
		role == sharedmodels.RoleAdmin ||
		role == sharedmodels.RoleDepartmentHead
}

// canViewScheduleObj checks if user can view an already-loaded schedule object
func (u *scheduleUsecase) canViewScheduleObj(userID uint, schedule *models.Schedule, userRole sharedmodels.Role) (bool, error) {
	// Super admin and admin can view all
	if userRole == sharedmodels.RoleSuperAdmin || userRole == sharedmodels.RoleAdmin {
		return true, nil
	}

	// Creator can always view
	if schedule.CreatedBy == userID {
		return true, nil
	}

	// Check visibility based on setting
	switch schedule.Visibility {
	case models.VisibilityCreatorOnly:
		return false, nil

	case models.VisibilityManagement:
		return isManagement(userRole), nil

	case models.VisibilityParticipants:
		isAssigned, err := u.scheduleRepo.IsUserAssignedToSchedule(schedule.ID, userID)
		if err != nil {
			return false, err
		}
		return isAssigned, nil

	case models.VisibilitySpecificUsers:
		isViewer, err := u.scheduleRepo.IsUserScheduleViewer(schedule.ID, userID)
		if err != nil {
			return false, err
		}
		return isViewer, nil

	case models.VisibilityAll:
		return true, nil

	default:
		return isManagement(userRole), nil
	}
}

// CanViewSchedule checks if user can view a schedule
func (u *scheduleUsecase) CanViewSchedule(userID, scheduleID uint, userRole sharedmodels.Role) (bool, error) {
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return false, err
	}

	return u.canViewScheduleObj(userID, schedule, userRole)
}

// CanEditSchedule checks if user can edit a schedule
func (u *scheduleUsecase) CanEditSchedule(userID, scheduleID uint, userRole sharedmodels.Role) (bool, error) {
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return false, err
	}

	// Super admin and admin can edit all
	if userRole == sharedmodels.RoleSuperAdmin || userRole == sharedmodels.RoleAdmin {
		return true, nil
	}

	// Check edit permission based on setting
	switch schedule.EditPermission {
	case models.EditPermissionCreatorOnly:
		// Only creator can edit
		return schedule.CreatedBy == userID, nil

	case models.EditPermissionManagement:
		// Creator + DepartmentHead and above can edit
		if schedule.CreatedBy == userID {
			return true, nil
		}
		return isManagement(userRole), nil

	case models.EditPermissionSpecificUsers:
		// Creator + specific users can edit
		if schedule.CreatedBy == userID {
			return true, nil
		}
		isEditor, err := u.scheduleRepo.IsUserScheduleEditor(scheduleID, userID)
		if err != nil {
			return false, err
		}
		return isEditor, nil

	case models.EditPermissionAll:
		// Everyone can edit
		return true, nil

	default:
		// Default to creator only for backwards compatibility
		return schedule.CreatedBy == userID, nil
	}
}

// GetScheduleGroupMembers returns the user group members for a schedule's linked group
func (u *scheduleUsecase) GetScheduleGroupMembers(scheduleID uint) ([]*clients.UserGroupMember, error) {
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	if schedule.UserGroupID == nil {
		return []*clients.UserGroupMember{}, nil
	}

	members, err := u.userClient.GetUserGroupMembers(*schedule.UserGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user group members: %w", err)
	}

	return members, nil
}
