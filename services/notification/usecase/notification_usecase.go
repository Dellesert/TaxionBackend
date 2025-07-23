// File: services/notification/usecase/notification_usecase.go
package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/notification/email"
	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/services/notification/repository"
	"tachyon-messenger/shared/logger"

	"gorm.io/gorm"
)

// NotificationUsecase defines the interface for notification business logic
type NotificationUsecase interface {
	// Send notifications
	SendNotification(req *models.CreateNotificationRequest) (*models.NotificationResponse, error)
	SendBulkNotification(req *models.BulkCreateNotificationRequest) error
	SendTemplatedNotification(req *TemplatedNotificationRequest) (*models.NotificationResponse, error)
	SendSystemAnnouncement(req *SystemAnnouncementRequest) error

	// Get notifications
	GetUserNotifications(userID uint, filter *models.NotificationFilterRequest) (*NotificationListResponse, error)
	GetNotificationByID(userID, notificationID uint) (*models.NotificationResponse, error)
	GetUnreadCount(userID uint) (int64, error)
	GetNotificationStats(userID uint) (*models.NotificationStatsResponse, error)

	// Mark as read
	MarkAsRead(userID uint, req *models.MarkAsReadRequest) error
	MarkAllAsRead(userID uint) error
	MarkAllAsReadByType(userID uint, notificationType models.NotificationType) error

	// Search and filtering
	SearchNotifications(userID uint, query string, filter *models.NotificationFilterRequest) (*NotificationListResponse, error)
	GetNotificationsByRelatedObject(relatedType string, relatedID uint, userID *uint) ([]*models.NotificationResponse, error)

	// User preferences
	GetUserPreferences(userID uint) ([]*models.UserNotificationPreference, error)
	UpdateUserPreference(userID uint, req *models.UserPreferenceRequest) error
	GetUserPreference(userID uint, notificationType models.NotificationType) (*models.UserNotificationPreference, error)

	// Admin operations
	DeleteOldNotifications(beforeDate time.Time) (int64, error)
	GetSystemStats() (*repository.SystemNotificationStats, error)
	ProcessScheduledNotifications() error
	RetryFailedDeliveries() error
}

// notificationUsecase implements NotificationUsecase interface
type notificationUsecase struct {
	notificationRepo repository.NotificationRepository
	emailSender      email.EmailSender
}

// Custom request/response models for usecase layer

// TemplatedNotificationRequest represents a templated notification request
type TemplatedNotificationRequest struct {
	UserID       uint                         `json:"user_id" validate:"required,min=1"`
	Type         models.NotificationType      `json:"type" validate:"required"`
	TemplateName string                       `json:"template_name" validate:"required"`
	Variables    map[string]interface{}       `json:"variables,omitempty"`
	Priority     *models.NotificationPriority `json:"priority,omitempty"`
	RelatedID    *uint                        `json:"related_id,omitempty"`
	RelatedType  string                       `json:"related_type,omitempty"`
	ActionURL    string                       `json:"action_url,omitempty"`
	ImageURL     string                       `json:"image_url,omitempty"`
	ScheduledAt  *time.Time                   `json:"scheduled_at,omitempty"`
	ExpiresAt    *time.Time                   `json:"expires_at,omitempty"`
	Channels     []models.DeliveryChannel     `json:"channels,omitempty"`
}

// SystemAnnouncementRequest represents a system announcement request
type SystemAnnouncementRequest struct {
	UserIDs        []uint                      `json:"user_ids,omitempty"` // If empty, send to all users
	Title          string                      `json:"title" validate:"required,min=1,max=255"`
	Content        string                      `json:"content" validate:"required,min=1"`
	Priority       models.NotificationPriority `json:"priority"`
	IsImportant    bool                        `json:"is_important"`
	ActionRequired string                      `json:"action_required,omitempty"`
	ReadMoreURL    string                      `json:"read_more_url,omitempty"`
	ExpiresAt      *time.Time                  `json:"expires_at,omitempty"`
	Channels       []models.DeliveryChannel    `json:"channels,omitempty"`
}

// NotificationListResponse represents a paginated list of notifications
type NotificationListResponse struct {
	Notifications []*models.NotificationResponse `json:"notifications"`
	Total         int64                          `json:"total"`
	Limit         int                            `json:"limit"`
	Offset        int                            `json:"offset"`
	HasMore       bool                           `json:"has_more"`
}

