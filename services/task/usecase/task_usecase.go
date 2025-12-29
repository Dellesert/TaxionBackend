package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/permissions"
	"tachyon-messenger/services/task/repository"
	sharedmodels "tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// TaskUsecase defines the interface for task business logic
type TaskUsecase interface {
	CreateTask(userID uint, userRole sharedmodels.Role, userDepartment string, req *models.CreateTaskRequest) (*models.TaskResponse, error)
	GetTaskByID(userID uint, userRole sharedmodels.Role, taskID uint) (*models.TaskResponse, error)
	GetTaskByIDInternal(taskID uint) (*models.Task, error) // Internal method without access control
	UpdateTask(userID uint, userRole sharedmodels.Role, taskID uint, req *models.UpdateTaskRequest) (*models.TaskResponse, error)
	DeleteTask(userID uint, userRole sharedmodels.Role, taskID uint) error
	AssignTask(userID, taskID uint, req *models.AssignTaskRequest) (*models.TaskResponse, error)
	UnassignTask(userID, taskID uint) (*models.TaskResponse, error)
	UpdateTaskStatus(userID, taskID uint, req *models.UpdateTaskStatusRequest) (*models.TaskResponse, error)
	GetUserTasks(userID uint, userRole sharedmodels.Role, filter *models.TaskFilterRequest) ([]*models.TaskResponse, int64, error)
	GetTaskStats(userID uint) (*models.TaskStatsResponse, error)
	GetTaskStatsInternal(period string) (*models.TaskStatsInternalResponse, error) // Internal method for analytics with optional period filter

	// Sync methods
	GetDeletedTaskIDsSince(since time.Time) ([]uint, error)

	// Hierarchy methods
	CreateSubtask(userID uint, parentTaskID uint, req *models.CreateTaskRequest) (*models.TaskResponse, error)
	GetSubtasks(userID uint, parentTaskID uint) ([]*models.TaskResponse, error)
	GetTaskHierarchy(userID uint, taskID uint) (*models.TaskResponse, error)

	// Delegation methods
	DelegateTask(userID uint, taskID uint, toUserID uint) (*models.TaskResponse, error)
	GetDelegationChain(userID uint, taskID uint) ([]models.UserInfo, error)

	// First-view tracking
	MarkTaskAsViewed(userID uint, taskID uint) (*models.TaskResponse, error)

	// Progress methods
	RecalculateTaskProgress(taskID uint) error
	UpdateTaskProgress(userID uint, taskID uint, progress int) (*models.TaskResponse, error)

	// Comment methods
	AddComment(userID, taskID uint, req *models.CreateTaskCommentRequest) (*models.TaskCommentResponse, error)
	GetTaskComments(userID uint, userRole sharedmodels.Role, taskID uint, filter *models.CommentFilterRequest) (*models.CommentListResponse, error)
	UpdateComment(userID, commentID uint, req *models.UpdateTaskCommentRequest) (*models.TaskCommentResponse, error)
	DeleteComment(userID, commentID uint) error

	// Analytics methods (internal use)
	GetDepartmentTaskStats(period string) (interface{}, error)
	GetTopPerformers(limit int, period string, departmentID *uint) (interface{}, error)
	GetTaskTrends(period string, interval string) (interface{}, error)
	GetPriorityDistribution(period string) (interface{}, error)

	// Permission methods
	GetTaskPermissions(ctx context.Context, taskID uint, userID uint) (*models.TaskPermissions, error)
	CheckPermission(ctx context.Context, taskID uint, userID uint, action string) (bool, error)
	EmergencyCompleteTask(ctx context.Context, taskID uint, userID uint) error

	// Notification methods
	SendAttachmentAddedNotification(taskID, userID uint, fileName string)
}

// taskUsecase implements TaskUsecase interface
type taskUsecase struct {
	taskRepo           repository.TaskRepository
	commentRepo        repository.CommentRepository
	activityRepo       repository.ActivityRepository
	attachmentRepo     repository.AttachmentRepository
	checklistRepo      repository.ChecklistRepository
	attachmentUsecase  AttachmentUsecase
	syncRepo           repository.SyncRepository
	userClient         *clients.UserClient
	notificationClient *clients.NotificationClient
}

// NewTaskUsecase creates a new task usecase
func NewTaskUsecase(
	taskRepo repository.TaskRepository,
	commentRepo repository.CommentRepository,
	activityRepo repository.ActivityRepository,
	attachmentRepo repository.AttachmentRepository,
	checklistRepo repository.ChecklistRepository,
	attachmentUsecase AttachmentUsecase,
) TaskUsecase {
	return &taskUsecase{
		taskRepo:           taskRepo,
		commentRepo:        commentRepo,
		activityRepo:       activityRepo,
		attachmentRepo:     attachmentRepo,
		checklistRepo:      checklistRepo,
		attachmentUsecase:  attachmentUsecase,
		userClient:         clients.NewUserClient(),
		notificationClient: clients.NewNotificationClient(),
	}
}

// NewTaskUsecaseWithSync creates a new task usecase with sync support
func NewTaskUsecaseWithSync(
	taskRepo repository.TaskRepository,
	commentRepo repository.CommentRepository,
	activityRepo repository.ActivityRepository,
	attachmentRepo repository.AttachmentRepository,
	checklistRepo repository.ChecklistRepository,
	attachmentUsecase AttachmentUsecase,
	syncRepo repository.SyncRepository,
) TaskUsecase {
	return &taskUsecase{
		taskRepo:           taskRepo,
		commentRepo:        commentRepo,
		activityRepo:       activityRepo,
		attachmentRepo:     attachmentRepo,
		checklistRepo:      checklistRepo,
		attachmentUsecase:  attachmentUsecase,
		syncRepo:           syncRepo,
		userClient:         clients.NewUserClient(),
		notificationClient: clients.NewNotificationClient(),
	}
}

// CreateTask creates a new task
func (u *taskUsecase) CreateTask(userID uint, userRole sharedmodels.Role, userDepartment string, req *models.CreateTaskRequest) (*models.TaskResponse, error) {
	// Validate request
	if err := u.validateCreateTaskRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check permission to assign tasks
	if err := u.validateTaskAssignmentPermissions(userID, userRole, userDepartment, req); err != nil {
		return nil, err
	}

	// Create task model
	task := &models.Task{
		Title:                strings.TrimSpace(req.Title),
		Description:          strings.TrimSpace(req.Description),
		CreatedBy:            userID,
		CreatedByUserID:      userID, // NEW: Set created_by_user_id
		DueDate:              req.DueDate,
		AssignedToDepartment: req.AssignedToDepartment,
		Status:               models.TaskStatusNew, // NEW: Explicitly set status
	}

	// Set priority (default to medium if not provided)
	if req.Priority != nil {
		task.Priority = *req.Priority
	} else {
		task.Priority = models.TaskPriorityMedium
	}

	// Backward compatibility: Set assigned user if provided
	if req.AssignedTo != nil {
		task.AssignedTo = req.AssignedTo
	}

	// Save task
	if err := u.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Create task assignees
	// If no assignees specified and no department assignment, assign to creator (self)
	assigneeIDs := req.AssigneeIDs
	if len(assigneeIDs) == 0 && req.AssignedToDepartment == nil {
		assigneeIDs = []uint{userID} // Assign to self
	}

	if len(assigneeIDs) > 0 {
		for _, assigneeID := range assigneeIDs {
			assignee := &models.TaskAssignee{
				TaskID: task.ID,
				UserID: assigneeID,
			}
			if err := u.taskRepo.CreateAssignee(assignee); err != nil {
				return nil, fmt.Errorf("failed to create task assignee: %w", err)
			}
			task.Assignees = append(task.Assignees, *assignee)
		}
	}

	// Create checklists if provided
	if len(req.Checklists) > 0 {
		for i, checklistReq := range req.Checklists {
			// Create checklist
			checklist := &models.TaskChecklist{
				TaskID:      task.ID,
				Title:       strings.TrimSpace(checklistReq.Title),
				Description: strings.TrimSpace(checklistReq.Description),
				Position:    i,
			}
			if err := u.checklistRepo.CreateChecklist(checklist); err != nil {
				return nil, fmt.Errorf("failed to create checklist: %w", err)
			}

			// Create checklist items if provided
			if len(checklistReq.Items) > 0 {
				for j, itemTitle := range checklistReq.Items {
					item := &models.TaskChecklistItem{
						ChecklistID: checklist.ID,
						Title:       strings.TrimSpace(itemTitle),
						IsCompleted: false,
						Position:    j,
					}
					if err := u.checklistRepo.CreateChecklistItem(item); err != nil {
						return nil, fmt.Errorf("failed to create checklist item: %w", err)
					}
				}
			}
		}
	}

	// Log activity: task created
	u.logActivity(task.ID, userID, "task_created", "", "Task created", map[string]interface{}{
		"title":       task.Title,
		"priority":    task.Priority,
		"assignees":   assigneeIDs,
		"created_at":  time.Now(),
	})

	// Load checklists with items if any were created
	if len(req.Checklists) > 0 {
		checklistsPtr, err := u.checklistRepo.GetChecklistsWithItems(task.ID)
		if err == nil {
			// Convert []*TaskChecklist to []TaskChecklist
			checklists := make([]models.TaskChecklist, len(checklistsPtr))
			for i, checklistPtr := range checklistsPtr {
				checklists[i] = *checklistPtr
			}
			task.Checklists = checklists
		}
	}

	response := task.ToResponse()

	// Enrich with user info
	if err := u.enrichTaskWithUserInfo(response); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich task with user info: %v\n", err)
	}

	// Send notifications to assignees
	if len(assigneeIDs) > 0 {
		// Don't send notification to creator if they assigned to themselves
		for _, assigneeID := range assigneeIDs {
			if assigneeID == userID {
				continue // Skip notification to creator
			}

			// Get creator name for notification message
			creatorName := "Кто-то"
			if response.Creator != nil {
				creatorName = response.Creator.Name
			}

			// Prepare notification
			priority := string(task.Priority) // Convert task priority to string
			notificationReq := &clients.NotificationRequest{
				UserID:      assigneeID,
				Type:        "task",
				Title:       "✅ Новая задача назначена",
				Message:     fmt.Sprintf("%s назначил(а) вам задачу: %s", creatorName, task.Title),
				Priority:    &priority, // Pointer to priority string
				RelatedID:   &task.ID,
				RelatedType: "task",
				// ActionURL not set - validator requires full URL (with scheme and host), not just path
				Data: map[string]interface{}{
					"task_id":    task.ID,
					"creator_id": userID, // Add creator_id for sender info enrichment
				},
				Channels: []string{"in_app", "email", "push"},
			}

			// Send notification (async, don't block on error)
			go func(req *clients.NotificationRequest) {
				if err := u.notificationClient.SendNotification(req); err != nil {
					fmt.Printf("Failed to send task notification to user %d: %v\n", req.UserID, err)
				}
			}(notificationReq)
		}
	}

	return response, nil
}

