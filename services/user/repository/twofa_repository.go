package repository

import (
	"fmt"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// TwoFARepository defines interface for 2FA code data access
type TwoFARepository interface {
	Create(code *models.TwoFactorCode) error
	GetValidCode(email, code string) (*models.TwoFactorCode, error)
	MarkAsVerified(id uint) error
	DeleteExpired() error
	DeleteByUserID(userID uint) error
}

// twoFARepository implements TwoFARepository interface
type twoFARepository struct {
	db *database.DB
}

// NewTwoFARepository creates a new 2FA repository
func NewTwoFARepository(db *database.DB) TwoFARepository {
	return &twoFARepository{
		db: db,
	}
}

// Create creates a new 2FA code
func (r *twoFARepository) Create(code *models.TwoFactorCode) error {
	if err := r.db.DB.Create(code).Error; err != nil {
		return fmt.Errorf("failed to create 2FA code: %w", err)
	}
	return nil
}

// GetValidCode retrieves a valid (not expired, not verified) 2FA code
func (r *twoFARepository) GetValidCode(email, code string) (*models.TwoFactorCode, error) {
	var twoFACode models.TwoFactorCode
	err := r.db.DB.Where("email = ? AND code = ? AND verified = ? AND expires_at > ?",
		email, code, false, time.Now()).
		Order("created_at DESC").
		First(&twoFACode).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("2FA code not found or expired")
		}
		return nil, fmt.Errorf("failed to get 2FA code: %w", err)
	}

	return &twoFACode, nil
}

// MarkAsVerified marks a 2FA code as verified
func (r *twoFARepository) MarkAsVerified(id uint) error {
	err := r.db.DB.Model(&models.TwoFactorCode{}).
		Where("id = ?", id).
		Update("verified", true).Error

	if err != nil {
		return fmt.Errorf("failed to mark 2FA code as verified: %w", err)
	}

	return nil
}

// DeleteExpired deletes all expired 2FA codes
func (r *twoFARepository) DeleteExpired() error {
	err := r.db.DB.Where("expires_at < ?", time.Now()).
		Delete(&models.TwoFactorCode{}).Error

	if err != nil {
		return fmt.Errorf("failed to delete expired 2FA codes: %w", err)
	}

	return nil
}

// DeleteByUserID deletes all 2FA codes for a specific user
func (r *twoFARepository) DeleteByUserID(userID uint) error {
	err := r.db.DB.Where("user_id = ?", userID).
		Delete(&models.TwoFactorCode{}).Error

	if err != nil {
		return fmt.Errorf("failed to delete 2FA codes for user: %w", err)
	}

	return nil
}
