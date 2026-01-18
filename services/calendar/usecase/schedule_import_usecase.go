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
	fileClient   *clients.FileClient
	parser       *importschedule.ScheduleParser
}

// NewScheduleImportUsecase creates a new schedule import usecase
func NewScheduleImportUsecase(scheduleRepo repository.ScheduleRepository, fileClient *clients.FileClient) ScheduleImportUsecase {
	return &scheduleImportUsecase{
		scheduleRepo: scheduleRepo,
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

	// Create schedule
	schedule := &models.Schedule{
		Title:         req.Title,
		Description:   req.Description,
		Type:          req.Type,
		Visibility:    models.VisibilityManagement,
		CreatedBy:     userID,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		IsActive:      true,
		ImportedFrom:  &metadata.FileName,
		MorningStart:  "10:00",
		MorningEnd:    "14:00",
		EveningStart:  "14:00",
		EveningEnd:    "18:00",
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
		MorningStart: "10:00",
		MorningEnd:   "14:00",
		EveningStart: "14:00",
		EveningEnd:   "18:00",
	}

	// Create preview entries
	entries, warnings := u.createEntriesFromParsed(userID, 0, parsed)

	// Convert to response format
	entryResponses := make([]*models.ScheduleEntryResponse, len(entries))
	for i, entry := range entries {
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

	for _, parsedEntry := range parsed.Entries {
		// Get matched user ID
		importedUser, exists := parsed.Users[parsedEntry.UserName]
		if !exists || importedUser.UserID == nil {
			warnings = append(warnings, fmt.Sprintf("Skipping entry for unmatched user: %s", parsedEntry.UserName))
			continue
		}

		// Parse times
		startTime := u.parseTimeOnDate(parsedEntry.Date, parsedEntry.StartTime)
		endTime := u.parseTimeOnDate(parsedEntry.Date, parsedEntry.EndTime)

		entry := &models.ScheduleEntry{
			ScheduleID: scheduleID,
			UserID:     *importedUser.UserID,
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