// enrichTaskWithUserInfo enriches task response with user information
func (u *taskUsecase) enrichTaskWithUserInfo(response *models.TaskResponse) error {
	// Collect all user IDs we need to fetch
	userIDs := make([]uint, 0)

	// Add creator ID
	userIDs = append(userIDs, response.CreatedBy)

	// Add assignee IDs
	userIDs = append(userIDs, response.AssigneeIDs...)

	// Add last status changer ID
	if response.LastStatusChangedBy != nil {
		userIDs = append(userIDs, *response.LastStatusChangedBy)
	}

	// Remove duplicates
	uniqueIDs := make(map[uint]bool)
	for _, id := range userIDs {
		uniqueIDs[id] = true
	}

	finalIDs := make([]uint, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		finalIDs = append(finalIDs, id)
	}

	if len(finalIDs) == 0 {
		return nil
	}

	// Fetch users from user-service
	users, err := u.userClient.GetUsersByIDs(finalIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch users: %w", err)
	}

	// Set creator info
	if creator, exists := users[response.CreatedBy]; exists {
		response.Creator = &models.UserInfo{
			ID:       creator.ID,
			Name:     creator.Name,
			Email:    creator.Email,
			Avatar:   creator.Avatar,
			Position: creator.Position,
		}
	}

	// Set assignees info
	response.Assignees = make([]models.UserInfo, 0, len(response.AssigneeIDs))
	for _, assigneeID := range response.AssigneeIDs {
		if assignee, exists := users[assigneeID]; exists {
			response.Assignees = append(response.Assignees, models.UserInfo{
				ID:       assignee.ID,
				Name:     assignee.Name,
				Email:    assignee.Email,
				Avatar:   assignee.Avatar,
				Position: assignee.Position,
			})
		}
	}

	// Set last status changer info
	if response.LastStatusChangedBy != nil {
		if statusChanger, exists := users[*response.LastStatusChangedBy]; exists {
			response.LastStatusChanger = &models.UserInfo{
				ID:       statusChanger.ID,
				Name:     statusChanger.Name,
				Email:    statusChanger.Email,
				Avatar:   statusChanger.Avatar,
				Position: statusChanger.Position,
			}
		}
	}

	return nil
}

// validateTaskAssignmentPermissions validates if user has permission to assign tasks
func (u *taskUsecase) validateTaskAssignmentPermissions(userID uint, userRole sharedmodels.Role, userDepartment string, req *models.CreateTaskRequest) error {
	// Admin and super_admin can assign to anyone
	if userRole == sharedmodels.RoleAdmin || userRole == sharedmodels.RoleSuperAdmin {
		return nil
	}

	// If no assignees and no department specified, task is for self - allowed
	if len(req.AssigneeIDs) == 0 && req.AssignedToDepartment == nil && req.AssignedTo == nil {
		return nil
	}

	// If assigning to self only - allowed
	if len(req.AssigneeIDs) == 1 && req.AssigneeIDs[0] == userID && req.AssignedToDepartment == nil {
		return nil
	}

	// If using deprecated AssignedTo field for self - allowed
	if req.AssignedTo != nil && *req.AssignedTo == userID && len(req.AssigneeIDs) == 0 && req.AssignedToDepartment == nil {
		return nil
	}

	// Department head can assign to:
	// 1. Any employee in their department
	// 2. Other department heads (from any department)
	if userRole == sharedmodels.RoleDepartmentHead {
		// Check if all assignees are valid
		if len(req.AssigneeIDs) > 0 {
			// If assigning only to self, allow
			if len(req.AssigneeIDs) == 1 && req.AssigneeIDs[0] == userID {
				return nil
			}

			// Get current user's department from user-service
			currentUsers, err := u.userClient.GetUsersByIDs([]uint{userID})
			if err != nil {
				return fmt.Errorf("failed to get user information: %w", err)
			}

			currentUser, exists := currentUsers[userID]
			if !exists || currentUser.DepartmentID == nil {
				return fmt.Errorf("access denied: department head must belong to a department")
			}

			userDeptID := *currentUser.DepartmentID

			// Fetch user information to validate assignees
			users, err := u.userClient.GetUsersByIDs(req.AssigneeIDs)
			if err != nil {
				return fmt.Errorf("failed to validate assignees: %w", err)
			}

			// Check each assignee:
			// - Can assign to anyone in their own department (any role)
			// - Can assign to other department heads (from any department)
			for _, assigneeID := range req.AssigneeIDs {
				user, exists := users[assigneeID]
				if !exists {
					return fmt.Errorf("access denied: user %d not found", assigneeID)
				}

				// Check if user is a department head (can assign to any department head)
				if user.Role == sharedmodels.RoleDepartmentHead {
					continue // Allow assignment to any department head
				}

				// For non-department heads, must be in the same department
				if user.DepartmentID == nil || *user.DepartmentID != userDeptID {
					return fmt.Errorf("access denied: you can only assign tasks to members of your department or to other department heads")
				}
			}
			return nil
		}

		// If no specific assignees, allow (task will be assigned to self or department)
		return nil
	}

	// Employee can only create tasks for themselves
	return fmt.Errorf("access denied: employees can only create tasks for themselves")
}

// GetTaskByID retrieves a task by ID with access control
func (u *taskUsecase) GetTaskByID(userID uint, userRole sharedmodels.Role, taskID uint) (*models.TaskResponse, error) {
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Only super_admin can access any task
	isSuperAdmin := userRole == sharedmodels.RoleSuperAdmin

	// Check access rights: user must be creator, assignee, or super_admin
	if !isSuperAdmin && !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	response := task.ToResponse()

	// Enrich with user info
	if err := u.enrichTaskWithUserInfo(response); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich task with user info: %v\n", err)
	}

	// Enrich with permissions
	ctx := context.Background()
	if err := u.enrichTaskResponseWithPermissions(ctx, response, userID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich task with permissions: %v\n", err)
	}

	return response, nil
}

// GetTaskByIDInternal retrieves a task by ID WITHOUT access control
// This is for internal use only (e.g., inter-service communication)
func (u *taskUsecase) GetTaskByIDInternal(taskID uint) (*models.Task, error) {
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return task, nil
}

