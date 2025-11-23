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

	// Group tasks by user and deadline category
	type deadlineCategory string
	const (
		categoryOverdue   deadlineCategory = "overdue"
		category3Hours    deadlineCategory = "3hours"
		category24Hours   deadlineCategory = "24hours"
	)

	// Map: userID -> category -> tasks
	tasksByUserAndCategory := make(map[uint]map[deadlineCategory][]*models.Task)

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
		if !w.shouldSendDeadlineNotification(task, timeUntilDue) {
			continue
		}

		// Determine category
		var category deadlineCategory
		if timeUntilDue < 0 {
			category = categoryOverdue
		} else if timeUntilDue <= 3*time.Hour {
			category = category3Hours
		} else if timeUntilDue <= 24*time.Hour {
			category = category24Hours
		} else {
			continue
		}

		// Add task to each assignee's list
		for _, assignee := range task.Assignees {
			if tasksByUserAndCategory[assignee.UserID] == nil {
				tasksByUserAndCategory[assignee.UserID] = make(map[deadlineCategory][]*models.Task)
			}
			tasksByUserAndCategory[assignee.UserID][category] = append(tasksByUserAndCategory[assignee.UserID][category], task)
		}
	}

	// Send grouped notifications
	notificationsSent := 0
	for userID, categoriesMap := range tasksByUserAndCategory {
		for category, userTasks := range categoriesMap {
			if err := w.sendGroupedDeadlineNotification(userID, string(category), userTasks, now); err != nil {
				w.log.WithFields(map[string]interface{}{
					"user_id":  userID,
					"category": category,
					"error":    err.Error(),
				}).Error("Failed to send grouped deadline notification")
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

// sendGroupedDeadlineNotification sends a grouped deadline notification for multiple tasks
func (w *NotificationWorker) sendGroupedDeadlineNotification(userID uint, category string, tasks []*models.Task, now time.Time) error {
	if len(tasks) == 0 {
		return nil
	}

	var title, message, emoji string
	var priority string = "high"
	var groupKey string

	// Collect task IDs for the data field
	taskIDs := make([]uint, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	// Format date for group key
	dateKey := now.Format("2006-01-02")

	switch category {
	case "overdue":
		emoji = "⏰"
		priority = "critical"
		groupKey = fmt.Sprintf("task_deadline_overdue_%s", dateKey)
		if len(tasks) == 1 {
			title = "Задача просрочена"
			overdueDuration := -tasks[0].DueDate.Sub(now)
			if overdueDuration < 24*time.Hour {
				message = fmt.Sprintf("Задача \"%s\" просрочена на %d ч.", tasks[0].Title, int(overdueDuration.Hours()))
			} else {
				message = fmt.Sprintf("Задача \"%s\" просрочена на %d дн.", tasks[0].Title, int(overdueDuration.Hours()/24))
			}
		} else {
			title = "Просроченные задачи"
			message = fmt.Sprintf("У вас %d просроченных задач", len(tasks))
		}

	case "3hours":
		emoji = "⚠️"
		priority = "high"
		groupKey = fmt.Sprintf("task_deadline_3h_%s", dateKey)
		if len(tasks) == 1 {
			title = "Задача истекает через 3 часа"
			message = fmt.Sprintf("Срок выполнения задачи \"%s\" истекает через %.1f ч.", tasks[0].Title, tasks[0].DueDate.Sub(now).Hours())
		} else {
			title = "Задачи истекают скоро"
			message = fmt.Sprintf("У вас %d задач, срок которых истекает в ближайшие 3 часа", len(tasks))
		}

	case "24hours":
		emoji = "📅"
		priority = "medium"
		groupKey = fmt.Sprintf("task_deadline_24h_%s", dateKey)
		if len(tasks) == 1 {
			title = "Задача истекает завтра"
			message = fmt.Sprintf("Срок выполнения задачи \"%s\" истекает через %.1f ч.", tasks[0].Title, tasks[0].DueDate.Sub(now).Hours())
		} else {
			title = "Задачи истекают завтра"
			message = fmt.Sprintf("У вас %d задач, срок которых истекает в ближайшие 24 часа", len(tasks))
		}
	}

	// Prepare notification request
	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       fmt.Sprintf("%s %s", emoji, title),
		Message:     message,
		Priority:    &priority,
		GroupKey:    groupKey,
		TaskCount:   len(tasks),
		RelatedType: "task_group",
		Data: map[string]interface{}{
			"task_ids":      taskIDs,
			"category":      category,
			"task_count":    len(tasks),
			"notification_type": "deadline_reminder",
		},
		Channels: []string{"in_app", "email", "push"},
	}

	// Send notification
	if err := w.notificationClient.SendNotification(notificationReq); err != nil {
		return fmt.Errorf("failed to send grouped notification: %w", err)
	}

	// Update last notification timestamp for all tasks
	for _, task := range tasks {
		task.LastDeadlineNotificationSentAt = &now
		if err := w.taskRepo.Update(task); err != nil {
			w.log.WithFields(map[string]interface{}{
				"task_id": task.ID,
				"error":   err.Error(),
			}).Warn("Failed to update last notification timestamp")
		}
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

	// Group tasks by user
	tasksByUser := make(map[uint][]*models.Task)
	for _, task := range tasks {
		if len(task.Assignees) == 0 {
			continue
		}
		for _, assignee := range task.Assignees {
			tasksByUser[assignee.UserID] = append(tasksByUser[assignee.UserID], task)
		}
	}

	// Send grouped notifications
	for userID, userTasks := range tasksByUser {
		if err := w.sendGroupedUnviewedNotification(userID, userTasks, now); err != nil {
			w.log.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			}).Error("Failed to send grouped unviewed tasks notification")
		}
	}
}

// sendGroupedUnviewedNotification sends a grouped notification for unviewed tasks
func (w *NotificationWorker) sendGroupedUnviewedNotification(userID uint, tasks []*models.Task, now time.Time) error {
	if len(tasks) == 0 {
		return nil
	}

	// Collect task IDs
	taskIDs := make([]uint, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	var title, message string
	priority := "medium"
	dateKey := now.Format("2006-01-02")
	groupKey := fmt.Sprintf("task_unviewed_%s", dateKey)

	if len(tasks) == 1 {
		title = "📋 Напоминание о непросмотренной задаче"
		message = fmt.Sprintf("У вас есть непросмотренная задача: %s", tasks[0].Title)
	} else {
		title = "📋 Напоминание о непросмотренных задачах"
		message = fmt.Sprintf("У вас %d непросмотренных задач", len(tasks))
	}

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       title,
		Message:     message,
		Priority:    &priority,
		GroupKey:    groupKey,
		TaskCount:   len(tasks),
		RelatedType: "task_group",
		Data: map[string]interface{}{
			"task_ids":          taskIDs,
			"task_count":        len(tasks),
			"notification_type": "unviewed_reminder",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := w.notificationClient.SendNotification(notificationReq); err != nil {
		return fmt.Errorf("failed to send grouped unviewed notification: %w", err)
	}

	return nil
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

	// Group tasks by user
	tasksByUser := make(map[uint][]*models.Task)
	for _, task := range tasks {
		if len(task.Assignees) == 0 {
			continue
		}
		for _, assignee := range task.Assignees {
			tasksByUser[assignee.UserID] = append(tasksByUser[assignee.UserID], task)
		}
	}

	// Send grouped notifications
	for userID, userTasks := range tasksByUser {
		if err := w.sendGroupedStaleTaskNotification(userID, userTasks, now); err != nil {
			w.log.WithFields(map[string]interface{}{
				"user_id": userID,
				"error":   err.Error(),
			}).Error("Failed to send grouped stale tasks notification")
		}
	}
}

// sendGroupedStaleTaskNotification sends a grouped notification for stale tasks
func (w *NotificationWorker) sendGroupedStaleTaskNotification(userID uint, tasks []*models.Task, now time.Time) error {
	if len(tasks) == 0 {
		return nil
	}

	// Collect task IDs
	taskIDs := make([]uint, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	var title, message string
	priority := "medium"
	dateKey := now.Format("2006-01-02")
	groupKey := fmt.Sprintf("task_stale_%s", dateKey)

	if len(tasks) == 1 {
		title = "⏸️ Напоминание о задаче без прогресса"
		message = fmt.Sprintf("Задача \"%s\" долгое время без обновлений", tasks[0].Title)
	} else {
		title = "⏸️ Напоминание о задачах без прогресса"
		message = fmt.Sprintf("У вас %d задач в работе без обновлений более 3 дней", len(tasks))
	}

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       title,
		Message:     message,
		Priority:    &priority,
		GroupKey:    groupKey,
		TaskCount:   len(tasks),
		RelatedType: "task_group",
		Data: map[string]interface{}{
			"task_ids":          taskIDs,
			"task_count":        len(tasks),
			"notification_type": "stale_task_reminder",
		},
		Channels: []string{"in_app"},
	}

	if err := w.notificationClient.SendNotification(notificationReq); err != nil {
		return fmt.Errorf("failed to send grouped stale task notification: %w", err)
	}

	return nil
}
