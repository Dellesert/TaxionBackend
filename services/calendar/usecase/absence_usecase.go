package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
)

// AbsenceUsecase defines the interface for absence business logic
type AbsenceUsecase interface {
	CreateAbsence(creatorID uint, req *models.CreateAbsenceRequest) (*models.AbsenceResponse, error)
	GetAbsence(id uint) (*models.AbsenceResponse, error)
	GetAbsences(filter AbsenceFilterParams) (*models.AbsenceListResponse, error)
	GetUserAbsences(userID uint, startDate, endDate time.Time) ([]*models.AbsenceResponse, error)
	UpdateAbsence(userID, absenceID uint, req *models.UpdateAbsenceRequest) (*models.AbsenceResponse, error)
	DeleteAbsence(userID, absenceID uint) error

	// For schedule integration
	IsUserAbsent(userID uint, date time.Time) (bool, *models.Absence, error)
	GetAbsentUsersForPeriod(userIDs []uint, startDate, endDate time.Time) (map[uint][]*models.Absence, error)

	// Substitution methods
	GetSubstitutions(absenceID uint) ([]*models.SubstitutionResponse, error)
	CreateSubstitution(creatorID, absenceID uint, req *models.CreateSubstitutionRequest) (*models.SubstitutionResponse, error)
	UpdateSubstitution(userID, absenceID, subID uint, req *models.UpdateSubstitutionRequest) (*models.SubstitutionResponse, error)
	DeleteSubstitution(userID, absenceID, subID uint) error
	GetUserSubstitutions(userID uint, startDate, endDate time.Time) ([]*models.SubstitutionResponse, error)
}

// AbsenceFilterParams represents filtering parameters
type AbsenceFilterParams struct {
	UserID    *uint
	Type      *models.AbsenceType
	StartDate *time.Time
	EndDate   *time.Time
	Limit     int
	Offset    int
	SortOrder string // "asc" or "desc", default "desc"
}

// absenceUsecase implements AbsenceUsecase interface
type absenceUsecase struct {
	absenceRepo      repository.AbsenceRepository
	eventRepo        repository.EventRepository
	participantRepo  repository.ParticipantRepository
	substitutionRepo repository.SubstitutionRepository
}

// NewAbsenceUsecase creates a new absence usecase
func NewAbsenceUsecase(
	absenceRepo repository.AbsenceRepository,
	eventRepo repository.EventRepository,
	participantRepo repository.ParticipantRepository,
	substitutionRepo repository.SubstitutionRepository,
) AbsenceUsecase {
	return &absenceUsecase{
		absenceRepo:      absenceRepo,
		eventRepo:        eventRepo,
		participantRepo:  participantRepo,
		substitutionRepo: substitutionRepo,
	}
}

// CreateAbsence creates a new absence record
func (u *absenceUsecase) CreateAbsence(creatorID uint, req *models.CreateAbsenceRequest) (*models.AbsenceResponse, error) {
	// Validate request
	if err := u.validateCreateAbsenceRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for overlapping absences
	hasOverlap, err := u.absenceRepo.CheckAbsenceOverlap(req.UserID, req.StartDate, req.EndDate, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check overlap: %w", err)
	}
	if hasOverlap {
		return nil, errors.New("пользователь уже имеет отсутствие на этот период")
	}

	// Create absence model
	absence := &models.Absence{
		UserID:    req.UserID,
		Type:      req.Type,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Reason:    strings.TrimSpace(req.Reason),
		CreatedBy: creatorID,
	}

	// Save absence
	if err := u.absenceRepo.CreateAbsence(absence); err != nil {
		return nil, fmt.Errorf("failed to create absence: %w", err)
	}

	// Create calendar event for this absence
	if err := u.createAbsenceEvent(absence); err != nil {
		// Log error but don't fail the absence creation
		fmt.Printf("Warning: failed to create calendar event for absence %d: %v\n", absence.ID, err)
	}

	// Get absence with relations
	createdAbsence, err := u.absenceRepo.GetAbsenceByID(absence.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created absence: %w", err)
	}

	return createdAbsence.ToResponse(), nil
}