// UpdateTask updates an existing task
func (u *taskUsecase) UpdateTask(userID uint, userRole sharedmodels.Role, taskID uint, req *models.UpdateTaskRequest) (*models.TaskResponse, error) {
	fmt.Printf("[UpdateTask] ENTRY - Task ID: %d, UserID: %d, Status in request: %v\n", taskID, userID, req.Status)

	// Validate request
	if err := u.validateUpdateTaskRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: creator, assignee, or super_admin can update
	// Parent task creator cannot edit subtasks (only view them)
	isSuperAdmin := userRole == sharedmodels.RoleSuperAdmin
	if !isSuperAdmin && !u.hasTaskEditAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Track changes for activity logging
	changes := make(map[string]interface{})
	statusChanged := false
	dueDateChanged := false
	var oldStatus models.TaskStatus
	var newStatus models.TaskStatus

	// Update fields if provided
	if req.Title != nil && strings.TrimSpace(*req.Title) != task.Title {
		oldTitle := task.Title
		task.Title = strings.TrimSpace(*req.Title)
		u.logActivity(taskID, userID, "task_updated_title", oldTitle, task.Title, nil)
	}
	if req.Description != nil && strings.TrimSpace(*req.Description) != task.Description {
		oldDesc := task.Description
		task.Description = strings.TrimSpace(*req.Description)
		u.logActivity(taskID, userID, "task_updated_description", oldDesc, task.Description, nil)
	}
	if req.Status != nil && *req.Status != task.Status {
		fmt.Printf("[UpdateTask] Task ID: %d - Status change detected: %s -> %s\n", taskID, task.Status, *req.Status)
		// Check can_change_status permission before allowing status change
		ctx := context.Background()
		canChangeStatus, err := permissions.HasPermission(ctx, task, userID, "change_status")
		if err != nil {
			return nil, fmt.Errorf("failed to check permissions: %w", err)
		}
		if !canChangeStatus {
			fmt.Printf("[UpdateTask] Task ID: %d - Permission denied for status change\n", taskID)
			return nil, fmt.Errorf("access denied: insufficient permissions to change task status")
		}

		oldStatus = task.Status
		newStatus = *req.Status
		task.Status = newStatus
		task.LastStatusChangedBy = &userID
		statusChanged = true
		fmt.Printf("[UpdateTask] Task ID: %d - statusChanged set to TRUE\n", taskID)
		u.logActivity(taskID, userID, "task_status_changed", string(oldStatus), string(task.Status), changes)
	}
	if req.Priority != nil && *req.Priority != task.Priority {
		oldPriority := task.Priority
		task.Priority = *req.Priority
		u.logActivity(taskID, userID, "task_updated_priority", string(oldPriority), string(task.Priority), nil)

		// Send notification about priority change (async)
		go u.sendPriorityChangedNotification(task, oldPriority, *req.Priority, userID)
	}
	if req.AssignedTo != nil {
		task.AssignedTo = req.AssignedTo
	}
	if req.AssignedToDepartment != nil {
		task.AssignedToDepartment = req.AssignedToDepartment
	}
	if req.DueDate != nil {
		// Check if due date actually changed
		oldDueDate := task.DueDate
		if oldDueDate == nil || !oldDueDate.Equal(*req.DueDate) {
			task.DueDate = req.DueDate
			dueDateChanged = true
			u.logActivity(taskID, userID, "task_updated_due_date", "", req.DueDate.Format("2006-01-02"), nil)
		}
	}

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	// Send notification when task moved to review status
	if statusChanged && newStatus == models.TaskStatusReview && oldStatus != models.TaskStatusReview {
		// Determine who should receive the notification:
		// - If task is delegated -> notify delegator (DelegatedFromUserID)
		// - Otherwise -> notify creator (CreatedByUserID)
		var recipientID uint
		if task.DelegatedFromUserID != nil {
			recipientID = *task.DelegatedFromUserID
		} else {
			recipientID = task.CreatedByUserID
		}

		// Don't send notification to self
		if recipientID != userID {
			// Get assignee info for notification message
			assigneeInfo, err := u.userClient.GetUserByID(userID)
			assigneeName := "Кто-то"
			if err == nil && assigneeInfo != nil {
				assigneeName = assigneeInfo.Name
			}

			priority := string(task.Priority)
			notificationReq := &clients.NotificationRequest{
				UserID:      recipientID,
				Type:        "task",
				Title:       "✅ Задача сдана на проверку",
				Message:     fmt.Sprintf("%s сдал(а) задачу на проверку: %s", assigneeName, task.Title),
				Priority:    &priority,
				RelatedID:   &task.ID,
				RelatedType: "task",
				Data: map[string]interface{}{
					"task_id":   task.ID,
					"sender_id": userID, // ID of person who submitted for review
				},
				Channels:    []string{"in_app", "email", "push"},
			}

			// Send notification async
			go func(req *clients.NotificationRequest) {
				if err := u.notificationClient.SendNotification(req); err != nil {
					fmt.Printf("Failed to send review notification to user %d: %v\n", req.UserID, err)
				}
			}(notificationReq)
		}
	}

	// Send notification when task returned to work from review/done
	if statusChanged &&
	   (newStatus == models.TaskStatusInProgress || newStatus == models.TaskStatusNew) &&
	   (oldStatus == models.TaskStatusReview || oldStatus == models.TaskStatusDone) {
		// Use assignees from task object
		if len(task.Assignees) > 0 {
			// Get reviewer info (person who returned the task)
			reviewerInfo, err := u.userClient.GetUserByID(userID)
			reviewerName := "Кто-то"
			if err == nil && reviewerInfo != nil {
				reviewerName = reviewerInfo.Name
			}

			priority := string(task.Priority)

			// Send notification to each assignee
			for _, assignee := range task.Assignees {
				// Don't send notification to self
				if assignee.UserID == userID {
					continue
				}

				notificationReq := &clients.NotificationRequest{
					UserID:      assignee.UserID,
					Type:        "task",
					Title:       "🔄 Задача возвращена на доработку",
					Message:     fmt.Sprintf("%s вернул(а) задачу на доработку: %s", reviewerName, task.Title),
					Priority:    &priority,
					RelatedID:   &task.ID,
					RelatedType: "task",
					Data: map[string]interface{}{
						"task_id":   task.ID,
						"sender_id": userID, // ID of reviewer who returned the task
					},
					Channels:    []string{"in_app", "email", "push"},
				}

				// Send notification async
				go func(req *clients.NotificationRequest) {
					if err := u.notificationClient.SendNotification(req); err != nil {
						fmt.Printf("Failed to send return-to-work notification to user %d: %v\n", req.UserID, err)
					}
				}(notificationReq)
			}
		}
	}

	// Send notification when due date changed
	if dueDateChanged && task.DueDate != nil {
		// Use assignees from task object
		if len(task.Assignees) > 0 {
			// Get changer info (person who changed the due date)
			changerInfo, err := u.userClient.GetUserByID(userID)
			changerName := "Кто-то"
			if err == nil && changerInfo != nil {
				changerName = changerInfo.Name
			}

			priority := string(task.Priority)
			formattedDate := task.DueDate.Format("02.01.2006")

			// Send notification to each assignee
			for _, assignee := range task.Assignees {
				// Don't send notification to self
				if assignee.UserID == userID {
					continue
				}

				notificationReq := &clients.NotificationRequest{
					UserID:      assignee.UserID,
					Type:        "task",
					Title:       "📅 Изменён срок выполнения задачи",
					Message:     fmt.Sprintf("%s изменил(а) срок выполнения задачи \"%s\" на %s", changerName, task.Title, formattedDate),
					Priority:    &priority,
					RelatedID:   &task.ID,
					RelatedType: "task",
					Data: map[string]interface{}{
						"task_id":   task.ID,
						"sender_id": userID, // ID of person who changed due date
					},
					Channels:    []string{"in_app", "email", "push"},
				}

				// Send notification async
				go func(req *clients.NotificationRequest) {
					if err := u.notificationClient.SendNotification(req); err != nil {
						fmt.Printf("Failed to send due date change notification to user %d: %v\n", req.UserID, err)
					}
				}(notificationReq)
			}
		}
	}

	// Update assignees if provided
	if req.AssigneeIDs != nil {
		// Delete existing assignees
		if err := u.taskRepo.DeleteAllAssignees(taskID); err != nil {
			return nil, fmt.Errorf("failed to delete existing assignees: %w", err)
		}

		// Create new assignees
		for _, assigneeID := range req.AssigneeIDs {
			assignee := &models.TaskAssignee{
				TaskID: taskID,
				UserID: assigneeID,
			}
			if err := u.taskRepo.CreateAssignee(assignee); err != nil {
				return nil, fmt.Errorf("failed to create task assignee: %w", err)
			}
		}

		// Reload task to get updated assignees
		task, err = u.taskRepo.GetByID(taskID)
		if err != nil {
			return nil, fmt.Errorf("failed to reload task: %w", err)
		}
	}

	// Recalculate progress if status changed (for tasks without subtasks/checklists)
	fmt.Printf("[UpdateTask] Task ID: %d - statusChanged=%v\n", taskID, statusChanged)
	if statusChanged {
		fmt.Printf("[UpdateTask] Task ID: %d - calling RecalculateTaskProgress\n", taskID)
		u.RecalculateTaskProgress(taskID)
		// Reload task to get updated progress
		task, _ = u.taskRepo.GetByID(taskID)
		fmt.Printf("[UpdateTask] Task ID: %d - after recalculation, progress=%d%%\n", taskID, task.ProgressPercentage)
	}

	response := task.ToResponse()

	// Enrich with user info
	if err := u.enrichTaskWithUserInfo(response); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich task with user info: %v\n", err)
	}

	return response, nil
}

