package repository

import (
	"fmt"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"
)

// PasskeyRepository defines the interface for passkey credential data access
type PasskeyRepository interface {
	Create(passkey *models.PasskeyCredential) error
	GetByID(id uint) (*models.PasskeyCredential, error)
	GetByCredentialID(credentialID []byte) (*models.PasskeyCredential, error)
	GetByUserID(userID uint) ([]*models.PasskeyCredential, error)
	Update(passkey *models.PasskeyCredential) error
	Delete(id uint) error
	DeleteByUserID(userID uint) error
	CountByUserID(userID uint) (int64, error)
	CountTotal() (int64, error)
}

// passkeyRepository implements PasskeyRepository interface
type passkeyRepository struct {
	db *database.DB
}

// NewPasskeyRepository creates a new passkey repository
func NewPasskeyRepository(db *database.DB) PasskeyRepository {
	return &passkeyRepository{
		db: db,
	}
}

// Create creates a new passkey credential
func (r *passkeyRepository) Create(passkey *models.PasskeyCredential) error {
	if err := r.db.DB.Create(passkey).Error; err != nil {
		return fmt.Errorf("failed to create passkey credential: %w", err)
	}
	return nil
}

// GetByID retrieves a passkey by ID
func (r *passkeyRepository) GetByID(id uint) (*models.PasskeyCredential, error) {
	var passkey models.PasskeyCredential
	if err := r.db.DB.First(&passkey, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get passkey credential: %w", err)
	}
	return &passkey, nil
}

// GetByCredentialID retrieves a passkey by credential ID
func (r *passkeyRepository) GetByCredentialID(credentialID []byte) (*models.PasskeyCredential, error) {
	var passkey models.PasskeyCredential
	if err := r.db.DB.Where("credential_id = ?", credentialID).First(&passkey).Error; err != nil {
		return nil, fmt.Errorf("failed to get passkey by credential ID: %w", err)
	}
	return &passkey, nil
}

// GetByUserID retrieves all passkeys for a user
func (r *passkeyRepository) GetByUserID(userID uint) ([]*models.PasskeyCredential, error) {
	var passkeys []*models.PasskeyCredential
	if err := r.db.DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&passkeys).Error; err != nil {
		return nil, fmt.Errorf("failed to get passkeys for user: %w", err)
	}
	return passkeys, nil
}

// Update updates a passkey credential
func (r *passkeyRepository) Update(passkey *models.PasskeyCredential) error {
	if err := r.db.DB.Save(passkey).Error; err != nil {
		return fmt.Errorf("failed to update passkey credential: %w", err)
	}
	return nil
}

// Delete deletes a passkey credential
func (r *passkeyRepository) Delete(id uint) error {
	if err := r.db.DB.Delete(&models.PasskeyCredential{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete passkey credential: %w", err)
	}
	return nil
}

// DeleteByUserID deletes all passkeys for a user
func (r *passkeyRepository) DeleteByUserID(userID uint) error {
	if err := r.db.DB.Where("user_id = ?", userID).Delete(&models.PasskeyCredential{}).Error; err != nil {
		return fmt.Errorf("failed to delete passkeys for user: %w", err)
	}
	return nil
}

// CountByUserID counts passkeys for a user
func (r *passkeyRepository) CountByUserID(userID uint) (int64, error) {
	var count int64
	if err := r.db.DB.Model(&models.PasskeyCredential{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count passkeys for user: %w", err)
	}
	return count, nil
}

// CountTotal counts total passkeys in the system
func (r *passkeyRepository) CountTotal() (int64, error) {
	var count int64
	if err := r.db.DB.Model(&models.PasskeyCredential{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count total passkeys: %w", err)
	}
	return count, nil
}
