// File: services/poll/usecase/notification_helpers.go
package usecase

import (
	"fmt"

	"tachyon-messenger/services/poll/clients"
	"tachyon-messenger/services/poll/models"
)

// sendPollCreatedNotification sends notification when poll is created to invited participants
func (u *pollUsecase) sendPollCreatedNotification(poll *models.Poll, creatorID uint, participantIDs []uint) {
	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(creatorID)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	// Send to each participant
	for _, participantID := range participantIDs {
		if participantID == creatorID {
			continue // Don't notify creator
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "poll",
			Title:       "📊 Новый опрос",
			Message:     fmt.Sprintf("%s создал(а) опрос: %s", creatorName, poll.Title),
			Priority:    &priority,
			RelatedID:   &poll.ID,
			RelatedType: "poll",
			Data: map[string]interface{}{
				"poll_id":   poll.ID,
				"sender_id": creatorID,
			},
			Channels: []string{"in_app", "email", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send poll created notification to user %d: %v\n", participantID, err)
		}
	}
}

// sendPollActivatedNotification sends notification when poll becomes active
func (u *pollUsecase) sendPollActivatedNotification(poll *models.Poll, participantIDs []uint) {
	// Get creator info
	creatorInfo, err := u.userClient.GetUserByID(poll.CreatedBy)
	creatorName := "Кто-то"
	if err == nil && creatorInfo != nil {
		creatorName = creatorInfo.Name
	}

	priority := "medium"

	// Send to each participant
	for _, participantID := range participantIDs {
		if participantID == poll.CreatedBy {
			continue // Don't notify creator
		}

		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "poll",
			Title:       "📊 Опрос начался",
			Message:     fmt.Sprintf("%s запустил(а) опрос: %s", creatorName, poll.Title),
			Priority:    &priority,
			RelatedID:   &poll.ID,
			RelatedType: "poll",
			Data: map[string]interface{}{
				"poll_id":   poll.ID,
				"sender_id": poll.CreatedBy,
			},
			Channels: []string{"in_app", "push"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send poll activated notification to user %d: %v\n", participantID, err)
		}
	}
}

// sendPollClosedNotification sends notification when poll is closed with results
func (u *pollUsecase) sendPollClosedNotification(poll *models.Poll, participantIDs []uint) {
	priority := "low"

	// Send to each participant who voted
	for _, participantID := range participantIDs {
		notificationReq := &clients.NotificationRequest{
			UserID:      participantID,
			Type:        "poll",
			Title:       "📊 Опрос завершён",
			Message:     fmt.Sprintf("Опрос \"%s\" завершён. Результаты доступны", poll.Title),
			Priority:    &priority,
			RelatedID:   &poll.ID,
			RelatedType: "poll",
			Data: map[string]interface{}{
				"poll_id":           poll.ID,
				"sender_id":         poll.CreatedBy,
				"notification_type": "poll_closed",
			},
			Channels: []string{"in_app"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send poll closed notification to user %d: %v\n", participantID, err)
		}
	}
}

// sendPollCommentNotification sends notification when someone comments on a poll
func (u *pollUsecase) sendPollCommentNotification(poll *models.Poll, comment *models.PollComment, commenterID uint) {
	// Get commenter info
	commenterInfo, err := u.userClient.GetUserByID(commenterID)
	commenterName := "Кто-то"
	if err == nil && commenterInfo != nil {
		commenterName = commenterInfo.Name
	}

	priority := "low"

	// Notify poll creator (if different from commenter)
	if poll.CreatedBy != commenterID {
		notificationReq := &clients.NotificationRequest{
			UserID:      poll.CreatedBy,
			Type:        "poll",
			Title:       "💬 Новый комментарий к опросу",
			Message:     fmt.Sprintf("%s оставил комментарий к опросу \"%s\"", commenterName, poll.Title),
			Priority:    &priority,
			RelatedID:   &poll.ID,
			RelatedType: "poll",
			Data: map[string]interface{}{
				"poll_id":    poll.ID,
				"comment_id": comment.ID,
				"sender_id":  commenterID,
			},
			Channels: []string{"in_app"},
		}

		if err := u.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send poll comment notification to creator %d: %v\n", poll.CreatedBy, err)
		}
	}

	// If this is a reply, notify parent comment author
	if comment.ParentID != nil {
		// Get parent comment to find author
		parentComment, err := u.commentRepo.GetByID(*comment.ParentID)
		if err == nil && parentComment.UserID != commenterID {
			notificationReq := &clients.NotificationRequest{
				UserID:      parentComment.UserID,
				Type:        "poll",
				Title:       "💬 Ответ на комментарий",
				Message:     fmt.Sprintf("%s ответил на ваш комментарий к опросу \"%s\"", commenterName, poll.Title),
				Priority:    &priority,
				RelatedID:   &poll.ID,
				RelatedType: "poll",
				Data: map[string]interface{}{
					"poll_id":    poll.ID,
					"comment_id": comment.ID,
					"sender_id":  commenterID,
				},
				Channels: []string{"in_app", "push"},
			}

			if err := u.notificationClient.SendNotification(notificationReq); err != nil {
				fmt.Printf("Failed to send poll comment reply notification to user %d: %v\n", parentComment.UserID, err)
			}
		}
	}
}