// DeleteTask deletes a task
func (u *taskUsecase) DeleteTask(userID uint, userRole sharedmodels.Role, taskID uint) error {
	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("task not found")
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: only creator or super_admin can delete
	isCreator := task.CreatedBy == userID
	isSuperAdmin := userRole == sharedmodels.RoleSuperAdmin

	if !isCreator && !isSuperAdmin {
		return fmt.Errorf("access denied: only task creator or super administrator can delete the task")
	}

	// Save ParentTaskID before deleting (for progress recalculation)
	parentTaskID := task.ParentTaskID

	// Log activity before deleting
	u.logActivity(taskID, userID, "task_deleted", task.Title, "Task deleted", map[string]interface{}{
		"task_id":    taskID,
		"task_title": task.Title,
		"deleted_by": userID,
	})

	// Delete task
	if err := u.taskRepo.Delete(taskID); err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	// Record deletion for sync tracking
	if u.syncRepo != nil {
		if err := u.syncRepo.RecordDeletion(taskID, &userID); err != nil {
			// Log error but don't fail the request - deletion tracking is not critical
			fmt.Printf("[DeleteTask] Failed to record deletion for sync: %v\n", err)
		}
	}

	// If task had a parent, recalculate parent progress after deletion
	if parentTaskID != nil {
		fmt.Printf("[DeleteTask] Task ID: %d had parent task ID: %d, recalculating parent progress after deletion\n", taskID, *parentTaskID)

		// Update parent task's updated_at timestamp to reflect the subtask deletion
		if parentTask, err := u.taskRepo.GetByID(*parentTaskID); err == nil {
			if err := u.taskRepo.Update(parentTask); err != nil {
				fmt.Printf("Warning: failed to update parent task timestamp after subtask deletion: %v\n", err)
			}
		}

		u.RecalculateTaskProgress(*parentTaskID)
	}

	return nil
}

// AssignTask assigns a task to a user
func (u *taskUsecase) AssignTask(userID, taskID uint, req *models.AssignTaskRequest) (*models.TaskResponse, error) {
	// Validate request
	if err := u.validateAssignTaskRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: only creator or current assignee can reassign
	// Parent task creator cannot edit subtasks (only view them)
	if !u.hasTaskEditAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Store old assignee for logging
	oldAssignee := uint(0)
	if task.AssignedTo != nil {
		oldAssignee = *task.AssignedTo
	}

	// Assign task
	task.AssignedTo = &req.AssignedTo

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to assign task: %w", err)
	}

	// Log activity
	u.logActivity(taskID, userID, "task_assigned", fmt.Sprintf("User %d", oldAssignee), fmt.Sprintf("User %d", req.AssignedTo), map[string]interface{}{
		"old_assignee": oldAssignee,
		"new_assignee": req.AssignedTo,
		"assigned_by":  userID,
	})

	return task.ToResponse(), nil
}

// UnassignTask removes assignment from a task
func (u *taskUsecase) UnassignTask(userID, taskID uint) (*models.TaskResponse, error) {
	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: only creator or current assignee can unassign
	// Parent task creator cannot edit subtasks (only view them)
	if !u.hasTaskEditAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Unassign task
	task.AssignedTo = nil

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to unassign task: %w", err)
	}

	return task.ToResponse(), nil
}

// UpdateTaskStatus updates only the status of a task
func (u *taskUsecase) UpdateTaskStatus(userID, taskID uint, req *models.UpdateTaskStatusRequest) (*models.TaskResponse, error) {
	// Validate request
	if err := u.validateUpdateTaskStatusRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: use permissions system to verify can_change_status
	ctx := context.Background()

	// Debug logging
	fmt.Printf("UpdateTaskStatus - Task ID: %d, User ID: %d\n", taskID, userID)
	fmt.Printf("Task CreatedByUserID: %d, ParentTaskID: %v\n", task.CreatedByUserID, task.ParentTaskID)
	fmt.Printf("Task Assignees: %+v\n", task.Assignees)

	// Get user role for debugging
	role, _ := permissions.GetUserTaskRole(ctx, task, userID)
	fmt.Printf("User role: %s\n", role)

	canChangeStatus, err := permissions.HasPermission(ctx, task, userID, "change_status")
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %w", err)
	}

	fmt.Printf("Can change status: %v\n", canChangeStatus)

	if !canChangeStatus {
		return nil, fmt.Errorf("access denied: insufficient permissions to change task status")
	}

	// Update status
	oldStatus := task.Status
	task.Status = req.Status
	task.LastStatusChangedBy = &userID

	// If status changed to "done", set completed_at
	if req.Status == models.TaskStatusDone && task.CompletedAt == nil {
		now := time.Now()
		task.CompletedAt = &now
	}

	// Debug: Log ParentTaskID before Update
	if task.ParentTaskID != nil {
		fmt.Printf("[UpdateTaskStatus] Before Update - Task ID: %d has ParentTaskID: %d\n", taskID, *task.ParentTaskID)
	} else {
		fmt.Printf("[UpdateTaskStatus] Before Update - Task ID: %d has NO ParentTaskID\n", taskID)
	}

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// Debug: Log ParentTaskID after Update
	if task.ParentTaskID != nil {
		fmt.Printf("[UpdateTaskStatus] After Update - Task ID: %d has ParentTaskID: %d\n", taskID, *task.ParentTaskID)
	} else {
		fmt.Printf("[UpdateTaskStatus] After Update - Task ID: %d has NO ParentTaskID\n", taskID)
	}

	// Log activity
	u.logActivity(taskID, userID, "task_status_changed", string(oldStatus), string(req.Status), map[string]interface{}{
		"old_status": oldStatus,
		"new_status": req.Status,
		"changed_at": time.Now(),
	})

	// Send notification when task moved to review status
	if req.Status == models.TaskStatusReview && oldStatus != models.TaskStatusReview {
		// Determine who should receive the notification:
		// - If task is delegated -> notify delegator (DelegatedFromUserID)
		// - Otherwise -> notify creator (CreatedByUserID)
		var recipientID uint
		if task.DelegatedFromUserID != nil {
			recipientID = *task.DelegatedFromUserID
		} else {
			recipientID = task.CreatedByUserID
		}

		// Don't send notification to self
		if recipientID != userID {
			// Get assignee info for notification message
			assigneeInfo, err := u.userClient.GetUserByID(userID)
			assigneeName := "Кто-то"
			if err == nil && assigneeInfo != nil {
				assigneeName = assigneeInfo.Name
			}

			priority := string(task.Priority)
			notificationReq := &clients.NotificationRequest{
				UserID:      recipientID,
				Type:        "task",
				Title:       "✅ Задача сдана на проверку",
				Message:     fmt.Sprintf("%s сдал(а) задачу на проверку: %s", assigneeName, task.Title),
				Priority:    &priority,
				RelatedID:   &task.ID,
				RelatedType: "task",
				Data: map[string]interface{}{
					"task_id":   task.ID,
					"sender_id": userID, // ID of person who submitted for review
				},
				Channels:    []string{"in_app", "email", "push"},
			}

			// Send notification async
			go func(req *clients.NotificationRequest) {
				if err := u.notificationClient.SendNotification(req); err != nil {
					fmt.Printf("Failed to send review notification to user %d: %v\n", req.UserID, err)
				}
			}(notificationReq)
		}
	}

	// Send notification when task returned to work from review/done
	if (req.Status == models.TaskStatusInProgress || req.Status == models.TaskStatusNew) &&
	   (oldStatus == models.TaskStatusReview || oldStatus == models.TaskStatusDone) {
		// Use assignees from task object
		if len(task.Assignees) > 0 {
			// Get reviewer info (person who returned the task)
			reviewerInfo, err := u.userClient.GetUserByID(userID)
			reviewerName := "Кто-то"
			if err == nil && reviewerInfo != nil {
				reviewerName = reviewerInfo.Name
			}

			priority := string(task.Priority)

			// Send notification to each assignee
			for _, assignee := range task.Assignees {
				// Don't send notification to self
				if assignee.UserID == userID {
					continue
				}

				notificationReq := &clients.NotificationRequest{
					UserID:      assignee.UserID,
					Type:        "task",
					Title:       "🔄 Задача возвращена на доработку",
					Message:     fmt.Sprintf("%s вернул(а) задачу на доработку: %s", reviewerName, task.Title),
					Priority:    &priority,
					RelatedID:   &task.ID,
					RelatedType: "task",
					Data: map[string]interface{}{
						"task_id":   task.ID,
						"sender_id": userID, // ID of reviewer who returned the task
					},
					Channels:    []string{"in_app", "email", "push"},
				}

				// Send notification async
				go func(req *clients.NotificationRequest) {
					if err := u.notificationClient.SendNotification(req); err != nil {
						fmt.Printf("Failed to send return-to-work notification to user %d: %v\n", req.UserID, err)
					}
				}(notificationReq)
			}
		}
	}

	// Send notification when task is cancelled
	if req.Status == models.TaskStatusCancelled && oldStatus != models.TaskStatusCancelled {
		go u.sendTaskCancelledNotification(task, userID)
	}

	// Send notification when task is completed
	if req.Status == models.TaskStatusDone && oldStatus != models.TaskStatusDone {
		go u.sendTaskCompletedNotification(task, userID)

		// If this is a subtask, notify parent task creator
		if task.ParentTaskID != nil {
			go u.sendSubtaskCompletedNotification(task, *task.ParentTaskID, userID)
		}
	}

	// Recalculate progress (for all tasks, regardless of subtasks/checklists)
	u.RecalculateTaskProgress(taskID)

	// If task has parent, recalculate parent progress and log activity
	if task.ParentTaskID != nil {
		// Update parent task's updated_at timestamp to reflect the subtask status change
		if parentTask, err := u.taskRepo.GetByID(*task.ParentTaskID); err == nil {
			if err := u.taskRepo.Update(parentTask); err != nil {
				fmt.Printf("Warning: failed to update parent task timestamp after subtask status change: %v\n", err)
			}
		}

		u.RecalculateTaskProgress(*task.ParentTaskID)

		// Log activity in parent task about subtask status change
		statusChangeMessage := fmt.Sprintf("Подзадача '%s' изменила статус: %s → %s", task.Title, oldStatus, req.Status)
		u.logActivity(*task.ParentTaskID, userID, "subtask_status_changed", string(oldStatus), string(req.Status), map[string]interface{}{
			"subtask_id":    taskID,
			"subtask_title": task.Title,
			"old_status":    oldStatus,
			"new_status":    req.Status,
			"message":       statusChangeMessage,
		})
	}

	// Reload task to get updated progress
	task, _ = u.taskRepo.GetByID(taskID)

	response := task.ToResponse()

	// Enrich with user info
	if err := u.enrichTaskWithUserInfo(response); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich task with user info: %v\n", err)
	}

	return response, nil
}

