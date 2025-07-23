// File: services/notification/repository/notification_repository.go
package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// NotificationRepository defines the interface for notification data operations
type NotificationRepository interface {
	// Basic CRUD operations
	CreateNotification(notification *models.Notification) error
	CreateBulkNotifications(notifications []*models.Notification) error
	GetNotificationByID(id uint) (*models.Notification, error)
	UpdateNotification(notification *models.Notification) error
	DeleteNotification(id uint) error

	// User notification queries
	GetUserNotifications(userID uint, filter *models.NotificationFilterRequest) ([]*models.Notification, int64, error)
	GetUnreadCount(userID uint) (int64, error)
	GetUnreadCountByType(userID uint, notificationType models.NotificationType) (int64, error)

	// Mark as read operations
	MarkAsRead(notificationID, userID uint) error
	MarkMultipleAsRead(notificationIDs []uint, userID uint) error
	MarkAllAsRead(userID uint) error
	MarkAllAsReadByType(userID uint, notificationType models.NotificationType) error

	// Scheduled notifications
	GetScheduledNotifications(before time.Time, limit int) ([]*models.Notification, error)
	GetExpiredNotifications(before time.Time, limit int) ([]*models.Notification, error)

	// Statistics and analytics
	GetNotificationStats(userID uint) (*models.NotificationStatsResponse, error)
	GetSystemStats() (*SystemNotificationStats, error)

	// Cleanup operations
	DeleteOldNotifications(beforeDate time.Time) (int64, error)
	DeleteReadNotifications(beforeDate time.Time, userID *uint) (int64, error)
	DeleteExpiredNotifications() (int64, error)

	// Delivery tracking
	CreateDelivery(delivery *models.NotificationDelivery) error
	UpdateDeliveryStatus(deliveryID uint, status models.NotificationStatus, errorMsg string) error
	GetPendingDeliveries(limit int) ([]*models.NotificationDelivery, error)
	GetFailedDeliveries(maxAttempts int, limit int) ([]*models.NotificationDelivery, error)

	// Search and filtering
	SearchNotifications(userID uint, query string, filter *models.NotificationFilterRequest) ([]*models.Notification, int64, error)
	GetNotificationsByRelatedObject(relatedType string, relatedID uint, userID *uint) ([]*models.Notification, error)

	// Preferences
	GetUserPreferences(userID uint) ([]*models.UserNotificationPreference, error)
	GetUserPreference(userID uint, notificationType models.NotificationType) (*models.UserNotificationPreference, error)
	UpsertUserPreference(preference *models.UserNotificationPreference) error
	DeleteUserPreference(userID uint, notificationType models.NotificationType) error
}

// notificationRepository implements NotificationRepository interface
type notificationRepository struct {
	db *database.DB
}

// SystemNotificationStats represents system-wide notification statistics
type SystemNotificationStats struct {
	TotalNotifications     int64   `json:"total_notifications"`
	PendingNotifications   int64   `json:"pending_notifications"`
	DeliveredNotifications int64   `json:"delivered_notifications"`
	FailedNotifications    int64   `json:"failed_notifications"`
	TodayNotifications     int64   `json:"today_notifications"`
	WeekNotifications      int64   `json:"week_notifications"`
	MonthNotifications     int64   `json:"month_notifications"`
	ActiveUsers            int64   `json:"active_users"`
	AverageDeliveryTime    float64 `json:"average_delivery_time_minutes"`
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(db *database.DB) NotificationRepository {
	return &notificationRepository{
		db: db,
	}
}

// Basic CRUD operations

// CreateNotification creates a new notification
func (r *notificationRepository) CreateNotification(notification *models.Notification) error {
	if err := r.db.Create(notification).Error; err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}
	return nil
}

