package worker

import (
	"fmt"
	"time"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
	"tachyon-messenger/shared/logger"
)

// NotificationWorker handles background task notifications
type NotificationWorker struct {
	taskRepo           repository.TaskRepository
	notificationClient *clients.NotificationClient
	userClient         *clients.UserClient
	log                *logger.Logger
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	taskRepo repository.TaskRepository,
	notificationClient *clients.NotificationClient,
	userClient *clients.UserClient,
) *NotificationWorker {
	return &NotificationWorker{
		taskRepo:           taskRepo,
		notificationClient: notificationClient,
		userClient:         userClient,
		log: logger.New(&logger.Config{
			Level:  "info",
			Format: "json",
		}),
	}
}

// Start starts the notification worker
func (w *NotificationWorker) Start() {
	w.log.Info("Starting task notification worker...")

	// Check deadlines every hour
	go w.runDeadlineChecker()

	// Check for stale tasks every 6 hours
	go w.runStaleTaskChecker()

	w.log.Info("Task notification worker started")
}

// runDeadlineChecker checks for upcoming and overdue deadlines
func (w *NotificationWorker) runDeadlineChecker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	w.checkDeadlines()

	for range ticker.C {
		w.checkDeadlines()
	}
}

// checkDeadlines checks for tasks with approaching or passed deadlines
func (w *NotificationWorker) checkDeadlines() {
	w.log.Info("Checking task deadlines...")

	now := time.Now()

	// Get all tasks with due dates that are not completed or cancelled
	tasks, err := w.taskRepo.GetTasksWithUpcomingDeadlines()
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get tasks with deadlines")
		return
	}

	notificationsSent := 0
	for _, task := range tasks {
		// Skip if task is done or cancelled
		if task.Status == models.TaskStatusDone || task.Status == models.TaskStatusCancelled {
			continue
		}

		if task.DueDate == nil {
			continue
		}

		timeUntilDue := task.DueDate.Sub(now)

		// Check if we should send notification based on time remaining
		if w.shouldSendDeadlineNotification(task, timeUntilDue) {
			if err := w.sendDeadlineNotification(task, timeUntilDue); err != nil {
				w.log.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"error":   err.Error(),
				}).Error("Failed to send deadline notification")
			} else {
				notificationsSent++
			}
		}
	}

	w.log.WithField("notifications_sent", notificationsSent).Info("Deadline check completed")
}

// shouldSendDeadlineNotification determines if a notification should be sent
func (w *NotificationWorker) shouldSendDeadlineNotification(task *models.Task, timeUntilDue time.Duration) bool {
	now := time.Now()

	// Check if we already sent a notification for this deadline stage
	lastNotification := task.LastDeadlineNotificationSentAt

	// Overdue (past due date)
	if timeUntilDue < 0 {
		// Send daily reminders for overdue tasks
		if lastNotification == nil || now.Sub(*lastNotification) > 24*time.Hour {
			return true
		}
	}

	// Due in less than 3 hours
	if timeUntilDue > 0 && timeUntilDue <= 3*time.Hour {
		// Only send if we haven't sent a 3-hour notification yet
		if lastNotification == nil || now.Sub(*lastNotification) > 3*time.Hour {
			return true
		}
	}

	// Due in less than 24 hours (but more than 3 hours)
	if timeUntilDue > 3*time.Hour && timeUntilDue <= 24*time.Hour {
		// Only send if we haven't sent a 24-hour notification yet
		if lastNotification == nil || now.Sub(*lastNotification) > 24*time.Hour {
			return true
		}
	}

	return false
}

// sendDeadlineNotification sends a deadline notification to assignees
func (w *NotificationWorker) sendDeadlineNotification(task *models.Task, timeUntilDue time.Duration) error {
	// Get all assignees
	if len(task.Assignees) == 0 {
		return nil // No one to notify
	}

	var title, message, emoji string
	var priority string = "high"

	if timeUntilDue < 0 {
		// Overdue
		emoji = "⏰"
		title = "Задача просрочена"
		overdueDuration := -timeUntilDue
		if overdueDuration < 24*time.Hour {
			message = fmt.Sprintf("Задача \"%s\" просрочена на %d ч.", task.Title, int(overdueDuration.Hours()))
		} else {
			message = fmt.Sprintf("Задача \"%s\" просрочена на %d дн.", task.Title, int(overdueDuration.Hours()/24))
		}
		priority = "critical"
	} else if timeUntilDue <= 3*time.Hour {
		// Due in 3 hours
		emoji = "⚠️"
		title = "Задача истекает через 3 часа"
		message = fmt.Sprintf("Срок выполнения задачи \"%s\" истекает через %.1f ч.", task.Title, timeUntilDue.Hours())
		priority = "high"
	} else if timeUntilDue <= 24*time.Hour {
		// Due in 24 hours
		emoji = "📅"
		title = "Задача истекает завтра"
		message = fmt.Sprintf("Срок выполнения задачи \"%s\" истекает через %.1f ч.", task.Title, timeUntilDue.Hours())
		priority = "medium"
	}

	// Send notification to each assignee
	for _, assignee := range task.Assignees {
		notificationReq := &clients.NotificationRequest{
			UserID:      assignee.UserID,
			Type:        "task",
			Title:       fmt.Sprintf("%s %s", emoji, title),
			Message:     message,
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":       task.ID,
				"due_date":      task.DueDate,
				"time_until":    timeUntilDue.String(),
				"is_overdue":    timeUntilDue < 0,
				"creator_id":    task.CreatedByUserID,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := w.notificationClient.SendNotification(notificationReq); err != nil {
			w.log.WithFields(map[string]interface{}{
				"task_id": task.ID,
				"user_id": assignee.UserID,
				"error":   err.Error(),
			}).Error("Failed to send deadline notification")
			continue
		}
	}

	// Update last notification timestamp
	now := time.Now()
	task.LastDeadlineNotificationSentAt = &now
	if err := w.taskRepo.Update(task); err != nil {
		w.log.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		}).Warn("Failed to update last notification timestamp")
	}

	return nil
}

