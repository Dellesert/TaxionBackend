// File: services/notification/models/device_token.go
package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// DevicePlatform represents the mobile platform
type DevicePlatform string

const (
	DevicePlatformIOS     DevicePlatform = "ios"
	DevicePlatformAndroid DevicePlatform = "android"
	DevicePlatformWeb     DevicePlatform = "web"
)

// DeviceToken represents a push notification device token (FCM token)
type DeviceToken struct {
	models.BaseModel
	UserID     uint           `gorm:"not null;index:idx_user_device" json:"user_id" validate:"required,min=1"`
	Token      string         `gorm:"not null;uniqueIndex;size:500" json:"token" validate:"required,max=500"`
	Platform   DevicePlatform `gorm:"not null;size:20;index" json:"platform" validate:"required,oneof=ios android web"`
	DeviceID   string         `gorm:"size:255;index:idx_user_device" json:"device_id,omitempty" validate:"omitempty,max=255"`      // Unique device identifier
	DeviceName string         `gorm:"size:255" json:"device_name,omitempty" validate:"omitempty,max=255"`                          // e.g., "iPhone 13 Pro", "Pixel 7"
	AppVersion string         `gorm:"size:50" json:"app_version,omitempty" validate:"omitempty,max=50"`                            // e.g., "1.2.3"
	OSVersion  string         `gorm:"size:50" json:"os_version,omitempty" validate:"omitempty,max=50"`                             // e.g., "iOS 17.1", "Android 14"
	IsActive   bool           `gorm:"not null;default:true;index" json:"is_active"`                                                // Whether token is active
	LastUsedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"last_used_at"`                                      // Last time token was used successfully
	ExpiresAt  *time.Time     `gorm:"index" json:"expires_at,omitempty"`                                                           // Token expiration time (optional)
	Metadata   string         `gorm:"type:jsonb;default:'{}'" json:"metadata,omitempty" validate:"omitempty"`                    // Additional metadata (JSON)
}

// TableName returns the table name for DeviceToken model
func (DeviceToken) TableName() string {
	return "device_tokens"
}

// BeforeCreate hook is called before creating a device token
func (dt *DeviceToken) BeforeCreate(tx *gorm.DB) error {
	// Set default values
	if dt.Platform == "" {
		dt.Platform = DevicePlatformAndroid
	}
	if dt.LastUsedAt.IsZero() {
		dt.LastUsedAt = time.Now()
	}
	if dt.Metadata == "" {
		dt.Metadata = "{}"
	}
	return nil
}

// BeforeUpdate hook is called before updating a device token
func (dt *DeviceToken) BeforeUpdate(tx *gorm.DB) error {
	// Update last_used_at if token is being activated
	if dt.IsActive {
		dt.LastUsedAt = time.Now()
	}
	// Ensure Metadata is valid JSON
	if dt.Metadata == "" {
		dt.Metadata = "{}"
	}
	return nil
}

// Request/Response Models

// RegisterDeviceRequest represents request for registering a device token
type RegisterDeviceRequest struct {
	Token      string         `json:"token" binding:"required,min=10,max=500" validate:"required,min=10,max=500"`
	Platform   DevicePlatform `json:"platform" binding:"required,oneof=ios android web" validate:"required,oneof=ios android web"`
	DeviceID   string         `json:"device_id,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	DeviceName string         `json:"device_name,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	AppVersion string         `json:"app_version,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	OSVersion  string         `json:"os_version,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
}

// UpdateDeviceRequest represents request for updating a device token
type UpdateDeviceRequest struct {
	Token      string `json:"token,omitempty" binding:"omitempty,min=10,max=500" validate:"omitempty,min=10,max=500"`
	DeviceName string `json:"device_name,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	AppVersion string `json:"app_version,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	OSVersion  string `json:"os_version,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	IsActive   *bool  `json:"is_active,omitempty"`
}

// DeviceTokenResponse represents device token in API responses
type DeviceTokenResponse struct {
	ID         uint           `json:"id"`
	UserID     uint           `json:"user_id"`
	Token      string         `json:"token"`
	Platform   DevicePlatform `json:"platform"`
	DeviceID   string         `json:"device_id,omitempty"`
	DeviceName string         `json:"device_name,omitempty"`
	AppVersion string         `json:"app_version,omitempty"`
	OSVersion  string         `json:"os_version,omitempty"`
	IsActive   bool           `json:"is_active"`
	LastUsedAt time.Time      `json:"last_used_at"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ToResponse converts DeviceToken model to DeviceTokenResponse
func (dt *DeviceToken) ToResponse() *DeviceTokenResponse {
	return &DeviceTokenResponse{
		ID:         dt.ID,
		UserID:     dt.UserID,
		Token:      dt.Token,
		Platform:   dt.Platform,
		DeviceID:   dt.DeviceID,
		DeviceName: dt.DeviceName,
		AppVersion: dt.AppVersion,
		OSVersion:  dt.OSVersion,
		IsActive:   dt.IsActive,
		LastUsedAt: dt.LastUsedAt,
		ExpiresAt:  dt.ExpiresAt,
		CreatedAt:  dt.CreatedAt,
		UpdatedAt:  dt.UpdatedAt,
	}
}

// DeviceTokenListResponse represents a list of device tokens
type DeviceTokenListResponse struct {
	Devices []*DeviceTokenResponse `json:"devices"`
	Total   int64                  `json:"total"`
}