// CreateBulkNotifications creates multiple notifications in a batch
func (r *notificationRepository) CreateBulkNotifications(notifications []*models.Notification) error {
	if len(notifications) == 0 {
		return nil
	}

	// Use batch insert for better performance
	batchSize := 100
	for i := 0; i < len(notifications); i += batchSize {
		end := i + batchSize
		if end > len(notifications) {
			end = len(notifications)
		}

		batch := notifications[i:end]
		if err := r.db.CreateInBatches(batch, batchSize).Error; err != nil {
			return fmt.Errorf("failed to create notifications batch: %w", err)
		}
	}

	return nil
}

// GetNotificationByID retrieves a notification by ID with delivery channels
func (r *notificationRepository) GetNotificationByID(id uint) (*models.Notification, error) {
	var notification models.Notification
	err := r.db.Preload("DeliveryChannels").First(&notification, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}
	return &notification, nil
}

// UpdateNotification updates an existing notification
func (r *notificationRepository) UpdateNotification(notification *models.Notification) error {
	if err := r.db.Save(notification).Error; err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}
	return nil
}

// DeleteNotification soft deletes a notification
func (r *notificationRepository) DeleteNotification(id uint) error {
	if err := r.db.Delete(&models.Notification{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete notification: %w", err)
	}
	return nil
}

// User notification queries

// GetUserNotifications retrieves notifications for a user with filtering and pagination
func (r *notificationRepository) GetUserNotifications(userID uint, filter *models.NotificationFilterRequest) ([]*models.Notification, int64, error) {
	query := r.db.Model(&models.Notification{}).Where("user_id = ?", userID)

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	// Load notifications with delivery channels
	var notifications []*models.Notification
	err := query.Preload("DeliveryChannels").Find(&notifications).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user notifications: %w", err)
	}

	return notifications, total, nil
}

// GetUnreadCount returns the count of unread notifications for a user
func (r *notificationRepository) GetUnreadCount(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// GetUnreadCountByType returns the count of unread notifications by type for a user
func (r *notificationRepository) GetUnreadCountByType(userID uint, notificationType models.NotificationType) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ? AND is_read = ?", userID, notificationType, false).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count by type: %w", err)
	}
	return count, nil
}

// Mark as read operations

// MarkAsRead marks a single notification as read
func (r *notificationRepository) MarkAsRead(notificationID, userID uint) error {
	now := time.Now()
	result := r.db.Model(&models.Notification{}).
		Where("id = ? AND user_id = ? AND is_read = ?", notificationID, userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark notification as read: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("notification not found or already read")
	}

	return nil
}

// MarkMultipleAsRead marks multiple notifications as read
func (r *notificationRepository) MarkMultipleAsRead(notificationIDs []uint, userID uint) error {
	if len(notificationIDs) == 0 {
		return nil
	}

	now := time.Now()
	result := r.db.Model(&models.Notification{}).
		Where("id IN ? AND user_id = ? AND is_read = ?", notificationIDs, userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark notifications as read: %w", result.Error)
	}

	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (r *notificationRepository) MarkAllAsRead(userID uint) error {
	now := time.Now()
	result := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", result.Error)
	}

	return nil
}

// MarkAllAsReadByType marks all notifications of a specific type as read for a user
func (r *notificationRepository) MarkAllAsReadByType(userID uint, notificationType models.NotificationType) error {
	now := time.Now()
	result := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ? AND is_read = ?", userID, notificationType, false).
		Updates(map[string]interface{}{
			"is_read": true,
			"read_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark notifications as read by type: %w", result.Error)
	}

	return nil
}

// Scheduled notifications

// GetScheduledNotifications returns notifications that are scheduled to be sent
func (r *notificationRepository) GetScheduledNotifications(before time.Time, limit int) ([]*models.Notification, error) {
	var notifications []*models.Notification
	err := r.db.Preload("DeliveryChannels").
		Where("scheduled_at IS NOT NULL AND scheduled_at <= ? AND status = ?", before, models.NotificationStatusPending).
		Limit(limit).
		Order("scheduled_at ASC").
		Find(&notifications).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled notifications: %w", err)
	}

	return notifications, nil
}