// GetAbsence retrieves an absence by ID
func (u *absenceUsecase) GetAbsence(id uint) (*models.AbsenceResponse, error) {
	absence, err := u.absenceRepo.GetAbsenceByID(id)
	if err != nil {
		return nil, err
	}

	return absence.ToResponse(), nil
}

// GetAbsences retrieves absences with filtering
func (u *absenceUsecase) GetAbsences(filter AbsenceFilterParams) (*models.AbsenceListResponse, error) {
	// Convert to repository filter
	repoFilter := repository.AbsenceFilter{
		UserID:    filter.UserID,
		Type:      filter.Type,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
		SortOrder: filter.SortOrder,
	}

	absences, total, err := u.absenceRepo.GetAbsences(repoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	// Convert to responses
	responses := make([]*models.AbsenceResponse, len(absences))
	for i, absence := range absences {
		responses[i] = absence.ToResponse()
	}

	return &models.AbsenceListResponse{
		Absences: responses,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	}, nil
}

// GetUserAbsences retrieves all absences for a user in a date range
func (u *absenceUsecase) GetUserAbsences(userID uint, startDate, endDate time.Time) ([]*models.AbsenceResponse, error) {
	absences, err := u.absenceRepo.GetUserAbsences(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user absences: %w", err)
	}

	// Convert to responses
	responses := make([]*models.AbsenceResponse, len(absences))
	for i, absence := range absences {
		responses[i] = absence.ToResponse()
	}

	return responses, nil
}

// UpdateAbsence updates an existing absence
func (u *absenceUsecase) UpdateAbsence(userID, absenceID uint, req *models.UpdateAbsenceRequest) (*models.AbsenceResponse, error) {
	// Get existing absence
	absence, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Type != nil {
		absence.Type = *req.Type
	}
	if req.StartDate != nil {
		absence.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		absence.EndDate = *req.EndDate
	}
	if req.Reason != nil {
		absence.Reason = strings.TrimSpace(*req.Reason)
	}

	// Validate updated dates
	if absence.EndDate.Before(absence.StartDate) {
		return nil, errors.New("дата окончания должна быть после даты начала")
	}

	// Check for overlapping absences (excluding current)
	hasOverlap, err := u.absenceRepo.CheckAbsenceOverlap(absence.UserID, absence.StartDate, absence.EndDate, &absenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to check overlap: %w", err)
	}
	if hasOverlap {
		return nil, errors.New("пользователь уже имеет отсутствие на этот период")
	}

	// Save updated absence
	if err := u.absenceRepo.UpdateAbsence(absence); err != nil {
		return nil, fmt.Errorf("failed to update absence: %w", err)
	}

	// Update calendar event for this absence
	if err := u.updateAbsenceEvent(absence); err != nil {
		// Log error but don't fail the absence update
		fmt.Printf("Warning: failed to update calendar event for absence %d: %v\n", absence.ID, err)
	}

	// Get updated absence with relations
	updatedAbsence, err := u.absenceRepo.GetAbsenceByID(absence.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated absence: %w", err)
	}

	return updatedAbsence.ToResponse(), nil
}

// DeleteAbsence deletes an absence
func (u *absenceUsecase) DeleteAbsence(userID, absenceID uint) error {
	// Check if absence exists
	_, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return err
	}

	// Delete all substitutions first
	if err := u.substitutionRepo.DeleteSubstitutionsByAbsenceID(absenceID); err != nil {
		// Log error but don't fail the absence deletion
		fmt.Printf("Warning: failed to delete substitutions for absence %d: %v\n", absenceID, err)
	}

	// Delete calendar event (before deleting absence)
	if err := u.deleteAbsenceEvent(absenceID); err != nil {
		// Log error but don't fail the absence deletion
		fmt.Printf("Warning: failed to delete calendar event for absence %d: %v\n", absenceID, err)
	}

	// Delete absence
	if err := u.absenceRepo.DeleteAbsence(absenceID); err != nil {
		return fmt.Errorf("failed to delete absence: %w", err)
	}

	return nil
}

