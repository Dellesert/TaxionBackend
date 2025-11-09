package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
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
	GetTaskStatsInternal() (*models.TaskStatsInternalResponse, error) // Internal method for analytics

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
}

// taskUsecase implements TaskUsecase interface
type taskUsecase struct {
	taskRepo       repository.TaskRepository
	commentRepo    repository.CommentRepository
	activityRepo   repository.ActivityRepository
	attachmentRepo repository.AttachmentRepository
	checklistRepo  repository.ChecklistRepository
	userClient     *clients.UserClient
}

// NewTaskUsecase creates a new task usecase
func NewTaskUsecase(
	taskRepo repository.TaskRepository,
	commentRepo repository.CommentRepository,
	activityRepo repository.ActivityRepository,
	attachmentRepo repository.AttachmentRepository,
	checklistRepo repository.ChecklistRepository,
) TaskUsecase {
	return &taskUsecase{
		taskRepo:       taskRepo,
		commentRepo:    commentRepo,
		activityRepo:   activityRepo,
		attachmentRepo: attachmentRepo,
		checklistRepo:  checklistRepo,
		userClient:     clients.NewUserClient(),
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
	isSuperAdmin := userRole == sharedmodels.RoleSuperAdmin
	if !isSuperAdmin && !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Track changes for activity logging
	changes := make(map[string]interface{})

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
		oldStatus := task.Status
		task.Status = *req.Status
		task.LastStatusChangedBy = &userID
		u.logActivity(taskID, userID, "task_status_changed", string(oldStatus), string(task.Status), changes)
	}
	if req.Priority != nil && *req.Priority != task.Priority {
		oldPriority := task.Priority
		task.Priority = *req.Priority
		u.logActivity(taskID, userID, "task_updated_priority", string(oldPriority), string(task.Priority), nil)
	}
	if req.AssignedTo != nil {
		task.AssignedTo = req.AssignedTo
	}
	if req.AssignedToDepartment != nil {
		task.AssignedToDepartment = req.AssignedToDepartment
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
		u.logActivity(taskID, userID, "task_updated_due_date", "", req.DueDate.Format("2006-01-02"), nil)
	}

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
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
	if !u.hasTaskAccess(userID, task) {
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
	if !u.hasTaskAccess(userID, task) {
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

	// Check permissions: only creator or assignee can update status
	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
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

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// Log activity
	u.logActivity(taskID, userID, "task_status_changed", string(oldStatus), string(req.Status), map[string]interface{}{
		"old_status": oldStatus,
		"new_status": req.Status,
		"changed_at": time.Now(),
	})

	// Recalculate progress if status changed
	if oldStatus != task.Status {
		u.RecalculateTaskProgress(taskID)
		// If task has parent, recalculate parent progress and log activity
		if task.ParentTaskID != nil {
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
	}

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

	return responses, total, nil
}

// enrichTasksWithUserInfo enriches multiple tasks with user information in a single batch
func (u *taskUsecase) enrichTasksWithUserInfo(responses []*models.TaskResponse) error {
	if len(responses) == 0 {
		return nil
	}

	// Collect all unique user IDs from all tasks
	uniqueIDs := make(map[uint]bool)
	for _, response := range responses {
		uniqueIDs[response.CreatedBy] = true
		for _, assigneeID := range response.AssigneeIDs {
			uniqueIDs[assigneeID] = true
		}
		if response.LastStatusChangedBy != nil {
			uniqueIDs[*response.LastStatusChangedBy] = true
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
	}

	return nil
}

// GetTaskStats retrieves task statistics for a user
func (u *taskUsecase) GetTaskStats(userID uint) (*models.TaskStatsResponse, error) {
	stats, err := u.taskRepo.GetTaskStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task stats: %w", err)
	}

	return stats, nil
}

// GetTaskStatsInternal returns task statistics for analytics (no user filtering)
func (u *taskUsecase) GetTaskStatsInternal() (*models.TaskStatsInternalResponse, error) {
	return u.taskRepo.GetTaskStatsInternal()
}

// Helper methods

// hasTaskAccess checks if user has access to the task (creator or assignee)
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

	// Log activity for parent task with assignee information
	details := map[string]interface{}{
		"subtask_id":    task.ID,
		"subtask_title": task.Title,
		"assignee_ids":  assigneeIDs,
	}
	u.logActivity(parentTaskID, userID, "subtask_created", "", task.Title, details)

	// Recalculate parent progress
	u.RecalculateTaskProgress(parentTaskID)

	response := task.ToResponse()
	u.enrichTaskWithUserInfo(response)

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
	if !u.hasTaskAccess(userID, task) {
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

	// Add new assignee to task_assignees
	// Previous assignees remain in the table so they can still view the delegated task
	// The new assignee becomes the active owner (assigned_to field)
	newAssignee := &models.TaskAssignee{
		TaskID: taskID,
		UserID: toUserID,
	}
	if err := u.taskRepo.CreateAssignee(newAssignee); err != nil {
		return nil, fmt.Errorf("failed to add new assignee: %w", err)
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

// RecalculateTaskProgress recalculates progress based on subtasks
func (u *taskUsecase) RecalculateTaskProgress(taskID uint) error {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Count subtasks
	totalSubtasks, err := u.taskRepo.CountSubtasks(taskID)
	if err != nil {
		return fmt.Errorf("failed to count subtasks: %w", err)
	}

	// If no subtasks, calculate progress based on status
	if totalSubtasks == 0 {
		progress := u.calculateProgressByStatus(task.Status)
		if task.ProgressPercentage != progress {
			return u.taskRepo.UpdateProgress(taskID, progress)
		}
		return nil
	}

	// Count completed subtasks
	completedSubtasks, err := u.taskRepo.CountCompletedSubtasks(taskID)
	if err != nil {
		return fmt.Errorf("failed to count completed subtasks: %w", err)
	}

	// Calculate progress percentage
	progress := 0
	if totalSubtasks > 0 {
		progress = int((float64(completedSubtasks) / float64(totalSubtasks)) * 100)
	}

	// Update progress if changed
	if task.ProgressPercentage != progress {
		if err := u.taskRepo.UpdateProgress(taskID, progress); err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}

		// If task has parent, recalculate parent progress too
		if task.ParentTaskID != nil {
			u.RecalculateTaskProgress(*task.ParentTaskID)
		}
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
	if !u.hasTaskAccess(userID, task) {
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
