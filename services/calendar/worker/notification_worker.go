package worker

import (
	"fmt"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"
)

// NotificationWorker handles background event notifications
type NotificationWorker struct {
	eventRepo          repository.EventRepository
	participantRepo    repository.ParticipantRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
	redisClient        *redis.Client
	log                *logger.Logger
}

// Notification thresholds in minutes
const (
	Threshold24Hours = 1440 // 24 hours
	Threshold1Hour   = 60   // 1 hour
	Threshold15Min   = 15   // 15 minutes
	Threshold5Min    = 5    // 5 minutes
)

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	eventRepo repository.EventRepository,
	participantRepo repository.ParticipantRepository,
	notificationClient *clients.NotificationClient,
	userClient *clients.UserClient,
	redisClient *redis.Client,
) *NotificationWorker {
	return &NotificationWorker{
		eventRepo:          eventRepo,
		participantRepo:    participantRepo,
		notificationClient: notificationClient,
		userClient:         userClient,
		redisClient:        redisClient,
		log: logger.New(&logger.Config{
			Level:  "info",
			Format: "json",
		}),
	}
}

// Start starts the notification worker
func (w *NotificationWorker) Start() {
	w.log.Info("Starting calendar notification worker...")

	// Check upcoming events every hour
	go w.runUpcomingEventsChecker()

	w.log.Info("Calendar notification worker started")
}

// runUpcomingEventsChecker checks for upcoming events and sends reminders
func (w *NotificationWorker) runUpcomingEventsChecker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	w.checkUpcomingEvents()

	for range ticker.C {
		w.checkUpcomingEvents()
	}
}

// checkUpcomingEvents checks for events that need reminders
func (w *NotificationWorker) checkUpcomingEvents() {
	w.log.Info("Checking upcoming events for notifications...")

	now := time.Now()

	// Get events starting in the next 25 hours (to catch 24h reminders)
	// We check 25 hours to account for the hourly check interval + timezone variations
	endTime := now.Add(25 * time.Hour)

	// Get all upcoming events
	events, err := w.getAllUpcomingEvents(now, endTime)
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get upcoming events")
		return
	}

	w.log.WithField("events_count", len(events)).Info("Found upcoming events")

	// Process each event and send appropriate notifications
	stats := &notificationStats{
		totalEvents:     len(events),
		notificationsSent: make(map[int]int),
		errors:          0,
	}

	for _, event := range events {
		w.processEventNotifications(event, now, stats)
	}

	w.log.WithFields(map[string]interface{}{
		"total_events":        stats.totalEvents,
		"notifications_sent":  stats.getTotalSent(),
		"sent_24h":           stats.notificationsSent[Threshold24Hours],
		"sent_1h":            stats.notificationsSent[Threshold1Hour],
		"sent_15m":           stats.notificationsSent[Threshold15Min],
		"sent_5m":            stats.notificationsSent[Threshold5Min],
		"errors":             stats.errors,
	}).Info("Upcoming events check completed")
}

// notificationStats tracks notification sending statistics
type notificationStats struct {
	totalEvents       int
	notificationsSent map[int]int // threshold -> count
	errors            int
}

func (s *notificationStats) getTotalSent() int {
	total := 0
	for _, count := range s.notificationsSent {
		total += count
	}
	return total
}

// getAllUpcomingEvents retrieves all events in the specified time range
func (w *NotificationWorker) getAllUpcomingEvents(start, end time.Time) ([]*models.Event, error) {
	events, err := w.eventRepo.GetEventsInTimeRange(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get events in time range: %w", err)
	}
	return events, nil
}

// processEventNotifications processes notifications for a single event
func (w *NotificationWorker) processEventNotifications(event *models.Event, now time.Time, stats *notificationStats) {
	// Skip past events
	if event.StartTime.Before(now) {
		return
	}

	timeUntilEvent := time.Until(event.StartTime)

	// Get all participants for this event
	participants, err := w.participantRepo.GetEventParticipants(event.ID)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"event_id": event.ID,
			"error":    err.Error(),
		}).Error("Failed to get event participants")
		stats.errors++
		return
	}

	// Determine which notification thresholds we should send
	thresholds := w.determineNotificationThresholds(timeUntilEvent)
	if len(thresholds) == 0 {
		return
	}

	// Send notifications for each threshold to all eligible participants
	for _, threshold := range thresholds {
		for _, participant := range participants {
			// Skip declined participants
			if participant.Status == models.ParticipantStatusDeclined {
				continue
			}

			// Check if we already sent this notification
			if w.hasRecentNotification(event.ID, participant.UserID, threshold) {
				continue
			}

			if err := w.sendUpcomingEventNotification(event, participant.UserID, threshold); err != nil {
				w.log.WithFields(map[string]interface{}{
					"event_id":       event.ID,
					"user_id":        participant.UserID,
					"threshold":      threshold,
					"error":          err.Error(),
				}).Error("Failed to send upcoming event notification")
				stats.errors++
			} else {
				stats.notificationsSent[threshold]++
				// Mark notification as sent
				w.markNotificationSent(event.ID, participant.UserID, threshold)
			}
		}
	}
}

