package usecase

import (
	"fmt"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/logger"
)

// sendScheduleCreatedNotification sends notification when a new schedule is created
func (u *scheduleUsecase) sendScheduleCreatedNotification(schedule *models.Schedule, creatorID uint) {
	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	// Determine recipients based on schedule settings
	var recipientIDs []uint

	if schedule.IsForAllUsers {
		// Get all users
		allUsers, err := u.userClient.GetAllUsers()
		if err != nil {
			fmt.Printf("Failed to get all users for schedule notification: %v\n", err)
			return
		}
		for _, user := range allUsers {
			if user.ID != creatorID {
				recipientIDs = append(recipientIDs, user.ID)
			}
		}
	} else if schedule.DepartmentID != nil {
		// Get users from the department
		deptUsers, err := u.userClient.GetUsersByDepartment(*schedule.DepartmentID)
		if err != nil {
			fmt.Printf("Failed to get department users for schedule notification: %v\n", err)
			return
		}
		for _, user := range deptUsers {
			if user.ID != creatorID {
				recipientIDs = append(recipientIDs, user.ID)
			}
		}
	}

	if len(recipientIDs) == 0 {
		return
	}

	priority := "medium"

	// Format date range
	dateRange := fmt.Sprintf("%s - %s",
		schedule.StartDate.Format("02.01.2006"),
		schedule.EndDate.Format("02.01.2006"),
	)

	// Get schedule type name in Russian
	typeName := getScheduleTypeName(schedule.Type)

	// Send to each recipient
	for _, recipientID := range recipientIDs {
		message := fmt.Sprintf("%s создал(а) новый график \"%s\" (%s) на период %s",
			creatorName, schedule.Title, typeName, dateRange)

		notificationReq := &clients.NotificationRequest{
			UserID:      recipientID,
			Type:        "calendar",
			Title:       "📅 Новый график",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &schedule.ID,
			RelatedType: "schedule",
			Data: map[string]interface{}{
				"schedule_id":   schedule.ID,
				"schedule_type": schedule.Type,
				"start_date":    schedule.StartDate,
				"end_date":      schedule.EndDate,
				"creator_id":    creatorID,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send schedule created notification to user %d: %v\n", recipientID, err)
		}
	}
}

// sendScheduleEntryNotification sends notification when user is added to a schedule
func (u *scheduleUsecase) sendScheduleEntryNotification(schedule *models.Schedule, entry *models.ScheduleEntry, creatorID uint) {
	// Don't notify if user added themselves
	if entry.UserID == creatorID {
		return
	}

	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	// Format date range
	dateRange := fmt.Sprintf("%s - %s",
		schedule.StartDate.Format("02.01.2006"),
		schedule.EndDate.Format("02.01.2006"),
	)

	// Get schedule type name
	typeName := getScheduleTypeName(schedule.Type)

	message := fmt.Sprintf("%s опубликовал(а) график %s \"%s\" на период %s",
		creatorName, typeName, schedule.Title, dateRange)

	notificationReq := &clients.NotificationRequest{
		UserID:      entry.UserID,
		Type:        "calendar",
		Title:       "📅 Новый график",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"schedule_type": schedule.Type,
			"start_date":    schedule.StartDate,
			"end_date":      schedule.EndDate,
			"creator_id":    creatorID,
			"action":        "open_schedule",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send schedule entry notification to user %d: %v\n", entry.UserID, err)
	}
}

// sendBatchScheduleEntryNotifications sends one notification to all participants about new schedule
func (u *scheduleUsecase) sendBatchScheduleEntryNotifications(schedule *models.Schedule, entries []*models.ScheduleEntry, creatorID uint) {
	logger.WithFields(map[string]interface{}{
		"schedule_id":   schedule.ID,
		"schedule_title": schedule.Title,
		"entries_count": len(entries),
		"creator_id":    creatorID,
	}).Info("Sending schedule notifications to participants")

	// Collect unique user IDs (excluding creator)
	userIDSet := make(map[uint]bool)
	for _, entry := range entries {
		if entry.UserID != creatorID {
			userIDSet[entry.UserID] = true
		}
	}

	logger.WithField("unique_users_count", len(userIDSet)).Info("Unique users to notify")

	if len(userIDSet) == 0 {
		logger.Info("No users to notify (all entries belong to creator)")
		return
	}

	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	// Format date range
	dateRange := fmt.Sprintf("%s - %s",
		schedule.StartDate.Format("02.01.2006"),
		schedule.EndDate.Format("02.01.2006"),
	)

	// Get schedule type name
	typeName := getScheduleTypeName(schedule.Type)

	message := fmt.Sprintf("%s опубликовал(а) график %s \"%s\" на период %s",
		creatorName, typeName, schedule.Title, dateRange)

	// Send one notification to each participant
	for userID := range userIDSet {
		notificationReq := &clients.NotificationRequest{
			UserID:      userID,
			Type:        "calendar",
			Title:       "📅 Новый график",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &schedule.ID,
			RelatedType: "schedule",
			Data: map[string]interface{}{
				"schedule_id":   schedule.ID,
				"schedule_type": schedule.Type,
				"start_date":    schedule.StartDate,
				"end_date":      schedule.EndDate,
				"creator_id":    creatorID,
				"action":        "open_schedule",
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id":     userID,
				"schedule_id": schedule.ID,
				"error":       err.Error(),
			}).Error("Failed to send schedule notification")
		} else {
			logger.WithFields(map[string]interface{}{
				"user_id":     userID,
				"schedule_id": schedule.ID,
			}).Info("Schedule notification sent successfully")
		}
	}
}