// NewNotificationUsecase creates a new notification usecase
func NewNotificationUsecase(
	notificationRepo repository.NotificationRepository,
	emailSender email.EmailSender,
) NotificationUsecase {
	return &notificationUsecase{
		notificationRepo: notificationRepo,
		emailSender:      emailSender,
	}
}

// Send notifications

// SendNotification sends a notification through multiple channels
func (u *notificationUsecase) SendNotification(req *models.CreateNotificationRequest) (*models.NotificationResponse, error) {
	// Validate request
	if err := u.validateCreateNotificationRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check user preferences
	shouldSend, channels, err := u.checkUserPreferences(req.UserID, req.Type, req.Channels)
	if err != nil {
		return nil, fmt.Errorf("failed to check user preferences: %w", err)
	}

	if !shouldSend {
		logger.WithFields(map[string]interface{}{
			"user_id": req.UserID,
			"type":    req.Type,
		}).Info("Notification skipped due to user preferences")
		return nil, nil
	}

	// Create notification
	notification := &models.Notification{
		UserID:      req.UserID,
		Type:        req.Type,
		Title:       strings.TrimSpace(req.Title),
		Message:     strings.TrimSpace(req.Message),
		Priority:    models.NotificationPriorityMedium, // default
		Status:      models.NotificationStatusPending,
		RelatedID:   req.RelatedID,
		RelatedType: req.RelatedType,
		ActionURL:   req.ActionURL,
		ImageURL:    req.ImageURL,
		ScheduledAt: req.ScheduledAt,
		ExpiresAt:   req.ExpiresAt,
	}

	// Set priority if provided
	if req.Priority != nil {
		notification.Priority = *req.Priority
	}

	// Save notification to database
	if err := u.notificationRepo.CreateNotification(notification); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// Send through channels
	if err := u.sendThroughChannels(notification, channels); err != nil {
		logger.WithFields(map[string]interface{}{
			"notification_id": notification.ID,
			"error":           err.Error(),
		}).Error("Failed to send notification through channels")

		// Update status to failed
		notification.Status = models.NotificationStatusFailed
		u.notificationRepo.UpdateNotification(notification)

		return nil, fmt.Errorf("failed to send notification: %w", err)
	}

	// Update status to delivered
	notification.Status = models.NotificationStatusDelivered
	if err := u.notificationRepo.UpdateNotification(notification); err != nil {
		logger.WithField("notification_id", notification.ID).Error("Failed to update notification status")
	}

	logger.WithFields(map[string]interface{}{
		"notification_id": notification.ID,
		"user_id":         req.UserID,
		"type":            req.Type,
		"channels":        len(channels),
	}).Info("Notification sent successfully")

	return notification.ToResponse(), nil
}

// SendBulkNotification sends notifications to multiple users
func (u *notificationUsecase) SendBulkNotification(req *models.BulkCreateNotificationRequest) error {
	// Validate request
	if err := u.validateBulkCreateNotificationRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create notifications for each user
	notifications := make([]*models.Notification, 0, len(req.UserIDs))
	for _, userID := range req.UserIDs {
		// Check user preferences
		shouldSend, _, err := u.checkUserPreferences(userID, req.Type, req.Channels)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			}).Warn("Failed to check user preferences, skipping")
			continue
		}

		if !shouldSend {
			continue
		}

		notification := &models.Notification{
			UserID:      userID,
			Type:        req.Type,
			Title:       strings.TrimSpace(req.Title),
			Message:     strings.TrimSpace(req.Message),
			Priority:    models.NotificationPriorityMedium,
			Status:      models.NotificationStatusPending,
			RelatedID:   req.RelatedID,
			RelatedType: req.RelatedType,
			ActionURL:   req.ActionURL,
			ImageURL:    req.ImageURL,
			ScheduledAt: req.ScheduledAt,
			ExpiresAt:   req.ExpiresAt,
		}

		if req.Priority != nil {
			notification.Priority = *req.Priority
		}

		notifications = append(notifications, notification)
	}

	if len(notifications) == 0 {
		return fmt.Errorf("no notifications to send after filtering")
	}

	// Bulk create notifications
	if err := u.notificationRepo.CreateBulkNotifications(notifications); err != nil {
		return fmt.Errorf("failed to create bulk notifications: %w", err)
	}

	// Send through channels (async processing could be implemented here)
	successCount := 0
	for _, notification := range notifications {
		channels := req.Channels
		if len(channels) == 0 {
			channels = []models.DeliveryChannel{models.DeliveryChannelInApp}
		}

		if err := u.sendThroughChannels(notification, channels); err != nil {
			logger.WithFields(map[string]interface{}{
				"notification_id": notification.ID,
				"user_id":         notification.UserID,
				"error":           err.Error(),
			}).Error("Failed to send bulk notification")

			notification.Status = models.NotificationStatusFailed
		} else {
			notification.Status = models.NotificationStatusDelivered
			successCount++
		}

		u.notificationRepo.UpdateNotification(notification)
	}

	logger.WithFields(map[string]interface{}{
		"total_notifications": len(notifications),
		"successful":          successCount,
		"failed":              len(notifications) - successCount,
	}).Info("Bulk notification sending completed")

	return nil
}

