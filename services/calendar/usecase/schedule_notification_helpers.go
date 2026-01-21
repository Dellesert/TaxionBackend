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
	case models.ScheduleTypeShift:
		return "сменный"
	case models.ScheduleTypeCustom:
		return "особый"
	default:
		return "рабочий"
	}
}
