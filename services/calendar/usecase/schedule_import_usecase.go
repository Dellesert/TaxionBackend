package usecase

import (
	"fmt"
	"time"

	"tachyon-messenger/services/calendar/clients"
	importschedule "tachyon-messenger/services/calendar/import"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	"tachyon-messenger/shared/logger"
	sharedmodels "tachyon-messenger/shared/models"
)

// ScheduleImportUsecase defines interface for schedule import operations
type ScheduleImportUsecase interface {
	ImportSchedule(userID uint, req *models.ImportScheduleRequest, allUsers []*sharedmodels.User) (*models.ImportScheduleResponse, error)
	PreviewImport(userID uint, req *models.ImportScheduleRequest, allUsers []*sharedmodels.User) (*models.ImportPreviewResponse, error)
}

// scheduleImportUsecase implements ScheduleImportUsecase interface
type scheduleImportUsecase struct {
	scheduleRepo repository.ScheduleRepository
	eventRepo    repository.EventRepository
	absenceRepo  repository.AbsenceRepository
	fileClient   *clients.FileClient
	parser       *importschedule.ScheduleParser
}

// NewScheduleImportUsecase creates a new schedule import usecase
func NewScheduleImportUsecase(scheduleRepo repository.ScheduleRepository, eventRepo repository.EventRepository, absenceRepo repository.AbsenceRepository, fileClient *clients.FileClient) ScheduleImportUsecase {
	return &scheduleImportUsecase{
		scheduleRepo: scheduleRepo,
		eventRepo:    eventRepo,
		absenceRepo:  absenceRepo,
		fileClient:   fileClient,
		parser:       importschedule.NewScheduleParser(),
	}
}

// ImportSchedule imports schedule from Word document
func (u *scheduleImportUsecase) ImportSchedule(userID uint, req *models.ImportScheduleRequest, allUsers []*sharedmodels.User) (*models.ImportScheduleResponse, error) {
	// Download and parse file
	parsed, metadata, err := u.parseScheduleFile(userID, req.FileID, allUsers)
	if err != nil {
		return nil, err
	}

	// Extract shift times from parsed entries
	morningStart, morningEnd, eveningStart, eveningEnd := u.extractShiftTimes(parsed)

	// Create schedule
	schedule := &models.Schedule{
		Title:        req.Title,
		Description:  req.Description,
		Type:         req.Type,
		Visibility:   models.VisibilityManagement,
		CreatedBy:    userID,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		IsActive:     true,
		ImportedFrom: &metadata.FileName,
		MorningStart: morningStart,
		MorningEnd:   morningEnd,
		EveningStart: eveningStart,
		EveningEnd:   eveningEnd,
	}

	if err := u.scheduleRepo.CreateSchedule(schedule); err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	// Create schedule entries
	entries, warnings := u.createEntriesFromParsed(userID, schedule.ID, parsed)

	if len(entries) > 0 {
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
		}
	}

	// Add parsing warnings
	allWarnings := append(parsed.Warnings, warnings...)

	// Get created schedule with full data
	createdSchedule, err := u.scheduleRepo.GetScheduleWithEntries(schedule.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created schedule: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"schedule_id":    schedule.ID,
		"entries_count":  len(entries),
		"warnings_count": len(allWarnings),
		"user_id":        userID,
	}).Info("Schedule imported successfully")

	return &models.ImportScheduleResponse{
		Schedule:     createdSchedule.ToResponse(),
		EntriesCount: len(entries),
		ImportedFrom: metadata.FileName,
		Warnings:     allWarnings,
	}, nil
}

