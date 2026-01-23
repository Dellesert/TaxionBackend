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
	absenceRepo repository.AbsenceRepository
}

// NewAbsenceUsecase creates a new absence usecase
func NewAbsenceUsecase(absenceRepo repository.AbsenceRepository) AbsenceUsecase {
	return &absenceUsecase{
		absenceRepo: absenceRepo,
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
		return "отпуск"
	case models.AbsenceTypeSickLeave:
		return "больничный"
	case models.AbsenceTypeDayOff:
		return "отгул"
	case models.AbsenceTypeBusinessTrip:
		return "командировка"
	case models.AbsenceTypeStudyLeave:
		return "учебный отпуск"
	default:
		return string(t)
	}
}