// sendScheduleEntryUpdatedNotification sends notification when user's schedule entry is updated
func (u *scheduleUsecase) sendScheduleEntryUpdatedNotification(schedule *models.Schedule, oldEntry, newEntry *models.ScheduleEntry, updaterID uint) {
	logger.WithFields(map[string]interface{}{
		"old_user_id":  oldEntry.UserID,
		"new_user_id":  newEntry.UserID,
		"schedule_id":  schedule.ID,
		"entry_id":     newEntry.ID,
		"updater_id":   updaterID,
		"user_changed": oldEntry.UserID != newEntry.UserID,
	}).Info("sendScheduleEntryUpdatedNotification called")

	// Get updater info
	updaterInfo, err := u.userClient.GetUserByID(updaterID)
	updaterName := "Кто-то"
	if err == nil && updaterInfo != nil {
		updaterName = updaterInfo.Name
	}

	priority := "high"

	// Check if user was changed (substitution case)
	userChanged := oldEntry.UserID != newEntry.UserID

	if userChanged {
		// Send notification to OLD user that they were removed from the schedule
		if oldEntry.UserID != updaterID {
			u.sendUserRemovedFromScheduleNotification(schedule, oldEntry, updaterName, updaterID)
		}

		// Send notification to NEW user that they were added to the schedule
		if newEntry.UserID != updaterID {
			u.sendUserAddedToScheduleNotification(schedule, newEntry, updaterName, updaterID)
		}
		return
	}

	// Don't notify if user updated their own entry (and user didn't change)
	if newEntry.UserID == updaterID {
		return
	}

	// Build change description for regular updates
	var changes []string

	if !oldEntry.Date.Equal(newEntry.Date) {
		changes = append(changes, fmt.Sprintf("дата: %s → %s",
			oldEntry.Date.Format("02.01"),
			newEntry.Date.Format("02.01")))
	}

	if !oldEntry.StartTime.Equal(newEntry.StartTime) || !oldEntry.EndTime.Equal(newEntry.EndTime) {
		changes = append(changes, fmt.Sprintf("время: %s-%s → %s-%s",
			oldEntry.StartTime.Format("15:04"),
			oldEntry.EndTime.Format("15:04"),
			newEntry.StartTime.Format("15:04"),
			newEntry.EndTime.Format("15:04")))
	}

	if oldEntry.ShiftType != newEntry.ShiftType {
		changes = append(changes, fmt.Sprintf("смена: %s → %s",
			getShiftTypeName(oldEntry.ShiftType),
			getShiftTypeName(newEntry.ShiftType)))
	}

	if len(changes) == 0 {
		return // No significant changes
	}

	message := fmt.Sprintf("%s изменил(а) вашу смену в графике \"%s\" на %s: %s",
		updaterName,
		schedule.Title,
		newEntry.Date.Format("02.01.2006"),
		joinChanges(changes))

	notificationReq := &clients.NotificationRequest{
		UserID:      newEntry.UserID,
		Type:        "calendar",
		Title:       "📅 Изменение в графике",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":    schedule.ID,
			"schedule_type":  schedule.Type,
			"entry_id":       newEntry.ID,
			"old_date":       oldEntry.Date,
			"new_date":       newEntry.Date,
			"old_start_time": oldEntry.StartTime,
			"new_start_time": newEntry.StartTime,
			"updater_id":     updaterID,
			"action":         "open_schedule",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":     newEntry.UserID,
			"schedule_id": schedule.ID,
			"entry_id":    newEntry.ID,
			"error":       err.Error(),
		}).Error("Failed to send schedule entry updated notification")
	}
}

// sendUserRemovedFromScheduleNotification sends notification when user is removed from schedule (replaced by another user)
func (u *scheduleUsecase) sendUserRemovedFromScheduleNotification(schedule *models.Schedule, entry *models.ScheduleEntry, updaterName string, updaterID uint) {
	logger.WithFields(map[string]interface{}{
		"user_id":      entry.UserID,
		"schedule_id":  schedule.ID,
		"updater_id":   updaterID,
		"updater_name": updaterName,
	}).Info("Sending user removed from schedule notification")

	if u.notificationClient == nil {
		logger.Error("notificationClient is nil, cannot send notification")
		return
	}

	priority := "high"

	message := fmt.Sprintf("%s снял(а) вас со смены в графике \"%s\" на %s (%s-%s)",
		updaterName,
		schedule.Title,
		entry.Date.Format("02.01.2006"),
		entry.StartTime.Format("15:04"),
		entry.EndTime.Format("15:04"))

	notificationReq := &clients.NotificationRequest{
		UserID:      entry.UserID,
		Type:        "calendar",
		Title:       "🔄 Замена в графике",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"schedule_type": schedule.Type,
			"date":          entry.Date,
			"updater_id":    updaterID,
			"action":        "open_schedule",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":     entry.UserID,
			"schedule_id": schedule.ID,
			"error":       err.Error(),
		}).Error("Failed to send user removed from schedule notification")
	} else {
		logger.WithFields(map[string]interface{}{
			"user_id":     entry.UserID,
			"schedule_id": schedule.ID,
		}).Info("Successfully sent user removed from schedule notification")
	}
}

