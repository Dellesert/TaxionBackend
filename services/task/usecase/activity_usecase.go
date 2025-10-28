package usecase

import (
	"encoding/json"
	"fmt"
	"time"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
)

// UserClientInterface defines the interface for user service client operations
type UserClientInterface interface {
	GetUsersByIDs(ids []uint) (map[uint]*clients.UserInfo, error)
}

// ActivityUsecase defines the interface for task activity business logic
type ActivityUsecase interface {
	// Log activity
	LogTaskCreated(taskID, userID uint, taskDetails interface{}) error
	LogTaskUpdated(taskID, userID uint, field, oldValue, newValue string, details interface{}) error
	LogTaskStatusChanged(taskID, userID uint, oldStatus, newStatus string) error
	LogTaskAssigned(taskID, userID, assignedToUserID uint, assignedToDepartmentID *uint) error
	LogTaskDelegated(taskID, fromUserID, toUserID uint) error
	LogTaskViewed(taskID, userID uint) error
	LogCommentAdded(taskID, userID, commentID uint) error
	LogAttachmentAdded(taskID, userID, attachmentID uint, fileName string) error
	LogAttachmentDeleted(taskID, userID, attachmentID uint, fileName string) error
	LogChecklistAdded(taskID, userID, checklistID uint, checklistTitle string) error
	LogChecklistItemCompleted(taskID, userID, checklistID, itemID uint, itemTitle string, isCompleted bool) error

	// Query activities
	GetTaskActivities(taskID uint, limit, offset int) ([]*models.TaskActivity, int64, error)
	GetActivityByID(id uint) (*models.TaskActivity, error)
	EnrichActivitiesWithUserInfo(activities []*models.TaskActivity) ([]*models.TaskActivityResponse, error)
}

// activityUsecase implements ActivityUsecase interface
type activityUsecase struct {
	activityRepo repository.ActivityRepository
	taskRepo     repository.TaskRepository
	userClient   UserClientInterface
}

// NewActivityUsecase creates a new activity usecase
func NewActivityUsecase(
	activityRepo repository.ActivityRepository,
	taskRepo repository.TaskRepository,
	userClient UserClientInterface,
) ActivityUsecase {
	return &activityUsecase{
		activityRepo: activityRepo,
		taskRepo:     taskRepo,
		userClient:   userClient,
	}
}

// Helper function to marshal details to JSON string
func marshalDetails(details interface{}) *string {
	if details == nil {
		return nil
	}
	jsonBytes, err := json.Marshal(details)
	if err != nil {
		return nil
	}
	result := string(jsonBytes)
	return &result
}

// LogTaskCreated logs when a task is created
func (u *activityUsecase) LogTaskCreated(taskID, userID uint, taskDetails interface{}) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "task_created",
		NewValue:   "Task created",
		Details:    marshalDetails(taskDetails),
	}
	return u.activityRepo.Create(activity)
}

// LogTaskUpdated logs when a task field is updated
func (u *activityUsecase) LogTaskUpdated(taskID, userID uint, field, oldValue, newValue string, details interface{}) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: fmt.Sprintf("task_updated_%s", field),
		OldValue:   oldValue,
		NewValue:   newValue,
		Details:    marshalDetails(details),
	}
	return u.activityRepo.Create(activity)
}

