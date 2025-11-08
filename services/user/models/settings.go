package models

import (
	"time"
)

// SecurityLevel represents predefined security configurations
type SecurityLevel string

const (
	SecurityLevelMinimal SecurityLevel = "minimal"
	SecurityLevelMedium  SecurityLevel = "medium" // recommended
	SecurityLevelMaximum SecurityLevel = "maximum"
	SecurityLevelCustom  SecurityLevel = "custom"
)

// AuthMode represents primary authentication mode
type AuthMode string

const (
	AuthModePassword          AuthMode = "password"            // Password only
	AuthModePasskey           AuthMode = "passkey"             // Passkey only
	AuthModePasswordOrPasskey AuthMode = "password_or_passkey" // Password OR Passkey (user choice)
	AuthModePasswordAndPasskey AuthMode = "password_and_passkey" // Password AND Passkey (both required)
)

// SecondFactorMode represents second factor authentication settings
type SecondFactorMode string

const (
	SecondFactorMode2FANone          SecondFactorMode = "none"              // No 2FA
	SecondFactorMode2FAOptional      SecondFactorMode = "optional"          // 2FA optional
	SecondFactorMode2FARequired      SecondFactorMode = "required"          // 2FA required
	SecondFactorMode2FAPasskeyOrEmail SecondFactorMode = "passkey_or_email" // Passkey OR Email
)

// SystemSettings represents system-wide authentication and security settings
type SystemSettings struct {
	ID uint `gorm:"primarykey" json:"id"`

	// Security level
	SecurityLevel SecurityLevel `gorm:"not null;default:'medium';size:20" json:"security_level"`

	// Primary authentication mode
	AuthMode AuthMode `gorm:"not null;default:'password';size:30" json:"auth_mode"`

	// Second factor settings
	SecondFactorMode SecondFactorMode `gorm:"not null;default:'optional';size:20" json:"second_factor_mode"`

	// Passkey configuration
	PasskeyAsSecondFactor bool `gorm:"not null;default:false" json:"passkey_as_second_factor"`
	AllowMultiplePasskeys bool `gorm:"not null;default:true" json:"allow_multiple_passkeys"`
	MaxPasskeysPerUser    int  `gorm:"not null;default:5" json:"max_passkeys_per_user"`

	// Session settings
	SessionDurationHours int `gorm:"not null;default:72" json:"session_duration_hours"`

	// Password policy
	MinPasswordLength         int  `gorm:"not null;default:8" json:"min_password_length"`
	RequirePasswordComplexity bool `gorm:"not null;default:true" json:"require_password_complexity"`
	PasswordExpirationDays    int  `gorm:"not null;default:90" json:"password_expiration_days"` // 0 = never

	// Metadata
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy uint      `gorm:"index" json:"updated_by"` // Admin user ID
}

// TableName returns the table name for SystemSettings model
func (SystemSettings) TableName() string {
	return "system_settings"
}

// SystemAuthSettings is an alias for API responses
type SystemAuthSettings struct {
	SecurityLevel             SecurityLevel    `json:"security_level"`
	AuthMode                  AuthMode         `json:"auth_mode"`
	SecondFactorMode          SecondFactorMode `json:"second_factor_mode"`
	PasskeyAsSecondFactor     bool             `json:"passkey_as_second_factor"`
	AllowMultiplePasskeys     bool             `json:"allow_multiple_passkeys"`
	MaxPasskeysPerUser        int              `json:"max_passkeys_per_user"`
	SessionDurationHours      int              `json:"session_duration_hours"`
	MinPasswordLength         int              `json:"min_password_length"`
	RequirePasswordComplexity bool             `json:"require_password_complexity"`
	PasswordExpirationDays    int              `json:"password_expiration_days"`
	UpdatedAt                 time.Time        `json:"updated_at"`
	UpdatedBy                 uint             `json:"updated_by"`
}