// SendTemplatedNotification sends a notification using a template
func (u *notificationUsecase) SendTemplatedNotification(req *TemplatedNotificationRequest) (*models.NotificationResponse, error) {
	// Validate request
	if err := u.validateTemplatedNotificationRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check user preferences
	shouldSend, channels, err := u.checkUserPreferences(req.UserID, req.Type, req.Channels)
	if err != nil {
		return nil, fmt.Errorf("failed to check user preferences: %w", err)
	}

	if !shouldSend {
		return nil, nil
	}

	// For templated emails, we'll send directly through email sender
	// and create a simple in-app notification
	if u.shouldSendEmail(channels) {
		emailReq := &email.TemplatedEmailRequest{
			To:           []string{}, // This would need user email lookup
			TemplateName: req.TemplateName,
			Variables:    req.Variables,
			Priority:     u.convertPriorityForEmail(req.Priority),
		}

		// TODO: Get user email from user service
		// For now, we'll skip email sending in templates
		_ = emailReq
	}

	// Create in-app notification with rendered title
	title, err := u.renderTemplateString(req.TemplateName+"_title", req.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render notification title: %w", err)
	}

	message, err := u.renderTemplateString(req.TemplateName+"_message", req.Variables)
	if err != nil {
		// If message template fails, use empty message
		message = ""
	}

	createReq := &models.CreateNotificationRequest{
		UserID:      req.UserID,
		Type:        req.Type,
		Title:       title,
		Message:     message,
		Priority:    req.Priority,
		RelatedID:   req.RelatedID,
		RelatedType: req.RelatedType,
		ActionURL:   req.ActionURL,
		ImageURL:    req.ImageURL,
		ScheduledAt: req.ScheduledAt,
		ExpiresAt:   req.ExpiresAt,
		Channels:    []models.DeliveryChannel{models.DeliveryChannelInApp},
	}

	return u.SendNotification(createReq)
}

// SendSystemAnnouncement sends a system-wide announcement
func (u *notificationUsecase) SendSystemAnnouncement(req *SystemAnnouncementRequest) error {
	// Validate request
	if err := u.validateSystemAnnouncementRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	userIDs := req.UserIDs

	// If no specific users provided, send to all users (would need user service integration)
	if len(userIDs) == 0 {
		// TODO: Get all active user IDs from user service
		return fmt.Errorf("sending to all users not implemented yet")
	}

	// Send bulk notification
	bulkReq := &models.BulkCreateNotificationRequest{
		UserIDs:   userIDs,
		Type:      models.NotificationTypeAnnounce,
		Title:     req.Title,
		Message:   req.Content,
		Priority:  &req.Priority,
		ExpiresAt: req.ExpiresAt,
		Channels:  req.Channels,
	}

	return u.SendBulkNotification(bulkReq)
}

// Get notifications

// GetUserNotifications retrieves notifications for a user with filtering and pagination
func (u *notificationUsecase) GetUserNotifications(userID uint, filter *models.NotificationFilterRequest) (*NotificationListResponse, error) {
	notifications, total, err := u.notificationRepo.GetUserNotifications(userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get user notifications: %w", err)
	}

	// Convert to response format
	responses := make([]*models.NotificationResponse, len(notifications))
	for i, notification := range notifications {
		responses[i] = notification.ToResponse()
	}

	// Calculate pagination info
	limit := 20 // default
	offset := 0
	if filter != nil {
		if filter.Limit > 0 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}

	hasMore := int64(offset+len(responses)) < total

	return &NotificationListResponse{
		Notifications: responses,
		Total:         total,
		Limit:         limit,
		Offset:        offset,
		HasMore:       hasMore,
	}, nil
}

// GetNotificationByID retrieves a single notification by ID
func (u *notificationUsecase) GetNotificationByID(userID, notificationID uint) (*models.NotificationResponse, error) {
	notification, err := u.notificationRepo.GetNotificationByID(notificationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("notification not found")
		}
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	// Check if notification belongs to user
	if notification.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	return notification.ToResponse(), nil
}

