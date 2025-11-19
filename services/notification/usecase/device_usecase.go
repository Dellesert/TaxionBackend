// File: services/notification/usecase/device_usecase.go
package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/services/notification/push"
	"tachyon-messenger/services/notification/repository"
	"tachyon-messenger/shared/logger"
)

// DeviceUsecase defines the interface for device token business logic
type DeviceUsecase interface {
	// Device registration and management
	RegisterDevice(userID uint, req *models.RegisterDeviceRequest) (*models.DeviceTokenResponse, error)
	UpdateDevice(userID, deviceID uint, req *models.UpdateDeviceRequest) (*models.DeviceTokenResponse, error)
	DeleteDevice(userID, deviceID uint) error
	DeactivateDevice(userID, deviceID uint) error

	// Device queries
	GetUserDevices(userID uint) (*models.DeviceTokenListResponse, error)
	GetDeviceByID(userID, deviceID uint) (*models.DeviceTokenResponse, error)

	// Device validation
	ValidateDeviceToken(token string) error

	// Cleanup operations
	CleanupInactiveDevices(inactiveDays int) (int64, error)
	CleanupOldDevices(inactiveDays int) (int64, error)

	// Statistics
	GetDeviceStats(userID *uint) (*repository.DeviceStats, error)
}

// deviceUsecase implements DeviceUsecase interface
type deviceUsecase struct {
	deviceRepo   repository.DeviceTokenRepository
	pushProvider push.PushProvider
}

// NewDeviceUsecase creates a new device usecase
func NewDeviceUsecase(
	deviceRepo repository.DeviceTokenRepository,
	pushProvider push.PushProvider,
) DeviceUsecase {
	return &deviceUsecase{
		deviceRepo:   deviceRepo,
		pushProvider: pushProvider,
	}
}

// Device registration and management

// RegisterDevice registers a new device or updates existing one
func (u *deviceUsecase) RegisterDevice(userID uint, req *models.RegisterDeviceRequest) (*models.DeviceTokenResponse, error) {
	// Validate request
	if err := u.validateRegisterDeviceRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Validate token with FCM (if provider is available)
	if u.pushProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := u.pushProvider.ValidateToken(ctx, req.Token); err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			}).Warn("Device token validation failed")
			// Don't fail registration, just log warning
		}
	}

	// Create device token model
	deviceToken := &models.DeviceToken{
		UserID:     userID,
		Token:      req.Token,
		Platform:   req.Platform,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		AppVersion: req.AppVersion,
		OSVersion:  req.OSVersion,
		IsActive:   true,
		LastUsedAt: time.Now(),
	}

	// Upsert device token (create or update)
	if err := u.deviceRepo.UpsertDeviceToken(deviceToken); err != nil {
		return nil, fmt.Errorf("failed to register device: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":  userID,
		"platform": req.Platform,
		"device_id": req.DeviceID,
	}).Info("Device registered successfully")

	// Reload from database to get ID
	registered, err := u.deviceRepo.GetDeviceTokenByToken(req.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to get registered device: %w", err)
	}

	return registered.ToResponse(), nil
}

// UpdateDevice updates an existing device
func (u *deviceUsecase) UpdateDevice(userID, deviceID uint, req *models.UpdateDeviceRequest) (*models.DeviceTokenResponse, error) {
	// Validate request
	if err := u.validateUpdateDeviceRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing device
	device, err := u.deviceRepo.GetDeviceTokenByID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Check ownership
	if device.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	// Update fields
	if req.Token != "" {
		device.Token = req.Token
	}
	if req.DeviceName != "" {
		device.DeviceName = req.DeviceName
	}
	if req.AppVersion != "" {
		device.AppVersion = req.AppVersion
	}
	if req.OSVersion != "" {
		device.OSVersion = req.OSVersion
	}
	if req.IsActive != nil {
		device.IsActive = *req.IsActive
	}

	device.LastUsedAt = time.Now()

	// Save changes
	if err := u.deviceRepo.UpdateDeviceToken(device); err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":   userID,
		"device_id": deviceID,
	}).Info("Device updated successfully")

	return device.ToResponse(), nil
}

// DeleteDevice permanently deletes a device
func (u *deviceUsecase) DeleteDevice(userID, deviceID uint) error {
	// Get existing device
	device, err := u.deviceRepo.GetDeviceTokenByID(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Check ownership
	if device.UserID != userID {
		return fmt.Errorf("access denied")
	}

	// Delete device
	if err := u.deviceRepo.DeleteDeviceToken(deviceID); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":   userID,
		"device_id": deviceID,
	}).Info("Device deleted successfully")

	return nil
}

