package usecase

import (
	"fmt"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
)

// sendTaskCancelledNotification sends notification when task is cancelled
func (u *taskUsecase) sendTaskCancelledNotification(task *models.Task, userID uint) {
	// Get canceller info
	cancellerInfo, err := u.userClient.GetUserByID(userID)
	cancellerName := "Кто-то"
	if err == nil && cancellerInfo != nil {
		cancellerName = cancellerInfo.Name
	}

	// Notify creator if different from canceller
	if task.CreatedByUserID != userID {
		priority := string(task.Priority)
		notificationReq := &clients.NotificationRequest{
			UserID:      task.CreatedByUserID,
			Type:        "task",
			Title:       "❌ Задача отменена",
			Message:     fmt.Sprintf("%s отменил(а) задачу: %s", cancellerName, task.Title),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":   task.ID,
				"sender_id": userID,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send task cancelled notification to creator %d: %v\n", task.CreatedByUserID, err)
		}
	}

	// Notify all assignees (except canceller)
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			continue
		}

		priority := string(task.Priority)
		notificationReq := &clients.NotificationRequest{
			UserID:      assignee.UserID,
			Type:        "task",
			Title:       "❌ Задача отменена",
			Message:     fmt.Sprintf("%s отменил(а) задачу: %s", cancellerName, task.Title),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":   task.ID,
				"sender_id": userID,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send task cancelled notification to assignee %d: %v\n", assignee.UserID, err)
		}
	}
}

// sendTaskCompletedNotification sends notification when task is completed
func (u *taskUsecase) sendTaskCompletedNotification(task *models.Task, userID uint) {
	// Get completer info
	completerInfo, err := u.userClient.GetUserByID(userID)
	completerName := "Кто-то"
	if err == nil && completerInfo != nil {
		completerName = completerInfo.Name
	}

	// Determine who should receive notification
	var recipientID uint
	if task.DelegatedFromUserID != nil {
		recipientID = *task.DelegatedFromUserID
	} else if task.CreatedByUserID != userID {
		recipientID = task.CreatedByUserID
	} else {
		return // Completer is the creator, no notification needed
	}

	priority := string(task.Priority)
	notificationReq := &clients.NotificationRequest{
		UserID:      recipientID,
		Type:        "task",
		Title:       "✅ Задача выполнена",
		Message:     fmt.Sprintf("%s завершил(а) задачу: %s", completerName, task.Title),
		Priority:    &priority,
		RelatedID:   &task.ID,
		RelatedType: "task",
		Data: map[string]interface{}{
			"task_id":   task.ID,
			"sender_id": userID,
		},
		Channels: []string{"in_app", "email", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send task completed notification to user %d: %v\n", recipientID, err)
	}
}

// sendSubtaskCompletedNotification sends notification when subtask is completed
func (u *taskUsecase) sendSubtaskCompletedNotification(task *models.Task, parentTaskID uint, userID uint) {
	// Get completer info
	completerInfo, err := u.userClient.GetUserByID(userID)
	completerName := "Кто-то"
	if err == nil && completerInfo != nil {
		completerName = completerInfo.Name
	}

	// Get parent task
	parentTask, err := u.taskRepo.GetByID(parentTaskID)
	if err != nil {
		fmt.Printf("Failed to get parent task %d for subtask completion notification: %v\n", parentTaskID, err)
		return
	}

	// Notify parent task creator (if different from subtask completer)
	if parentTask.CreatedByUserID != userID {
		priority := string(task.Priority)
		notificationReq := &clients.NotificationRequest{
			UserID:      parentTask.CreatedByUserID,
			Type:        "task",
			Title:       "✅ Подзадача выполнена",
			Message:     fmt.Sprintf("%s завершил(а) подзадачу \"%s\" в задаче \"%s\"", completerName, task.Title, parentTask.Title),
			Priority:    &priority,
			RelatedID:   &parentTaskID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":    parentTaskID,
				"subtask_id": task.ID,
				"sender_id":  userID,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send subtask completed notification to user %d: %v\n", parentTask.CreatedByUserID, err)
		}
	}
}

// sendPriorityChangedNotification sends notification when task priority changes
func (u *taskUsecase) sendPriorityChangedNotification(task *models.Task, oldPriority, newPriority models.TaskPriority, userID uint) {
	// Get changer info
	changerInfo, err := u.userClient.GetUserByID(userID)
	changerName := "Кто-то"
	if err == nil && changerInfo != nil {
		changerName = changerInfo.Name
	}

	// Map priority to Russian names
	priorityNames := map[models.TaskPriority]string{
		models.TaskPriorityLow:      "низкий",
		models.TaskPriorityMedium:   "средний",
		models.TaskPriorityHigh:     "высокий",
		models.TaskPriorityCritical: "критический",
	}

	oldPriorityName := priorityNames[oldPriority]
	newPriorityName := priorityNames[newPriority]

	priority := string(newPriority)

	// Notify all assignees (except changer)
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			continue
		}

		var emoji string
		if newPriority == models.TaskPriorityCritical {
			emoji = "🔴"
		} else if newPriority == models.TaskPriorityHigh {
			emoji = "🟠"
		} else {
			emoji = "📊"
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      assignee.UserID,
			Type:        "task",
			Title:       fmt.Sprintf("%s Изменён приоритет задачи", emoji),
			Message:     fmt.Sprintf("%s изменил приоритет задачи \"%s\" с \"%s\" на \"%s\"", changerName, task.Title, oldPriorityName, newPriorityName),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":      task.ID,
				"old_priority": oldPriority,
				"new_priority": newPriority,
				"sender_id":    userID,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send priority changed notification to user %d: %v\n", assignee.UserID, err)
		}
	}
}