// LogTaskStatusChanged logs when task status changes
func (u *activityUsecase) LogTaskStatusChanged(taskID, userID uint, oldStatus, newStatus string) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "task_status_changed",
		OldValue:   oldStatus,
		NewValue:   newStatus,
		Details:    marshalDetails(map[string]interface{}{
			"old_status": oldStatus,
			"new_status": newStatus,
			"changed_at": time.Now(),
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogTaskAssigned logs when a task is assigned
func (u *activityUsecase) LogTaskAssigned(taskID, userID, assignedToUserID uint, assignedToDepartmentID *uint) error {
	details := map[string]interface{}{
		"assigned_to_user_id": assignedToUserID,
	}
	if assignedToDepartmentID != nil {
		details["assigned_to_department_id"] = *assignedToDepartmentID
	}

	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "task_assigned",
		NewValue:   fmt.Sprintf("Assigned to user %d", assignedToUserID),
		Details:    marshalDetails(details),
	}
	return u.activityRepo.Create(activity)
}

// LogTaskDelegated logs when a task is delegated
func (u *activityUsecase) LogTaskDelegated(taskID, fromUserID, toUserID uint) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     fromUserID,
		ActionType: "task_delegated",
		OldValue:   fmt.Sprintf("User %d", fromUserID),
		NewValue:   fmt.Sprintf("User %d", toUserID),
		Details:    marshalDetails(map[string]interface{}{
			"from_user_id": fromUserID,
			"to_user_id":   toUserID,
			"delegated_at": time.Now(),
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogTaskViewed logs when a task is viewed for the first time
func (u *activityUsecase) LogTaskViewed(taskID, userID uint) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "task_viewed",
		NewValue:   "Task viewed",
		Details:    marshalDetails(map[string]interface{}{
			"viewed_at": time.Now(),
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogCommentAdded logs when a comment is added
func (u *activityUsecase) LogCommentAdded(taskID, userID, commentID uint) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "comment_added",
		NewValue:   fmt.Sprintf("Comment %d added", commentID),
		Details:    marshalDetails(map[string]interface{}{
			"comment_id": commentID,
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogAttachmentAdded logs when an attachment is added
func (u *activityUsecase) LogAttachmentAdded(taskID, userID, attachmentID uint, fileName string) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "attachment_added",
		NewValue:   fmt.Sprintf("File added: %s", fileName),
		Details:    marshalDetails(map[string]interface{}{
			"attachment_id": attachmentID,
			"file_name":     fileName,
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogAttachmentDeleted logs when an attachment is deleted
func (u *activityUsecase) LogAttachmentDeleted(taskID, userID, attachmentID uint, fileName string) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "attachment_deleted",
		OldValue:   fmt.Sprintf("File deleted: %s", fileName),
		Details:    marshalDetails(map[string]interface{}{
			"attachment_id": attachmentID,
			"file_name":     fileName,
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogChecklistAdded logs when a checklist is added
func (u *activityUsecase) LogChecklistAdded(taskID, userID, checklistID uint, checklistTitle string) error {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: "checklist_added",
		NewValue:   fmt.Sprintf("Checklist added: %s", checklistTitle),
		Details:    marshalDetails(map[string]interface{}{
			"checklist_id":    checklistID,
			"checklist_title": checklistTitle,
		}),
	}
	return u.activityRepo.Create(activity)
}

// LogChecklistItemCompleted logs when a checklist item is completed/uncompleted
func (u *activityUsecase) LogChecklistItemCompleted(taskID, userID, checklistID, itemID uint, itemTitle string, isCompleted bool) error {
	action := "uncompleted"
	if isCompleted {
		action = "completed"
	}

	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: fmt.Sprintf("checklist_item_%s", action),
		NewValue:   fmt.Sprintf("Checklist item %s: %s", action, itemTitle),
		Details:    marshalDetails(map[string]interface{}{
			"checklist_id": checklistID,
			"item_id":      itemID,
			"item_title":   itemTitle,
			"is_completed": isCompleted,
		}),
	}
	return u.activityRepo.Create(activity)
}

// GetTaskActivities retrieves activities for a task with pagination, including subtask activities
func (u *activityUsecase) GetTaskActivities(taskID uint, limit, offset int) ([]*models.TaskActivity, int64, error) {
	// Get subtasks for the parent task
	subtasks, err := u.taskRepo.GetSubtasks(taskID)
	if err != nil {
		// If error getting subtasks, just return activities for main task
		return u.activityRepo.GetByTaskID(taskID, limit, offset)
	}

	// Extract subtask IDs
	subtaskIDs := make([]uint, 0, len(subtasks))
	for _, subtask := range subtasks {
		subtaskIDs = append(subtaskIDs, subtask.ID)
	}

	// Get activities for task and subtasks
	if len(subtaskIDs) > 0 {
		return u.activityRepo.GetByTaskIDWithSubtasks(taskID, subtaskIDs, limit, offset)
	}

	// No subtasks, just get activities for main task
	return u.activityRepo.GetByTaskID(taskID, limit, offset)
}

// GetActivityByID retrieves an activity by ID
func (u *activityUsecase) GetActivityByID(id uint) (*models.TaskActivity, error) {
	return u.activityRepo.GetByID(id)
}

// EnrichActivitiesWithUserInfo enriches activities with user information and task titles
func (u *activityUsecase) EnrichActivitiesWithUserInfo(activities []*models.TaskActivity) ([]*models.TaskActivityResponse, error) {
	if len(activities) == 0 {
		return []*models.TaskActivityResponse{}, nil
	}

	// Collect unique user IDs and task IDs
	userIDs := make(map[uint]bool)
	taskIDs := make(map[uint]bool)
	for _, activity := range activities {
		userIDs[activity.UserID] = true
		taskIDs[activity.TaskID] = true

		// Parse details to extract assignee_ids if present
		if activity.Details != nil && *activity.Details != "" {
			var details map[string]interface{}
			if err := json.Unmarshal([]byte(*activity.Details), &details); err == nil {
				if assigneeIDs, ok := details["assignee_ids"].([]interface{}); ok {
					for _, id := range assigneeIDs {
						if floatID, ok := id.(float64); ok {
							userIDs[uint(floatID)] = true
						}
					}
				}
			}
		}
	}

	// Convert maps to slices
	userIDList := make([]uint, 0, len(userIDs))
	for id := range userIDs {
		userIDList = append(userIDList, id)
	}

	taskIDList := make([]uint, 0, len(taskIDs))
	for id := range taskIDs {
		taskIDList = append(taskIDList, id)
	}

	// Fetch users from user service
	usersMap, err := u.userClient.GetUsersByIDs(userIDList)
	if err != nil {
		// Log error but continue - we can return activities without user info
		fmt.Printf("Warning: failed to fetch users: %v\n", err)
		usersMap = make(map[uint]*clients.UserInfo)
	}

	// Fetch task titles
	tasksMap := make(map[uint]string)
	for _, taskID := range taskIDList {
		task, err := u.taskRepo.GetByID(taskID)
		if err != nil {
			// If task not found, skip it
			continue
		}
		tasksMap[taskID] = task.Title
	}

	// Build response
	responses := make([]*models.TaskActivityResponse, 0, len(activities))
	for _, activity := range activities {
		response := &models.TaskActivityResponse{
			ID:         activity.ID,
			TaskID:     activity.TaskID,
			TaskTitle:  tasksMap[activity.TaskID],
			UserID:     activity.UserID,
			ActionType: activity.ActionType,
			OldValue:   activity.OldValue,
			NewValue:   activity.NewValue,
			Details:    activity.Details,
			CreatedAt:  activity.CreatedAt,
		}

		// Add user info if available
		if user, ok := usersMap[activity.UserID]; ok {
			response.User = &models.UserInfo{
				ID:       user.ID,
				Name:     user.Name,
				Email:    user.Email,
				Avatar:   user.Avatar,
				Position: user.Position,
			}
		}

		// Parse details to extract assignees for subtask_created activities
		if activity.ActionType == "subtask_created" && activity.Details != nil && *activity.Details != "" {
			var details map[string]interface{}
			if err := json.Unmarshal([]byte(*activity.Details), &details); err == nil {
				if assigneeIDs, ok := details["assignee_ids"].([]interface{}); ok {
					assignees := make([]*models.UserInfo, 0, len(assigneeIDs))
					for _, id := range assigneeIDs {
						if floatID, ok := id.(float64); ok {
							assigneeID := uint(floatID)
							if user, ok := usersMap[assigneeID]; ok {
								assignees = append(assignees, &models.UserInfo{
									ID:       user.ID,
									Name:     user.Name,
									Email:    user.Email,
									Avatar:   user.Avatar,
									Position: user.Position,
								})
							}
						}
					}
					response.Assignees = assignees
				}
			}
		}

		responses = append(responses, response)
	}

	return responses, nil
}
