// File: services/notification/repository/device_token_repository.go
package repository

import (
	"fmt"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// DeviceTokenRepository defines the interface for device token data operations
type DeviceTokenRepository interface {
	// Basic CRUD operations
	CreateDeviceToken(token *models.DeviceToken) error
	GetDeviceTokenByID(id uint) (*models.DeviceToken, error)
	GetDeviceTokenByToken(token string) (*models.DeviceToken, error)
	UpdateDeviceToken(token *models.DeviceToken) error
	DeleteDeviceToken(id uint) error

	// User device queries
	GetUserDevices(userID uint, activeOnly bool) ([]*models.DeviceToken, error)
	GetUserDeviceByToken(userID uint, token string) (*models.DeviceToken, error)
	GetActiveDevicesByUserIDs(userIDs []uint) (map[uint][]*models.DeviceToken, error)

	// Platform-specific queries
	GetDevicesByPlatform(platform models.DevicePlatform, activeOnly bool) ([]*models.DeviceToken, error)
	GetUserDevicesByPlatform(userID uint, platform models.DevicePlatform) ([]*models.DeviceToken, error)

	// Device management
	DeactivateDevice(id uint) error
	DeactivateDeviceByToken(token string) error
	DeactivateUserDevices(userID uint) error
	DeactivateOldDevices(inactiveSince time.Time) (int64, error)

	// Upsert (register or update)
	UpsertDeviceToken(token *models.DeviceToken) error

	// Cleanup operations
	DeleteInactiveDevices(inactiveSince time.Time) (int64, error)
	DeleteExpiredDevices() (int64, error)

	// Statistics
	GetDeviceStats(userID *uint) (*DeviceStats, error)
	GetPlatformStats() (map[models.DevicePlatform]int64, error)
}

// deviceTokenRepository implements DeviceTokenRepository interface
type deviceTokenRepository struct {
	db *database.DB
}

// DeviceStats represents device token statistics
type DeviceStats struct {
	TotalDevices    int64                               `json:"total_devices"`
	ActiveDevices   int64                               `json:"active_devices"`
	InactiveDevices int64                               `json:"inactive_devices"`
	ByPlatform      map[models.DevicePlatform]int64     `json:"by_platform"`
	RecentDevices   int64                               `json:"recent_devices"` // Last 7 days
}

// NewDeviceTokenRepository creates a new device token repository
func NewDeviceTokenRepository(db *database.DB) DeviceTokenRepository {
	return &deviceTokenRepository{
		db: db,
	}
}

// Basic CRUD operations

// CreateDeviceToken creates a new device token
func (r *deviceTokenRepository) CreateDeviceToken(token *models.DeviceToken) error {
	if err := r.db.Create(token).Error; err != nil {
		return fmt.Errorf("failed to create device token: %w", err)
	}
	return nil
}

// GetDeviceTokenByID retrieves a device token by ID
func (r *deviceTokenRepository) GetDeviceTokenByID(id uint) (*models.DeviceToken, error) {
	var token models.DeviceToken
	err := r.db.First(&token, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("device token not found")
		}
		return nil, fmt.Errorf("failed to get device token: %w", err)
	}
	return &token, nil
}

// GetDeviceTokenByToken retrieves a device token by token string
func (r *deviceTokenRepository) GetDeviceTokenByToken(token string) (*models.DeviceToken, error) {
	var deviceToken models.DeviceToken
	err := r.db.Where("token = ?", token).First(&deviceToken).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("device token not found")
		}
		return nil, fmt.Errorf("failed to get device token: %w", err)
	}
	return &deviceToken, nil
}

// UpdateDeviceToken updates an existing device token
func (r *deviceTokenRepository) UpdateDeviceToken(token *models.DeviceToken) error {
	if err := r.db.Save(token).Error; err != nil {
		return fmt.Errorf("failed to update device token: %w", err)
	}
	return nil
}

