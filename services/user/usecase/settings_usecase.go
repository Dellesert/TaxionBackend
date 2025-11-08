package usecase

import (
	"fmt"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/logger"
)

// SettingsUsecase defines interface for settings business logic
type SettingsUsecase interface {
	GetSettings() (*models.SystemAuthSettings, error)
	GetPresets() []models.SecurityPresetInfo
	ApplyPreset(preset models.SecurityLevel, adminID uint) (*models.SystemAuthSettings, error)
	UpdateCustomSettings(req *models.UpdateCustomSettingsRequest, adminID uint) (*models.SystemAuthSettings, error)
	GetSummary() (*models.SecuritySummaryResponse, error)
	GetUserSettings(userID uint) (*models.UserSettingsResponse, error)
	UpdateUserSettings(userID uint, req *models.UpdateUserSettingsRequest) (*models.UserSettingsResponse, error)
}

// settingsUsecase implements SettingsUsecase interface
type settingsUsecase struct {
	settingsRepo repository.SettingsRepository
	userRepo     repository.UserRepository
	passkeyRepo  repository.PasskeyRepository
}

// NewSettingsUsecase creates a new settings usecase
func NewSettingsUsecase(
	settingsRepo repository.SettingsRepository,
	userRepo repository.UserRepository,
	passkeyRepo repository.PasskeyRepository,
) SettingsUsecase {
	return &settingsUsecase{
		settingsRepo: settingsRepo,
		userRepo:     userRepo,
		passkeyRepo:  passkeyRepo,
	}
}

// GetSettings retrieves current system settings
func (u *settingsUsecase) GetSettings() (*models.SystemAuthSettings, error) {
	settings, err := u.settingsRepo.GetOrCreate()
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to get system settings")
		return nil, fmt.Errorf("failed to get system settings")
	}
	return settings.ToResponse(), nil
}

// GetPresets returns all available security presets
func (u *settingsUsecase) GetPresets() []models.SecurityPresetInfo {
	return models.GetSecurityPresetsInfo()
}

// ApplyPreset applies a security preset
func (u *settingsUsecase) ApplyPreset(preset models.SecurityLevel, adminID uint) (*models.SystemAuthSettings, error) {
	// Validate preset
	if preset != models.SecurityLevelMinimal &&
		preset != models.SecurityLevelMedium &&
		preset != models.SecurityLevelMaximum {
		return nil, fmt.Errorf("invalid security preset: %s", preset)
	}

	// Get preset configuration
	presets := models.GetSecurityPresets()
	presetConfig, ok := presets[preset]
	if !ok {
		return nil, fmt.Errorf("preset configuration not found")
	}

	// Get or create current settings
	settings, err := u.settingsRepo.GetOrCreate()
	if err != nil {
		return nil, fmt.Errorf("failed to get current settings: %w", err)
	}

	// Apply preset values
	settings.SecurityLevel = presetConfig.SecurityLevel
	settings.AuthMode = presetConfig.AuthMode
	settings.SecondFactorMode = presetConfig.SecondFactorMode
	settings.PasskeyAsSecondFactor = presetConfig.PasskeyAsSecondFactor
	settings.AllowMultiplePasskeys = presetConfig.AllowMultiplePasskeys
	settings.MaxPasskeysPerUser = presetConfig.MaxPasskeysPerUser
	settings.SessionDurationHours = presetConfig.SessionDurationHours
	settings.MinPasswordLength = presetConfig.MinPasswordLength
	settings.RequirePasswordComplexity = presetConfig.RequirePasswordComplexity
	settings.PasswordExpirationDays = presetConfig.PasswordExpirationDays
	settings.UpdatedBy = adminID

	// Save settings
	if err := u.settingsRepo.Update(settings); err != nil {
		logger.WithFields(map[string]interface{}{
			"error":  err.Error(),
			"preset": preset,
		}).Error("Failed to apply security preset")
		return nil, fmt.Errorf("failed to apply security preset")
	}

	logger.WithFields(map[string]interface{}{
		"preset":   preset,
		"admin_id": adminID,
	}).Info("Security preset applied successfully")

	return settings.ToResponse(), nil
}

