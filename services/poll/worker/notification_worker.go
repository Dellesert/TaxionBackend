// File: services/poll/worker/notification_worker.go
package worker

import (
	"fmt"
	"time"

	"tachyon-messenger/services/poll/clients"
	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/services/poll/repository"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"
)

// NotificationWorker handles background poll notifications
type NotificationWorker struct {
	pollRepo           repository.PollRepository
	participantRepo    repository.PollParticipantRepository
	voteRepo           repository.PollVoteRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
	redisClient        *redis.Client
	log                *logger.Logger
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	pollRepo repository.PollRepository,
	participantRepo repository.PollParticipantRepository,
	voteRepo repository.PollVoteRepository,
	notificationClient *clients.NotificationClient,
	userClient *clients.UserClient,
	redisClient *redis.Client,
) *NotificationWorker {
	return &NotificationWorker{
		pollRepo:           pollRepo,
		participantRepo:    participantRepo,
		voteRepo:           voteRepo,
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
	w.log.Info("Starting poll notification worker...")

	// Check for polls ending soon every 6 hours
	go w.runExpiringPollsChecker()

	w.log.Info("Poll notification worker started")
}

// runExpiringPollsChecker checks for polls that are about to expire
func (w *NotificationWorker) runExpiringPollsChecker() {
	checkInterval := 6 * time.Hour

	w.log.Info("Expiring polls checker goroutine started")

	// Run immediately on start
	w.checkExpiringPolls()

	// Run checks periodically
	for {
		w.log.WithField("next_check_in_hours", checkInterval.Hours()).Info("Sleeping until next check")
		time.Sleep(checkInterval)
		w.log.Info("Woke up, running next check")
		w.checkExpiringPolls()
	}
}

// checkExpiringPolls checks for active polls expiring in 24 hours
func (w *NotificationWorker) checkExpiringPolls() {
	w.log.Info("Checking for expiring polls...")

	now := time.Now()
	twentyFourHoursFromNow := now.Add(24 * time.Hour)

	w.log.WithFields(map[string]interface{}{
		"now":                     now.Format(time.RFC3339),
		"twentyFourHoursFromNow": twentyFourHoursFromNow.Format(time.RFC3339),
	}).Info("Searching for polls expiring in this time range")

	// Get active polls expiring in the next 24 hours
	polls, err := w.pollRepo.GetExpiringPolls(twentyFourHoursFromNow)
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get expiring polls")
		return
	}

	w.log.WithField("poll_count", len(polls)).Info("Found expiring polls")

	notificationsSent := 0

	for _, poll := range polls {
		w.log.WithFields(map[string]interface{}{
			"poll_id":    poll.ID,
			"title":      poll.Title,
			"visibility": poll.Visibility,
			"end_time":   poll.EndTime,
		}).Info("Processing expiring poll")

		// Get participants who haven't voted yet
		var participantIDs []uint

		if poll.Visibility == models.PollVisibilityInviteOnly {
			// For invite-only polls, get invited participants
			participants, err := w.participantRepo.GetByPollID(poll.ID)
			if err != nil {
				w.log.WithFields(map[string]interface{}{
					"poll_id": poll.ID,
					"error":   err.Error(),
				}).Warn("Failed to get poll participants")
				continue
			}

			// Check who hasn't voted
			for _, participant := range participants {
				if participant.VotedAt == nil {
					participantIDs = append(participantIDs, participant.UserID)
				}
			}
		} else if poll.Visibility == models.PollVisibilityDepartment && poll.DepartmentID != nil {
			// For department polls, get all department users
			w.log.WithFields(map[string]interface{}{
				"poll_id":       poll.ID,
				"department_id": *poll.DepartmentID,
			}).Info("Fetching department users for expiring poll")

			departmentUsers, err := w.userClient.GetUsersByDepartment(*poll.DepartmentID)
			if err != nil {
				w.log.WithFields(map[string]interface{}{
					"poll_id":       poll.ID,
					"department_id": *poll.DepartmentID,
					"error":         err.Error(),
				}).Error("Failed to get department users for expiring poll")
				continue
			}

			w.log.WithFields(map[string]interface{}{
				"poll_id":    poll.ID,
				"user_count": len(departmentUsers),
			}).Info("Found department users for expiring poll")

			// Get users who haven't voted yet
			votes, err := w.voteRepo.GetByPollID(poll.ID)
			if err == nil {
				votedUsers := make(map[uint]bool)
				for _, vote := range votes {
					if vote.UserID != nil {
						votedUsers[*vote.UserID] = true
					}
				}

				// Only notify users who haven't voted
				for _, userID := range departmentUsers {
					if !votedUsers[userID] {
						participantIDs = append(participantIDs, userID)
					}
				}
			} else {
				// If can't get votes, notify all department users
				participantIDs = departmentUsers
			}
		}

		// Send notification to users who haven't voted
		if len(participantIDs) > 0 {
			w.log.WithFields(map[string]interface{}{
				"poll_id":    poll.ID,
				"user_count": len(participantIDs),
			}).Info("Sending expiring poll notifications")

			if err := w.sendExpiringPollNotification(poll, participantIDs, now); err != nil {
				w.log.WithFields(map[string]interface{}{
					"poll_id": poll.ID,
					"error":   err.Error(),
				}).Error("Failed to send expiring poll notification")
			} else {
				notificationsSent++
			}
		} else {
			w.log.WithFields(map[string]interface{}{
				"poll_id": poll.ID,
			}).Info("No users to notify for expiring poll (all have voted)")
		}
	}

	w.log.WithField("notifications_sent", notificationsSent).Info("Expiring polls check completed")
}

