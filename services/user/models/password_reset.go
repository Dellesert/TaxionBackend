package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// PasswordResetStatus represents the status of a password reset
type PasswordResetStatus string

const (
	PasswordResetStatusPending   PasswordResetStatus = "pending"
	PasswordResetStatusUsed      PasswordResetStatus = "used"
	PasswordResetStatusExpired   PasswordResetStatus = "expired"
	PasswordResetStatusCancelled PasswordResetStatus = "cancelled"
)

// PasswordReset represents a password reset token
type PasswordReset struct {
	models.BaseModel
	Token       string              `gorm:"uniqueIndex;not null;size:255" json:"token"`
	UserID      uint                `gorm:"not null;index" json:"user_id"`
	User        *User               `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Email       string              `gorm:"index;not null;size:255" json:"email"`
	Status      PasswordResetStatus `gorm:"not null;default:'pending';index;size:20" json:"status"`
	ExpiresAt   time.Time           `gorm:"not null;index" json:"expires_at"`
	UsedAt      *time.Time          `json:"used_at,omitempty"`
	CreatedByID *uint               `gorm:"index" json:"created_by_id,omitempty"` // Admin who initiated reset
	CreatedBy   *User               `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
}

// TableName returns the table name for PasswordReset model
func (PasswordReset) TableName() string {
	return "password_resets"
}

// BeforeCreate hook is called before creating a password reset
func (pr *PasswordReset) BeforeCreate(tx *gorm.DB) error {
	// Set default status if not provided
	if pr.Status == "" {
		pr.Status = PasswordResetStatusPending
	}
	return nil
}

// IsValid checks if the password reset token is valid and not expired
func (pr *PasswordReset) IsValid() bool {
	return pr.Status == PasswordResetStatusPending && time.Now().Before(pr.ExpiresAt)
}

// IsExpired checks if the password reset token has expired
func (pr *PasswordReset) IsExpired() bool {
	return time.Now().After(pr.ExpiresAt)
}

// InitiatePasswordResetRequest represents admin request to initiate password reset
type InitiatePasswordResetRequest struct {
	UserID uint `json:"user_id" binding:"required" validate:"required"`
}

// ResetPasswordRequest represents request for resetting password (public endpoint)
type ResetPasswordRequest struct {
	Password        string `json:"password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
}

// PasswordResetResponse represents password reset response
type PasswordResetResponse struct {
	ID          uint                `json:"id"`
	Token       string              `json:"token"`
	UserID      uint                `json:"user_id"`
	Email       string              `json:"email"`
	Status      PasswordResetStatus `json:"status"`
	ExpiresAt   time.Time           `json:"expires_at"`
	UsedAt      *time.Time          `json:"used_at,omitempty"`
	CreatedByID *uint               `json:"created_by_id,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	ResetLink   string              `json:"reset_link,omitempty"` // Only included in certain contexts
}

// PublicPasswordResetResponse represents public password reset validation response
type PublicPasswordResetResponse struct {
	Valid     bool      `json:"valid"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ToResponse converts PasswordReset to PasswordResetResponse
func (pr *PasswordReset) ToResponse() *PasswordResetResponse {
	return &PasswordResetResponse{
		ID:          pr.ID,
		Token:       pr.Token,
		UserID:      pr.UserID,
		Email:       pr.Email,
		Status:      pr.Status,
		ExpiresAt:   pr.ExpiresAt,
		UsedAt:      pr.UsedAt,
		CreatedByID: pr.CreatedByID,
		CreatedAt:   pr.CreatedAt,
		UpdatedAt:   pr.UpdatedAt,
	}
}
