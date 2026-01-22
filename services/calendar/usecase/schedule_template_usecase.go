package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
)

// ScheduleTemplateUsecase defines the interface for template business logic
type ScheduleTemplateUsecase interface {
	// Template CRUD
	CreateTemplate(userID uint, req *models.CreateScheduleTemplateRequest) (*models.ScheduleTemplateResponse, error)
	GetTemplate(userID, templateID uint) (*models.ScheduleTemplateResponse, error)
	GetTemplates(userID uint, filter TemplateFilterParams) (*models.ScheduleTemplateListResponse, error)
	UpdateTemplate(userID, templateID uint, req *models.UpdateScheduleTemplateRequest) (*models.ScheduleTemplateResponse, error)
	DeleteTemplate(userID, templateID uint) error

	// Template Entry CRUD
	AddTemplateEntry(userID, templateID uint, req *models.CreateTemplateEntryRequest) (*models.ScheduleTemplateEntryResponse, error)
	GetTemplateEntries(userID, templateID uint) ([]*models.ScheduleTemplateEntryResponse, error)
	DeleteTemplateEntry(userID, templateID, entryID uint) error

	// Apply template
	ApplyTemplate(userID, templateID, scheduleID uint, req *models.ApplyTemplateRequest) (int, error)
}

// TemplateFilterParams represents filtering parameters for templates
type TemplateFilterParams struct {
	Type         *models.ScheduleType
	IsActive     *bool
	DepartmentID *uint
	Limit        int
	Offset       int
}

// scheduleTemplateUsecase implements ScheduleTemplateUsecase interface
type scheduleTemplateUsecase struct {
	scheduleRepo repository.ScheduleRepository
}

// NewScheduleTemplateUsecase creates a new template usecase
func NewScheduleTemplateUsecase(scheduleRepo repository.ScheduleRepository) ScheduleTemplateUsecase {
	return &scheduleTemplateUsecase{
		scheduleRepo: scheduleRepo,
	}
}

// CreateTemplate creates a new schedule template
func (u *scheduleTemplateUsecase) CreateTemplate(userID uint, req *models.CreateScheduleTemplateRequest) (*models.ScheduleTemplateResponse, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, errors.New("title is required")
	}

	template := &models.ScheduleTemplate{
		Title:        strings.TrimSpace(req.Title),
		Description:  strings.TrimSpace(req.Description),
		Type:         req.Type,
		CreatedBy:    userID,
		DepartmentID: req.DepartmentID,
		IsActive:     true,
	}

	// Set color
	if req.Color != "" {
		template.Color = req.Color
	} else {
		template.Color = "#4CAF50"
	}

	if err := u.scheduleRepo.CreateScheduleTemplate(template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	createdTemplate, err := u.scheduleRepo.GetScheduleTemplate(template.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created template: %w", err)
	}

	return createdTemplate.ToResponse(), nil
}

// GetTemplate retrieves a template by ID
func (u *scheduleTemplateUsecase) GetTemplate(userID, templateID uint) (*models.ScheduleTemplateResponse, error) {
	template, err := u.scheduleRepo.GetTemplateWithEntries(templateID)
	if err != nil {
		return nil, err
	}

	return template.ToResponse(), nil
}

// GetTemplates retrieves templates with filtering
func (u *scheduleTemplateUsecase) GetTemplates(userID uint, filter TemplateFilterParams) (*models.ScheduleTemplateListResponse, error) {
	repoFilter := repository.TemplateFilter{
		Type:         filter.Type,
		IsActive:     filter.IsActive,
		DepartmentID: filter.DepartmentID,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
	}

	templates, total, err := u.scheduleRepo.GetScheduleTemplates(repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get templates: %w", err)
	}

	responses := make([]*models.ScheduleTemplateResponse, len(templates))
	for i, template := range templates {
		responses[i] = template.ToResponse()
	}

	return &models.ScheduleTemplateListResponse{
		Templates: responses,
		Total:     total,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}, nil
}

// UpdateTemplate updates an existing template
func (u *scheduleTemplateUsecase) UpdateTemplate(userID, templateID uint, req *models.UpdateScheduleTemplateRequest) (*models.ScheduleTemplateResponse, error) {
	template, err := u.scheduleRepo.GetScheduleTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Title != nil {
		template.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		template.Description = strings.TrimSpace(*req.Description)
	}
	if req.Type != nil {
		template.Type = *req.Type
	}
	if req.DepartmentID != nil {
		template.DepartmentID = req.DepartmentID
	}
	if req.Color != nil {
		template.Color = *req.Color
	}
	if req.IsActive != nil {
		template.IsActive = *req.IsActive
	}

	if err := u.scheduleRepo.UpdateScheduleTemplate(template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	updatedTemplate, err := u.scheduleRepo.GetScheduleTemplate(template.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated template: %w", err)
	}

	return updatedTemplate.ToResponse(), nil
}

// DeleteTemplate deletes a template
func (u *scheduleTemplateUsecase) DeleteTemplate(userID, templateID uint) error {
	if err := u.scheduleRepo.DeleteScheduleTemplate(templateID); err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	return nil
}

// AddTemplateEntry adds an entry to a template
func (u *scheduleTemplateUsecase) AddTemplateEntry(userID, templateID uint, req *models.CreateTemplateEntryRequest) (*models.ScheduleTemplateEntryResponse, error) {
	// Validate day of week
	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		return nil, errors.New("day_of_week must be between 0 and 6")
	}

	entry := &models.ScheduleTemplateEntry{
		TemplateID: templateID,
		UserID:     req.UserID,
		DayOfWeek:  req.DayOfWeek,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		ShiftType:  req.ShiftType,
		Title:      strings.TrimSpace(req.Title),
		Location:   strings.TrimSpace(req.Location),
	}

	if err := u.scheduleRepo.CreateTemplateEntry(entry); err != nil {
		return nil, fmt.Errorf("failed to create template entry: %w", err)
	}

	// Get entry with relations
	entries, err := u.scheduleRepo.GetTemplateEntries(templateID)
	if err != nil {
		return nil, err
	}

	// Find the created entry
	for _, e := range entries {
		if e.ID == entry.ID {
			return e.ToResponse(), nil
		}
	}

	return entry.ToResponse(), nil
}