// DeleteDeviceToken soft deletes a device token
func (r *deviceTokenRepository) DeleteDeviceToken(id uint) error {
	if err := r.db.Delete(&models.DeviceToken{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete device token: %w", err)
	}
	return nil
}

// User device queries

// GetUserDevices retrieves all devices for a user
func (r *deviceTokenRepository) GetUserDevices(userID uint, activeOnly bool) ([]*models.DeviceToken, error) {
	var devices []*models.DeviceToken
	query := r.db.Where("user_id = ?", userID)

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	err := query.Order("last_used_at DESC").Find(&devices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user devices: %w", err)
	}

	return devices, nil
}

// GetUserDeviceByToken retrieves a specific device token for a user
func (r *deviceTokenRepository) GetUserDeviceByToken(userID uint, token string) (*models.DeviceToken, error) {
	var deviceToken models.DeviceToken
	err := r.db.Where("user_id = ? AND token = ?", userID, token).First(&deviceToken).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("device token not found")
		}
		return nil, fmt.Errorf("failed to get user device token: %w", err)
	}
	return &deviceToken, nil
}

// GetActiveDevicesByUserIDs retrieves active devices for multiple users (bulk query)
func (r *deviceTokenRepository) GetActiveDevicesByUserIDs(userIDs []uint) (map[uint][]*models.DeviceToken, error) {
	if len(userIDs) == 0 {
		return make(map[uint][]*models.DeviceToken), nil
	}

	var devices []*models.DeviceToken
	err := r.db.Where("user_id IN ? AND is_active = ?", userIDs, true).
		Order("user_id, last_used_at DESC").
		Find(&devices).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get active devices for users: %w", err)
	}

	// Group by user ID
	result := make(map[uint][]*models.DeviceToken)
	for _, device := range devices {
		result[device.UserID] = append(result[device.UserID], device)
	}

	return result, nil
}

// Platform-specific queries

// GetDevicesByPlatform retrieves devices by platform
func (r *deviceTokenRepository) GetDevicesByPlatform(platform models.DevicePlatform, activeOnly bool) ([]*models.DeviceToken, error) {
	var devices []*models.DeviceToken
	query := r.db.Where("platform = ?", platform)

	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	err := query.Order("created_at DESC").Find(&devices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get devices by platform: %w", err)
	}

	return devices, nil
}

// GetUserDevicesByPlatform retrieves user's devices by platform
func (r *deviceTokenRepository) GetUserDevicesByPlatform(userID uint, platform models.DevicePlatform) ([]*models.DeviceToken, error) {
	var devices []*models.DeviceToken
	err := r.db.Where("user_id = ? AND platform = ?", userID, platform).
		Order("last_used_at DESC").
		Find(&devices).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user devices by platform: %w", err)
	}

	return devices, nil
}

// Device management

// DeactivateDevice deactivates a device by ID
func (r *deviceTokenRepository) DeactivateDevice(id uint) error {
	result := r.db.Model(&models.DeviceToken{}).
		Where("id = ?", id).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to deactivate device: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("device not found")
	}

	return nil
}

// DeactivateDeviceByToken deactivates a device by token string
func (r *deviceTokenRepository) DeactivateDeviceByToken(token string) error {
	result := r.db.Model(&models.DeviceToken{}).
		Where("token = ?", token).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to deactivate device by token: %w", result.Error)
	}

	return nil
}

// DeactivateUserDevices deactivates all devices for a user
func (r *deviceTokenRepository) DeactivateUserDevices(userID uint) error {
	result := r.db.Model(&models.DeviceToken{}).
		Where("user_id = ?", userID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to deactivate user devices: %w", result.Error)
	}

	return nil
}