// PreviewImport previews schedule import without creating
func (u *scheduleImportUsecase) PreviewImport(userID uint, req *models.ImportScheduleRequest, allUsers []*sharedmodels.User) (*models.ImportPreviewResponse, error) {
	// Download and parse file
	parsed, _, err := u.parseScheduleFile(userID, req.FileID, allUsers)
	if err != nil {
		return nil, err
	}

	// Extract shift times from parsed entries
	morningStart, morningEnd, eveningStart, eveningEnd := u.extractShiftTimes(parsed)

	// Create preview schedule (not saved)
	schedule := &models.Schedule{
		Title:        req.Title,
		Description:  req.Description,
		Type:         req.Type,
		Visibility:   models.VisibilityManagement,
		CreatedBy:    userID,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		IsActive:     true,
		MorningStart: morningStart,
		MorningEnd:   morningEnd,
		EveningStart: eveningStart,
		EveningEnd:   eveningEnd,
	}

	// Build user map for quick lookup
	userMap := make(map[uint]*sharedmodels.User)
	for _, user := range allUsers {
		userMap[user.ID] = user
	}

	// Create preview entries with user data
	entries, warnings := u.createEntriesFromParsed(userID, 0, parsed)

	// Convert to response format with user data
	entryResponses := make([]*models.ScheduleEntryResponse, len(entries))
	for i, entry := range entries {
		// Attach user data for preview
		if user, exists := userMap[entry.UserID]; exists {
			entry.User = user
		}
		entryResponses[i] = entry.ToResponse()
	}

	// Convert users to response format
	users := make([]*models.ImportedUser, 0, len(parsed.Users))
	for _, user := range parsed.Users {
		users = append(users, user)
	}

	// Add parsing warnings
	allWarnings := append(parsed.Warnings, warnings...)

	return &models.ImportPreviewResponse{
		Schedule:     schedule.ToResponse(),
		Entries:      entryResponses,
		EntriesCount: len(entries),
		Users:        users,
		Warnings:     allWarnings,
	}, nil
}

// parseScheduleFile downloads and parses schedule file
func (u *scheduleImportUsecase) parseScheduleFile(userID uint, fileID string, allUsers []*sharedmodels.User) (*importschedule.ParsedSchedule, *clients.FileMetadata, error) {
	// Download file
	content, metadata, err := u.fileClient.DownloadAndValidate(fileID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download file: %w", err)
	}

	// Parse document
	parsed, err := u.parser.ParseDocument(content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse document: %w", err)
	}

	// Match users
	u.parser.MatchUsers(parsed, allUsers)

	logger.WithFields(map[string]interface{}{
		"file_id":       fileID,
		"file_name":     metadata.FileName,
		"format":        importschedule.GetFormatName(parsed.Format),
		"entries_count": len(parsed.Entries),
		"users_count":   len(parsed.Users),
	}).Info("Document parsed successfully")

	return parsed, metadata, nil
}

