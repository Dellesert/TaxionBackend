package repository

import (
	"errors"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// SubstitutionRepository defines the interface for substitution data operations
type SubstitutionRepository interface {
	// CRUD operations
	CreateSubstitution(sub *models.AbsenceSubstitution) error
	GetSubstitutionByID(id uint) (*models.AbsenceSubstitution, error)
	GetSubstitutionsByAbsenceID(absenceID uint) ([]*models.AbsenceSubstitution, error)
	GetSubstitutionsByAbsenceIDs(absenceIDs []uint, date time.Time) (map[uint][]*models.AbsenceSubstitution, error)
	GetSubstitutionsBySubstituteID(userID uint, startDate, endDate time.Time) ([]*models.AbsenceSubstitution, error)
	UpdateSubstitution(sub *models.AbsenceSubstitution) error
	DeleteSubstitution(id uint) error
	DeleteSubstitutionsByAbsenceID(absenceID uint) error

	// Overlap checking
	CheckSubstitutionOverlap(absenceID, substituteID uint, startDate, endDate time.Time, excludeID *uint) (bool, error)
}

// substitutionRepository implements SubstitutionRepository interface
type substitutionRepository struct {
	db *database.DB
}

// NewSubstitutionRepository creates a new substitution repository
func NewSubstitutionRepository(db *database.DB) SubstitutionRepository {
	return &substitutionRepository{
		db: db,
	}
}

// CreateSubstitution creates a new substitution record
func (r *substitutionRepository) CreateSubstitution(sub *models.AbsenceSubstitution) error {
	if sub == nil {
		return errors.New("substitution cannot be nil")
	}

	return r.db.Create(sub).Error
}

// GetSubstitutionByID retrieves a substitution by ID
func (r *substitutionRepository) GetSubstitutionByID(id uint) (*models.AbsenceSubstitution, error) {
	var sub models.AbsenceSubstitution
	err := r.db.
		Preload("Substitute").
		Preload("Creator").
		First(&sub, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("substitution not found")
		}
		return nil, err
	}
	return &sub, nil
}

// GetSubstitutionsByAbsenceID retrieves all substitutions for an absence
func (r *substitutionRepository) GetSubstitutionsByAbsenceID(absenceID uint) ([]*models.AbsenceSubstitution, error) {
	var subs []*models.AbsenceSubstitution

	err := r.db.
		Preload("Substitute").
		Preload("Creator").
		Where("absence_id = ?", absenceID).
		Order("start_date ASC").
		Find(&subs).Error

	if err != nil {
		return nil, err
	}

	return subs, nil
}

// GetSubstitutionsByAbsenceIDs retrieves substitutions for multiple absences that cover a specific date, grouped by absence ID
func (r *substitutionRepository) GetSubstitutionsByAbsenceIDs(absenceIDs []uint, date time.Time) (map[uint][]*models.AbsenceSubstitution, error) {
	if len(absenceIDs) == 0 {
		return make(map[uint][]*models.AbsenceSubstitution), nil
	}

	var subs []*models.AbsenceSubstitution

	err := r.db.
		Preload("Substitute").
		Where("absence_id IN ? AND start_date <= ? AND end_date >= ?", absenceIDs, date, date).
		Order("absence_id ASC, id ASC").
		Find(&subs).Error

	if err != nil {
		return nil, err
	}

	result := make(map[uint][]*models.AbsenceSubstitution)
	for _, sub := range subs {
		result[sub.AbsenceID] = append(result[sub.AbsenceID], sub)
	}

	return result, nil
}

// GetSubstitutionsBySubstituteID retrieves all substitutions where user is a substitute
func (r *substitutionRepository) GetSubstitutionsBySubstituteID(userID uint, startDate, endDate time.Time) ([]*models.AbsenceSubstitution, error) {
	var subs []*models.AbsenceSubstitution

	err := r.db.
		Preload("Absence").
		Preload("Absence.User").
		Preload("Creator").
		Where("substitute_id = ? AND end_date >= ? AND start_date <= ?", userID, startDate, endDate).
		Order("start_date ASC").
		Find(&subs).Error

	if err != nil {
		return nil, err
	}

	return subs, nil
}

// UpdateSubstitution updates an existing substitution
func (r *substitutionRepository) UpdateSubstitution(sub *models.AbsenceSubstitution) error {
	if sub == nil {
		return errors.New("substitution cannot be nil")
	}

	return r.db.Save(sub).Error
}

// DeleteSubstitution deletes a substitution
func (r *substitutionRepository) DeleteSubstitution(id uint) error {
	result := r.db.Delete(&models.AbsenceSubstitution{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("substitution not found")
	}
	return nil
}

// DeleteSubstitutionsByAbsenceID deletes all substitutions for an absence
func (r *substitutionRepository) DeleteSubstitutionsByAbsenceID(absenceID uint) error {
	return r.db.Where("absence_id = ?", absenceID).Delete(&models.AbsenceSubstitution{}).Error
}

// CheckSubstitutionOverlap checks if there is an overlapping substitution for the same substitute in the same absence
func (r *substitutionRepository) CheckSubstitutionOverlap(absenceID, substituteID uint, startDate, endDate time.Time, excludeID *uint) (bool, error) {
	query := r.db.Model(&models.AbsenceSubstitution{}).
		Where("absence_id = ?", absenceID).
		Where("substitute_id = ?", substituteID).
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