// GetUnreadCount returns the count of unread notifications for a user
func (u *notificationUsecase) GetUnreadCount(userID uint) (int64, error) {
	count, err := u.notificationRepo.GetUnreadCount(userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// GetNotificationStats returns notification statistics for a user
func (u *notificationUsecase) GetNotificationStats(userID uint) (*models.NotificationStatsResponse, error) {
	stats, err := u.notificationRepo.GetNotificationStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notification stats: %w", err)
	}
	return stats, nil
}

// Mark as read operations

// MarkAsRead marks specific notifications as read
func (u *notificationUsecase) MarkAsRead(userID uint, req *models.MarkAsReadRequest) error {
	if err := u.validateMarkAsReadRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := u.notificationRepo.MarkMultipleAsRead(req.NotificationIDs, userID); err != nil {
		return fmt.Errorf("failed to mark notifications as read: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":            userID,
		"notification_count": len(req.NotificationIDs),
	}).Info("Notifications marked as read")

	return nil
}

// MarkAllAsRead marks all notifications as read for a user
func (u *notificationUsecase) MarkAllAsRead(userID uint) error {
	if err := u.notificationRepo.MarkAllAsRead(userID); err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	logger.WithField("user_id", userID).Info("All notifications marked as read")
	return nil
}

// MarkAllAsReadByType marks all notifications of a specific type as read
func (u *notificationUsecase) MarkAllAsReadByType(userID uint, notificationType models.NotificationType) error {
	if err := u.notificationRepo.MarkAllAsReadByType(userID, notificationType); err != nil {
		return fmt.Errorf("failed to mark notifications as read by type: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"type":    notificationType,
	}).Info("Notifications marked as read by type")

	return nil
}

// Search and filtering

// SearchNotifications searches notifications for a user
func (u *notificationUsecase) SearchNotifications(userID uint, query string, filter *models.NotificationFilterRequest) (*NotificationListResponse, error) {
	notifications, total, err := u.notificationRepo.SearchNotifications(userID, query, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search notifications: %w", err)
	}

	// Convert to response format
	responses := make([]*models.NotificationResponse, len(notifications))
	for i, notification := range notifications {
		responses[i] = notification.ToResponse()
	}

	// Calculate pagination info
	limit := 20
	offset := 0
	if filter != nil {
		if filter.Limit > 0 {
			limit = filter.Limit
		}
		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}

	hasMore := int64(offset+len(responses)) < total

	return &NotificationListResponse{
		Notifications: responses,
		Total:         total,
		Limit:         limit,
		Offset:        offset,
		HasMore:       hasMore,
	}, nil
}

// GetNotificationsByRelatedObject returns notifications related to a specific object
func (u *notificationUsecase) GetNotificationsByRelatedObject(relatedType string, relatedID uint, userID *uint) ([]*models.NotificationResponse, error) {
	notifications, err := u.notificationRepo.GetNotificationsByRelatedObject(relatedType, relatedID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get notifications by related object: %w", err)
	}

	responses := make([]*models.NotificationResponse, len(notifications))
	for i, notification := range notifications {
		responses[i] = notification.ToResponse()
	}

	return responses, nil
}

// User preferences

// GetUserPreferences returns all notification preferences for a user
func (u *notificationUsecase) GetUserPreferences(userID uint) ([]*models.UserNotificationPreference, error) {
	preferences, err := u.notificationRepo.GetUserPreferences(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}
	return preferences, nil
}

// UpdateUserPreference updates a user's notification preference
func (u *notificationUsecase) UpdateUserPreference(userID uint, req *models.UserPreferenceRequest) error {
	if err := u.validateUserPreferenceRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	preference := &models.UserNotificationPreference{
		UserID:           userID,
		NotificationType: req.NotificationType,
		InAppEnabled:     true,                           // default
		EmailEnabled:     true,                           // default
		PushEnabled:      true,                           // default
		SMSEnabled:       false,                          // default
		MinPriority:      models.NotificationPriorityLow, // default
		WeekendEnabled:   true,                           // default
		DigestEnabled:    false,                          // default
	}

	// Update fields if provided
	if req.InAppEnabled != nil {
		preference.InAppEnabled = *req.InAppEnabled
	}
	if req.EmailEnabled != nil {
		preference.EmailEnabled = *req.EmailEnabled
	}
	if req.PushEnabled != nil {
		preference.PushEnabled = *req.PushEnabled
	}
	if req.SMSEnabled != nil {
		preference.SMSEnabled = *req.SMSEnabled
	}
	if req.MinPriority != nil {
		preference.MinPriority = *req.MinPriority
	}
	if req.QuietHoursStart != nil {
		preference.QuietHoursStart = req.QuietHoursStart
	}
	if req.QuietHoursEnd != nil {
		preference.QuietHoursEnd = req.QuietHoursEnd
	}
	if req.WeekendEnabled != nil {
		preference.WeekendEnabled = *req.WeekendEnabled
	}
	if req.DigestEnabled != nil {
		preference.DigestEnabled = *req.DigestEnabled
	}
	if req.DigestFrequency != nil {
		preference.DigestFrequency = req.DigestFrequency
	}

	if err := u.notificationRepo.UpsertUserPreference(preference); err != nil {
		return fmt.Errorf("failed to update user preference: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"type":    req.NotificationType,
	}).Info("User notification preference updated")

	return nil
}

// GetUserPreference returns a specific notification preference for a user
func (u *notificationUsecase) GetUserPreference(userID uint, notificationType models.NotificationType) (*models.UserNotificationPreference, error) {
	preference, err := u.notificationRepo.GetUserPreference(userID, notificationType)
	if err != nil {
		return nil, fmt.Errorf("failed to get user preference: %w", err)
	}

	// Return default preferences if not found
	if preference == nil {
		preference = &models.UserNotificationPreference{
			UserID:           userID,
			NotificationType: notificationType,
			InAppEnabled:     true,
			EmailEnabled:     true,
			PushEnabled:      true,
			SMSEnabled:       false,
			MinPriority:      models.NotificationPriorityLow,
			WeekendEnabled:   true,
			DigestEnabled:    false,
		}
	}

	return preference, nil
}

// Admin operations

// DeleteOldNotifications deletes notifications older than the specified date
func (u *notificationUsecase) DeleteOldNotifications(beforeDate time.Time) (int64, error) {
	count, err := u.notificationRepo.DeleteOldNotifications(beforeDate)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old notifications: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"deleted_count": count,
		"before_date":   beforeDate,
	}).Info("Old notifications deleted")

	return count, nil
}