// createEntriesFromParsed creates schedule entries from parsed data
func (u *scheduleImportUsecase) createEntriesFromParsed(userID, scheduleID uint, parsed *importschedule.ParsedSchedule) ([]*models.ScheduleEntry, []string) {
	entries := make([]*models.ScheduleEntry, 0)
	warnings := make([]string, 0)

	// Collect all user IDs and date range
	var userIDs []uint
	var minDate, maxDate time.Time
	userIDSet := make(map[uint]bool)

	for _, parsedEntry := range parsed.Entries {
		importedUser, exists := parsed.Users[parsedEntry.UserName]
		if exists && importedUser.UserID != nil && !userIDSet[*importedUser.UserID] {
			userIDs = append(userIDs, *importedUser.UserID)
			userIDSet[*importedUser.UserID] = true
		}
		if minDate.IsZero() || parsedEntry.Date.Before(minDate) {
			minDate = parsedEntry.Date
		}
		if maxDate.IsZero() || parsedEntry.Date.After(maxDate) {
			maxDate = parsedEntry.Date
		}
	}

	// Get absences for all users in the date range
	absenceMap := make(map[uint][]*models.Absence)
	if len(userIDs) > 0 && !minDate.IsZero() && !maxDate.IsZero() {
		absMap, err := u.absenceRepo.GetAbsentUsersForPeriod(userIDs, minDate, maxDate)
		if err == nil {
			absenceMap = absMap
		}
	}

	for _, parsedEntry := range parsed.Entries {
		// Get matched user ID
		importedUser, exists := parsed.Users[parsedEntry.UserName]
		if !exists || importedUser.UserID == nil {
			warnings = append(warnings, fmt.Sprintf("Skipping entry for unmatched user: %s", parsedEntry.UserName))
			continue
		}

		targetUserID := *importedUser.UserID

		// Check if user is absent on this date
		if userAbsences, ok := absenceMap[targetUserID]; ok {
			isAbsent := false
			var absenceType string
			for _, absence := range userAbsences {
				if !parsedEntry.Date.Before(absence.StartDate) && !parsedEntry.Date.After(absence.EndDate) {
					isAbsent = true
					absenceType = GetAbsenceTypeName(absence.Type)
					break
				}
			}
			if isAbsent {
				warnings = append(warnings, fmt.Sprintf("Пропущена запись для %s на %s: пользователь в отсутствии (%s)",
					parsedEntry.UserName, parsedEntry.Date.Format("02.01.2006"), absenceType))
				continue
			}
		}

		// Parse times
		startTime := u.parseTimeOnDate(parsedEntry.Date, parsedEntry.StartTime)
		endTime := u.parseTimeOnDate(parsedEntry.Date, parsedEntry.EndTime)

		entry := &models.ScheduleEntry{
			ScheduleID: scheduleID,
			UserID:     targetUserID,
			Date:       parsedEntry.Date,
			ShiftType:  parsedEntry.ShiftType,
			StartTime:  startTime,
			EndTime:    endTime,
			CreatedBy:  userID,
		}

		entries = append(entries, entry)
	}

	return entries, warnings
}

// parseTimeOnDate parses time string and combines with date
func (u *scheduleImportUsecase) parseTimeOnDate(date time.Time, timeStr string) time.Time {
	var hour, minute int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)

	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location())
}

// extractShiftTimes extracts morning and evening shift times from parsed entries
func (u *scheduleImportUsecase) extractShiftTimes(parsed *importschedule.ParsedSchedule) (morningStart, morningEnd, eveningStart, eveningEnd string) {
	// Default values
	morningStart = "09:00"
	morningEnd = "14:00"
	eveningStart = "14:00"
	eveningEnd = "19:00"

	foundMorning := false
	foundEvening := false

	// Look for actual times from parsed entries
	for _, entry := range parsed.Entries {
		switch entry.ShiftType {
		case models.ShiftMorning:
			if !foundMorning && entry.StartTime != "" && entry.EndTime != "" {
				morningStart = entry.StartTime
				morningEnd = entry.EndTime
				foundMorning = true
			}
		case models.ShiftEvening:
			if !foundEvening && entry.StartTime != "" && entry.EndTime != "" {
				eveningStart = entry.StartTime
				eveningEnd = entry.EndTime
				foundEvening = true
			}
		}
		// Once we have both, we can stop
		if foundMorning && foundEvening {
			break
		}
	}

	return morningStart, morningEnd, eveningStart, eveningEnd
}

// createEventForScheduleEntry creates a calendar event for a schedule entry
func (u *scheduleImportUsecase) createEventForScheduleEntry(schedule *models.Schedule, entry *models.ScheduleEntry) (*models.Event, error) {
	title := schedule.Title
	if entry.Title != "" {
		title = entry.Title
	}

	event := &models.Event{
		Title:           title,
		Description:     entry.Description,
		StartTime:       entry.StartTime,
		EndTime:         entry.EndTime,
		Location:        entry.Location,
		Type:            models.EventTypeSchedule,
		CreatedBy:       entry.UserID, // Event belongs to the assigned user
		Color:           schedule.Color,
		IsPrivate:       true,
		ScheduleEntryID: &entry.ID,
	}

	if err := u.eventRepo.CreateEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}
