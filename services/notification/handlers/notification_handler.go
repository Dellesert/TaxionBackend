// File: services/notification/handlers/notification_handler.go
package handlers

import (
	"net/http"
	"strconv"
	"strings"

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

	logger.WithFields(map[string]interface{}{
		"request_id":        requestID,
		"user_id":           userID,
		"preferences_count": len(preferences),
	}).Info("User preferences retrieved successfully")

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
