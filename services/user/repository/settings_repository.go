package repository

import (
	"fmt"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"
)

// SettingsRepository defines the interface for settings data access
type SettingsRepository interface {
	Get() (*models.SystemSettings, error)
	Create(settings *models.SystemSettings) error
	Update(settings *models.SystemSettings) error
	GetOrCreate() (*models.SystemSettings, error)
	GetUserSettings(userID uint) (*models.UserSettings, error)
	SaveUserSettings(settings *models.UserSettings) error
}

// settingsRepository implements SettingsRepository interface
type settingsRepository struct {
	db *database.DB
}

// NewSettingsRepository creates a new settings repository
func NewSettingsRepository(db *database.DB) SettingsRepository {
	return &settingsRepository{
		db: db,
	}
}

// Get retrieves the system settings (there should only be one record)
func (r *settingsRepository) Get() (*models.SystemSettings, error) {
	var settings models.SystemSettings
	if err := r.db.DB.First(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to get system settings: %w", err)
	}
	return &settings, nil
}

// Create creates new system settings
func (r *settingsRepository) Create(settings *models.SystemSettings) error {
	if err := r.db.DB.Create(settings).Error; err != nil {
		return fmt.Errorf("failed to create system settings: %w", err)
	}
	return nil
}

// Update updates existing system settings
func (r *settingsRepository) Update(settings *models.SystemSettings) error {
	if err := r.db.DB.Save(settings).Error; err != nil {
		return fmt.Errorf("failed to update system settings: %w", err)
	}
	return nil
}

// GetOrCreate retrieves existing settings or creates default ones
func (r *settingsRepository) GetOrCreate() (*models.SystemSettings, error) {
	settings, err := r.Get()
	if err != nil {
		// If not found, create default settings (medium security level)
		presets := models.GetSecurityPresets()
		defaultSettings := presets[models.SecurityLevelMedium]

		newSettings := &models.SystemSettings{
			SecurityLevel:             defaultSettings.SecurityLevel,
			AuthMode:                  defaultSettings.AuthMode,
			SecondFactorMode:          defaultSettings.SecondFactorMode,
			PasskeyAsSecondFactor:     defaultSettings.PasskeyAsSecondFactor,
			AllowMultiplePasskeys:     defaultSettings.AllowMultiplePasskeys,
			MaxPasskeysPerUser:        defaultSettings.MaxPasskeysPerUser,
			SessionDurationHours:      defaultSettings.SessionDurationHours,
			MinPasswordLength:         defaultSettings.MinPasswordLength,
			RequirePasswordComplexity: defaultSettings.RequirePasswordComplexity,
			PasswordExpirationDays:    defaultSettings.PasswordExpirationDays,
		}

		if err := r.Create(newSettings); err != nil {
			return nil, err
		}
		return newSettings, nil
	}
	return settings, nil
}

// GetUserSettings retrieves user-specific settings
func (r *settingsRepository) GetUserSettings(userID uint) (*models.UserSettings, error) {
	var settings models.UserSettings
	err := r.db.DB.Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user settings: %w", err)
	}
	return &settings, nil
}

// SaveUserSettings creates or updates user-specific settings
func (r *settingsRepository) SaveUserSettings(settings *models.UserSettings) error {
	// Use Save which will create if not exists, or update if exists
	if err := r.db.DB.Save(settings).Error; err != nil {
		return fmt.Errorf("failed to save user settings: %w", err)
	}
	return nil
}
