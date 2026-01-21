package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	sharedmodels "tachyon-messenger/shared/models"
)

// ScheduleUsecase defines the interface for schedule business logic
type ScheduleUsecase interface {
	// Schedule management
	CreateSchedule(userID uint, req *models.CreateScheduleRequest) (*models.ScheduleResponse, error)
	GetScheduleByID(userID, scheduleID uint) (*models.ScheduleResponse, error)
	GetSchedules(userID uint, filter ScheduleFilterParams) (*models.ScheduleListResponse, error)
	UpdateSchedule(userID, scheduleID uint, req *models.UpdateScheduleRequest) (*models.ScheduleResponse, error)
	DeleteSchedule(userID, scheduleID uint) error

	// Schedule entry management
	CreateScheduleEntry(userID, scheduleID uint, req *models.CreateScheduleEntryRequest) (*models.ScheduleEntryResponse, error)
	CreateScheduleEntries(userID, scheduleID uint, req *models.BatchCreateScheduleEntriesRequest) ([]*models.ScheduleEntryResponse, error)
	GetScheduleEntries(userID, scheduleID uint, filter EntryFilterParams) (*models.ScheduleEntryListResponse, error)
	UpdateScheduleEntry(userID, scheduleID, entryID uint, req *models.UpdateScheduleEntryRequest) (*models.ScheduleEntryResponse, error)
	DeleteScheduleEntry(userID, scheduleID, entryID uint) error
	GetMyScheduleEntries(userID uint, startDate, endDate time.Time) ([]*models.ScheduleEntryResponse, error)

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
	scheduleRepo repository.ScheduleRepository
	eventRepo    repository.EventRepository
	absenceRepo  repository.AbsenceRepository
}

// NewScheduleUsecase creates a new schedule usecase
func NewScheduleUsecase(
	scheduleRepo repository.ScheduleRepository,
	eventRepo repository.EventRepository,
	absenceRepo repository.AbsenceRepository,
) ScheduleUsecase {
	return &scheduleUsecase{
		scheduleRepo: scheduleRepo,
		eventRepo:    eventRepo,
		absenceRepo:  absenceRepo,
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
		TemplateID:    req.TemplateID,
		IsActive:      true,
	}

	// Set visibility (default to management)
	if req.Visibility != "" {
		schedule.Visibility = req.Visibility
	} else {
		schedule.Visibility = models.VisibilityManagement
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

	// Get schedule with creator info
	createdSchedule, err := u.scheduleRepo.GetScheduleByID(schedule.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created schedule: %w", err)
	}

	return createdSchedule.ToResponse(), nil
}

// GetScheduleByID retrieves a schedule by ID with permission check
func (u *scheduleUsecase) GetScheduleByID(userID, scheduleID uint) (*models.ScheduleResponse, error) {
	schedule, err := u.scheduleRepo.GetScheduleWithEntries(scheduleID)
	if err != nil {
		return nil, err
	}

	return schedule.ToResponse(), nil
}

// GetSchedules retrieves schedules based on user permissions
func (u *scheduleUsecase) GetSchedules(userID uint, filter ScheduleFilterParams) (*models.ScheduleListResponse, error) {
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

	schedules, total, err := u.scheduleRepo.GetSchedules(repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get schedules: %w", err)
	}

	// Convert to responses
	responses := make([]*models.ScheduleResponse, len(schedules))
	for i, schedule := range schedules {
		responses[i] = schedule.ToResponse()
	}

	return &models.ScheduleListResponse{
		Schedules: responses,
		Total:     total,
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

	// Get updated schedule with creator info
	updatedSchedule, err := u.scheduleRepo.GetScheduleByID(schedule.ID)
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

	// Create associated calendar event
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

	// Get created entry with relations
	createdEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created entry: %w", err)
	}

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

	// Create calendar events for each entry
	for _, entry := range entries {
		event, err := u.createEventForScheduleEntry(schedule, entry)
		if err == nil {
			entry.EventID = &event.ID
			u.scheduleRepo.UpdateScheduleEntry(entry)
		}

		// Get entry with relations
		createdEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
		if err == nil {
			responses = append(responses, createdEntry.ToResponse())
		}
	}

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

	// Update fields if provided
	if req.UserID != nil {
		entry.UserID = *req.UserID
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

	// Save updated entry
	if err := u.scheduleRepo.UpdateScheduleEntry(entry); err != nil {
		return nil, fmt.Errorf("failed to update schedule entry: %w", err)
	}

	// Update associated event if exists
	if entry.EventID != nil {
		event, err := u.eventRepo.GetEventByID(*entry.EventID)
		if err == nil {
			event.StartTime = entry.StartTime
			event.EndTime = entry.EndTime
			event.Title = u.generateEventTitle(schedule, entry)
			event.Description = entry.Description
			event.Location = entry.Location
			u.eventRepo.UpdateEvent(event)
		}
	}

	// Get updated entry with relations
	updatedEntry, err := u.scheduleRepo.GetScheduleEntry(entry.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated entry: %w", err)
	}

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

	// Delete associated event if exists
	if entry.EventID != nil {
		if err := u.eventRepo.DeleteEvent(*entry.EventID); err != nil {
			// Log error but continue
		}
	}

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

		// Create events for entries
		for _, entry := range entries {
			event, err := u.createEventForScheduleEntry(schedule, entry)
			if err == nil {
				entry.EventID = &event.ID
				u.scheduleRepo.UpdateScheduleEntry(entry)
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

// CanViewSchedule checks if user can view a schedule
func (u *scheduleUsecase) CanViewSchedule(userID, scheduleID uint, userRole sharedmodels.Role) (bool, error) {
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return false, err
	}

	// Super admin and admin can view all
	if userRole == sharedmodels.RoleSuperAdmin || userRole == sharedmodels.RoleAdmin {
		return true, nil
	}

	// Creator can always view
	if schedule.CreatedBy == userID {
		return true, nil
	}

	// Department head can view department schedules
	if userRole == sharedmodels.RoleDepartmentHead && schedule.DepartmentID != nil {
		// TODO: Check if user is head of this department
		return true, nil
	}

	// Check visibility
	if schedule.Visibility == models.VisibilityParticipants {
		// Check if user is assigned to this schedule
		isAssigned, err := u.scheduleRepo.IsUserAssignedToSchedule(scheduleID, userID)
		if err != nil {
			return false, err
		}
		return isAssigned, nil
	}

	return false, nil
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

	// Creator can edit
	if schedule.CreatedBy == userID {
		return true, nil
	}

	// Department head can edit department schedules
	if userRole == sharedmodels.RoleDepartmentHead && schedule.DepartmentID != nil {
		// TODO: Check if user is head of this department
		return true, nil
	}

	return false, nil
}
