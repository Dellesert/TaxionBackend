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