// IsUserAbsent checks if a user is absent on a specific date
func (u *absenceUsecase) IsUserAbsent(userID uint, date time.Time) (bool, *models.Absence, error) {
	return u.absenceRepo.IsUserAbsent(userID, date)
}

// GetAbsentUsersForPeriod returns a map of user IDs to their absences
func (u *absenceUsecase) GetAbsentUsersForPeriod(userIDs []uint, startDate, endDate time.Time) (map[uint][]*models.Absence, error) {
	return u.absenceRepo.GetAbsentUsersForPeriod(userIDs, startDate, endDate)
}

// Helper functions

// validateCreateAbsenceRequest validates create absence request
func (u *absenceUsecase) validateCreateAbsenceRequest(req *models.CreateAbsenceRequest) error {
	if req.UserID == 0 {
		return errors.New("user_id is required")
	}

	if req.Type == "" {
		return errors.New("type is required")
	}

	if req.StartDate.IsZero() {
		return errors.New("start_date is required")
	}

	if req.EndDate.IsZero() {
		return errors.New("end_date is required")
	}

	if req.EndDate.Before(req.StartDate) {
		return errors.New("дата окончания должна быть после даты начала")
	}

	return nil
}

// GetAbsenceTypeName returns human-readable name for absence type
func GetAbsenceTypeName(t models.AbsenceType) string {
	switch t {
	case models.AbsenceTypeVacation:
		return "Отпуск"
	case models.AbsenceTypeSickLeave:
		return "Больничный"
	case models.AbsenceTypeDayOff:
		return "Отгул"
	case models.AbsenceTypeBusinessTrip:
		return "Командировка"
	case models.AbsenceTypeStudyLeave:
		return "Учебный отпуск"
	default:
		return string(t)
	}
}

// getAbsenceColor returns a color for the absence event based on type
func getAbsenceColor(t models.AbsenceType) string {
	switch t {
	case models.AbsenceTypeVacation:
		return "#4CAF50" // Green
	case models.AbsenceTypeSickLeave:
		return "#F44336" // Red
	case models.AbsenceTypeDayOff:
		return "#FF9800" // Orange
	case models.AbsenceTypeBusinessTrip:
		return "#2196F3" // Blue
	case models.AbsenceTypeStudyLeave:
		return "#9C27B0" // Purple
	default:
		return "#757575" // Gray
	}
}

// createAbsenceEvent creates a calendar event for an absence
func (u *absenceUsecase) createAbsenceEvent(absence *models.Absence) error {
	// Create event for the entire absence period (all-day event)
	// StartTime: beginning of start date, EndTime: end of end date
	startTime := time.Date(absence.StartDate.Year(), absence.StartDate.Month(), absence.StartDate.Day(), 0, 0, 0, 0, absence.StartDate.Location())
	endTime := time.Date(absence.EndDate.Year(), absence.EndDate.Month(), absence.EndDate.Day(), 23, 59, 59, 0, absence.EndDate.Location())

	event := &models.Event{
		Title:       GetAbsenceTypeName(absence.Type),
		Description: absence.Reason,
		StartTime:   startTime,
		EndTime:     endTime,
		AllDay:      true,
		Type:        models.EventTypeAbsence,
		CreatedBy:   absence.UserID, // Event should belong to the user who is absent
		Color:       getAbsenceColor(absence.Type),
		IsPrivate:   false,
		AbsenceID:   &absence.ID,
	}

	// Prepare participants list - only the user who is absent
	// Creator should NOT see the absence event in their calendar
	participants := []*models.EventParticipant{
		{
			UserID:      absence.UserID,
			Status:      models.ParticipantStatusAccepted,
			IsOrganizer: true,
		},
	}

	// Create event with participants in a single transaction
	if err := u.eventRepo.CreateEventWithParticipants(event, participants); err != nil {
		return fmt.Errorf("failed to create absence event: %w", err)
	}

	return nil
}