// ToResponse converts SystemSettings to SystemAuthSettings
func (s *SystemSettings) ToResponse() *SystemAuthSettings {
	return &SystemAuthSettings{
		SecurityLevel:             s.SecurityLevel,
		AuthMode:                  s.AuthMode,
		SecondFactorMode:          s.SecondFactorMode,
		PasskeyAsSecondFactor:     s.PasskeyAsSecondFactor,
		AllowMultiplePasskeys:     s.AllowMultiplePasskeys,
		MaxPasskeysPerUser:        s.MaxPasskeysPerUser,
		SessionDurationHours:      s.SessionDurationHours,
		MinPasswordLength:         s.MinPasswordLength,
		RequirePasswordComplexity: s.RequirePasswordComplexity,
		PasswordExpirationDays:    s.PasswordExpirationDays,
		UpdatedAt:                 s.UpdatedAt,
		UpdatedBy:                 s.UpdatedBy,
	}
}

// SecurityPresetInfo represents information about a security preset
type SecurityPresetInfo struct {
	Level       SecurityLevel      `json:"level"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Recommended bool               `json:"recommended"`
	Features    []string           `json:"features"`
	Settings    SystemAuthSettings `json:"settings"`
}

// GetSecurityPresets returns all available security presets
func GetSecurityPresets() map[SecurityLevel]SystemAuthSettings {
	now := time.Now()
	return map[SecurityLevel]SystemAuthSettings{
		SecurityLevelMinimal: {
			SecurityLevel:             SecurityLevelMinimal,
			AuthMode:                  AuthModePassword,
			SecondFactorMode:          SecondFactorMode2FANone,
			PasskeyAsSecondFactor:     false,
			AllowMultiplePasskeys:     false,
			MaxPasskeysPerUser:        0,
			SessionDurationHours:      4320, // 180 days (6 months)
			MinPasswordLength:         6,
			RequirePasswordComplexity: false,
			PasswordExpirationDays:    0, // never expire
			UpdatedAt:                 now,
		},

		SecurityLevelMedium: { // RECOMMENDED
			SecurityLevel:             SecurityLevelMedium,
			AuthMode:                  AuthModePasswordOrPasskey,
			SecondFactorMode:          SecondFactorMode2FAOptional,
			PasskeyAsSecondFactor:     true,
			AllowMultiplePasskeys:     true,
			MaxPasskeysPerUser:        5,
			SessionDurationHours:      2160, // 90 days (3 months)
			MinPasswordLength:         8,
			RequirePasswordComplexity: true,
			PasswordExpirationDays:    0, // never expire (optional for medium)
			UpdatedAt:                 now,
		},

		SecurityLevelMaximum: {
			SecurityLevel:             SecurityLevelMaximum,
			AuthMode:                  AuthModePasswordAndPasskey,
			SecondFactorMode:          SecondFactorMode2FARequired,
			PasskeyAsSecondFactor:     true,
			AllowMultiplePasskeys:     true,
			MaxPasskeysPerUser:        10,
			SessionDurationHours:      720, // 30 days (1 month)
			MinPasswordLength:         12,
			RequirePasswordComplexity: true,
			PasswordExpirationDays:    90, // 90 days for maximum security
			UpdatedAt:                 now,
		},
	}
}

// GetSecurityPresetsInfo returns detailed information about all presets
func GetSecurityPresetsInfo() []SecurityPresetInfo {
	presets := GetSecurityPresets()

	return []SecurityPresetInfo{
		{
			Level:       SecurityLevelMinimal,
			Name:        "Минимальный",
			Description: "Максимальное удобство для небольших команд с низкими требованиями к безопасности",
			Recommended: false,
			Features: []string{
				"Вход только по паролю",
				"Без обязательной двухфакторной аутентификации",
				"Простые требования к паролю (минимум 6 символов)",
				"Длительные сессии (180 дней неактивности)",
				"Пароли не истекают",
			},
			Settings: presets[SecurityLevelMinimal],
		},
		{
			Level:       SecurityLevelMedium,
			Name:        "Средний",
			Description: "Оптимальный баланс между безопасностью и удобством для большинства организаций",
			Recommended: true,
			Features: []string{
				"Вход по паролю ИЛИ passkey (выбор пользователя)",
				"Опциональная двухфакторная аутентификация",
				"Passkey может использоваться как второй фактор",
				"До 5 passkey на пользователя",
				"Средние требования к паролю (минимум 8 символов, сложность)",
				"Сессии 90 дней неактивности",
				"Пароли не истекают",
			},
			Settings: presets[SecurityLevelMedium],
		},
		{
			Level:       SecurityLevelMaximum,
			Name:        "Максимальный",
			Description: "Высший уровень защиты для организаций с критичными данными",
			Recommended: false,
			Features: []string{
				"Обязательно пароль И passkey (оба фактора)",
				"Обязательная двухфакторная аутентификация (email код)",
				"Три фактора: пароль + passkey + email код",
				"До 10 passkey на пользователя",
				"Строгие требования к паролю (минимум 12 символов, сложность)",
				"Сессии 30 дней неактивности",
				"Обязательная смена пароля каждые 90 дней",
			},
			Settings: presets[SecurityLevelMaximum],
		},
	}
}

// ApplyPresetRequest represents request to apply a security preset
type ApplyPresetRequest struct {
	Preset SecurityLevel `json:"preset" binding:"required,oneof=minimal medium maximum" validate:"required,oneof=minimal medium maximum"`
}

// UpdateCustomSettingsRequest represents request to update custom settings
type UpdateCustomSettingsRequest struct {
	AuthMode                  *AuthMode         `json:"auth_mode,omitempty" binding:"omitempty,oneof=password passkey password_or_passkey password_and_passkey"`
	SecondFactorMode          *SecondFactorMode `json:"second_factor_mode,omitempty" binding:"omitempty,oneof=none optional required passkey_or_email"`
	PasskeyAsSecondFactor     *bool             `json:"passkey_as_second_factor,omitempty"`
	AllowMultiplePasskeys     *bool             `json:"allow_multiple_passkeys,omitempty"`
	MaxPasskeysPerUser        *int              `json:"max_passkeys_per_user,omitempty" binding:"omitempty,min=0,max=20"`
	SessionDurationHours      *int              `json:"session_duration_hours,omitempty" binding:"omitempty,min=1,max=720"`
	MinPasswordLength         *int              `json:"min_password_length,omitempty" binding:"omitempty,min=6,max=128"`
	RequirePasswordComplexity *bool             `json:"require_password_complexity,omitempty"`
	PasswordExpirationDays    *int              `json:"password_expiration_days,omitempty" binding:"omitempty,min=0,max=365"`
}

// SecuritySummaryResponse represents current security configuration summary
type SecuritySummaryResponse struct {
	CurrentLevel   SecurityLevel `json:"current_level"`
	IsCustom       bool          `json:"is_custom"`
	LevelName      string        `json:"level_name"`
	Description    string        `json:"description"`
	ActiveFeatures []string      `json:"active_features"`
	Settings       SystemAuthSettings `json:"settings"`
	Statistics     SecurityStatistics `json:"statistics"`
}

// SecurityStatistics represents statistics about user security adoption
type SecurityStatistics struct {
	TotalUsers         int `json:"total_users"`
	UsersWith2FA       int `json:"users_with_2fa"`
	UsersWithPasskey   int `json:"users_with_passkey"`
	ActiveSessions     int `json:"active_sessions"`
	TotalPasskeys      int `json:"total_passkeys"`
}

// UserSettings represents individual user preferences and settings
type UserSettings struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	UserID         uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	ShowSetupGuide bool      `gorm:"not null;default:true" json:"show_setup_guide"`
	Theme          string    `gorm:"size:20;default:'light'" json:"theme,omitempty"` // light, dark
	Language       string    `gorm:"size:10;default:'ru'" json:"language,omitempty"` // ru, en
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TableName returns the table name for UserSettings model
func (UserSettings) TableName() string {
	return "user_settings"
}

// UserSettingsResponse represents the API response for user settings
type UserSettingsResponse struct {
	ShowSetupGuide bool   `json:"show_setup_guide"`
	Theme          string `json:"theme,omitempty"`
	Language       string `json:"language,omitempty"`
}

// UpdateUserSettingsRequest represents the request to update user settings
type UpdateUserSettingsRequest struct {
	ShowSetupGuide *bool   `json:"show_setup_guide,omitempty"`
	Theme          *string `json:"theme,omitempty" binding:"omitempty,oneof=light dark"`
	Language       *string `json:"language,omitempty" binding:"omitempty,oneof=ru en"`
}