// sendExpiringPollNotification sends notification about poll expiring soon
func (w *NotificationWorker) sendExpiringPollNotification(poll *models.Poll, participantIDs []uint, now time.Time) error {
	if len(participantIDs) == 0 {
		return nil
	}

	// Calculate time until expiration
	var timeUntilExpiry time.Duration
	if poll.EndTime != nil {
		timeUntilExpiry = poll.EndTime.Sub(now)
	}

	priority := "medium"
	var title, message string

	if timeUntilExpiry <= 3*time.Hour {
		priority = "high"
		title = "⏰ Опрос истекает скоро"
		message = fmt.Sprintf("Опрос \"%s\" истекает через %.1f ч. Успейте проголосовать!", poll.Title, timeUntilExpiry.Hours())
	} else {
		title = "📊 Напоминание об опросе"
		message = fmt.Sprintf("Опрос \"%s\" истекает через %.0f ч. Не забудьте проголосовать", poll.Title, timeUntilExpiry.Hours())
	}

	// Send notification to each participant with duplicate check
	sentCount := 0
	skippedCount := 0
	errorCount := 0

	for _, participantID := range participantIDs {
		// Check if we already sent this notification recently (within 24h)
		if w.hasRecentExpiringNotification(poll.ID, participantID) {
			skippedCount++
			w.log.WithFields(map[string]interface{}{
				"poll_id": poll.ID,
				"user_id": participantID,
			}).Debug("Skipping duplicate expiring poll notification")
			continue
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "reminder",
			Title:       title,
			Message:     message,
			Priority:    &priority,
			RelatedID:   &poll.ID,
			RelatedType: "poll",
			GroupKey:    fmt.Sprintf("poll:expiring:%d", poll.ID),
			Data: map[string]interface{}{
				"poll_id":           poll.ID,
				"notification_type": "poll_expiring",
				"end_time":          poll.EndTime,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := w.notificationClient.SendNotification(notificationReq); err != nil {
			errorCount++
			w.log.WithFields(map[string]interface{}{
				"poll_id": poll.ID,
				"user_id": participantID,
				"error":   err.Error(),
			}).Warn("Failed to send expiring poll notification to user")
		} else {
			sentCount++
			// Mark as sent in Redis (24h TTL to prevent spam)
			w.markExpiringNotificationSent(poll.ID, participantID, 24*time.Hour)
		}
	}

	w.log.WithFields(map[string]interface{}{
		"poll_id":       poll.ID,
		"sent":          sentCount,
		"skipped":       skippedCount,
		"errors":        errorCount,
		"total_targets": len(participantIDs),
	}).Info("Completed expiring poll notifications")

	return nil
}

// hasRecentExpiringNotification checks if we recently sent an expiring notification
func (w *NotificationWorker) hasRecentExpiringNotification(pollID, userID uint) bool {
	if w.redisClient == nil {
		return false
	}

	key := fmt.Sprintf("poll:expiring_notification:%d:%d", pollID, userID)

	exists, err := w.redisClient.Exists(key)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"poll_id": pollID,
			"user_id": userID,
			"error":   err.Error(),
		}).Warn("Failed to check expiring notification status in Redis")
		return false
	}

	return exists
}

// markExpiringNotificationSent marks an expiring notification as sent in Redis
func (w *NotificationWorker) markExpiringNotificationSent(pollID, userID uint, ttl time.Duration) {
	if w.redisClient == nil {
		return
	}

	key := fmt.Sprintf("poll:expiring_notification:%d:%d", pollID, userID)

	err := w.redisClient.Set(key, "1", ttl)
	if err != nil {
		w.log.WithFields(map[string]interface{}{
			"poll_id": pollID,
			"user_id": userID,
			"ttl":     ttl,
			"error":   err.Error(),
		}).Warn("Failed to mark expiring notification as sent in Redis")
	}
}