// runStaleTaskChecker checks for stale tasks (unviewed or no progress)
func (w *NotificationWorker) runStaleTaskChecker() {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	// Run immediately on start
	w.checkStaleTasks()

	for range ticker.C {
		w.checkStaleTasks()
	}
}

// checkStaleTasks checks for tasks that need reminders
func (w *NotificationWorker) checkStaleTasks() {
	w.log.Info("Checking for stale tasks...")

	now := time.Now()

	// Check for unviewed tasks older than 24 hours
	w.checkUnviewedTasks(now)

	// Check for tasks in progress without progress updates
	w.checkTasksWithoutProgress(now)

	w.log.Info("Stale task check completed")
}

// checkUnviewedTasks checks for tasks that haven't been viewed
func (w *NotificationWorker) checkUnviewedTasks(now time.Time) {
	// Get tasks that are in "new" status and created more than 24 hours ago
	cutoffTime := now.Add(-24 * time.Hour)
	tasks, err := w.taskRepo.GetUnviewedTasksOlderThan(cutoffTime)
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get unviewed tasks")
		return
	}

	for _, task := range tasks {
		// Send notification to assignees
		if len(task.Assignees) == 0 {
			continue
		}

		priority := string(task.Priority)
		for _, assignee := range task.Assignees {
			notificationReq := &clients.NotificationRequest{
				UserID:      assignee.UserID,
				Type:        "reminder",
				Title:       "📋 Напоминание о непросмотренной задаче",
				Message:     fmt.Sprintf("У вас есть непросмотренная задача: %s", task.Title),
				Priority:    &priority,
				RelatedID:   &task.ID,
				RelatedType: "task",
				Data: map[string]interface{}{
					"task_id":    task.ID,
					"created_at": task.CreatedAt,
					"creator_id": task.CreatedByUserID,
				},
				Channels: []string{"in_app", "push"},
			}

			if err := w.notificationClient.SendNotification(notificationReq); err != nil {
				w.log.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"user_id": assignee.UserID,
					"error":   err.Error(),
				}).Error("Failed to send unviewed task notification")
			}
		}
	}
}

// checkTasksWithoutProgress checks for tasks in progress without updates
func (w *NotificationWorker) checkTasksWithoutProgress(now time.Time) {
	// Get tasks in progress that haven't been updated in 3 days
	cutoffTime := now.Add(-72 * time.Hour)
	tasks, err := w.taskRepo.GetInProgressTasksWithoutUpdates(cutoffTime)
	if err != nil {
		w.log.WithField("error", err.Error()).Error("Failed to get stale in-progress tasks")
		return
	}

	for _, task := range tasks {
		// Send notification to assignees
		if len(task.Assignees) == 0 {
			continue
		}

		priority := string(task.Priority)
		for _, assignee := range task.Assignees {
			notificationReq := &clients.NotificationRequest{
				UserID:      assignee.UserID,
				Type:        "reminder",
				Title:       "⏸️ Напоминание о задаче без прогресса",
				Message:     fmt.Sprintf("Задача \"%s\" долгое время без обновлений", task.Title),
				Priority:    &priority,
				RelatedID:   &task.ID,
				RelatedType: "task",
				Data: map[string]interface{}{
					"task_id":    task.ID,
					"updated_at": task.UpdatedAt,
					"creator_id": task.CreatedByUserID,
				},
				Channels: []string{"in_app"},
			}

			if err := w.notificationClient.SendNotification(notificationReq); err != nil {
				w.log.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"user_id": assignee.UserID,
					"error":   err.Error(),
				}).Error("Failed to send stale task notification")
			}
		}
	}
}
