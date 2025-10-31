package models

import (
	"time"

	"tachyon-messenger/shared/models"
)

// TwoFactorCode represents a 2FA verification code
type TwoFactorCode struct {
	models.BaseModel
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Code      string    `gorm:"not null;size:6" json:"code"`
	Email     string    `gorm:"not null;size:255" json:"email"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	Verified  bool      `gorm:"not null;default:false" json:"verified"`
	IPAddress string    `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent string    `gorm:"size:500" json:"user_agent,omitempty"`
}

// TableName returns the table name for TwoFactorCode model
func (TwoFactorCode) TableName() string {
	return "two_factor_codes"
}

// IsExpired checks if the code has expired
func (t *TwoFactorCode) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the code is valid (not expired and not verified)
func (t *TwoFactorCode) IsValid() bool {
	return !t.IsExpired() && !t.Verified
}

// Send2FARequest represents request to send 2FA code
type Send2FARequest struct {
	Email    string `json:"email" binding:"required,email,max=255" validate:"required,email,max=255"`
	Password string `json:"password" binding:"required" validate:"required"`
}

// Verify2FARequest represents request to verify 2FA code
type Verify2FARequest struct {
	Email string `json:"email" binding:"required,email,max=255" validate:"required,email,max=255"`
	Code  string `json:"code" binding:"required,len=6" validate:"required,len=6"`
}
