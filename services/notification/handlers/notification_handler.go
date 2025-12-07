// File: services/notification/handlers/notification_handler.go
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/services/notification/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// NotificationHandler handles HTTP requests for notification operations
type NotificationHandler struct {
	notificationUsecase usecase.NotificationUsecase
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(notificationUsecase usecase.NotificationUsecase) *NotificationHandler {
	return &NotificationHandler{
		notificationUsecase: notificationUsecase,
	}
}

// GetNotifications handles getting notifications for a user with filtering and pagination
// GET /api/v1/notifications
func (h *NotificationHandler) GetNotifications(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	filter := &models.NotificationFilterRequest{}
	if err := c.ShouldBindQuery(filter); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid query parameters for get notifications")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid query parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set default values if not provided
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Get notifications
	notifications, err := h.notificationUsecase.GetUserNotifications(userID, filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user notifications")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get notifications",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":         requestID,
		"user_id":            userID,
		"notification_count": len(notifications.Notifications),
		"total":              notifications.Total,
	}).Info("User notifications retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications.Notifications,
		"total":         notifications.Total,
		"limit":         notifications.Limit,
		"offset":        notifications.Offset,
		"has_more":      notifications.HasMore,
		"request_id":    requestID,
	})
}

// GetNotificationByID handles getting a single notification by ID
// GET /api/v1/notifications/:id
func (h *NotificationHandler) GetNotificationByID(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse notification ID from URL parameter
	idStr := c.Param("id")
	notificationID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": idStr,
			"error":           err.Error(),
		}).Warn("Invalid notification ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid notification ID",
			"request_id": requestID,
		})
		return
	}

	// Get notification
	notification, err := h.notificationUsecase.GetNotificationByID(userID, uint(notificationID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": notificationID,
			"error":           err.Error(),
		}).Error("Failed to get notification")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get notification"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Notification not found"
		} else if strings.Contains(err.Error(), "access denied") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"notification_id": notificationID,
	}).Info("Notification retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"notification": notification,
		"request_id":   requestID,
	})
}

// MarkAsRead handles marking a single notification as read
// PUT /api/v1/notifications/:id/read
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse notification ID from URL parameter
	idStr := c.Param("id")
	notificationID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": idStr,
			"error":           err.Error(),
		}).Warn("Invalid notification ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid notification ID",
			"request_id": requestID,
		})
		return
	}

	// Mark notification as read
	err = h.notificationUsecase.MarkAsRead(userID, &models.MarkAsReadRequest{
		NotificationIDs: []uint{uint(notificationID)},
	})
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": notificationID,
			"error":           err.Error(),
		}).Error("Failed to mark notification as read")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to mark notification as read"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Notification not found"
		} else if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "already read") {
			statusCode = http.StatusConflict
			errorMessage = "Notification already read"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"notification_id": notificationID,
	}).Info("Notification marked as read successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Notification marked as read",
		"request_id": requestID,
	})
}

// MarkMultipleAsRead handles marking multiple notifications as read
// PUT /api/v1/notifications/read
func (h *NotificationHandler) MarkMultipleAsRead(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req models.MarkAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for mark as read")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Mark notifications as read
	err = h.notificationUsecase.MarkAsRead(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":         requestID,
			"user_id":            userID,
			"notification_count": len(req.NotificationIDs),
			"error":              err.Error(),
		}).Error("Failed to mark notifications as read")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to mark notifications as read"

		if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":         requestID,
		"user_id":            userID,
		"notification_count": len(req.NotificationIDs),
	}).Info("Notifications marked as read successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Notifications marked as read",
		"count":      len(req.NotificationIDs),
		"request_id": requestID,
	})
}

// MarkAllAsRead handles marking all notifications as read for a user
// PUT /api/v1/notifications/read-all
func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Check if type filter is provided
	notificationType := c.Query("type")
	if notificationType != "" {
		// Mark all notifications of specific type as read
		err = h.notificationUsecase.MarkAllAsReadByType(userID, models.NotificationType(notificationType))
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"type":       notificationType,
				"error":      err.Error(),
			}).Error("Failed to mark all notifications as read by type")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to mark all notifications as read",
				"request_id": requestID,
			})
			return
		}

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"type":       notificationType,
		}).Info("All notifications marked as read by type")

		c.JSON(http.StatusOK, gin.H{
			"message":    "All notifications marked as read",
			"type":       notificationType,
			"request_id": requestID,
		})
		return
	}

	// Mark all notifications as read
	err = h.notificationUsecase.MarkAllAsRead(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to mark all notifications as read")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to mark all notifications as read",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("All notifications marked as read successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "All notifications marked as read",
		"request_id": requestID,
	})
}

