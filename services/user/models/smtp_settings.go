package models

import (
	"time"
)

// SMTPSettings represents system-wide SMTP configuration
type SMTPSettings struct {
	ID uint `gorm:"primarykey" json:"id"`

	// SMTP Server Configuration
	Host     string `gorm:"not null;size:255" json:"host" validate:"required"`
	Port     int    `gorm:"not null" json:"port" validate:"required,min=1,max=65535"`
	Username string `gorm:"not null;size:255" json:"username" validate:"required"`
	Password string `gorm:"not null;size:255" json:"password" validate:"required"`

	// Sender Information
	FromEmail string `gorm:"not null;size:255" json:"from_email" validate:"required,email"`
	FromName  string `gorm:"size:255" json:"from_name"`

	// Security Settings
	UseTLS bool `gorm:"not null;default:true" json:"use_tls"`
	UseSSL bool `gorm:"not null;default:false" json:"use_ssl"`

	// Performance Settings
	TimeoutSeconds     int `gorm:"not null;default:30" json:"timeout_seconds" validate:"min=1,max=300"`
	MaxRetries         int `gorm:"not null;default:3" json:"max_retries" validate:"min=0,max=10"`
	RetryDelaySeconds  int `gorm:"not null;default:5" json:"retry_delay_seconds" validate:"min=1,max=60"`
	PoolSize           int `gorm:"not null;default:10" json:"pool_size" validate:"min=1,max=100"`
	RateLimitRPS       int `gorm:"not null;default:5" json:"rate_limit_rps" validate:"min=1,max=100"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy uint      `gorm:"index" json:"updated_by"` // Admin user ID
}

// TableName returns the table name for SMTPSettings model
func (SMTPSettings) TableName() string {
	return "smtp_settings"
}

// SMTPSettingsResponse represents SMTP settings for API response (without password)
type SMTPSettingsResponse struct {
	ID                uint      `json:"id"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Username          string    `json:"username"`
	FromEmail         string    `json:"from_email"`
	FromName          string    `json:"from_name"`
	UseTLS            bool      `json:"use_tls"`
	UseSSL            bool      `json:"use_ssl"`
	TimeoutSeconds    int       `json:"timeout_seconds"`
	MaxRetries        int       `json:"max_retries"`
	RetryDelaySeconds int       `json:"retry_delay_seconds"`
	PoolSize          int       `json:"pool_size"`
	RateLimitRPS      int       `json:"rate_limit_rps"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	UpdatedBy         uint      `json:"updated_by"`
}

// ToResponse converts SMTPSettings to SMTPSettingsResponse (without password)
func (s *SMTPSettings) ToResponse() *SMTPSettingsResponse {
	return &SMTPSettingsResponse{
		ID:                s.ID,
		Host:              s.Host,
		Port:              s.Port,
		Username:          s.Username,
		FromEmail:         s.FromEmail,
		FromName:          s.FromName,
		UseTLS:            s.UseTLS,
		UseSSL:            s.UseSSL,
		TimeoutSeconds:    s.TimeoutSeconds,
		MaxRetries:        s.MaxRetries,
		RetryDelaySeconds: s.RetryDelaySeconds,
		PoolSize:          s.PoolSize,
		RateLimitRPS:      s.RateLimitRPS,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
		UpdatedBy:         s.UpdatedBy,
	}
}

// UpdateSMTPSettingsRequest represents request to update SMTP settings
type UpdateSMTPSettingsRequest struct {
	Host              string  `json:"host" binding:"required" validate:"required"`
	Port              int     `json:"port" binding:"required,min=1,max=65535" validate:"required,min=1,max=65535"`
	Username          string  `json:"username" binding:"required" validate:"required"`
	Password          *string `json:"password,omitempty"` // Optional: only update if provided
	FromEmail         string  `json:"from_email" binding:"required,email" validate:"required,email"`
	FromName          string  `json:"from_name"`
	UseTLS            bool    `json:"use_tls"`
	UseSSL            bool    `json:"use_ssl"`
	TimeoutSeconds    int     `json:"timeout_seconds" binding:"min=1,max=300" validate:"min=1,max=300"`
	MaxRetries        int     `json:"max_retries" binding:"min=0,max=10" validate:"min=0,max=10"`
	RetryDelaySeconds int     `json:"retry_delay_seconds" binding:"min=1,max=60" validate:"min=1,max=60"`
	PoolSize          int     `json:"pool_size" binding:"min=1,max=100" validate:"min=1,max=100"`
	RateLimitRPS      int     `json:"rate_limit_rps" binding:"min=1,max=100" validate:"min=1,max=100"`
}

// TestSMTPConnectionRequest represents request to test SMTP connection
type TestSMTPConnectionRequest struct {
	Host       string `json:"host" binding:"required" validate:"required"`
	Port       int    `json:"port" binding:"required,min=1,max=65535" validate:"required,min=1,max=65535"`
	Username   string `json:"username" binding:"required" validate:"required"`
	Password   string `json:"password" binding:"required" validate:"required"`
	FromEmail  string `json:"from_email" binding:"required,email" validate:"required,email"`
	UseTLS     bool   `json:"use_tls"`
	UseSSL     bool   `json:"use_ssl"`
	TestEmail  string `json:"test_email" binding:"required,email" validate:"required,email"`
}

// TestSMTPConnectionResponse represents response of SMTP connection test
type TestSMTPConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
