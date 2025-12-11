package worker

import (
	"fmt"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	"tachyon-messenger/shared/logger"
)

// NotificationWorker handles background event notifications
type NotificationWorker struct {
	eventRepo          repository.EventRepository
	participantRepo    repository.ParticipantRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
	log                *logger.Logger
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	eventRepo repository.EventRepository,
	participantRepo repository.ParticipantRepository,
	notificationClient *clients.NotificationClient,
	userClient *clients.UserClient,
) *NotificationWorker {
	return &NotificationWorker{
		eventRepo:          eventRepo,
		participantRepo:    participantRepo,
		log: logger.New(&logger.Config{
			Level:  "info",
			Format: "json",
		}),
		notificationClient: notificationClient,
		userClient:         userClient,
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
	// We check 25 hours to account for the hourly check interval
	endTime := now.Add(25 * time.Hour)

	// Get all upcoming events (we'll filter by user later)
	// For simplicity, we'll get events without specific user filter
	// and process participants individually
	events, err := w.getAllUpcomingEvents(now, endTime)
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get upcoming events")
		return
	}

	w.log.WithField("events_count", len(events)).Info("Found upcoming events")

	// Process each event and send appropriate notifications
	notificationsSent := 0
	for _, event := range events {
		sent := w.processEventNotifications(event, now)
		notificationsSent += sent
	}

	w.log.WithField("notifications_sent", notificationsSent).Info("Upcoming events check completed")
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
func (w *NotificationWorker) processEventNotifications(event *models.Event, now time.Time) int {
	// Skip past events
	if event.StartTime.Before(now) {
		return 0
	}

	timeUntilEvent := time.Until(event.StartTime)

	// Get all participants for this event
	participants, err := w.participantRepo.GetEventParticipants(event.ID)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"event_id": event.ID,
			"error":    err.Error(),
		}).Error("Failed to get event participants")
		return 0
	}

	notificationsSent := 0

	// Determine which notification threshold we're in
	var minutesBefore int
	var shouldSend bool

	// 24 hours notification (1440 minutes)
	if timeUntilEvent > 23*time.Hour && timeUntilEvent <= 24*time.Hour {
		minutesBefore = 1440
		shouldSend = true
	}
	// 1 hour notification (60 minutes)
	if timeUntilEvent > 59*time.Minute && timeUntilEvent <= 1*time.Hour {
		minutesBefore = 60
		shouldSend = true
	}
	// 15 minutes notification
	if timeUntilEvent > 14*time.Minute && timeUntilEvent <= 15*time.Minute {
		minutesBefore = 15
		shouldSend = true
	}

	if !shouldSend {
		return 0
	}

	// Send notification to all accepted/pending participants
	for _, participant := range participants {
		// Skip declined participants
		if participant.Status == models.ParticipantStatusDeclined {
			continue
		}

		// Check if we already sent this notification
		if w.hasRecentNotification(event.ID, participant.UserID, minutesBefore) {
			continue
		}

		if err := w.sendUpcomingEventNotification(event, participant.UserID, minutesBefore); err != nil {
			w.log.WithFields(map[string]interface{}{
				"event_id": event.ID,
				"user_id":  participant.UserID,
				"error":    err.Error(),
			}).Error("Failed to send upcoming event notification")
		} else {
			notificationsSent++
		}
	}

	return notificationsSent
}

// hasRecentNotification checks if we recently sent a notification for this event/user/threshold
// This prevents duplicate notifications in case of multiple worker runs
func (w *NotificationWorker) hasRecentNotification(eventID, userID uint, minutesBefore int) bool {
	// For now, we'll send notifications - in production you'd want to track this
	// You could use Redis or a database table to track sent notifications
	return false
}

// sendUpcomingEventNotification sends a notification about an upcoming event
func (w *NotificationWorker) sendUpcomingEventNotification(event *models.Event, userID uint, minutesBefore int) error {
	var priority string
	var timeDesc string
	var channels []string
	var emoji string

	switch minutesBefore {
	case 15:
		priority = "high"
		timeDesc = "через 15 минут"
		channels = []string{"in_app", "push"}
		emoji = "🔔"
	case 60:
		priority = "medium"
		timeDesc = "через 1 час"
		channels = []string{"in_app", "push"}
		emoji = "⏰"
	case 1440:
		priority = "medium"
		timeDesc = "завтра"
		channels = []string{"in_app", "email"}
		emoji = "📅"
	default:
		priority = "low"
		timeDesc = fmt.Sprintf("через %d минут", minutesBefore)
		channels = []string{"in_app"}
		emoji = "🔔"
	}

	message := fmt.Sprintf("Событие \"%s\" начнется %s", event.Title, timeDesc)
	if event.Location != "" {
		message += fmt.Sprintf(" (место: %s)", event.Location)
	}

	// Create group key to avoid duplicate notifications
	groupKey := fmt.Sprintf("calendar:event_%d:reminder_%dm", event.ID, minutesBefore)

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       fmt.Sprintf("%s Напоминание о событии", emoji),
		Message:     message,
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		GroupKey:    groupKey,
		Data: map[string]interface{}{
			"event_id":       event.ID,
			"reminder_type":  "event",
			"minutes_before": minutesBefore,
			"start_time":     event.StartTime,
			"location":       event.Location,
		},
		Channels: channels,
	}

	if err := w.notificationClient.SendNotification(notificationReq); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	w.log.WithFields(map[string]interface{}{
		"event_id":       event.ID,
		"user_id":        userID,
		"minutes_before": minutesBefore,
	}).Info("Sent upcoming event notification")

	return nil
}