// UpdateCustomSettings updates custom security settings
func (u *settingsUsecase) UpdateCustomSettings(req *models.UpdateCustomSettingsRequest, adminID uint) (*models.SystemAuthSettings, error) {
	// Get current settings
	settings, err := u.settingsRepo.GetOrCreate()
	if err != nil {
		return nil, fmt.Errorf("failed to get current settings: %w", err)
	}

	// Update only provided fields
	if req.AuthMode != nil {
		settings.AuthMode = *req.AuthMode
	}
	if req.SecondFactorMode != nil {
		settings.SecondFactorMode = *req.SecondFactorMode
	}
	if req.PasskeyAsSecondFactor != nil {
		settings.PasskeyAsSecondFactor = *req.PasskeyAsSecondFactor
	}
	if req.AllowMultiplePasskeys != nil {
		settings.AllowMultiplePasskeys = *req.AllowMultiplePasskeys
	}
	if req.MaxPasskeysPerUser != nil {
		settings.MaxPasskeysPerUser = *req.MaxPasskeysPerUser
	}
	if req.SessionDurationHours != nil {
		settings.SessionDurationHours = *req.SessionDurationHours
	}
	if req.MinPasswordLength != nil {
		settings.MinPasswordLength = *req.MinPasswordLength
	}
	if req.RequirePasswordComplexity != nil {
		settings.RequirePasswordComplexity = *req.RequirePasswordComplexity
	}
	if req.PasswordExpirationDays != nil {
		settings.PasswordExpirationDays = *req.PasswordExpirationDays
	}

	// Mark as custom
	settings.SecurityLevel = models.SecurityLevelCustom
	settings.UpdatedBy = adminID

	// Save settings
	if err := u.settingsRepo.Update(settings); err != nil {
		logger.WithFields(map[string]interface{}{
			"error":    err.Error(),
			"admin_id": adminID,
		}).Error("Failed to update custom settings")
		return nil, fmt.Errorf("failed to update custom settings")
	}

	logger.WithFields(map[string]interface{}{
		"admin_id": adminID,
	}).Info("Custom security settings updated successfully")

	return settings.ToResponse(), nil
}

// GetSummary returns current security configuration summary with statistics
func (u *settingsUsecase) GetSummary() (*models.SecuritySummaryResponse, error) {
	// Get current settings
	settings, err := u.settingsRepo.GetOrCreate()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	// Get statistics
	totalUsers, err := u.userRepo.Count()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to count total users")
		totalUsers = 0
	}

	usersWith2FA, err := u.userRepo.CountByTwoFactorEnabled()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to count users with 2FA")
		usersWith2FA = 0
	}

	usersWithPasskey, err := u.userRepo.CountByPasskeyEnabled()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to count users with passkey")
		usersWithPasskey = 0
	}

	totalPasskeys, err := u.passkeyRepo.CountTotal()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to count total passkeys")
		totalPasskeys = 0
	}

	// Get preset info for description
	var levelName, description string
	var features []string
	isCustom := settings.SecurityLevel == models.SecurityLevelCustom

	if isCustom {
		levelName = "Пользовательский"
		description = "Настройки безопасности были изменены вручную"
		features = u.buildFeatureList(settings)
	} else {
		presets := models.GetSecurityPresetsInfo()
		for _, preset := range presets {
			if preset.Level == settings.SecurityLevel {
				levelName = preset.Name
				description = preset.Description
				features = preset.Features
				break
			}
		}
	}

	return &models.SecuritySummaryResponse{
		CurrentLevel:   settings.SecurityLevel,
		IsCustom:       isCustom,
		LevelName:      levelName,
		Description:    description,
		ActiveFeatures: features,
		Settings:       *settings.ToResponse(),
		Statistics: models.SecurityStatistics{
			TotalUsers:       int(totalUsers),
			UsersWith2FA:     int(usersWith2FA),
			UsersWithPasskey: int(usersWithPasskey),
			ActiveSessions:   0, // TODO: implement session counting
			TotalPasskeys:    int(totalPasskeys),
		},
	}, nil
}

