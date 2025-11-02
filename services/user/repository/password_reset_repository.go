package repository

import (
	"fmt"
	"time"

	"tachyon-messenger/services/user/models"

	"gorm.io/gorm"
)

// PasswordResetRepository defines the interface for password reset data operations
type PasswordResetRepository interface {
	Create(reset *models.PasswordReset) error
	GetByID(id uint) (*models.PasswordReset, error)
	GetByToken(token string) (*models.PasswordReset, error)
	GetByUserID(userID uint) (*models.PasswordReset, error)
	GetWithRelations(id uint) (*models.PasswordReset, error)
	GetByTokenWithRelations(token string) (*models.PasswordReset, error)
	Update(reset *models.PasswordReset) error
	Delete(id uint) error
	ExpireOldResets() (int64, error)
	HasPendingReset(userID uint) (bool, error)
	CancelPendingResetsByUserID(userID uint) error
}

// passwordResetRepository implements PasswordResetRepository interface
type passwordResetRepository struct {
	db *gorm.DB
}

// NewPasswordResetRepository creates a new password reset repository
func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &passwordResetRepository{
		db: db,
	}
}

// Create creates a new password reset
func (r *passwordResetRepository) Create(reset *models.PasswordReset) error {
	return r.db.Create(reset).Error
}

// GetByID retrieves a password reset by ID
func (r *passwordResetRepository) GetByID(id uint) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	err := r.db.Where("id = ?", id).First(&reset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("password reset not found")
		}
		return nil, err
	}
	return &reset, nil
}

// GetByToken retrieves a password reset by token
func (r *passwordResetRepository) GetByToken(token string) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	err := r.db.Where("token = ?", token).First(&reset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("password reset not found")
		}
		return nil, err
	}
	return &reset, nil
}

// GetByUserID retrieves the latest password reset by user ID
func (r *passwordResetRepository) GetByUserID(userID uint) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").First(&reset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("password reset not found")
		}
		return nil, err
	}
	return &reset, nil
}

// GetWithRelations retrieves a password reset with user and created_by relations
func (r *passwordResetRepository) GetWithRelations(id uint) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	err := r.db.
		Preload("User").
		Preload("CreatedBy").
		Where("id = ?", id).
		First(&reset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("password reset not found")
		}
		return nil, err
	}
	return &reset, nil
}

// GetByTokenWithRelations retrieves a password reset by token with relations
func (r *passwordResetRepository) GetByTokenWithRelations(token string) (*models.PasswordReset, error) {
	var reset models.PasswordReset
	err := r.db.
		Preload("User").
		Preload("CreatedBy").
		Where("token = ?", token).
		First(&reset).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("password reset not found")
		}
		return nil, err
	}
	return &reset, nil
}

// Update updates a password reset
func (r *passwordResetRepository) Update(reset *models.PasswordReset) error {
	return r.db.Save(reset).Error
}

// Delete deletes a password reset
func (r *passwordResetRepository) Delete(id uint) error {
	return r.db.Delete(&models.PasswordReset{}, id).Error
}

// ExpireOldResets marks expired password resets as expired
func (r *passwordResetRepository) ExpireOldResets() (int64, error) {
	result := r.db.
		Model(&models.PasswordReset{}).
		Where("status = ? AND expires_at < ?", models.PasswordResetStatusPending, time.Now()).
		Update("status", models.PasswordResetStatusExpired)

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// HasPendingReset checks if a user has a pending password reset
func (r *passwordResetRepository) HasPendingReset(userID uint) (bool, error) {
	var count int64
	err := r.db.
		Model(&models.PasswordReset{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, models.PasswordResetStatusPending, time.Now()).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CancelPendingResetsByUserID cancels all pending password resets for a user
func (r *passwordResetRepository) CancelPendingResetsByUserID(userID uint) error {
	return r.db.
		Model(&models.PasswordReset{}).
		Where("user_id = ? AND status = ?", userID, models.PasswordResetStatusPending).
		Update("status", models.PasswordResetStatusCancelled).Error
}