// GetUserTasks retrieves tasks for a user with filtering
func (u *taskUsecase) GetUserTasks(userID uint, userRole sharedmodels.Role, filter *models.TaskFilterRequest) ([]*models.TaskResponse, int64, error) {
	// Set default pagination if not provided
	if filter == nil {
		filter = &models.TaskFilterRequest{
			Limit:  20,
			Offset: 0,
		}
	}

	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Get tasks from repository
	var tasks []*models.Task
	var total int64
	var err error

	// Only super_admin can see all tasks
	if userRole == sharedmodels.RoleSuperAdmin {
		tasks, total, err = u.taskRepo.GetAllTasks(filter)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get all tasks: %w", err)
		}
	} else {
		// Admin and regular users only see their own tasks (created by or assigned to)
		tasks, total, err = u.taskRepo.GetUserTasks(userID, filter)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user tasks: %w", err)
		}
	}

	// Convert to response format
	responses := make([]*models.TaskResponse, len(tasks))
	for i, task := range tasks {
		responses[i] = task.ToResponse()
	}

	// Enrich all tasks with user info in a single batch
	if err := u.enrichTasksWithUserInfo(responses); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich tasks with user info: %v\n", err)
	}

	// Enrich all tasks with permissions
	ctx := context.Background()
	if err := u.enrichTasksWithPermissions(ctx, responses, userID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to enrich tasks with permissions: %v\n", err)
	}

	return responses, total, nil
}

// enrichTasksWithUserInfo enriches multiple tasks with user information in a single batch
func (u *taskUsecase) enrichTasksWithUserInfo(responses []*models.TaskResponse) error {
	if len(responses) == 0 {
		return nil
	}

	// Collect all unique user IDs from all tasks (including delegation chain IDs)
	uniqueIDs := make(map[uint]bool)
	for _, response := range responses {
		uniqueIDs[response.CreatedBy] = true
		for _, assigneeID := range response.AssigneeIDs {
			uniqueIDs[assigneeID] = true
		}
		if response.LastStatusChangedBy != nil {
			uniqueIDs[*response.LastStatusChangedBy] = true
		}
		// Collect delegation chain IDs
		if response.DelegatedFromUserID != nil {
			uniqueIDs[*response.DelegatedFromUserID] = true
		}
		if response.OriginalAssigneeID != nil {
			uniqueIDs[*response.OriginalAssigneeID] = true
		}
		if response.AssignedToUserID != nil {
			uniqueIDs[*response.AssignedToUserID] = true
		}
	}

	if len(uniqueIDs) == 0 {
		return nil
	}

	// Convert to slice
	finalIDs := make([]uint, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		finalIDs = append(finalIDs, id)
	}

	// Fetch users from user-service (single batch request)
	users, err := u.userClient.GetUsersByIDs(finalIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch users: %w", err)
	}

	// Enrich each task
	for _, response := range responses {
		// Set creator info
		if creator, exists := users[response.CreatedBy]; exists {
			response.Creator = &models.UserInfo{
				ID:       creator.ID,
				Name:     creator.Name,
				Email:    creator.Email,
				Avatar:   creator.Avatar,
				Position: creator.Position,
			}
		}

		// Set assignees info
		response.Assignees = make([]models.UserInfo, 0, len(response.AssigneeIDs))
		for _, assigneeID := range response.AssigneeIDs {
			if assignee, exists := users[assigneeID]; exists {
				response.Assignees = append(response.Assignees, models.UserInfo{
					ID:       assignee.ID,
					Name:     assignee.Name,
					Email:    assignee.Email,
					Avatar:   assignee.Avatar,
					Position: assignee.Position,
				})
			}
		}

		// Set last status changer info
		if response.LastStatusChangedBy != nil {
			if statusChanger, exists := users[*response.LastStatusChangedBy]; exists {
				response.LastStatusChanger = &models.UserInfo{
					ID:       statusChanger.ID,
					Name:     statusChanger.Name,
					Email:    statusChanger.Email,
					Avatar:   statusChanger.Avatar,
					Position: statusChanger.Position,
				}
			}
		}

		// Set delegated from user info
		if response.DelegatedFromUserID != nil {
			if delegatedFrom, exists := users[*response.DelegatedFromUserID]; exists {
				response.DelegatedFromUser = &models.UserInfo{
					ID:       delegatedFrom.ID,
					Name:     delegatedFrom.Name,
					Email:    delegatedFrom.Email,
					Avatar:   delegatedFrom.Avatar,
					Position: delegatedFrom.Position,
				}
			}
		}

		// Set original assignee info
		if response.OriginalAssigneeID != nil {
			if originalAssignee, exists := users[*response.OriginalAssigneeID]; exists {
				response.OriginalAssignee = &models.UserInfo{
					ID:       originalAssignee.ID,
					Name:     originalAssignee.Name,
					Email:    originalAssignee.Email,
					Avatar:   originalAssignee.Avatar,
					Position: originalAssignee.Position,
				}
			}
		}

		// Set assigned to user info
		if response.AssignedToUserID != nil {
			if assignedTo, exists := users[*response.AssignedToUserID]; exists {
				response.AssignedToUser = &models.UserInfo{
					ID:       assignedTo.ID,
					Name:     assignedTo.Name,
					Email:    assignedTo.Email,
					Avatar:   assignedTo.Avatar,
					Position: assignedTo.Position,
				}
			}
		}

		// Build delegation chain for delegated tasks
		if response.DelegatedFromUserID != nil || response.OriginalAssigneeID != nil {
			response.DelegationChain = u.buildDelegationChain(response, users)
		}
	}

	return nil
}

// buildDelegationChain builds the delegation chain from collected user data
func (u *taskUsecase) buildDelegationChain(response *models.TaskResponse, users map[uint]*clients.UserInfo) []models.UserInfo {
	chain := make([]models.UserInfo, 0)
	addedIDs := make(map[uint]bool)

	// Helper to add user to chain if exists and not already added
	addToChain := func(userID uint) {
		if addedIDs[userID] {
			return
		}
		if user, exists := users[userID]; exists && user != nil {
			chain = append(chain, models.UserInfo{
				ID:       user.ID,
				Name:     user.Name,
				Email:    user.Email,
				Avatar:   user.Avatar,
				Position: user.Position,
			})
			addedIDs[userID] = true
		}
	}

	// Add creator first
	addToChain(response.CreatedByUserID)

	// Add original assignee if different from creator
	if response.OriginalAssigneeID != nil && *response.OriginalAssigneeID != response.CreatedByUserID {
		addToChain(*response.OriginalAssigneeID)
	}

	// Add delegated from user
	if response.DelegatedFromUserID != nil {
		addToChain(*response.DelegatedFromUserID)
	}

	// Add current assignee
	if response.AssignedToUserID != nil {
		addToChain(*response.AssignedToUserID)
	}

	return chain
}

// GetTaskStats retrieves task statistics for a user
func (u *taskUsecase) GetTaskStats(userID uint) (*models.TaskStatsResponse, error) {
	stats, err := u.taskRepo.GetTaskStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task stats: %w", err)
	}

	return stats, nil
}

// GetTaskStatsInternal returns task statistics for analytics (no user filtering) with optional period filter
func (u *taskUsecase) GetTaskStatsInternal(period string) (*models.TaskStatsInternalResponse, error) {
	var timeRange *repository.TimeRange
	if period != "" {
		tr, err := u.parseTimeRange(period)
		if err != nil {
			return nil, err
		}
		timeRange = tr
	}
	return u.taskRepo.GetTaskStatsInternal(timeRange)
}

// Helper methods