// determineNotificationThresholds determines which notification thresholds should be triggered
func (w *NotificationWorker) determineNotificationThresholds(timeUntil time.Duration) []int {
	var thresholds []int

	// More precise time windows to avoid missing notifications
	// We use 30 minute windows for hourly checks to account for timing variations

	// 5 minutes (window: 4-6 minutes)
	if timeUntil >= 4*time.Minute && timeUntil <= 6*time.Minute {
		thresholds = append(thresholds, Threshold5Min)
	}

	// 15 minutes (window: 14-16 minutes)
	if timeUntil >= 14*time.Minute && timeUntil <= 16*time.Minute {
		thresholds = append(thresholds, Threshold15Min)
	}

	// 1 hour (window: 50-70 minutes)
	if timeUntil >= 50*time.Minute && timeUntil <= 70*time.Minute {
		thresholds = append(thresholds, Threshold1Hour)
	}

	// 24 hours (window: 23.5-24.5 hours)
	if timeUntil >= 23*time.Hour+30*time.Minute && timeUntil <= 24*time.Hour+30*time.Minute {
		thresholds = append(thresholds, Threshold24Hours)
	}

	return thresholds
}

// hasRecentNotification checks if we recently sent a notification for this event/user/threshold
// This prevents duplicate notifications in case of multiple worker runs
func (w *NotificationWorker) hasRecentNotification(eventID, userID uint, threshold int) bool {
	if w.redisClient == nil {
		return false
	}

	key := fmt.Sprintf("calendar:notification_sent:%d:%d:%d", eventID, userID, threshold)

	exists, err := w.redisClient.Exists(key)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"event_id":  eventID,
			"user_id":   userID,
			"threshold": threshold,
			"error":     err.Error(),
		}).Warn("Failed to check notification status in Redis")
		return false
	}

	return exists
}

// markNotificationSent marks a notification as sent in Redis
func (w *NotificationWorker) markNotificationSent(eventID, userID uint, threshold int) {
	if w.redisClient == nil {
		return
	}

	key := fmt.Sprintf("calendar:notification_sent:%d:%d:%d", eventID, userID, threshold)

	// Store for 48 hours to prevent duplicates
	// This is longer than 24h to ensure we don't resend 24h notifications
	err := w.redisClient.Set(key, "1", 48*time.Hour)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"event_id":  eventID,
			"user_id":   userID,
			"threshold": threshold,
			"error":     err.Error(),
		}).Warn("Failed to mark notification as sent in Redis")
	}
}

// sendUpcomingEventNotification sends a notification about an upcoming event
func (w *NotificationWorker) sendUpcomingEventNotification(event *models.Event, userID uint, threshold int) error {
	var priority string
	var channels []string
	var emoji string

	switch threshold {
	case Threshold5Min:
		priority = "critical"
		channels = []string{"in_app", "push"}
		emoji = "🚨"
	case Threshold15Min:
		priority = "high"
		channels = []string{"in_app", "push"}
		emoji = "🔔"
	case Threshold1Hour:
		priority = "medium"
		channels = []string{"in_app", "push"}
		emoji = "⏰"
	case Threshold24Hours:
		priority = "medium"
		channels = []string{"in_app", "email"}
		emoji = "📅"
	default:
		priority = "low"
		channels = []string{"in_app"}
		emoji = "🔔"
	}

	// Calculate actual time until event for more accurate description
	timeUntil := time.Until(event.StartTime)
	actualTimeDesc := w.formatTimeUntil(timeUntil)

	message := fmt.Sprintf("Событие \"%s\" начнется %s", event.Title, actualTimeDesc)
	if event.Location != "" {
		message += fmt.Sprintf(" (место: %s)", event.Location)
	}

	// Add event type context
	var typeEmoji string
	switch event.Type {
	case models.EventTypeMeeting:
		typeEmoji = "👥"
	case models.EventTypeDeadline:
		typeEmoji = "⚠️"
	case models.EventTypePersonal:
		typeEmoji = "📋"
	default:
		typeEmoji = "📅"
	}

	// Create group key to avoid duplicate notifications
	groupKey := fmt.Sprintf("calendar:event_%d:reminder_%dm", event.ID, threshold)

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       fmt.Sprintf("%s %s Напоминание о событии", emoji, typeEmoji),
		Message:     message,
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		GroupKey:    groupKey,
		Data: map[string]interface{}{
			"event_id":       event.ID,
			"event_type":     event.Type,
			"reminder_type":  "event",
			"threshold":      threshold,
			"minutes_before": int(timeUntil.Minutes()),
			"start_time":     event.StartTime,
			"end_time":       event.EndTime,
			"location":       event.Location,
			"all_day":        event.AllDay,
		},
		Channels: channels,
	}

	if err := w.notificationClient.SendNotification(notificationReq); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	w.log.WithFields(map[string]interface{}{
		"event_id":       event.ID,
		"user_id":        userID,
		"threshold":      threshold,
		"time_until":     timeUntil.String(),
	}).Info("Sent upcoming event notification")

	return nil
}

// formatTimeUntil formats duration until event in human-readable format
func (w *NotificationWorker) formatTimeUntil(d time.Duration) string {
	minutes := int(d.Minutes())
	hours := int(d.Hours())

	if minutes < 1 {
		return "прямо сейчас"
	}
	if minutes < 60 {
		if minutes == 1 {
			return "через 1 минуту"
		}
		if minutes < 5 {
			return fmt.Sprintf("через %d минуты", minutes)
		}
		return fmt.Sprintf("через %d минут", minutes)
	}
	if hours == 1 {
		return "через 1 час"
	}
	if hours < 24 {
		if hours < 5 {
			return fmt.Sprintf("через %d часа", hours)
		}
		return fmt.Sprintf("через %d часов", hours)
	}
	days := hours / 24
	if days == 1 {
		return "завтра"
	}
	return fmt.Sprintf("через %d дня", days)
}