// GetExpiredNotifications returns notifications that have expired
func (r *notificationRepository) GetExpiredNotifications(before time.Time, limit int) ([]*models.Notification, error) {
	var notifications []*models.Notification
	err := r.db.Where("expires_at IS NOT NULL AND expires_at <= ?", before).
		Limit(limit).
		Order("expires_at ASC").
		Find(&notifications).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expired notifications: %w", err)
	}

	return notifications, nil
}

// Statistics and analytics

// GetNotificationStats returns notification statistics for a user
func (r *notificationRepository) GetNotificationStats(userID uint) (*models.NotificationStatsResponse, error) {
	stats := &models.NotificationStatsResponse{}

	// Total notifications
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ?", userID).
		Count(&stats.TotalNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get total notifications count: %w", err)
	}

	// Unread notifications
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&stats.UnreadNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get unread notifications count: %w", err)
	}

	// Pending notifications
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND status = ?", userID, models.NotificationStatusPending).
		Count(&stats.PendingNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending notifications count: %w", err)
	}

	// Failed notifications
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND status = ?", userID, models.NotificationStatusFailed).
		Count(&stats.FailedNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get failed notifications count: %w", err)
	}

	// Today's notifications
	today := time.Now().Truncate(24 * time.Hour)
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND created_at >= ?", userID, today).
		Count(&stats.TodayNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get today's notifications count: %w", err)
	}

	// Week's notifications
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	if err := r.db.Model(&models.Notification{}).
		Where("user_id = ? AND created_at >= ?", userID, weekAgo).
		Count(&stats.WeekNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get week notifications count: %w", err)
	}

	return stats, nil
}

// GetSystemStats returns system-wide notification statistics
func (r *notificationRepository) GetSystemStats() (*SystemNotificationStats, error) {
	stats := &SystemNotificationStats{}

	// Total notifications
	if err := r.db.Model(&models.Notification{}).Count(&stats.TotalNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get total notifications count: %w", err)
	}

	// Pending notifications
	if err := r.db.Model(&models.Notification{}).
		Where("status = ?", models.NotificationStatusPending).
		Count(&stats.PendingNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending notifications count: %w", err)
	}

	// Delivered notifications
	if err := r.db.Model(&models.Notification{}).
		Where("status = ?", models.NotificationStatusDelivered).
		Count(&stats.DeliveredNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get delivered notifications count: %w", err)
	}

	// Failed notifications
	if err := r.db.Model(&models.Notification{}).
		Where("status = ?", models.NotificationStatusFailed).
		Count(&stats.FailedNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get failed notifications count: %w", err)
	}

	// Time-based statistics
	today := time.Now().Truncate(24 * time.Hour)
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)
	monthAgo := time.Now().Add(-30 * 24 * time.Hour)

	if err := r.db.Model(&models.Notification{}).
		Where("created_at >= ?", today).
		Count(&stats.TodayNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get today's notifications count: %w", err)
	}

	if err := r.db.Model(&models.Notification{}).
		Where("created_at >= ?", weekAgo).
		Count(&stats.WeekNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get week notifications count: %w", err)
	}

	if err := r.db.Model(&models.Notification{}).
		Where("created_at >= ?", monthAgo).
		Count(&stats.MonthNotifications).Error; err != nil {
		return nil, fmt.Errorf("failed to get month notifications count: %w", err)
	}

	// Active users (users with notifications in the last 7 days)
	if err := r.db.Model(&models.Notification{}).
		Select("COUNT(DISTINCT user_id)").
		Where("created_at >= ?", weekAgo).
		Scan(&stats.ActiveUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to get active users count: %w", err)
	}

	// Average delivery time (in minutes)
	var result struct {
		AvgTime float64
	}
	if err := r.db.Raw(`
		SELECT AVG(EXTRACT(EPOCH FROM (updated_at - created_at))/60) as avg_time
		FROM notifications 
		WHERE status = ? AND updated_at > created_at
	`, models.NotificationStatusDelivered).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate average delivery time: %w", err)
	}
	stats.AverageDeliveryTime = result.AvgTime

	return stats, nil
}