// GetUnreadCount handles getting the count of unread notifications for a user
// GET /api/v1/notifications/unread-count
func (h *NotificationHandler) GetUnreadCount(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get unread count
	count, err := h.notificationUsecase.GetUnreadCount(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get unread count")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get unread count",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"user_id":      userID,
		"unread_count": count,
	}).Info("Unread count retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"unread_count": count,
		"request_id":   requestID,
	})
}

// GetNotificationStats handles getting notification statistics for a user
// GET /api/v1/notifications/stats
func (h *NotificationHandler) GetNotificationStats(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get notification statistics
	stats, err := h.notificationUsecase.GetNotificationStats(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get notification stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get notification statistics",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":           requestID,
		"user_id":              userID,
		"total_notifications":  stats.TotalNotifications,
		"unread_notifications": stats.UnreadNotifications,
	}).Info("Notification stats retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"stats":      stats,
		"request_id": requestID,
	})
}

// SearchNotifications handles searching notifications for a user
// GET /api/v1/notifications/search
func (h *NotificationHandler) SearchNotifications(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get search query
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Search query is required",
			"request_id": requestID,
		})
		return
	}

	// Parse filter parameters
	filter := &models.NotificationFilterRequest{}
	if err := c.ShouldBindQuery(filter); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid query parameters for search")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid query parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set default values
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	// Search notifications
	notifications, err := h.notificationUsecase.SearchNotifications(userID, query, filter)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"query":      query,
			"error":      err.Error(),
		}).Error("Failed to search notifications")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to search notifications",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"user_id":      userID,
		"query":        query,
		"result_count": len(notifications.Notifications),
		"total":        notifications.Total,
	}).Info("Notifications searched successfully")

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications.Notifications,
		"total":         notifications.Total,
		"query":         query,
		"limit":         notifications.Limit,
		"offset":        notifications.Offset,
		"has_more":      notifications.HasMore,
		"request_id":    requestID,
	})
}

// GetUserPreferences handles getting user notification preferences
// GET /api/v1/notifications/preferences
func (h *NotificationHandler) GetUserPreferences(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get user preferences
	preferences, err := h.notificationUsecase.GetUserPreferences(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user preferences")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get user preferences",
			"request_id": requestID,
		})
		return
	}

	// Log preference values for debugging
	logFields := map[string]interface{}{
		"request_id":        requestID,
		"user_id":           userID,
		"preferences_count": len(preferences),
	}
	for _, pref := range preferences {
		key := fmt.Sprintf("push_%s", pref.NotificationType)
		logFields[key] = pref.PushEnabled
	}
	logger.WithFields(logFields).Info("User preferences retrieved successfully - returning to client")

	c.JSON(http.StatusOK, gin.H{
		"preferences": preferences,
		"request_id":  requestID,
	})
}

// UpdateUserPreference handles updating user notification preference
// PUT /api/v1/notifications/preferences/:type
func (h *NotificationHandler) UpdateUserPreference(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get notification type from URL parameter
	notificationType := models.NotificationType(c.Param("type"))

	// Parse request body
	var req models.UserPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"type":       notificationType,
			"error":      err.Error(),
		}).Warn("Invalid request body for update preference")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set notification type from URL
	req.NotificationType = notificationType

	// Log raw request body for debugging - dereference pointers to see actual values
	logFields := map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"type":       notificationType,
	}
	if req.InAppEnabled != nil {
		logFields["in_app_enabled"] = *req.InAppEnabled
	} else {
		logFields["in_app_enabled"] = "<nil>"
	}
	if req.EmailEnabled != nil {
		logFields["email_enabled"] = *req.EmailEnabled
	} else {
		logFields["email_enabled"] = "<nil>"
	}
	if req.PushEnabled != nil {
		logFields["push_enabled"] = *req.PushEnabled
	} else {
		logFields["push_enabled"] = "<nil>"
	}
	if req.SMSEnabled != nil {
		logFields["sms_enabled"] = *req.SMSEnabled
	} else {
		logFields["sms_enabled"] = "<nil>"
	}
	if req.WeekendEnabled != nil {
		logFields["weekend_enabled"] = *req.WeekendEnabled
	} else {
		logFields["weekend_enabled"] = "<nil>"
	}
	if req.DigestEnabled != nil {
		logFields["digest_enabled"] = *req.DigestEnabled
	} else {
		logFields["digest_enabled"] = "<nil>"
	}
	logger.WithFields(logFields).Info("Handler received preference update request")

	// Update user preference
	err = h.notificationUsecase.UpdateUserPreference(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"type":       notificationType,
			"error":      err.Error(),
		}).Error("Failed to update user preference")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update user preference"

		if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"type":       notificationType,
	}).Info("User preference updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "User preference updated successfully",
		"type":       notificationType,
		"request_id": requestID,
	})
}

