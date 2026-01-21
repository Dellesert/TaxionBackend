package usecase

import (
	"fmt"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/models"
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

	// Format date and time
	dateStr := entry.Date.Format("02.01.2006")
	timeStr := fmt.Sprintf("%s - %s", entry.StartTime.Format("15:04"), entry.EndTime.Format("15:04"))

	// Get shift type name
	shiftName := getShiftTypeName(entry.ShiftType)

	message := fmt.Sprintf("%s добавил(а) вас в график \"%s\" на %s (%s, %s)",
		creatorName, schedule.Title, dateStr, shiftName, timeStr)

	notificationReq := &clients.NotificationRequest{
		UserID:      entry.UserID,
		Type:        "calendar",
		Title:       "📅 Вас добавили в график",
		Message:     message,
		Priority:    &priority,
		RelatedID:   &schedule.ID,
		RelatedType: "schedule",
		Data: map[string]interface{}{
			"schedule_id":   schedule.ID,
			"entry_id":      entry.ID,
			"schedule_type": schedule.Type,
			"date":          entry.Date,
			"start_time":    entry.StartTime,
			"end_time":      entry.EndTime,
			"shift_type":    entry.ShiftType,
			"creator_id":    creatorID,
		},
		Channels: []string{"in_app", "push"},
	}

	if err := u.notificationClient.SendNotification(notificationReq); err != nil {
		fmt.Printf("Failed to send schedule entry notification to user %d: %v\n", entry.UserID, err)
	}
}

// sendBatchScheduleEntryNotifications sends notifications for batch entry creation
func (u *scheduleUsecase) sendBatchScheduleEntryNotifications(schedule *models.Schedule, entries []*models.ScheduleEntry, creatorID uint) {
	// Group entries by user to send one consolidated notification per user
	userEntries := make(map[uint][]*models.ScheduleEntry)
	for _, entry := range entries {
		if entry.UserID != creatorID {
			userEntries[entry.UserID] = append(userEntries[entry.UserID], entry)
		}
	}

	if len(userEntries) == 0 {
		return
	}

	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	for userID, userEntriesList := range userEntries {
		var message string
		if len(userEntriesList) == 1 {
			entry := userEntriesList[0]
			dateStr := entry.Date.Format("02.01.2006")
			timeStr := fmt.Sprintf("%s - %s", entry.StartTime.Format("15:04"), entry.EndTime.Format("15:04"))
			shiftName := getShiftTypeName(entry.ShiftType)
			message = fmt.Sprintf("%s добавил(а) вас в график \"%s\" на %s (%s, %s)",
				creatorName, schedule.Title, dateStr, shiftName, timeStr)
		} else {
			// Multiple entries - summarize
			message = fmt.Sprintf("%s добавил(а) вас в график \"%s\" (%d смен)",
				creatorName, schedule.Title, len(userEntriesList))
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      userID,
			Type:        "calendar",
			Title:       "📅 Вас добавили в график",
			Message:     message,
			Priority:    &priority,
			RelatedID:   &schedule.ID,
			RelatedType: "schedule",
			Data: map[string]interface{}{
				"schedule_id":  schedule.ID,
				"entries_count": len(userEntriesList),
				"creator_id":   creatorID,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send batch schedule entry notification to user %d: %v\n", userID, err)
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