// GetSystemStats returns system-wide notification statistics
func (u *notificationUsecase) GetSystemStats() (*repository.SystemNotificationStats, error) {
	stats, err := u.notificationRepo.GetSystemStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get system stats: %w", err)
	}
	return stats, nil
}

// ProcessScheduledNotifications processes notifications that are scheduled to be sent
func (u *notificationUsecase) ProcessScheduledNotifications() error {
	now := time.Now()
	notifications, err := u.notificationRepo.GetScheduledNotifications(now, 100)
	if err != nil {
		return fmt.Errorf("failed to get scheduled notifications: %w", err)
	}

	processedCount := 0
	for _, notification := range notifications {
		// Send the notification
		channels := []models.DeliveryChannel{models.DeliveryChannelInApp} // default
		if err := u.sendThroughChannels(notification, channels); err != nil {
			logger.WithFields(map[string]interface{}{
				"notification_id": notification.ID,
				"error":           err.Error(),
			}).Error("Failed to send scheduled notification")

			notification.Status = models.NotificationStatusFailed
		} else {
			notification.Status = models.NotificationStatusDelivered
			processedCount++
		}

		// Clear scheduled time
		notification.ScheduledAt = nil
		u.notificationRepo.UpdateNotification(notification)
	}

	if len(notifications) > 0 {
		logger.WithFields(map[string]interface{}{
			"total_scheduled": len(notifications),
			"processed":       processedCount,
			"failed":          len(notifications) - processedCount,
		}).Info("Scheduled notifications processed")
	}

	return nil
}

// RetryFailedDeliveries retries failed notification deliveries
func (u *notificationUsecase) RetryFailedDeliveries() error {
	maxAttempts := 3
	deliveries, err := u.notificationRepo.GetFailedDeliveries(maxAttempts, 50)
	if err != nil {
		return fmt.Errorf("failed to get failed deliveries: %w", err)
	}

	retriedCount := 0
	for _, delivery := range deliveries {
		// Get the notification for this delivery
		notification, err := u.notificationRepo.GetNotificationByID(delivery.NotificationID)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"delivery_id":     delivery.ID,
				"notification_id": delivery.NotificationID,
				"error":           err.Error(),
			}).Error("Failed to get notification for retry")

			// Update delivery status to failed
			u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, err.Error())
			continue
		}

		// Retry sending through the specific channel
		if err := u.sendThroughChannel(notification, delivery.Channel); err != nil {
			// Update delivery status
			u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, err.Error())
			continue
		}

		// Update delivery status to delivered
		u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusDelivered, "")
		retriedCount++
	}

	if len(deliveries) > 0 {
		logger.WithFields(map[string]interface{}{
			"total_retries": len(deliveries),
			"successful":    retriedCount,
			"failed":        len(deliveries) - retriedCount,
		}).Info("Failed deliveries retried")
	}

	return nil
}

