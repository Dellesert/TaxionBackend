package repository

import (
	"errors"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// AbsenceRepository defines the interface for absence data operations
type AbsenceRepository interface {
	// CRUD operations
	CreateAbsence(absence *models.Absence) error
	GetAbsenceByID(id uint) (*models.Absence, error)
	GetAbsences(filter AbsenceFilter) ([]*models.Absence, int64, error)
	GetUserAbsences(userID uint, startDate, endDate time.Time) ([]*models.Absence, error)
	UpdateAbsence(absence *models.Absence) error
	DeleteAbsence(id uint) error

	// Overlap checking
	CheckAbsenceOverlap(userID uint, startDate, endDate time.Time, excludeID *uint) (bool, error)

	// Integration with schedules
	IsUserAbsent(userID uint, date time.Time) (bool, *models.Absence, error)
	GetAbsentUsersForPeriod(userIDs []uint, startDate, endDate time.Time) (map[uint][]*models.Absence, error)
}

// AbsenceFilter defines filtering parameters for absences
type AbsenceFilter struct {
	UserID    *uint
	Type      *models.AbsenceType
	StartDate *time.Time
	EndDate   *time.Time
	CreatedBy *uint
	Limit     int
	Offset    int
	SortOrder string // "asc" or "desc", default "desc"
}

// absenceRepository implements AbsenceRepository interface
type absenceRepository struct {
	db *database.DB
}

// NewAbsenceRepository creates a new absence repository
func NewAbsenceRepository(db *database.DB) AbsenceRepository {
	return &absenceRepository{
		db: db,
	}
}

// CreateAbsence creates a new absence record
func (r *absenceRepository) CreateAbsence(absence *models.Absence) error {
	if absence == nil {
		return errors.New("absence cannot be nil")
	}

	return r.db.Create(absence).Error
}

// GetAbsenceByID retrieves an absence by ID
func (r *absenceRepository) GetAbsenceByID(id uint) (*models.Absence, error) {
	var absence models.Absence
	err := r.db.
		Preload("User").
		Preload("Creator").
		First(&absence, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("absence not found")
		}
		return nil, err
	}
	return &absence, nil
}

// GetAbsences retrieves absences with filtering
func (r *absenceRepository) GetAbsences(filter AbsenceFilter) ([]*models.Absence, int64, error) {
	var absences []*models.Absence
	var total int64

	query := r.db.Model(&models.Absence{}).
		Preload("User").
		Preload("Creator")

	// Apply filters
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.StartDate != nil {
		query = query.Where("end_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("start_date <= ?", *filter.EndDate)
	}
	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by start date (default DESC, or ASC if specified)
	if filter.SortOrder == "asc" {
		query = query.Order("start_date ASC")
	} else {
		query = query.Order("start_date DESC")
	}

	if err := query.Find(&absences).Error; err != nil {
		return nil, 0, err
	}

	return absences, total, nil
}

// GetUserAbsences retrieves all absences for a user in a date range
func (r *absenceRepository) GetUserAbsences(userID uint, startDate, endDate time.Time) ([]*models.Absence, error) {
	var absences []*models.Absence

	err := r.db.
		Preload("Creator").
		Where("user_id = ? AND end_date >= ? AND start_date <= ?", userID, startDate, endDate).
		Order("start_date ASC").
		Find(&absences).Error

	if err != nil {
		return nil, err
	}

	return absences, nil
}

// UpdateAbsence updates an existing absence
func (r *absenceRepository) UpdateAbsence(absence *models.Absence) error {
	if absence == nil {
		return errors.New("absence cannot be nil")
	}

	return r.db.Save(absence).Error
}

// DeleteAbsence deletes an absence
func (r *absenceRepository) DeleteAbsence(id uint) error {
	result := r.db.Delete(&models.Absence{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("absence not found")
	}
	return nil
}

// CheckAbsenceOverlap checks if there is an overlapping absence for a user
func (r *absenceRepository) CheckAbsenceOverlap(userID uint, startDate, endDate time.Time, excludeID *uint) (bool, error) {
	query := r.db.Model(&models.Absence{}).
		Where("user_id = ?", userID).
		Where("start_date <= ? AND end_date >= ?", endDate, startDate)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// IsUserAbsent checks if a user is absent on a specific date
func (r *absenceRepository) IsUserAbsent(userID uint, date time.Time) (bool, *models.Absence, error) {
	var absence models.Absence

	err := r.db.
		Where("user_id = ? AND start_date <= ? AND end_date >= ?", userID, date, date).
		First(&absence).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, &absence, nil
}

// GetAbsentUsersForPeriod returns a map of user IDs to their absences for a given period
func (r *absenceRepository) GetAbsentUsersForPeriod(userIDs []uint, startDate, endDate time.Time) (map[uint][]*models.Absence, error) {
	if len(userIDs) == 0 {
		return make(map[uint][]*models.Absence), nil
	}

	var absences []*models.Absence

	err := r.db.
		Where("user_id IN ? AND end_date >= ? AND start_date <= ?", userIDs, startDate, endDate).
		Order("user_id ASC, start_date ASC").
		Find(&absences).Error

	if err != nil {
		return nil, err
	}

	// Group by user ID
	result := make(map[uint][]*models.Absence)
	for _, absence := range absences {
		result[absence.UserID] = append(result[absence.UserID], absence)
	}

	return result, nil
}