// buildFeatureList creates a feature list based on current settings
func (u *settingsUsecase) buildFeatureList(settings *models.SystemSettings) []string {
	features := []string{}

	// Authentication mode
	switch settings.AuthMode {
	case models.AuthModePassword:
		features = append(features, "Вход только по паролю")
	case models.AuthModePasskey:
		features = append(features, "Вход только через passkey")
	case models.AuthModePasswordOrPasskey:
		features = append(features, "Вход по паролю ИЛИ passkey (выбор пользователя)")
	case models.AuthModePasswordAndPasskey:
		features = append(features, "Вход по паролю И passkey (оба фактора)")
	}

	// Second factor
	switch settings.SecondFactorMode {
	case models.SecondFactorMode2FANone:
		features = append(features, "Без обязательной двухфакторной аутентификации")
	case models.SecondFactorMode2FAOptional:
		features = append(features, "Опциональная двухфакторная аутентификация")
	case models.SecondFactorMode2FARequired:
		features = append(features, "Обязательная двухфакторная аутентификация")
	case models.SecondFactorMode2FAPasskeyOrEmail:
		features = append(features, "Второй фактор: passkey ИЛИ email")
	}

	// Passkey settings
	if settings.AllowMultiplePasskeys && settings.MaxPasskeysPerUser > 0 {
		features = append(features, fmt.Sprintf("До %d passkey на пользователя", settings.MaxPasskeysPerUser))
	}

	// Password policy
	if settings.RequirePasswordComplexity {
		features = append(features, fmt.Sprintf("Требования к паролю (минимум %d символов, сложность)", settings.MinPasswordLength))
	} else {
		features = append(features, fmt.Sprintf("Простые требования к паролю (минимум %d символов)", settings.MinPasswordLength))
	}

	// Session duration
	features = append(features, fmt.Sprintf("Сессии %d часов", settings.SessionDurationHours))

	// Password expiration
	if settings.PasswordExpirationDays > 0 {
		features = append(features, fmt.Sprintf("Смена пароля каждые %d дней", settings.PasswordExpirationDays))
	} else {
		features = append(features, "Пароли не истекают")
	}

	return features
}

// GetUserSettings retrieves user-specific settings
func (u *settingsUsecase) GetUserSettings(userID uint) (*models.UserSettingsResponse, error) {
	userSettings, err := u.settingsRepo.GetUserSettings(userID)
	if err != nil {
		// If not found, create default settings
		userSettings = &models.UserSettings{
			UserID:         userID,
			ShowSetupGuide: true,
			Theme:          "light",
			Language:       "ru",
		}

		// Save default settings
		if saveErr := u.settingsRepo.SaveUserSettings(userSettings); saveErr != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   saveErr.Error(),
			}).Error("Failed to create default user settings")
			return nil, fmt.Errorf("failed to create default user settings: %w", saveErr)
		}

		logger.WithFields(map[string]interface{}{
			"user_id": userID,
		}).Info("Created default user settings")
	}

	return &models.UserSettingsResponse{
		ShowSetupGuide: userSettings.ShowSetupGuide,
		Theme:          userSettings.Theme,
		Language:       userSettings.Language,
	}, nil
}

// UpdateUserSettings updates user-specific settings
func (u *settingsUsecase) UpdateUserSettings(userID uint, req *models.UpdateUserSettingsRequest) (*models.UserSettingsResponse, error) {
	// Get or create user settings
	userSettings, err := u.settingsRepo.GetUserSettings(userID)
	if err != nil {
		// If not found, create new settings
		userSettings = &models.UserSettings{
			UserID:         userID,
			ShowSetupGuide: true,
			Theme:          "light",
			Language:       "ru",
		}
	}

	// Update only provided fields
	if req.ShowSetupGuide != nil {
		userSettings.ShowSetupGuide = *req.ShowSetupGuide
	}
	if req.Theme != nil {
		userSettings.Theme = *req.Theme
	}
	if req.Language != nil {
		userSettings.Language = *req.Language
	}

	// Save settings
	if err := u.settingsRepo.SaveUserSettings(userSettings); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Error("Failed to save user settings")
		return nil, fmt.Errorf("failed to save user settings: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id": userID,
	}).Info("User settings updated successfully")

	return &models.UserSettingsResponse{
		ShowSetupGuide: userSettings.ShowSetupGuide,
		Theme:          userSettings.Theme,
		Language:       userSettings.Language,
	}, nil
}