// DeactivateDevice deactivates a device (soft delete)
func (u *deviceUsecase) DeactivateDevice(userID, deviceID uint) error {
	// Get existing device
	device, err := u.deviceRepo.GetDeviceTokenByID(deviceID)
	if err != nil {
		return fmt.Errorf("device not found: %w", err)
	}

	// Check ownership
	if device.UserID != userID {
		return fmt.Errorf("access denied")
	}

	// Deactivate device
	if err := u.deviceRepo.DeactivateDevice(deviceID); err != nil {
		return fmt.Errorf("failed to deactivate device: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":   userID,
		"device_id": deviceID,
	}).Info("Device deactivated successfully")

	return nil
}

// Device queries

// GetUserDevices returns all devices for a user
func (u *deviceUsecase) GetUserDevices(userID uint) (*models.DeviceTokenListResponse, error) {
	devices, err := u.deviceRepo.GetUserDevices(userID, false) // Include inactive
	if err != nil {
		return nil, fmt.Errorf("failed to get user devices: %w", err)
	}

	// Convert to response format
	responses := make([]*models.DeviceTokenResponse, len(devices))
	for i, device := range devices {
		responses[i] = device.ToResponse()
	}

	return &models.DeviceTokenListResponse{
		Devices: responses,
		Total:   int64(len(responses)),
	}, nil
}

// GetDeviceByID returns a single device by ID
func (u *deviceUsecase) GetDeviceByID(userID, deviceID uint) (*models.DeviceTokenResponse, error) {
	device, err := u.deviceRepo.GetDeviceTokenByID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Check ownership
	if device.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	return device.ToResponse(), nil
}

// Device validation

// ValidateDeviceToken validates a device token with FCM
func (u *deviceUsecase) ValidateDeviceToken(token string) error {
	if token == "" {
		return fmt.Errorf("token is required")
	}

	if u.pushProvider == nil {
		return fmt.Errorf("push provider not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := u.pushProvider.ValidateToken(ctx, token); err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	return nil
}

// Cleanup operations

// CleanupInactiveDevices deactivates devices that haven't been used recently
func (u *deviceUsecase) CleanupInactiveDevices(inactiveDays int) (int64, error) {
	if inactiveDays < 1 {
		inactiveDays = 30 // Default to 30 days
	}

	inactiveSince := time.Now().Add(-time.Duration(inactiveDays) * 24 * time.Hour)
	count, err := u.deviceRepo.DeactivateOldDevices(inactiveSince)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup inactive devices: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"deactivated_count": count,
		"inactive_days":     inactiveDays,
	}).Info("Inactive devices deactivated")

	return count, nil
}

// CleanupOldDevices permanently deletes old inactive devices
func (u *deviceUsecase) CleanupOldDevices(inactiveDays int) (int64, error) {
	if inactiveDays < 7 {
		inactiveDays = 90 // Default to 90 days for permanent deletion
	}

	inactiveSince := time.Now().Add(-time.Duration(inactiveDays) * 24 * time.Hour)
	count, err := u.deviceRepo.DeleteInactiveDevices(inactiveSince)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old devices: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"deleted_count": count,
		"inactive_days": inactiveDays,
	}).Info("Old inactive devices deleted")

	return count, nil
}

// Statistics

// GetDeviceStats returns device statistics
func (u *deviceUsecase) GetDeviceStats(userID *uint) (*repository.DeviceStats, error) {
	stats, err := u.deviceRepo.GetDeviceStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device stats: %w", err)
	}

	return stats, nil
}

// Validation methods

// validateRegisterDeviceRequest validates register device request
func (u *deviceUsecase) validateRegisterDeviceRequest(req *models.RegisterDeviceRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if strings.TrimSpace(req.Token) == "" {
		return fmt.Errorf("token is required")
	}

	if len(req.Token) < 10 || len(req.Token) > 500 {
		return fmt.Errorf("token length must be between 10 and 500 characters")
	}

	if req.Platform == "" {
		return fmt.Errorf("platform is required")
	}

	// Validate platform
	switch req.Platform {
	case models.DevicePlatformIOS, models.DevicePlatformAndroid, models.DevicePlatformWeb:
		// Valid platform
	default:
		return fmt.Errorf("invalid platform: must be ios, android, or web")
	}

	return nil
}

// validateUpdateDeviceRequest validates update device request
func (u *deviceUsecase) validateUpdateDeviceRequest(req *models.UpdateDeviceRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// At least one field must be provided
	if req.Token == "" && req.DeviceName == "" && req.AppVersion == "" &&
		req.OSVersion == "" && req.IsActive == nil {
		return fmt.Errorf("at least one field must be provided for update")
	}

	if req.Token != "" && (len(req.Token) < 10 || len(req.Token) > 500) {
		return fmt.Errorf("token length must be between 10 and 500 characters")
	}

	return nil
}
