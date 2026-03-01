package usecase

import (
	"fmt"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/logger"
)

// sendEventCreatedNotification sends notification when event is created to invited participants
func (u *calendarUsecase) sendEventCreatedNotification(event *models.Event, creatorID uint, participantIDs []uint) {
	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	// Format start time
	startTimeStr := event.StartTime.Format("02.01.2006 в 15:04")
	if event.AllDay {
		startTimeStr = event.StartTime.Format("02.01.2006")
	}

	// Send to each participant (except creator)
	for _, participantID := range participantIDs {
		if participantID == creatorID {
			continue
		}

		message := fmt.Sprintf("%s пригласил(а) вас на событие: %s (%s)", creatorName, event.Title, startTimeStr)
		if event.Location != "" {
			message += fmt.Sprintf(", место: %s", event.Location)
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "calendar",
			Title:       "📅 Приглашение на событие",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &event.ID,
			RelatedType: "event",
			Data: map[string]interface{}{
				"event_id":   event.ID,
				"event_type": event.Type,
				"start_time": event.StartTime,
				"end_time":   event.EndTime,
				"location":   event.Location,
				"sender_id":  creatorID,
				"all_day":    event.AllDay,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send event created notification to user %d: %v\n", participantID, err)
		}
	}
}

// sendEventUpdatedNotification sends notification when event is updated
func (u *calendarUsecase) sendEventUpdatedNotification(event *models.Event, updaterID uint, timeChanged bool) {
	// Get updater info
	updaterInfo, err := u.userClient.GetUserByID(updaterID)
	updaterName := "Кто-то"
	if err == nil && updaterInfo != nil {
		updaterName = updaterInfo.Name
	}

	// Determine priority based on what changed
	priority := "medium"
	channels := []string{"in_app", "push"}
	if timeChanged {
		priority = "high"
		channels = []string{"in_app", "email", "push"}
	}

	// Get all participants
	participants, err := u.participantRepo.GetEventParticipants(event.ID)
	if err != nil {
		fmt.Printf("Failed to get participants for event update notification: %v\n", err)
		return
	}

	message := fmt.Sprintf("%s изменил(а) детали события: %s", updaterName, event.Title)
	if timeChanged {
		startTimeStr := event.StartTime.Format("02.01.2006 в 15:04")
		message = fmt.Sprintf("%s изменил(а) время события \"%s\" на %s", updaterName, event.Title, startTimeStr)
	}

	// Notify all participants (except updater)
	for _, participant := range participants {
		if participant.UserID == updaterID {
			continue
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participant.UserID,
			Type:        "calendar",
			Title:       "🔄 Событие изменено",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &event.ID,
			RelatedType: "event",
			Data: map[string]interface{}{
				"event_id":     event.ID,
				"time_changed": timeChanged,
				"start_time":   event.StartTime,
				"end_time":     event.EndTime,
				"location":     event.Location,
				"sender_id":    updaterID,
			},
			Channels: channels,
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send event updated notification to user %d: %v\n", participant.UserID, err)
		}
	}
}

// sendEventCancelledNotification sends notification when event is cancelled
func (u *calendarUsecase) sendEventCancelledNotification(event *models.Event, cancellerID uint) {
	// Get canceller info
	cancellerInfo, err := u.userClient.GetUserByID(cancellerID)
	cancellerName := "Кто-то"
	if err == nil && cancellerInfo != nil {
		cancellerName = cancellerInfo.Name
	}

	priority := "high"

	// Get all participants
	participants, err := u.participantRepo.GetEventParticipants(event.ID)
	if err != nil {
		fmt.Printf("Failed to get participants for event cancellation notification: %v\n", err)
		return
	}

	startTimeStr := event.StartTime.Format("02.01.2006 в 15:04")
	if event.AllDay {
		startTimeStr = event.StartTime.Format("02.01.2006")
	}

	// Notify all participants (except canceller)
	for _, participant := range participants {
		if participant.UserID == cancellerID {
			continue
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participant.UserID,
			Type:        "calendar",
			Title:       "❌ Событие отменено",
			Message:     fmt.Sprintf("%s отменил(а) событие \"%s\" (%s)", cancellerName, event.Title, startTimeStr),
			Priority:    &priority,
			RelatedID:   &event.ID,
			RelatedType: "event",
			Data: map[string]interface{}{
				"event_id":     event.ID,
				"cancelled_at": time.Now(),
				"sender_id":    cancellerID,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send event cancelled notification to user %d: %v\n", participant.UserID, err)
		}
	}
}

// sendParticipantStatusNotification sends notification when participant changes their status
func (u *calendarUsecase) sendParticipantStatusNotification(event *models.Event, participantID uint, status models.ParticipantStatus) {
	// Only notify organizer
	if event.CreatedBy == participantID {
		return // Don't notify if organizer changes their own status
	}

	// Get participant info
	participantInfo, err := u.userClient.GetUserByID(participantID)
	participantName := "Кто-то"
	if err == nil && participantInfo != nil {
		participantName = participantInfo.Name
	}

	var title, message, emoji string
	priority := "low"
	channels := []string{"in_app"}

	switch status {
	case models.ParticipantStatusAccepted:
		emoji = "✅"
		title = "✅ Участник подтвердил присутствие"
		message = fmt.Sprintf("%s принял(а) приглашение на событие: %s", participantName, event.Title)

	case models.ParticipantStatusDeclined:
		emoji = "❌"
		title = "❌ Участник отклонил приглашение"
		message = fmt.Sprintf("%s не сможет присутствовать на событии: %s", participantName, event.Title)
		priority = "medium"
		channels = []string{"in_app", "push"}

	case models.ParticipantStatusMaybe:
		emoji = "❓"
		title = "❓ Участник под вопросом"
		message = fmt.Sprintf("%s возможно присоединится к событию: %s", participantName, event.Title)

	default:
		return // Don't send notification for pending or unknown statuses
	}

	// Create group key for grouping similar status changes
	groupKey := fmt.Sprintf("calendar:event_%d:status_%s", event.ID, status)

	notificationReq := &clients.NotificationRequest{
		UserID:      event.CreatedBy,
		Type:        "calendar",
		Title:       title,
		Message:     message,
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		GroupKey:    groupKey, // Enable grouping for multiple status changes
		Data: map[string]interface{}{
			"event_id":       event.ID,
			"participant_id": participantID,
			"status":         status,
			"emoji":          emoji,
		},
		Channels: channels,
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send participant status notification to organizer %d: %v\n", event.CreatedBy, err)
	}
}

// sendParticipantAddedNotification sends notification when participant is added after event creation
func (u *calendarUsecase) sendParticipantAddedNotification(event *models.Event, adderID uint, newParticipantIDs []uint) {
	// Get adder info
	adderInfo, err := u.userClient.GetUserByID(adderID)
	adderName := "Кто-то"
	if err == nil && adderInfo != nil {
		adderName = adderInfo.Name
	}

	priority := "medium"

	// Format start time
	startTimeStr := event.StartTime.Format("02.01.2006 в 15:04")
	if event.AllDay {
		startTimeStr = event.StartTime.Format("02.01.2006")
	}

	// Send to each new participant
	for _, participantID := range newParticipantIDs {
		message := fmt.Sprintf("%s добавил(а) вас к событию: %s (%s)", adderName, event.Title, startTimeStr)
		if event.Location != "" {
			message += fmt.Sprintf(", место: %s", event.Location)
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "calendar",
			Title:       "📅 Вас добавили к событию",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &event.ID,
			RelatedType: "event",
			Data: map[string]interface{}{
				"event_id":    event.ID,
				"added_by_id": adderID,
				"start_time":  event.StartTime,
				"end_time":    event.EndTime,
				"location":    event.Location,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send participant added notification to user %d: %v\n", participantID, err)
		}
	}
}

// sendParticipantRemovedNotification sends notification when participant is removed from event
func (u *calendarUsecase) sendParticipantRemovedNotification(event *models.Event, removedParticipantID uint) {
	priority := "medium"

	notificationReq := &clients.NotificationRequest{
		UserID:      removedParticipantID,
		Type:        "calendar",
		Title:       "❌ Вы исключены из события",
		Message:     fmt.Sprintf("Вас исключили из события: %s", event.Title),
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		Data: map[string]interface{}{
			"event_id": event.ID,
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send participant removed notification to user %d: %v\n", removedParticipantID, err)
	}
}

// sendEventReminderNotification sends reminder notification before event starts
func (u *calendarUsecase) sendEventReminderNotification(event *models.Event, userID uint, minutesBefore int) {
	// Determine priority and time description based on minutes before
	var priority string
	var timeDesc string
	var channels []string

	switch {
	case minutesBefore < 5:
		priority = "critical"
		timeDesc = "через несколько минут"
		channels = []string{"in_app", "push"}
	case minutesBefore <= 15:
		priority = "high"
		timeDesc = fmt.Sprintf("через %d минут", minutesBefore)
		channels = []string{"in_app", "push"}
	case minutesBefore <= 60:
		priority = "medium"
		timeDesc = fmt.Sprintf("через %d минут", minutesBefore)
		channels = []string{"in_app", "push"}
	case minutesBefore <= 1440: // 24 hours
		hours := minutesBefore / 60
		priority = "medium"
		if hours == 1 {
			timeDesc = "через 1 час"
		} else {
			timeDesc = fmt.Sprintf("через %d часов", hours)
		}
		channels = []string{"in_app", "push"}
	default:
		days := minutesBefore / 1440
		priority = "low"
		if days == 1 {
			timeDesc = "завтра"
		} else {
			timeDesc = fmt.Sprintf("через %d дней", days)
		}
		channels = []string{"in_app", "email"}
	}

	message := fmt.Sprintf("Событие \"%s\" начнется %s", event.Title, timeDesc)
	if event.Location != "" {
		message += fmt.Sprintf(" (место: %s)", event.Location)
	}

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "reminder",
		Title:       "⏰ Напоминание о событии",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		Data: map[string]interface{}{
			"event_id":       event.ID,
			"reminder_type":  "event",
			"minutes_before": minutesBefore,
			"start_time":     event.StartTime,
			"location":       event.Location,
		},
		Channels: channels,
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send event reminder notification to user %d: %v\n", userID, err)
	}
}

// sendDeadlineApproachingNotification sends notification for approaching deadline
func (u *calendarUsecase) sendDeadlineApproachingNotification(event *models.Event, userID uint, hoursRemaining int) {
	var priority string
	var timeDesc string
	var channels []string

	switch {
	case hoursRemaining <= 1:
		priority = "critical"
		timeDesc = "через 1 час"
		channels = []string{"in_app", "email", "push"}
	case hoursRemaining <= 3:
		priority = "critical"
		timeDesc = fmt.Sprintf("через %d часа", hoursRemaining)
		channels = []string{"in_app", "push"}
	case hoursRemaining <= 24:
		priority = "high"
		timeDesc = fmt.Sprintf("через %d часов", hoursRemaining)
		channels = []string{"in_app", "email", "push"}
	default:
		return // Don't send for > 24 hours
	}

	notificationReq := &clients.NotificationRequest{
		UserID:      userID,
		Type:        "calendar",
		Title:       "⚠️ Дедлайн приближается",
		Message:     fmt.Sprintf("Дедлайн \"%s\" истекает %s", event.Title, timeDesc),
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		Data: map[string]interface{}{
			"event_id":        event.ID,
			"deadline_time":   event.EndTime,
			"hours_remaining": hoursRemaining,
		},
		Channels: channels,
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send deadline notification to user %d: %v\n", userID, err)
	}
}

// sendPendingInvitationReminder sends reminder for pending invitation
func (u *calendarUsecase) sendPendingInvitationReminder(event *models.Event, participantID uint) {
	priority := "medium"

	timeUntil := time.Until(event.StartTime)
	timeDesc := "скоро"
	if timeUntil.Hours() < 24 {
		timeDesc = "завтра"
	} else if timeUntil.Hours() < 48 {
		timeDesc = "послезавтра"
	}

	notificationReq := &clients.NotificationRequest{
		UserID:      participantID,
		Type:        "calendar",
		Title:       "⏰ Не забудьте ответить на приглашение",
		Message:     fmt.Sprintf("Не забудьте ответить на приглашение: %s (%s)", event.Title, timeDesc),
		Priority:    &priority,
		RelatedID:   &event.ID,
		RelatedType: "event",
		Data: map[string]interface{}{
			"event_id":   event.ID,
			"start_time": event.StartTime,
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send pending invitation reminder to user %d: %v\n", participantID, err)
	}
}

// ProcessEventReminders processes and sends reminders that are due
func (u *calendarUsecase) ProcessEventReminders() error {
	// Get reminders that are due (trigger_time <= now AND is_sent = false)
	reminders, err := u.reminderRepo.GetPendingReminders(time.Now())
	if err != nil {
		return fmt.Errorf("failed to get pending reminders: %w", err)
	}

	logger.WithField("reminders_count", len(reminders)).Debug("Processing event reminders")

	for _, reminder := range reminders {
		// Get event details
		event, err := u.eventRepo.GetEventByID(reminder.EventID)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"reminder_id": reminder.ID,
				"event_id":    reminder.EventID,
				"error":       err.Error(),
			}).Error("Failed to get event for reminder")
			continue
		}

		// Skip absence events — no reminders needed for vacation, sick leave, etc.
		if event.Type == models.EventTypeAbsence {
			// Mark as sent so it won't be processed again
			if err := u.reminderRepo.MarkReminderSent(reminder.ID); err != nil {
				logger.WithFields(map[string]interface{}{
					"reminder_id": reminder.ID,
					"error":       err.Error(),
				}).Error("Failed to mark absence reminder as sent")
			}
			continue
		}

		// Calculate minutes before
		minutesBefore := int(time.Until(event.StartTime).Minutes())
		if minutesBefore < 0 {
			minutesBefore = 0
		}

		// Send reminder notification
		u.sendEventReminderNotification(event, reminder.UserID, minutesBefore)

		// Mark reminder as sent
		if err := u.reminderRepo.MarkReminderSent(reminder.ID); err != nil {
			logger.WithFields(map[string]interface{}{
				"reminder_id": reminder.ID,
				"error":       err.Error(),
			}).Error("Failed to mark reminder as sent")
		}
	}

	return nil
}