// hasTaskAccess checks if user has access to the task (creator or assignee or in delegation chain)
func (u *taskUsecase) hasTaskAccess(userID uint, task *models.Task) bool {
	// User is creator
	if task.CreatedBy == userID {
		return true
	}

	// User is assignee (old field for backward compatibility)
	if task.AssignedTo != nil && *task.AssignedTo == userID {
		return true
	}

	// User is in task_assignees (new many-to-many relationship)
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			return true
		}
	}

	// Check if user is in delegation chain
	if task.DelegatedFromUserID != nil && *task.DelegatedFromUserID == userID {
		return true
	}

	// Check if user was the original assignee
	if task.OriginalAssigneeID != nil && *task.OriginalAssigneeID == userID {
		return true
	}

	// Check if user is creator of parent task (for subtasks)
	if task.ParentTaskID != nil {
		parentTask, err := u.taskRepo.GetByID(*task.ParentTaskID)
		if err == nil && parentTask.CreatedBy == userID {
			return true
		}
	}

	return false
}

// hasTaskEditAccess checks if user has edit access to the task
// This is more restrictive than hasTaskAccess - doesn't allow parent task creator to edit subtasks
func (u *taskUsecase) hasTaskEditAccess(userID uint, task *models.Task) bool {
	// User is creator
	if task.CreatedBy == userID {
		return true
	}

	// User is assignee (old field for backward compatibility)
	if task.AssignedTo != nil && *task.AssignedTo == userID {
		return true
	}

	// User is in task_assignees (new many-to-many relationship)
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			return true
		}
	}

	// Parent task creator cannot edit subtasks
	return false
}

// Validation methods

// validateCreateTaskRequest validates task creation request
func (u *taskUsecase) validateCreateTaskRequest(req *models.CreateTaskRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate title
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return fmt.Errorf("task title is required")
	}
	if len(title) > 255 {
		return fmt.Errorf("task title must be less than 255 characters")
	}

	// Validate description if provided
	if req.Description != "" {
		description := strings.TrimSpace(req.Description)
		if len(description) > 2000 {
			return fmt.Errorf("task description must be less than 2000 characters")
		}
	}

	// Validate priority if provided
	if req.Priority != nil {
		if !u.isValidPriority(*req.Priority) {
			return fmt.Errorf("invalid priority value")
		}
	}

	// Validate assignee if provided
	if req.AssignedTo != nil && *req.AssignedTo == 0 {
		return fmt.Errorf("invalid assignee ID")
	}

	// Validate due date if provided
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		return fmt.Errorf("due date cannot be in the past")
	}

	return nil
}

// validateUpdateTaskRequest validates task update request
func (u *taskUsecase) validateUpdateTaskRequest(req *models.UpdateTaskRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate title if provided
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return fmt.Errorf("task title cannot be empty")
		}
		if len(title) > 255 {
			return fmt.Errorf("task title must be less than 255 characters")
		}
	}

	// Validate description if provided
	if req.Description != nil {
		description := strings.TrimSpace(*req.Description)
		if len(description) > 2000 {
			return fmt.Errorf("task description must be less than 2000 characters")
		}
	}

	// Validate status if provided
	if req.Status != nil {
		if !u.isValidStatus(*req.Status) {
			return fmt.Errorf("invalid status value")
		}
	}

	// Validate priority if provided
	if req.Priority != nil {
		if !u.isValidPriority(*req.Priority) {
			return fmt.Errorf("invalid priority value")
		}
	}

	// Validate assignee if provided
	if req.AssignedTo != nil && *req.AssignedTo == 0 {
		return fmt.Errorf("invalid assignee ID")
	}

	return nil
}

// validateUpdateTaskStatusRequest validates task status update request
func (u *taskUsecase) validateUpdateTaskStatusRequest(req *models.UpdateTaskStatusRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if !u.isValidStatus(req.Status) {
		return fmt.Errorf("invalid status value")
	}

	return nil
}

// validateAssignTaskRequest validates task assignment request
func (u *taskUsecase) validateAssignTaskRequest(req *models.AssignTaskRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.AssignedTo == 0 {
		return fmt.Errorf("assignee ID is required")
	}

	return nil
}

// isValidStatus checks if the status is valid
func (u *taskUsecase) isValidStatus(status models.TaskStatus) bool {
	switch status {
	case models.TaskStatusNew, models.TaskStatusViewed, models.TaskStatusInProgress,
		models.TaskStatusReview, models.TaskStatusDone, models.TaskStatusCancelled:
		return true
	default:
		return false
	}
}

// isValidPriority checks if the priority is valid
func (u *taskUsecase) isValidPriority(priority models.TaskPriority) bool {
	switch priority {
	case models.TaskPriorityLow, models.TaskPriorityMedium,
		models.TaskPriorityHigh, models.TaskPriorityCritical:
		return true
	default:
		return false
	}
}

// --- NEW METHODS FOR HIERARCHY, DELEGATION, AND TRACKING ---

// CreateSubtask creates a subtask under a parent task
func (u *taskUsecase) CreateSubtask(userID uint, parentTaskID uint, req *models.CreateTaskRequest) (*models.TaskResponse, error) {
	// Validate request
	if err := u.validateCreateTaskRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if parent task exists
	parentTask, err := u.taskRepo.GetByID(parentTaskID)
	if err != nil {
		return nil, fmt.Errorf("parent task not found: %w", err)
	}

	// Check permissions - only task creator, assignees, or admin can create subtasks
	if !u.hasTaskAccess(userID, parentTask) {
		return nil, fmt.Errorf("access denied: insufficient permissions to create subtask")
	}

	// Create subtask
	task := &models.Task{
		ParentTaskID:    &parentTaskID,
		Title:           strings.TrimSpace(req.Title),
		Description:     strings.TrimSpace(req.Description),
		CreatedBy:       userID,
		CreatedByUserID: userID,
		DueDate:         req.DueDate,
		Status:          models.TaskStatusNew,
	}

	// Set priority (default to medium)
	if req.Priority != nil {
		task.Priority = *req.Priority
	} else {
		task.Priority = models.TaskPriorityMedium
	}

	// Save subtask
	if err := u.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create subtask: %w", err)
	}

	// Create assignees if provided
	assigneeIDs := req.AssigneeIDs
	if len(assigneeIDs) == 0 {
		assigneeIDs = []uint{userID} // Assign to creator
	}

	for _, assigneeID := range assigneeIDs {
		assignee := &models.TaskAssignee{
			TaskID: task.ID,
			UserID: assigneeID,
		}
		if err := u.taskRepo.CreateAssignee(assignee); err != nil {
			return nil, fmt.Errorf("failed to create task assignee: %w", err)
		}
		task.Assignees = append(task.Assignees, *assignee)
	}

	// Copy attachments from parent task if specified
	if len(req.ParentAttachmentIDs) > 0 && u.attachmentUsecase != nil {
		err := u.attachmentUsecase.CopyAttachmentsToTask(parentTaskID, task.ID, userID, req.ParentAttachmentIDs)
		if err != nil {
			// Log warning but don't fail subtask creation
			fmt.Printf("Warning: failed to copy attachments to subtask: %v\n", err)
		}
	}

	// Log activity for parent task with assignee information
	details := map[string]interface{}{
		"subtask_id":    task.ID,
		"subtask_title": task.Title,
		"assignee_ids":  assigneeIDs,
	}
	u.logActivity(parentTaskID, userID, "subtask_created", "", task.Title, details)

	// Update parent task's updated_at timestamp to reflect the subtask creation
	if err := u.taskRepo.Update(parentTask); err != nil {
		fmt.Printf("Warning: failed to update parent task timestamp after creating subtask: %v\n", err)
	}

	// Recalculate parent progress
	u.RecalculateTaskProgress(parentTaskID)

	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

	// Send notifications to assignees
	if len(assigneeIDs) > 0 {
		for _, assigneeID := range assigneeIDs {
			if assigneeID == userID {
				continue // Skip notification to creator
			}

			// Get creator name for notification message
			creatorName := "Кто-то"
			if response.Creator != nil {
				creatorName = response.Creator.Name
			}

			// Prepare notification
			priority := string(task.Priority) // Convert task priority to string
			notificationReq := &clients.NotificationRequest{
				UserID:      assigneeID,
				Type:        "task",
				Title:       "✅ Новая подзадача назначена",
				Message:     fmt.Sprintf("%s назначил(а) вам подзадачу: %s", creatorName, task.Title),
				Priority:    &priority, // Pointer to priority string
				RelatedID:   &task.ID,
				RelatedType: "task",
				// ActionURL not set - validator requires full URL (with scheme and host), not just path
				Data: map[string]interface{}{
					"task_id":    task.ID,
					"creator_id": userID, // Add creator_id for sender info enrichment
				},
				Channels:    []string{"in_app", "email", "push"},
			}

			// Send notification (async, don't block on error)
			go func(req *clients.NotificationRequest) {
				if err := u.notificationClient.SendNotification(req); err != nil {
					fmt.Printf("Failed to send subtask notification to user %d: %v\n", req.UserID, err)
				}
			}(notificationReq)
		}
	}

	return response, nil
}