// updateAbsenceEvent updates the calendar event for an absence
func (u *absenceUsecase) updateAbsenceEvent(absence *models.Absence) error {
	// Find existing event
	event, err := u.eventRepo.GetEventByAbsenceID(absence.ID)
	if err != nil {
		return fmt.Errorf("failed to find event: %w", err)
	}

	if event == nil {
		// Event doesn't exist, create it
		return u.createAbsenceEvent(absence)
	}

	// Update event fields
	startTime := time.Date(absence.StartDate.Year(), absence.StartDate.Month(), absence.StartDate.Day(), 0, 0, 0, 0, absence.StartDate.Location())
	endTime := time.Date(absence.EndDate.Year(), absence.EndDate.Month(), absence.EndDate.Day(), 23, 59, 59, 0, absence.EndDate.Location())

	event.Title = GetAbsenceTypeName(absence.Type)
	event.Description = absence.Reason
	event.StartTime = startTime
	event.EndTime = endTime
	event.Color = getAbsenceColor(absence.Type)

	if err := u.eventRepo.UpdateEvent(event); err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}

	return nil
}

// deleteAbsenceEvent deletes the calendar event for an absence
func (u *absenceUsecase) deleteAbsenceEvent(absenceID uint) error {
	if err := u.eventRepo.DeleteEventByAbsenceID(absenceID); err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}
	return nil
}

// Substitution methods

// GetSubstitutions retrieves all substitutions for an absence
func (u *absenceUsecase) GetSubstitutions(absenceID uint) ([]*models.SubstitutionResponse, error) {
	// Check if absence exists
	_, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return nil, err
	}

	subs, err := u.substitutionRepo.GetSubstitutionsByAbsenceID(absenceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get substitutions: %w", err)
	}

	responses := make([]*models.SubstitutionResponse, len(subs))
	for i, sub := range subs {
		responses[i] = sub.ToResponse()
	}

	return responses, nil
}

// CreateSubstitution creates a new substitution for an absence
func (u *absenceUsecase) CreateSubstitution(creatorID, absenceID uint, req *models.CreateSubstitutionRequest) (*models.SubstitutionResponse, error) {
	// Get absence to validate
	absence, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return nil, err
	}

	// Validate request
	if err := u.validateCreateSubstitutionRequest(absence, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for overlapping substitutions for the same substitute
	hasOverlap, err := u.substitutionRepo.CheckSubstitutionOverlap(absenceID, req.SubstituteID, req.StartDate, req.EndDate, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check overlap: %w", err)
	}
	if hasOverlap {
		return nil, errors.New("этот сотрудник уже является замещающим на этот период")
	}

	// Create substitution model
	sub := &models.AbsenceSubstitution{
		AbsenceID:    absenceID,
		SubstituteID: req.SubstituteID,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Note:         strings.TrimSpace(req.Note),
		CreatedBy:    creatorID,
	}

	// Save substitution
	if err := u.substitutionRepo.CreateSubstitution(sub); err != nil {
		return nil, fmt.Errorf("failed to create substitution: %w", err)
	}

	// Get substitution with relations
	createdSub, err := u.substitutionRepo.GetSubstitutionByID(sub.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created substitution: %w", err)
	}

	return createdSub.ToResponse(), nil
}