// DeactivateOldDevices deactivates devices that haven't been used since a certain time
func (r *deviceTokenRepository) DeactivateOldDevices(inactiveSince time.Time) (int64, error) {
	result := r.db.Model(&models.DeviceToken{}).
		Where("last_used_at < ? AND is_active = ?", inactiveSince, true).
		Update("is_active", false)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to deactivate old devices: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// Upsert (register or update)

// UpsertDeviceToken creates a new device token or updates existing one
func (r *deviceTokenRepository) UpsertDeviceToken(token *models.DeviceToken) error {
	// Check if token already exists
	var existing models.DeviceToken
	err := r.db.Where("token = ?", token.Token).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// Token doesn't exist, create new
		return r.CreateDeviceToken(token)
	} else if err != nil {
		return fmt.Errorf("failed to check existing token: %w", err)
	}

	// Token exists, update it
	existing.Platform = token.Platform
	existing.DeviceID = token.DeviceID
	existing.DeviceName = token.DeviceName
	existing.AppVersion = token.AppVersion
	existing.OSVersion = token.OSVersion
	existing.IsActive = true
	existing.LastUsedAt = time.Now()
	// Keep existing metadata if new one is empty
	if token.Metadata != "" {
		existing.Metadata = token.Metadata
	} else if existing.Metadata == "" {
		existing.Metadata = "{}"
	}

	// If user changed, update user_id as well
	if token.UserID != 0 && token.UserID != existing.UserID {
		existing.UserID = token.UserID
	}

	return r.UpdateDeviceToken(&existing)
}

// Cleanup operations

// DeleteInactiveDevices permanently deletes inactive devices
func (r *deviceTokenRepository) DeleteInactiveDevices(inactiveSince time.Time) (int64, error) {
	result := r.db.Unscoped().
		Where("last_used_at < ? AND is_active = ?", inactiveSince, false).
		Delete(&models.DeviceToken{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete inactive devices: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// DeleteExpiredDevices permanently deletes expired devices
func (r *deviceTokenRepository) DeleteExpiredDevices() (int64, error) {
	now := time.Now()
	result := r.db.Unscoped().
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Delete(&models.DeviceToken{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete expired devices: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// Statistics

// GetDeviceStats returns device token statistics
func (r *deviceTokenRepository) GetDeviceStats(userID *uint) (*DeviceStats, error) {
	stats := &DeviceStats{
		ByPlatform: make(map[models.DevicePlatform]int64),
	}

	query := r.db.Model(&models.DeviceToken{})
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	// Total devices
	if err := query.Count(&stats.TotalDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to count total devices: %w", err)
	}

	// Active devices
	activeQuery := r.db.Model(&models.DeviceToken{}).Where("is_active = ?", true)
	if userID != nil {
		activeQuery = activeQuery.Where("user_id = ?", *userID)
	}
	if err := activeQuery.Count(&stats.ActiveDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to count active devices: %w", err)
	}

	// Inactive devices
	stats.InactiveDevices = stats.TotalDevices - stats.ActiveDevices

	// Recent devices (last 7 days)
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	recentQuery := r.db.Model(&models.DeviceToken{}).Where("created_at >= ?", weekAgo)
	if userID != nil {
		recentQuery = recentQuery.Where("user_id = ?", *userID)
	}
	if err := recentQuery.Count(&stats.RecentDevices).Error; err != nil {
		return nil, fmt.Errorf("failed to count recent devices: %w", err)
	}

	// By platform
	var platformCounts []struct {
		Platform models.DevicePlatform
		Count    int64
	}

	platformQuery := r.db.Model(&models.DeviceToken{}).
		Select("platform, COUNT(*) as count").
		Group("platform")

	if userID != nil {
		platformQuery = platformQuery.Where("user_id = ?", *userID)
	}

	if err := platformQuery.Scan(&platformCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get platform stats: %w", err)
	}

	for _, pc := range platformCounts {
		stats.ByPlatform[pc.Platform] = pc.Count
	}

	return stats, nil
}

// GetPlatformStats returns device count by platform
func (r *deviceTokenRepository) GetPlatformStats() (map[models.DevicePlatform]int64, error) {
	result := make(map[models.DevicePlatform]int64)

	var platformCounts []struct {
		Platform models.DevicePlatform
		Count    int64
	}

	err := r.db.Model(&models.DeviceToken{}).
		Select("platform, COUNT(*) as count").
		Where("is_active = ?", true).
		Group("platform").
		Scan(&platformCounts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get platform stats: %w", err)
	}

	for _, pc := range platformCounts {
		result[pc.Platform] = pc.Count
	}

	return result, nil
}