// GetSubtasks retrieves all subtasks for a parent task
func (u *taskUsecase) GetSubtasks(userID uint, parentTaskID uint) ([]*models.TaskResponse, error) {
	// Check if parent task exists and user has access
	parentTask, err := u.taskRepo.GetByID(parentTaskID)
	if err != nil {
		return nil, fmt.Errorf("parent task not found: %w", err)
	}

	if !u.hasTaskAccess(userID, parentTask) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Get subtasks
	subtasks, err := u.taskRepo.GetSubtasks(parentTaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtasks: %w", err)
	}

	// Convert to responses
	responses := make([]*models.TaskResponse, len(subtasks))
	for i, subtask := range subtasks {
		responses[i] = subtask.ToResponse()
	}

	// Enrich with user info
	u.enrichTasksWithUserInfo(responses)

	return responses, nil
}

// GetTaskHierarchy retrieves a task with all its subtasks and parent
func (u *taskUsecase) GetTaskHierarchy(userID uint, taskID uint) (*models.TaskResponse, error) {
	// Get task with details
	task, err := u.taskRepo.GetByIDWithDetails(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

	return response, nil
}

// DelegateTask delegates a task from one user to another
func (u *taskUsecase) DelegateTask(userID uint, taskID uint, toUserID uint) (*models.TaskResponse, error) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Check permissions - only current assignee or task creator can delegate
	// Parent task creator cannot delegate subtasks (only view them)
	if !u.hasTaskEditAccess(userID, task) {
		return nil, fmt.Errorf("access denied: only task assignee or creator can delegate")
	}

	// Store original assignee if not already set
	if task.OriginalAssigneeID == nil {
		task.OriginalAssigneeID = &userID
	}

	// Set delegation fields
	task.DelegatedFromUserID = &userID
	task.AssignedToUserID = &toUserID

	// Update task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to delegate task: %w", err)
	}

	// Remove all current assignees from task_assignees (they will be in delegation chain)
	// The new assignee becomes the sole active assignee
	if err := u.taskRepo.RemoveAllAssignees(taskID); err != nil {
		return nil, fmt.Errorf("failed to remove old assignees: %w", err)
	}

	// Add new assignee to task_assignees
	newAssignee := &models.TaskAssignee{
		TaskID: taskID,
		UserID: toUserID,
	}
	if err := u.taskRepo.CreateAssignee(newAssignee); err != nil {
		return nil, fmt.Errorf("failed to add new assignee: %w", err)
	}

	// Send notification to new assignee (if not delegating to self)
	if toUserID != userID {
		// Get delegator info for notification message
		delegatorInfo, err := u.userClient.GetUserByID(userID)
		delegatorName := "Кто-то"
		if err == nil && delegatorInfo != nil {
			delegatorName = delegatorInfo.Name
		}

		priority := string(task.Priority)
		notificationReq := &clients.NotificationRequest{
			UserID:      toUserID,
			Type:        "task",
			Title:       "📋 Задача делегирована вам",
			Message:     fmt.Sprintf("%s делегировал(а) вам задачу: %s", delegatorName, task.Title),
			Priority:    &priority,
			RelatedID:   &task.ID,
			RelatedType: "task",
			Data: map[string]interface{}{
				"task_id":   task.ID,
				"sender_id": userID, // ID of person who delegated the task
			},
			Channels:    []string{"in_app", "email", "push"},
		}

		// Send notification async
		go func(req *clients.NotificationRequest) {
			if err := u.notificationClient.SendNotification(req); err != nil {
				fmt.Printf("Failed to send delegation notification to user %d: %v\n", req.UserID, err)
			}
		}(notificationReq)
	}

	// Log activity
	u.logActivity(taskID, userID, "task_delegated", fmt.Sprintf("User %d", userID), fmt.Sprintf("User %d", toUserID), map[string]interface{}{
		"from_user_id": userID,
		"to_user_id":   toUserID,
		"delegated_at": time.Now(),
	})

	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

	return response, nil
}

// GetDelegationChain retrieves the delegation chain for a task
func (u *taskUsecase) GetDelegationChain(userID uint, taskID uint) ([]models.UserInfo, error) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Build delegation chain
	chain := make([]uint, 0)

	// Add original creator
	chain = append(chain, task.CreatedByUserID)

	// Add original assignee if different from creator
	if task.OriginalAssigneeID != nil && *task.OriginalAssigneeID != task.CreatedByUserID {
		chain = append(chain, *task.OriginalAssigneeID)
	}

	// Add delegated from user if exists
	if task.DelegatedFromUserID != nil {
		exists := false
		for _, id := range chain {
			if id == *task.DelegatedFromUserID {
				exists = true
				break
			}
		}
		if !exists {
			chain = append(chain, *task.DelegatedFromUserID)
		}
	}

	// Add current assignee if exists
	if task.AssignedToUserID != nil {
		exists := false
		for _, id := range chain {
			if id == *task.AssignedToUserID {
				exists = true
				break
			}
		}
		if !exists {
			chain = append(chain, *task.AssignedToUserID)
		}
	}

	// Fetch user information
	if len(chain) == 0 {
		return []models.UserInfo{}, nil
	}

	users, err := u.userClient.GetUsersByIDs(chain)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users: %w", err)
	}

	// Build user info chain in order
	result := make([]models.UserInfo, 0, len(chain))
	for _, userID := range chain {
		if user, exists := users[userID]; exists {
			result = append(result, models.UserInfo{
				ID:       user.ID,
				Name:     user.Name,
				Email:    user.Email,
				Avatar:   user.Avatar,
				Position: user.Position,
			})
		}
	}

	return result, nil
}

// MarkTaskAsViewed marks a task as viewed by the user (first-view tracking)
func (u *taskUsecase) MarkTaskAsViewed(userID uint, taskID uint) (*models.TaskResponse, error) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Check if user has access
	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// If task is new and hasn't been viewed yet, mark as viewed
	if task.Status == models.TaskStatusNew && task.FirstViewedAt == nil {
		now := time.Now()
		task.FirstViewedAt = &now
		task.FirstViewedByUserID = &userID
		task.Status = models.TaskStatusViewed

		if err := u.taskRepo.Update(task); err != nil {
			return nil, fmt.Errorf("failed to update task: %w", err)
		}

		// Log activity
		u.logActivity(taskID, userID, "task_viewed", string(models.TaskStatusNew), string(models.TaskStatusViewed), nil)
	}

	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

	return response, nil
}

// RecalculateTaskProgress recalculates progress based on subtasks and checklists
func (u *taskUsecase) RecalculateTaskProgress(taskID uint) error {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	fmt.Printf("[RecalculateTaskProgress] Task ID: %d, Title: %s, Current Progress: %d%%\n", taskID, task.Title, task.ProgressPercentage)

	// Count subtasks
	totalSubtasks, err := u.taskRepo.CountSubtasks(taskID)
	if err != nil {
		return fmt.Errorf("failed to count subtasks: %w", err)
	}

	// Count checklist items
	totalChecklistItems, err := u.checklistRepo.CountChecklistItemsByTaskID(taskID)
	if err != nil {
		return fmt.Errorf("failed to count checklist items: %w", err)
	}

	fmt.Printf("[RecalculateTaskProgress] Task ID: %d has %d subtasks and %d checklist items\n", taskID, totalSubtasks, totalChecklistItems)

	// If no subtasks and no checklist items, calculate progress based on status
	if totalSubtasks == 0 && totalChecklistItems == 0 {
		progress := u.calculateProgressByStatus(task.Status)
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - calculating by status '%s': %d%%\n", taskID, task.Status, progress)
		if task.ProgressPercentage != progress {
			fmt.Printf("[RecalculateTaskProgress] Task ID: %d - updating progress from %d%% to %d%%\n", taskID, task.ProgressPercentage, progress)

			// IMPORTANT: Save ParentTaskID BEFORE UpdateProgress
			parentTaskID := task.ParentTaskID
			fmt.Printf("[RecalculateTaskProgress] Task ID: %d - saved ParentTaskID: %v\n", taskID, parentTaskID)

			if err := u.taskRepo.UpdateProgress(taskID, progress); err != nil {
				return fmt.Errorf("failed to update progress: %w", err)
			}

			fmt.Printf("[RecalculateTaskProgress] Task ID: %d - after UpdateProgress, checking parent...\n", taskID)

			// If task has parent, recalculate parent progress too
			if parentTaskID != nil {
				fmt.Printf("[RecalculateTaskProgress] Task ID: %d has parent task ID: %d, recalculating parent progress\n", taskID, *parentTaskID)
				u.RecalculateTaskProgress(*parentTaskID)
			} else {
				fmt.Printf("[RecalculateTaskProgress] Task ID: %d has NO parent (ParentTaskID is nil)\n", taskID)
			}
			return nil
		}
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - progress unchanged, skipping update\n", taskID)
		return nil
	}

	var progress int

	// If task has only checklist items (no subtasks)
	if totalSubtasks == 0 && totalChecklistItems > 0 {
		completedChecklistItems, err := u.checklistRepo.CountCompletedChecklistItemsByTaskID(taskID)
		if err != nil {
			return fmt.Errorf("failed to count completed checklist items: %w", err)
		}
		progress = int((float64(completedChecklistItems) / float64(totalChecklistItems)) * 100)
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - calculating by checklist: %d/%d completed = %d%%\n", taskID, completedChecklistItems, totalChecklistItems, progress)
	} else if totalSubtasks > 0 {
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - has subtasks, calculating by average subtask progress\n", taskID)
		// If task has subtasks, calculate progress based on subtasks
		// Each subtask contributes to the overall progress based on its own progress

		// Get all subtasks with their progress
		subtasks, err := u.taskRepo.GetSubtasks(taskID)
		if err != nil {
			return fmt.Errorf("failed to get subtasks: %w", err)
		}

		// Calculate average progress of all subtasks
		totalProgress := 0
		for _, subtask := range subtasks {
			// First recalculate subtask progress (in case it has checklists)
			if err := u.RecalculateTaskProgress(subtask.ID); err != nil {
				fmt.Printf("WARNING: Failed to recalculate progress for subtask %d: %v\n", subtask.ID, err)
				// Continue with current progress value instead of failing
			}

			// Reload subtask to get updated progress after recalculation
			updatedSubtask, err := u.taskRepo.GetByID(subtask.ID)
			if err != nil {
				fmt.Printf("WARNING: Failed to reload subtask %d for progress calculation: %v\n", subtask.ID, err)
				// Use the original subtask progress if reload fails
				totalProgress += subtask.ProgressPercentage
			} else {
				totalProgress += updatedSubtask.ProgressPercentage
			}
		}

		if totalSubtasks > 0 {
			progress = totalProgress / int(totalSubtasks)
		}
	}

	// Update progress if changed
	if task.ProgressPercentage != progress {
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - updating progress from %d%% to %d%%\n", taskID, task.ProgressPercentage, progress)

		// IMPORTANT: Save ParentTaskID BEFORE UpdateProgress
		// because UpdateProgress might affect the task object in memory
		parentTaskID := task.ParentTaskID

		if err := u.taskRepo.UpdateProgress(taskID, progress); err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}

		// Debug: Check ParentTaskID
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - checking for parent... ParentTaskID is: %v (saved: %v)\n", taskID, task.ParentTaskID, parentTaskID)

		// If task has parent, recalculate parent progress too
		// Use the saved parentTaskID instead of task.ParentTaskID
		if parentTaskID != nil {
			fmt.Printf("[RecalculateTaskProgress] Task ID: %d has parent task ID: %d, recalculating parent progress\n", taskID, *parentTaskID)
			u.RecalculateTaskProgress(*parentTaskID)
		} else {
			fmt.Printf("[RecalculateTaskProgress] Task ID: %d has NO parent (ParentTaskID is nil)\n", taskID)
		}
	} else {
		fmt.Printf("[RecalculateTaskProgress] Task ID: %d - progress unchanged (%d%%), skipping update\n", taskID, progress)
	}

	return nil
}