// Helper methods

// checkUserPreferences checks if notification should be sent based on user preferences
func (u *notificationUsecase) checkUserPreferences(userID uint, notificationType models.NotificationType, requestedChannels []models.DeliveryChannel) (bool, []models.DeliveryChannel, error) {
	preference, err := u.GetUserPreference(userID, notificationType)
	if err != nil {
		return false, nil, err
	}

	// Check if notification type is enabled
	if !preference.InAppEnabled && !preference.EmailEnabled && !preference.PushEnabled && !preference.SMSEnabled {
		return false, nil, nil
	}

	// Check quiet hours
	if u.isInQuietHours(preference) {
		return false, nil, nil
	}

	// Check weekend preferences
	if !preference.WeekendEnabled && u.isWeekend() {
		return false, nil, nil
	}

	// Determine available channels based on preferences
	availableChannels := make([]models.DeliveryChannel, 0)
	if preference.InAppEnabled {
		availableChannels = append(availableChannels, models.DeliveryChannelInApp)
	}
	if preference.EmailEnabled {
		availableChannels = append(availableChannels, models.DeliveryChannelEmail)
	}
	if preference.PushEnabled {
		availableChannels = append(availableChannels, models.DeliveryChannelPush)
	}
	if preference.SMSEnabled {
		availableChannels = append(availableChannels, models.DeliveryChannelSMS)
	}

	// Filter requested channels by available channels
	finalChannels := make([]models.DeliveryChannel, 0)
	if len(requestedChannels) == 0 {
		// Use all available channels if none specified
		finalChannels = availableChannels
	} else {
		// Use intersection of requested and available channels
		for _, requested := range requestedChannels {
			for _, available := range availableChannels {
				if requested == available {
					finalChannels = append(finalChannels, requested)
					break
				}
			}
		}
	}

	if len(finalChannels) == 0 {
		return false, nil, nil
	}

	return true, finalChannels, nil
}

// isInQuietHours checks if current time is within user's quiet hours
func (u *notificationUsecase) isInQuietHours(preference *models.UserNotificationPreference) bool {
	if preference.QuietHoursStart == nil || preference.QuietHoursEnd == nil {
		return false
	}

	now := time.Now()
	currentHour := now.Hour()

	start := *preference.QuietHoursStart
	end := *preference.QuietHoursEnd

	// Handle quiet hours that span midnight
	if start > end {
		return currentHour >= start || currentHour < end
	}

	return currentHour >= start && currentHour < end
}

// isWeekend checks if current time is weekend
func (u *notificationUsecase) isWeekend() bool {
	now := time.Now()
	weekday := now.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// sendThroughChannels sends notification through multiple channels
func (u *notificationUsecase) sendThroughChannels(notification *models.Notification, channels []models.DeliveryChannel) error {
	var errors []string

	for _, channel := range channels {
		if err := u.sendThroughChannel(notification, channel); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", channel, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("channel delivery errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// sendThroughChannel sends notification through a specific channel
func (u *notificationUsecase) sendThroughChannel(notification *models.Notification, channel models.DeliveryChannel) error {
	// Create delivery record
	delivery := &models.NotificationDelivery{
		NotificationID: notification.ID,
		Channel:        channel,
		Status:         models.NotificationStatusPending,
		AttemptCount:   0,
	}

	if err := u.notificationRepo.CreateDelivery(delivery); err != nil {
		return fmt.Errorf("failed to create delivery record: %w", err)
	}

	switch channel {
	case models.DeliveryChannelInApp:
		// In-app notifications are stored in database, no additional action needed
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusDelivered, "")

	case models.DeliveryChannelEmail:
		return u.sendEmailNotification(notification, delivery)

	case models.DeliveryChannelPush:
		// TODO: Implement push notification sending
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "Push notifications not implemented")

	case models.DeliveryChannelSMS:
		// TODO: Implement SMS sending
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "SMS notifications not implemented")

	case models.DeliveryChannelSlack:
		// TODO: Implement Slack integration
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "Slack notifications not implemented")

	case models.DeliveryChannelWebhook:
		// TODO: Implement webhook sending
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "Webhook notifications not implemented")

	default:
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "Unknown delivery channel")
	}
}