// DeleteNotification handles deleting a single notification
// DELETE /api/v1/notifications/:id
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get notification ID from URL parameter
	notificationIDStr := c.Param("id")
	notificationID, err := strconv.ParseUint(notificationIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid notification ID",
			"request_id": requestID,
		})
		return
	}

	// Delete the notification
	if err := h.notificationUsecase.DeleteNotification(userID, uint(notificationID)); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": notificationID,
			"error":           err.Error(),
		}).Error("Failed to delete notification")

		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"notification_id": notificationID,
	}).Info("Notification deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Notification deleted successfully",
		"request_id": requestID,
	})
}

// DeleteAllNotifications handles deleting all notifications for a user
// DELETE /api/v1/notifications
func (h *NotificationHandler) DeleteAllNotifications(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Delete all user notifications
	deletedCount, err := h.notificationUsecase.DeleteAllUserNotifications(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to delete all notifications")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to delete all notifications",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"deleted_count": deletedCount,
	}).Info("All notifications deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":       "All notifications deleted successfully",
		"deleted_count": deletedCount,
		"request_id":    requestID,
	})
}

// GetGroupedNotificationTasks handles getting tasks from a grouped notification
// GET /api/v1/notifications/:id/tasks
func (h *NotificationHandler) GetGroupedNotificationTasks(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse notification ID from URL parameter
	idStr := c.Param("id")
	notificationID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": idStr,
			"error":           err.Error(),
		}).Warn("Invalid notification ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid notification ID",
			"request_id": requestID,
		})
		return
	}

	// Get notification to extract task_ids from data
	notification, err := h.notificationUsecase.GetNotificationByID(userID, uint(notificationID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":      requestID,
			"user_id":         userID,
			"notification_id": notificationID,
			"error":           err.Error(),
		}).Error("Failed to get notification")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get notification"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Notification not found"
		} else if strings.Contains(err.Error(), "access denied") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	// Check if this is a grouped notification
	if notification.Data == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "This notification does not contain grouped tasks",
			"request_id": requestID,
		})
		return
	}

	// Extract task_ids from notification data
	taskIDsInterface, ok := notification.Data["task_ids"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "This notification does not contain task information",
			"request_id": requestID,
		})
		return
	}

	// Convert task_ids to []uint
	var taskIDs []uint
	switch v := taskIDsInterface.(type) {
	case []interface{}:
		for _, id := range v {
			switch idValue := id.(type) {
			case float64:
				taskIDs = append(taskIDs, uint(idValue))
			case uint:
				taskIDs = append(taskIDs, idValue)
			case int:
				taskIDs = append(taskIDs, uint(idValue))
			}
		}
	case []uint:
		taskIDs = v
	default:
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Invalid task_ids format in notification",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"notification_id": notificationID,
		"task_count":      len(taskIDs),
	}).Info("Grouped notification tasks retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"task_ids":   taskIDs,
		"task_count": len(taskIDs),
		"category":   notification.Data["category"],
		"request_id": requestID,
	})
}

// SendTestPush handles sending a test push notification
// POST /api/v1/notifications/test-push
func (h *NotificationHandler) SendTestPush(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req struct {
		Title string                 `json:"title"`
		Body  string                 `json:"body"`
		Data  map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for test push")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set default values
	if req.Title == "" {
		req.Title = "Test Push Notification"
	}
	if req.Body == "" {
		req.Body = "This is a test push notification from Taxion backend"
	}
	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}
	req.Data["test"] = true
	req.Data["timestamp"] = fmt.Sprintf("%d", time.Now().Unix())

	// Create notification request
	notificationReq := &models.CreateNotificationRequest{
		UserID:   userID,
		Type:     models.NotificationTypeSystem,
		Title:    req.Title,
		Message:  req.Body,
		Priority: models.NotificationPriorityHigh,
		Channels: []models.NotificationChannel{
			models.NotificationChannelInApp,
			models.NotificationChannelPush,
		},
		Data: req.Data,
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"title":      req.Title,
		"body":       req.Body,
	}).Info("Sending test push notification")

	// Send notification
	notification, err := h.notificationUsecase.CreateNotification(notificationReq)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to send test push notification")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to send test push notification",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":      requestID,
		"user_id":         userID,
		"notification_id": notification.ID,
	}).Info("Test push notification sent successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":         "Test push notification sent successfully",
		"notification_id": notification.ID,
		"title":           req.Title,
		"body":            req.Body,
		"request_id":      requestID,
	})
}