// GetTemplateEntries retrieves all entries for a template
func (u *scheduleTemplateUsecase) GetTemplateEntries(userID, templateID uint) ([]*models.ScheduleTemplateEntryResponse, error) {
	entries, err := u.scheduleRepo.GetTemplateEntries(templateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template entries: %w", err)
	}

	responses := make([]*models.ScheduleTemplateEntryResponse, len(entries))
	for i, entry := range entries {
		responses[i] = entry.ToResponse()
	}

	return responses, nil
}

// DeleteTemplateEntry deletes a template entry
func (u *scheduleTemplateUsecase) DeleteTemplateEntry(userID, templateID, entryID uint) error {
	if err := u.scheduleRepo.DeleteTemplateEntry(entryID); err != nil {
		return fmt.Errorf("failed to delete template entry: %w", err)
	}

	return nil
}

// ApplyTemplate applies a template to a schedule for a date range
func (u *scheduleTemplateUsecase) ApplyTemplate(userID, templateID, scheduleID uint, req *models.ApplyTemplateRequest) (int, error) {
	// Validate date range
	if req.EndDate.Before(req.StartDate) {
		return 0, errors.New("end_date must be after start_date")
	}

	// Get template with entries
	template, err := u.scheduleRepo.GetTemplateWithEntries(templateID)
	if err != nil {
		return 0, fmt.Errorf("failed to get template: %w", err)
	}

	if len(template.Entries) == 0 {
		return 0, errors.New("template has no entries")
	}

	// Get schedule
	schedule, err := u.scheduleRepo.GetScheduleByID(scheduleID)
	if err != nil {
		return 0, fmt.Errorf("failed to get schedule: %w", err)
	}

	// Determine which users to apply template to
	var targetUserIDs []uint
	if len(req.UserIDs) > 0 {
		targetUserIDs = req.UserIDs
	} else {
		// If no user IDs specified, get all assigned users
		assignments, err := u.scheduleRepo.GetScheduleAssignments(scheduleID)
		if err != nil {
			return 0, fmt.Errorf("failed to get schedule assignments: %w", err)
		}
		for _, assignment := range assignments {
			targetUserIDs = append(targetUserIDs, assignment.UserID)
		}
	}

	if len(targetUserIDs) == 0 {
		return 0, errors.New("no users to apply template to")
	}

	// Generate schedule entries from template
	entries := make([]*models.ScheduleEntry, 0)
	currentDate := req.StartDate

	for !currentDate.After(req.EndDate) {
		dayOfWeek := int(currentDate.Weekday())

		// Find template entries for this day of week
		for _, templateEntry := range template.Entries {
			if templateEntry.DayOfWeek != dayOfWeek {
				continue
			}

			// Determine which users get this entry
			var usersForEntry []uint
			if templateEntry.UserID != nil {
				// Entry is for a specific user
				usersForEntry = []uint{*templateEntry.UserID}
			} else {
				// Entry is for all users
				usersForEntry = targetUserIDs
			}

			// Create entry for each user
			for _, uid := range usersForEntry {
				// Parse times
				startTime := parseTimeOnDate(currentDate, templateEntry.StartTime)
				endTime := parseTimeOnDate(currentDate, templateEntry.EndTime)

				// Determine shift type based on schedule settings
				shiftType := determineShiftType(schedule, templateEntry.StartTime, templateEntry.EndTime)

				entry := &models.ScheduleEntry{
					ScheduleID: scheduleID,
					UserID:     uid,
					Date:       currentDate,
					ShiftType:  shiftType,
					StartTime:  startTime,
					EndTime:    endTime,
					Title:      templateEntry.Title,
					Location:   templateEntry.Location,
					CreatedBy:  userID,
				}

				entries = append(entries, entry)
			}
		}

		// Move to next day
		currentDate = currentDate.AddDate(0, 0, 1)
	}

	if len(entries) == 0 {
		return 0, errors.New("no entries generated from template")
	}

	// Create all entries
	if err := u.scheduleRepo.CreateScheduleEntries(entries); err != nil {
		return 0, fmt.Errorf("failed to create schedule entries: %w", err)
	}

	return len(entries), nil
}

// Helper functions

// parseTimeOnDate parses a time string (HH:MM) and applies it to a date
func parseTimeOnDate(date time.Time, timeStr string) time.Time {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return date
	}

	var hour, minute int
	fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)

	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location())
}

// determineShiftType determines shift type based on times
func determineShiftType(schedule *models.Schedule, startTime, endTime string) models.ShiftType {
	if startTime == schedule.MorningStart && endTime == schedule.MorningEnd {
		return models.ShiftMorning
	}
	if startTime == schedule.EveningStart && endTime == schedule.EveningEnd {
		return models.ShiftEvening
	}
	if startTime == schedule.MorningStart && endTime == schedule.EveningEnd {
		return models.ShiftFullDay
	}
	return models.ShiftCustom
}