// UpdateSubstitution updates an existing substitution
func (u *absenceUsecase) UpdateSubstitution(userID, absenceID, subID uint, req *models.UpdateSubstitutionRequest) (*models.SubstitutionResponse, error) {
	// Get absence to validate
	absence, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return nil, err
	}

	// Get existing substitution
	sub, err := u.substitutionRepo.GetSubstitutionByID(subID)
	if err != nil {
		return nil, err
	}

	// Verify substitution belongs to the absence
	if sub.AbsenceID != absenceID {
		return nil, errors.New("substitution not found")
	}

	// Update fields if provided
	if req.SubstituteID != nil {
		sub.SubstituteID = *req.SubstituteID
	}
	if req.StartDate != nil {
		sub.StartDate = *req.StartDate
	}
	if req.EndDate != nil {
		sub.EndDate = *req.EndDate
	}
	if req.Note != nil {
		sub.Note = strings.TrimSpace(*req.Note)
	}

	// Validate updated substitution
	if err := u.validateSubstitutionDates(absence, sub.StartDate, sub.EndDate, sub.SubstituteID); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check for overlapping substitutions (excluding current)
	hasOverlap, err := u.substitutionRepo.CheckSubstitutionOverlap(absenceID, sub.SubstituteID, sub.StartDate, sub.EndDate, &subID)
	if err != nil {
		return nil, fmt.Errorf("failed to check overlap: %w", err)
	}
	if hasOverlap {
		return nil, errors.New("этот сотрудник уже является замещающим на этот период")
	}

	// Save updated substitution
	if err := u.substitutionRepo.UpdateSubstitution(sub); err != nil {
		return nil, fmt.Errorf("failed to update substitution: %w", err)
	}

	// Get updated substitution with relations
	updatedSub, err := u.substitutionRepo.GetSubstitutionByID(sub.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated substitution: %w", err)
	}

	return updatedSub.ToResponse(), nil
}

// DeleteSubstitution deletes a substitution
func (u *absenceUsecase) DeleteSubstitution(userID, absenceID, subID uint) error {
	// Check if absence exists
	_, err := u.absenceRepo.GetAbsenceByID(absenceID)
	if err != nil {
		return err
	}

	// Get substitution to verify it belongs to the absence
	sub, err := u.substitutionRepo.GetSubstitutionByID(subID)
	if err != nil {
		return err
	}

	if sub.AbsenceID != absenceID {
		return errors.New("substitution not found")
	}

	// Delete substitution
	if err := u.substitutionRepo.DeleteSubstitution(subID); err != nil {
		return fmt.Errorf("failed to delete substitution: %w", err)
	}

	return nil
}

// GetUserSubstitutions retrieves all substitutions where user is a substitute
func (u *absenceUsecase) GetUserSubstitutions(userID uint, startDate, endDate time.Time) ([]*models.SubstitutionResponse, error) {
	subs, err := u.substitutionRepo.GetSubstitutionsBySubstituteID(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get user substitutions: %w", err)
	}

	responses := make([]*models.SubstitutionResponse, len(subs))
	for i, sub := range subs {
		responses[i] = sub.ToResponse()
	}

	return responses, nil
}

// validateCreateSubstitutionRequest validates create substitution request
func (u *absenceUsecase) validateCreateSubstitutionRequest(absence *models.Absence, req *models.CreateSubstitutionRequest) error {
	if req.SubstituteID == 0 {
		return errors.New("substitute_id is required")
	}

	if req.StartDate.IsZero() {
		return errors.New("start_date is required")
	}

	if req.EndDate.IsZero() {
		return errors.New("end_date is required")
	}

	return u.validateSubstitutionDates(absence, req.StartDate, req.EndDate, req.SubstituteID)
}

// validateSubstitutionDates validates substitution dates against absence dates
func (u *absenceUsecase) validateSubstitutionDates(absence *models.Absence, startDate, endDate time.Time, substituteID uint) error {
	// EndDate must be >= StartDate
	if endDate.Before(startDate) {
		return errors.New("дата окончания должна быть после даты начала")
	}

	// Substitute cannot be the same as absent user
	if substituteID == absence.UserID {
		return errors.New("замещающий не может быть тем же сотрудником, который отсутствует")
	}

	// Substitution dates must be within absence dates
	absenceStart := time.Date(absence.StartDate.Year(), absence.StartDate.Month(), absence.StartDate.Day(), 0, 0, 0, 0, absence.StartDate.Location())
	absenceEnd := time.Date(absence.EndDate.Year(), absence.EndDate.Month(), absence.EndDate.Day(), 23, 59, 59, 0, absence.EndDate.Location())
	subStart := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	subEnd := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, endDate.Location())

	if subStart.Before(absenceStart) {
		return errors.New("дата начала замещения не может быть раньше даты начала отсутствия")
	}

	if subEnd.After(absenceEnd) {
		return errors.New("дата окончания замещения не может быть позже даты окончания отсутствия")
	}

	return nil
}