// Cleanup operations

// DeleteOldNotifications deletes notifications older than the specified date
func (r *notificationRepository) DeleteOldNotifications(beforeDate time.Time) (int64, error) {
	result := r.db.Unscoped().Where("created_at < ?", beforeDate).Delete(&models.Notification{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete old notifications: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteReadNotifications deletes read notifications older than the specified date
func (r *notificationRepository) DeleteReadNotifications(beforeDate time.Time, userID *uint) (int64, error) {
	query := r.db.Unscoped().Where("is_read = ? AND read_at < ?", true, beforeDate)

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	result := query.Delete(&models.Notification{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete read notifications: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// DeleteExpiredNotifications deletes notifications that have expired
func (r *notificationRepository) DeleteExpiredNotifications() (int64, error) {
	now := time.Now()
	result := r.db.Unscoped().Where("expires_at IS NOT NULL AND expires_at < ?", now).Delete(&models.Notification{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete expired notifications: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// Delivery tracking

// CreateDelivery creates a new notification delivery record
func (r *notificationRepository) CreateDelivery(delivery *models.NotificationDelivery) error {
	if err := r.db.Create(delivery).Error; err != nil {
		return fmt.Errorf("failed to create notification delivery: %w", err)
	}
	return nil
}

// UpdateDeliveryStatus updates the status of a notification delivery
func (r *notificationRepository) UpdateDeliveryStatus(deliveryID uint, status models.NotificationStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status":          status,
		"last_attempt_at": time.Now(),
	}

	if status == models.NotificationStatusDelivered {
		updates["delivered_at"] = time.Now()
	}

	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}

	// Increment attempt count
	if err := r.db.Model(&models.NotificationDelivery{}).
		Where("id = ?", deliveryID).
		Update("attempt_count", gorm.Expr("attempt_count + 1")).Error; err != nil {
		return fmt.Errorf("failed to increment attempt count: %w", err)
	}

	// Update other fields
	if err := r.db.Model(&models.NotificationDelivery{}).
		Where("id = ?", deliveryID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update delivery status: %w", err)
	}

	return nil
}

// GetPendingDeliveries returns pending notification deliveries
func (r *notificationRepository) GetPendingDeliveries(limit int) ([]*models.NotificationDelivery, error) {
	var deliveries []*models.NotificationDelivery
	err := r.db.Preload("Notification").
		Where("status = ?", models.NotificationStatusPending).
		Limit(limit).
		Order("created_at ASC").
		Find(&deliveries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get pending deliveries: %w", err)
	}

	return deliveries, nil
}

// GetFailedDeliveries returns failed notification deliveries that can be retried
func (r *notificationRepository) GetFailedDeliveries(maxAttempts int, limit int) ([]*models.NotificationDelivery, error) {
	var deliveries []*models.NotificationDelivery
	err := r.db.Preload("Notification").
		Where("status = ? AND attempt_count < ?", models.NotificationStatusFailed, maxAttempts).
		Limit(limit).
		Order("last_attempt_at ASC").
		Find(&deliveries).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get failed deliveries: %w", err)
	}

	return deliveries, nil
}

// Search and filtering

// SearchNotifications searches notifications by title and message content
func (r *notificationRepository) SearchNotifications(userID uint, query string, filter *models.NotificationFilterRequest) ([]*models.Notification, int64, error) {
	dbQuery := r.db.Model(&models.Notification{}).Where("user_id = ?", userID)

	// Apply search query
	if query != "" {
		searchPattern := "%" + query + "%"
		dbQuery = dbQuery.Where("title ILIKE ? OR message ILIKE ?", searchPattern, searchPattern)
	}

	// Apply filters
	dbQuery = r.applyFilters(dbQuery, filter)

	// Get total count
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply sorting and pagination
	dbQuery = r.applySortingAndPagination(dbQuery, filter)

	// Load notifications
	var notifications []*models.Notification
	err := dbQuery.Preload("DeliveryChannels").Find(&notifications).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search notifications: %w", err)
	}

	return notifications, total, nil
}

// GetNotificationsByRelatedObject returns notifications related to a specific object
func (r *notificationRepository) GetNotificationsByRelatedObject(relatedType string, relatedID uint, userID *uint) ([]*models.Notification, error) {
	query := r.db.Model(&models.Notification{}).
		Where("related_type = ? AND related_id = ?", relatedType, relatedID)

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	var notifications []*models.Notification
	err := query.Order("created_at DESC").Find(&notifications).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications by related object: %w", err)
	}

	return notifications, nil
}

// Preferences

// GetUserPreferences returns all notification preferences for a user
func (r *notificationRepository) GetUserPreferences(userID uint) ([]*models.UserNotificationPreference, error) {
	var preferences []*models.UserNotificationPreference
	err := r.db.Where("user_id = ?", userID).Find(&preferences).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}
	return preferences, nil
}

// GetUserPreference returns a specific notification preference for a user
func (r *notificationRepository) GetUserPreference(userID uint, notificationType models.NotificationType) (*models.UserNotificationPreference, error) {
	var preference models.UserNotificationPreference
	err := r.db.Where("user_id = ? AND notification_type = ?", userID, notificationType).
		First(&preference).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil if not found (use defaults)
		}
		return nil, fmt.Errorf("failed to get user preference: %w", err)
	}
	return &preference, nil
}

// UpsertUserPreference creates or updates a user notification preference
func (r *notificationRepository) UpsertUserPreference(preference *models.UserNotificationPreference) error {
	err := r.db.Where("user_id = ? AND notification_type = ?",
		preference.UserID, preference.NotificationType).
		Assign(preference).
		FirstOrCreate(&preference).Error
	if err != nil {
		return fmt.Errorf("failed to upsert user preference: %w", err)
	}
	return nil
}

// DeleteUserPreference deletes a user notification preference
func (r *notificationRepository) DeleteUserPreference(userID uint, notificationType models.NotificationType) error {
	result := r.db.Where("user_id = ? AND notification_type = ?", userID, notificationType).
		Delete(&models.UserNotificationPreference{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete user preference: %w", result.Error)
	}
	return nil
}

// Helper methods

// applyFilters applies filtering to the query based on filter request
func (r *notificationRepository) applyFilters(query *gorm.DB, filter *models.NotificationFilterRequest) *gorm.DB {
	if filter == nil {
		return query
	}

	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}

	if filter.Priority != nil {
		query = query.Where("priority = ?", *filter.Priority)
	}

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	if filter.IsRead != nil {
		query = query.Where("is_read = ?", *filter.IsRead)
	}

	if filter.RelatedType != "" {
		query = query.Where("related_type = ?", filter.RelatedType)
	}

	if filter.CreatedAfter != nil {
		query = query.Where("created_at > ?", *filter.CreatedAfter)
	}

	if filter.CreatedBefore != nil {
		query = query.Where("created_at < ?", *filter.CreatedBefore)
	}

	return query
}

// applySortingAndPagination applies sorting and pagination to the query
func (r *notificationRepository) applySortingAndPagination(query *gorm.DB, filter *models.NotificationFilterRequest) *gorm.DB {
	if filter == nil {
		return query.Order("created_at DESC").Limit(20)
	}

	// Apply sorting
	sortBy := "created_at"
	sortOrder := "DESC"

	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}

	if filter.SortOrder != "" {
		sortOrder = filter.SortOrder
	}

	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	if limit > 100 {
		limit = 100 // Maximum limit
	}

	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	return query.Limit(limit).Offset(offset)
}
