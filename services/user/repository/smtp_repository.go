package repository

import (
	"errors"
	"tachyon-messenger/services/user/models"

	"gorm.io/gorm"
)

// SMTPRepository defines interface for SMTP settings repository
type SMTPRepository interface {
	GetSettings() (*models.SMTPSettings, error)
	UpdateSettings(settings *models.SMTPSettings) error
	CreateSettings(settings *models.SMTPSettings) error
	SettingsExist() (bool, error)
}

// smtpRepository implements SMTPRepository
type smtpRepository struct {
	db *gorm.DB
}

// NewSMTPRepository creates a new SMTP repository
func NewSMTPRepository(db *gorm.DB) SMTPRepository {
	return &smtpRepository{db: db}
}

// GetSettings retrieves the SMTP settings (always returns the first/only record)
func (r *smtpRepository) GetSettings() (*models.SMTPSettings, error) {
	var settings models.SMTPSettings

	err := r.db.First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No settings configured yet
		}
		return nil, err
	}

	return &settings, nil
}

// UpdateSettings updates the SMTP settings
func (r *smtpRepository) UpdateSettings(settings *models.SMTPSettings) error {
	return r.db.Save(settings).Error
}

// CreateSettings creates new SMTP settings
func (r *smtpRepository) CreateSettings(settings *models.SMTPSettings) error {
	return r.db.Create(settings).Error
}

// SettingsExist checks if SMTP settings already exist
func (r *smtpRepository) SettingsExist() (bool, error) {
	var count int64
	err := r.db.Model(&models.SMTPSettings{}).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