// sendEmailNotification sends notification via email
func (u *notificationUsecase) sendEmailNotification(notification *models.Notification, delivery *models.NotificationDelivery) error {
	if u.emailSender == nil {
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, "Email sender not configured")
	}

	// TODO: Get user email from user service
	// For now, we'll simulate email sending
	userEmail := "user@example.com" // This should be fetched from user service

	emailReq := &email.SendEmailRequest{
		To:       []string{userEmail},
		Subject:  notification.Title,
		HTMLBody: u.buildEmailHTML(notification),
		TextBody: u.buildEmailText(notification),
		Priority: u.convertPriorityForEmail(&notification.Priority),
	}

	if err := u.emailSender.SendEmail(emailReq); err != nil {
		return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusFailed, err.Error())
	}

	return u.notificationRepo.UpdateDeliveryStatus(delivery.ID, models.NotificationStatusDelivered, "")
}

// buildEmailHTML builds HTML email content
func (u *notificationUsecase) buildEmailHTML(notification *models.Notification) string {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #007bff; color: white; padding: 20px; text-align: center; }
        .content { padding: 20px; background-color: #f9f9f9; }
        .footer { padding: 10px; text-align: center; color: #666; font-size: 12px; }
        .button { display: inline-block; background-color: #007bff; color: white; padding: 10px 20px; text-decoration: none; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>%s</h1>
        </div>
        <div class="content">
            <p>%s</p>
            %s
        </div>
        <div class="footer">
            <p>Это автоматическое сообщение от Tachyon Messenger</p>
        </div>
    </div>
</body>
</html>`,
		notification.Title,
		notification.Title,
		notification.Message,
		u.buildActionButton(notification),
	)

	return html
}

// buildEmailText builds plain text email content
func (u *notificationUsecase) buildEmailText(notification *models.Notification) string {
	text := fmt.Sprintf("%s\n\n%s", notification.Title, notification.Message)

	if notification.ActionURL != "" {
		text += fmt.Sprintf("\n\nДля получения дополнительной информации перейдите по ссылке: %s", notification.ActionURL)
	}

	text += "\n\n---\nЭто автоматическое сообщение от Tachyon Messenger"
	return text
}

// buildActionButton builds action button HTML if action URL exists
func (u *notificationUsecase) buildActionButton(notification *models.Notification) string {
	if notification.ActionURL == "" {
		return ""
	}

	return fmt.Sprintf(`
		<div style="text-align: center; margin: 20px 0;">
			<a href="%s" class="button">Открыть</a>
		</div>
	`, notification.ActionURL)
}

// convertPriorityForEmail converts notification priority to email priority
func (u *notificationUsecase) convertPriorityForEmail(priority *models.NotificationPriority) models.NotificationPriority {
	if priority == nil {
		return models.NotificationPriorityMedium
	}
	return *priority
}

// shouldSendEmail checks if email should be sent based on channels
func (u *notificationUsecase) shouldSendEmail(channels []models.DeliveryChannel) bool {
	for _, channel := range channels {
		if channel == models.DeliveryChannelEmail {
			return true
		}
	}
	return false
}

// renderTemplateString renders a simple template string (basic implementation)
func (u *notificationUsecase) renderTemplateString(templateName string, variables map[string]interface{}) (string, error) {
	// This is a simplified template rendering
	// In a real implementation, you'd use proper templating

	templates := map[string]string{
		"welcome_title":                "Добро пожаловать, {{.UserName}}!",
		"welcome_message":              "Ваш аккаунт успешно создан в Tachyon Messenger",
		"task_assigned_title":          "Новая задача: {{.TaskTitle}}",
		"task_assigned_message":        "Вам назначена задача с приоритетом {{.TaskPriority}}",
		"message_notification_title":   "Новое сообщение от {{.SenderName}}",
		"message_notification_message": "{{.MessageContent}}",
		"calendar_reminder_title":      "Напоминание: {{.EventTitle}}",
		"calendar_reminder_message":    "Событие начинается {{.StartTime}}",
	}

	template, exists := templates[templateName]
	if !exists {
		return templateName, nil // Return template name if not found
	}

	// Simple variable substitution (in real implementation, use proper templating)
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	return result, nil
}

// Validation methods

// validateCreateNotificationRequest validates create notification request
func (u *notificationUsecase) validateCreateNotificationRequest(req *models.CreateNotificationRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.UserID == 0 {
		return fmt.Errorf("user ID is required")
	}

	if req.Type == "" {
		return fmt.Errorf("notification type is required")
	}

	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(req.Title) > 255 {
		return fmt.Errorf("title too long (max 255 characters)")
	}

	if len(req.Message) > 2000 {
		return fmt.Errorf("message too long (max 2000 characters)")
	}

	// Validate channels
	for _, channel := range req.Channels {
		if !u.isValidChannel(channel) {
			return fmt.Errorf("invalid delivery channel: %s", channel)
		}
	}

	// Validate scheduled time
	if req.ScheduledAt != nil && req.ScheduledAt.Before(time.Now()) {
		return fmt.Errorf("scheduled time cannot be in the past")
	}

	// Validate expiration time
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("expiration time cannot be in the past")
	}

	return nil
}

// validateBulkCreateNotificationRequest validates bulk create notification request
func (u *notificationUsecase) validateBulkCreateNotificationRequest(req *models.BulkCreateNotificationRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if len(req.UserIDs) == 0 {
		return fmt.Errorf("at least one user ID is required")
	}

	if len(req.UserIDs) > 1000 {
		return fmt.Errorf("too many user IDs (max 1000)")
	}

	for _, userID := range req.UserIDs {
		if userID == 0 {
			return fmt.Errorf("invalid user ID: 0")
		}
	}

	if req.Type == "" {
		return fmt.Errorf("notification type is required")
	}

	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(req.Title) > 255 {
		return fmt.Errorf("title too long (max 255 characters)")
	}

	if len(req.Message) > 2000 {
		return fmt.Errorf("message too long (max 2000 characters)")
	}

	return nil
}

// validateTemplatedNotificationRequest validates templated notification request
func (u *notificationUsecase) validateTemplatedNotificationRequest(req *TemplatedNotificationRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.UserID == 0 {
		return fmt.Errorf("user ID is required")
	}

	if req.Type == "" {
		return fmt.Errorf("notification type is required")
	}

	if strings.TrimSpace(req.TemplateName) == "" {
		return fmt.Errorf("template name is required")
	}

	return nil
}

// validateSystemAnnouncementRequest validates system announcement request
func (u *notificationUsecase) validateSystemAnnouncementRequest(req *SystemAnnouncementRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title is required")
	}

	if len(req.Title) > 255 {
		return fmt.Errorf("title too long (max 255 characters)")
	}

	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required")
	}

	if len(req.Content) > 5000 {
		return fmt.Errorf("content too long (max 5000 characters)")
	}

	return nil
}

// validateMarkAsReadRequest validates mark as read request
func (u *notificationUsecase) validateMarkAsReadRequest(req *models.MarkAsReadRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if len(req.NotificationIDs) == 0 {
		return fmt.Errorf("at least one notification ID is required")
	}

	if len(req.NotificationIDs) > 100 {
		return fmt.Errorf("too many notification IDs (max 100)")
	}

	for _, id := range req.NotificationIDs {
		if id == 0 {
			return fmt.Errorf("invalid notification ID: 0")
		}
	}

	return nil
}

// validateUserPreferenceRequest validates user preference request
func (u *notificationUsecase) validateUserPreferenceRequest(req *models.UserPreferenceRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.NotificationType == "" {
		return fmt.Errorf("notification type is required")
	}

	// Validate quiet hours
	if req.QuietHoursStart != nil {
		if *req.QuietHoursStart < 0 || *req.QuietHoursStart > 23 {
			return fmt.Errorf("quiet hours start must be between 0 and 23")
		}
	}

	if req.QuietHoursEnd != nil {
		if *req.QuietHoursEnd < 0 || *req.QuietHoursEnd > 23 {
			return fmt.Errorf("quiet hours end must be between 0 and 23")
		}
	}

	// Validate digest frequency
	if req.DigestFrequency != nil {
		if *req.DigestFrequency < 15 || *req.DigestFrequency > 1440 {
			return fmt.Errorf("digest frequency must be between 15 and 1440 minutes")
		}
	}

	return nil
}

// isValidChannel checks if delivery channel is valid
func (u *notificationUsecase) isValidChannel(channel models.DeliveryChannel) bool {
	switch channel {
	case models.DeliveryChannelInApp, models.DeliveryChannelEmail, models.DeliveryChannelPush,
		models.DeliveryChannelSMS, models.DeliveryChannelSlack, models.DeliveryChannelWebhook:
		return true
	default:
		return false
	}
}