// sendUserAddedToScheduleNotification sends notification when user is added to schedule (replacing another user)
func (u *scheduleUsecase) sendUserAddedToScheduleNotification(schedule *models.Schedule, entry *models.ScheduleEntry, updaterName string, updaterID uint) {
	logger.WithFields(map[string]interface{}{
		"user_id":      entry.UserID,
		"schedule_id":  schedule.ID,
		"entry_id":     entry.ID,
		"updater_id":   updaterID,
		"updater_name": updaterName,
	}).Info("Sending user added to schedule notification")

	if u.notificationClient == nil {
		logger.Error("notificationClient is nil, cannot send notification")
		return
	}

	priority := "high"

	message := fmt.Sprintf("%s добавил(а) вас на смену в график \"%s\" на %s (%s-%s)",
		updaterName,
		schedule.Title,
		entry.Date.Format("02.01.2006"),
		entry.StartTime.Format("15:04"),
		entry.EndTime.Format("15:04"))

	notificationReq := &clients.NotificationRequest{
		UserID:      entry.UserID,
		Type:        "calendar",
		Title:       "📅 Новая смена",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"schedule_type": schedule.Type,
			"entry_id":      entry.ID,
			"date":          entry.Date,
			"start_time":    entry.StartTime,
			"end_time":      entry.EndTime,
			"updater_id":    updaterID,
			"action":        "open_schedule",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":     entry.UserID,
			"schedule_id": schedule.ID,
			"entry_id":    entry.ID,
			"error":       err.Error(),
		}).Error("Failed to send user added to schedule notification")
	} else {
		logger.WithFields(map[string]interface{}{
			"user_id":     entry.UserID,
			"schedule_id": schedule.ID,
			"entry_id":    entry.ID,
		}).Info("Successfully sent user added to schedule notification")
	}
}

// sendScheduleEntryCancelledNotification sends notification when user's shift is cancelled
func (u *scheduleUsecase) sendScheduleEntryCancelledNotification(schedule *models.Schedule, entry *models.ScheduleEntry, cancellerID uint) {
	// Don't notify if user cancelled their own entry
	if entry.UserID == cancellerID {
		return
	}

	// Get canceller info
	cancellerInfo, err := u.userClient.GetUserByID(cancellerID)
	cancellerName := "Кто-то"
	if err == nil && cancellerInfo != nil {
		cancellerName = cancellerInfo.Name
	}

	priority := "high"

	message := fmt.Sprintf("%s отменил(а) вашу смену в графике \"%s\" на %s (%s)",
		cancellerName,
		schedule.Title,
		entry.Date.Format("02.01.2006"),
		entry.StartTime.Format("15:04")+"-"+entry.EndTime.Format("15:04"))

	notificationReq := &clients.NotificationRequest{
		UserID:      entry.UserID,
		Type:        "calendar",
		Title:       "❌ Смена отменена",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"schedule_type": schedule.Type,
			"cancelled_date": entry.Date,
			"cancelled_time": entry.StartTime.Format("15:04") + "-" + entry.EndTime.Format("15:04"),
			"canceller_id":  cancellerID,
			"action":        "open_schedule",
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id":     entry.UserID,
			"schedule_id": schedule.ID,
			"entry_id":    entry.ID,
			"error":       err.Error(),
		}).Error("Failed to send schedule entry cancelled notification")
	}
}

// joinChanges joins change descriptions with commas
func joinChanges(changes []string) string {
	if len(changes) == 0 {
		return ""
	}
	if len(changes) == 1 {
		return changes[0]
	}
	result := ""
	for i, change := range changes {
		if i > 0 {
			result += ", "
		}
		result += change
	}
	return result
}

// getShiftTypeName returns Russian name for shift type
func getShiftTypeName(shiftType models.ShiftType) string {
	switch shiftType {
	case models.ShiftMorning:
		return "утренняя смена"
	case models.ShiftEvening:
		return "вечерняя смена"
	case models.ShiftFullDay:
		return "полный день"
	case models.ShiftCustom:
		return "особая смена"
	default:
		return "смена"
	}
}

// getScheduleTypeName returns Russian name for schedule type
func getScheduleTypeName(scheduleType models.ScheduleType) string {
	switch scheduleType {
	case models.ScheduleTypeWork:
		return "рабочий"
	case models.ScheduleTypePaidServices:
		return "платные услуги"
	case models.ScheduleTypeOnDuty:
		return "дежурство"
	case models.ScheduleTypeVK:
		return "ВК"
	case models.ScheduleTypeTrips:
		return "выезды"
	default:
		return "рабочий"
	}
}