// UpdateTaskProgress manually updates task progress
func (u *taskUsecase) UpdateTaskProgress(userID uint, taskID uint, progress int) (*models.TaskResponse, error) {
	// Validate progress range
	if progress < 0 || progress > 100 {
		return nil, fmt.Errorf("progress must be between 0 and 100")
	}

	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Check permissions
	// Parent task creator cannot edit subtasks (only view them)
	if !u.hasTaskEditAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Check if task has subtasks - if yes, progress is auto-calculated
	subtaskCount, err := u.taskRepo.CountSubtasks(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to check subtasks: %w", err)
	}

	if subtaskCount > 0 {
		return nil, fmt.Errorf("cannot manually update progress for tasks with subtasks - progress is auto-calculated")
	}

	// Check if task has checklists - if yes, progress is auto-calculated
	checklistItemCount, err := u.checklistRepo.CountChecklistItemsByTaskID(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to check checklist items: %w", err)
	}

	if checklistItemCount > 0 {
		return nil, fmt.Errorf("cannot manually update progress for tasks with checklists - progress is auto-calculated based on completed checklist items")
	}

	// Update progress
	oldProgress := task.ProgressPercentage
	if err := u.taskRepo.UpdateProgress(taskID, progress); err != nil {
		return nil, fmt.Errorf("failed to update progress: %w", err)
	}

	// Log activity
	u.logActivity(taskID, userID, "progress_updated", fmt.Sprintf("%d%%", oldProgress), fmt.Sprintf("%d%%", progress), nil)

	// If task has parent, recalculate parent progress
	if task.ParentTaskID != nil {
		u.RecalculateTaskProgress(*task.ParentTaskID)
	}

	// Reload task
	task, _ = u.taskRepo.GetByID(taskID)
	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

	return response, nil
}

// calculateProgressByStatus calculates progress based on task status
func (u *taskUsecase) calculateProgressByStatus(status models.TaskStatus) int {
	switch status {
	case models.TaskStatusNew:
		return 0
	case models.TaskStatusViewed:
		return 0
	case models.TaskStatusInProgress:
		return 50
	case models.TaskStatusReview:
		return 75
	case models.TaskStatusDone:
		return 100
	case models.TaskStatusCancelled:
		return 0
	default:
		return 0
	}
}

// logActivity is a helper to log activities (ignores errors)
func (u *taskUsecase) logActivity(taskID, userID uint, actionType, oldValue, newValue string, details interface{}) {
	activity := &models.TaskActivity{
		TaskID:     taskID,
		UserID:     userID,
		ActionType: actionType,
		OldValue:   oldValue,
		NewValue:   newValue,
	}

	if details != nil {
		if detailsJSON, err := u.marshalDetails(details); err == nil && detailsJSON != "" {
			activity.Details = &detailsJSON
		}
	}

	// Log error if activity creation fails
	if err := u.activityRepo.Create(activity); err != nil {
		fmt.Printf("WARNING: Failed to create activity log: %v (taskID=%d, actionType=%s)\n", err, taskID, actionType)
	}
}

// marshalDetails marshals details to JSON string
func (u *taskUsecase) marshalDetails(details interface{}) (string, error) {
	if details == nil {
		return "", nil
	}
	jsonBytes, err := json.Marshal(details)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}

// --- ANALYTICS METHODS ---

// GetDepartmentTaskStats returns task statistics grouped by department
func (u *taskUsecase) GetDepartmentTaskStats(period string) (interface{}, error) {
	timeRange, err := u.parseTimeRange(period)
	if err != nil {
		return nil, err
	}

	stats, err := u.taskRepo.GetAllDepartmentsStats(timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get department stats: %w", err)
	}

	return stats, nil
}

// GetTopPerformers returns top performing employees
func (u *taskUsecase) GetTopPerformers(limit int, period string, departmentID *uint) (interface{}, error) {
	timeRange, err := u.parseTimeRange(period)
	if err != nil {
		return nil, err
	}

	var performers []*repository.EmployeePerformance
	if departmentID != nil {
		performers, err = u.taskRepo.GetTopPerformersByDepartment(departmentID, limit, timeRange)
	} else {
		performers, err = u.taskRepo.GetTopPerformers(limit, timeRange)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get top performers: %w", err)
	}

	return performers, nil
}

// GetTaskTrends returns task completion trends over time
func (u *taskUsecase) GetTaskTrends(period string, interval string) (interface{}, error) {
	timeRange, err := u.parseTimeRange(period)
	if err != nil {
		return nil, err
	}

	trends, err := u.taskRepo.GetTaskCompletionTrends(timeRange, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get task trends: %w", err)
	}

	return trends, nil
}

// GetPriorityDistribution returns task distribution by priority
func (u *taskUsecase) GetPriorityDistribution(period string) (interface{}, error) {
	timeRange, err := u.parseTimeRange(period)
	if err != nil {
		return nil, err
	}

	distribution, err := u.taskRepo.GetTasksByPriority(timeRange)
	if err != nil {
		return nil, fmt.Errorf("failed to get priority distribution: %w", err)
	}

	return distribution, nil
}

// parseTimeRange converts period string to TimeRange
func (u *taskUsecase) parseTimeRange(period string) (*repository.TimeRange, error) {
	now := time.Now()
	var start, end time.Time

	switch period {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.AddDate(0, 0, 1)
	case "week":
		// Last 7 days
		start = now.AddDate(0, 0, -7)
		end = now
	case "month":
		// Last 30 days
		start = now.AddDate(0, 0, -30)
		end = now
	case "quarter":
		// Last 90 days
		start = now.AddDate(0, 0, -90)
		end = now
	case "year":
		// Last 365 days
		start = now.AddDate(0, 0, -365)
		end = now
	default:
		return nil, fmt.Errorf("invalid period: %s (valid values: today, week, month, quarter, year)", period)
	}

	return &repository.TimeRange{
		Start: start,
		End:   end,
	}, nil
}

// GetDeletedTaskIDsSince returns IDs of tasks deleted since the given timestamp
// Used for incremental sync to inform clients about deletions
func (u *taskUsecase) GetDeletedTaskIDsSince(since time.Time) ([]uint, error) {
	if u.syncRepo == nil {
		// If sync repo is not configured, return empty slice
		return []uint{}, nil
	}
	return u.syncRepo.GetDeletedIDsSince(since)
}