// SendAttachmentAddedNotification sends notification when attachment is added (exported for use in handlers)
func (u *taskUsecase) SendAttachmentAddedNotification(taskID uint, userID uint, fileName string) {
	u.sendAttachmentAddedNotification(taskID, userID, fileName)
}

// sendAttachmentAddedNotification sends notification when attachment is added
func (u *taskUsecase) sendAttachmentAddedNotification(taskID uint, userID uint, fileName string) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		fmt.Printf("Failed to get task %d for attachment notification: %v\n", taskID, err)
		return
	}

	// Get uploader info
	uploaderInfo, err := u.userClient.GetUserByID(userID)
	uploaderName := "Кто-то"
	if err == nil && uploaderInfo != nil {
		uploaderName = uploaderInfo.Name
	}

	priority := "low"

	// Notify creator if different from uploader
	if task.CreatedByUserID != userID {
		notificationReq := &clients.NotificationRequest{
			UserID:      task.CreatedByUserID,
			Type:        "task",
			Title:       "📎 Добавлено вложение",
			Message:     fmt.Sprintf("%s добавил(а) вложение \"%s\" к задаче \"%s\"", uploaderName, fileName, task.Title),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":   task.ID,
				"file_name": fileName,
				"sender_id": userID,
			},
			Channels: []string{"in_app"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send attachment notification to creator %d: %v\n", task.CreatedByUserID, err)
		}
	}

	// Notify assignees (except uploader)
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			continue
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      assignee.UserID,
			Type:        "task",
			Title:       "📎 Добавлено вложение",
			Message:     fmt.Sprintf("%s добавил(а) вложение к задаче \"%s\"", uploaderName, task.Title),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":   task.ID,
				"file_name": fileName,
				"sender_id": userID,
			},
			Channels: []string{"in_app"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send attachment notification to assignee %d: %v\n", assignee.UserID, err)
		}
	}
}

// sendAdminModifiedNotification sends notification when admin/super_admin modifies someone else's task
func (u *taskUsecase) sendAdminModifiedNotification(task *models.Task, modifierID uint, modifierRole string, changeDescription string) {
	// Only send if modifier is admin/super_admin and is not the creator
	if task.CreatedByUserID == modifierID {
		return // No notification needed if admin is modifying their own task
	}

	// Get modifier info
	modifierInfo, err := u.userClient.GetUserByID(modifierID)
	modifierName := "Администратор"
	if err == nil && modifierInfo != nil {
		modifierName = modifierInfo.Name
	}

	priority := "high"

	// Notify creator
	notificationReq := &clients.NotificationRequest{
		UserID:      task.CreatedByUserID,
		Type:        "system",
		Title:       "⚙️ Администратор изменил задачу",
		Message:     fmt.Sprintf("%s изменил(а) вашу задачу \"%s\": %s", modifierName, task.Title, changeDescription),
		Priority:    &priority,
		RelatedID:   &task.ID,
		RelatedType: "task",
		Data: map[string]interface{}{
			"task_id":      task.ID,
			"modifier_id":  modifierID,
			"modifier_role": modifierRole,
			"change":       changeDescription,
		},
		Channels: []string{"in_app", "email"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send admin modification notification to creator %d: %v\n", task.CreatedByUserID, err)
	}

	// Also notify all assignees (except modifier)
	for _, assignee := range task.Assignees {
		if assignee.UserID == modifierID {
			continue
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      assignee.UserID,
			Type:        "system",
			Title:       "⚙️ Администратор изменил задачу",
			Message:     fmt.Sprintf("%s изменил(а) задачу \"%s\": %s", modifierName, task.Title, changeDescription),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":      task.ID,
				"modifier_id":  modifierID,
				"modifier_role": modifierRole,
				"change":       changeDescription,
			},
			Channels: []string{"in_app"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send admin modification notification to assignee %d: %v\n", assignee.UserID, err)
		}
	}
}
