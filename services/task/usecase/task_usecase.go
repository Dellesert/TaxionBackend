package usecase

import (
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
	GetTaskByID(userID, taskID uint) (*models.TaskResponse, error)
	GetTaskByIDInternal(taskID uint) (*models.Task, error) // Internal method without access control
	UpdateTask(userID, taskID uint, req *models.UpdateTaskRequest) (*models.TaskResponse, error)
	DeleteTask(userID uint, userRole sharedmodels.Role, taskID uint) error
	AssignTask(userID, taskID uint, req *models.AssignTaskRequest) (*models.TaskResponse, error)
	UnassignTask(userID, taskID uint) (*models.TaskResponse, error)
	UpdateTaskStatus(userID, taskID uint, req *models.UpdateTaskStatusRequest) (*models.TaskResponse, error)
	GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.TaskResponse, int64, error)
	GetTaskStats(userID uint) (*models.TaskStatsResponse, error)

	// Comment methods
	AddComment(userID, taskID uint, req *models.CreateTaskCommentRequest) (*models.TaskCommentResponse, error)
	GetTaskComments(userID, taskID uint, filter *models.CommentFilterRequest) (*models.CommentListResponse, error)
	UpdateComment(userID, commentID uint, req *models.UpdateTaskCommentRequest) (*models.TaskCommentResponse, error)
	DeleteComment(userID, commentID uint) error
}

// taskUsecase implements TaskUsecase interface
type taskUsecase struct {
	taskRepo    repository.TaskRepository
	commentRepo repository.CommentRepository
	userClient  *clients.UserClient
}

// NewTaskUsecase creates a new task usecase
func NewTaskUsecase(taskRepo repository.TaskRepository, commentRepo repository.CommentRepository) TaskUsecase {
	return &taskUsecase{
		taskRepo:    taskRepo,
		commentRepo: commentRepo,
		userClient:  clients.NewUserClient(),
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
		DueDate:              req.DueDate,
		AssignedToDepartment: req.AssignedToDepartment,
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

	// Manager can assign to their department members
	if userRole == sharedmodels.RoleManager {
		// Can assign to department
		if req.AssignedToDepartment != nil && *req.AssignedToDepartment == userDepartment {
			return nil
		}

		// TODO: Check if all assignees are from manager's department
		// For now, allow managers to assign to specific users (will need user-service client to validate)
		if len(req.AssigneeIDs) > 0 {
			return nil // Temporary: trust manager's assignment
		}

		return nil // Allow managers to assign
	}

	// Employee can only create tasks for themselves
	return fmt.Errorf("access denied: employees can only create tasks for themselves")
}

// GetTaskByID retrieves a task by ID with access control
func (u *taskUsecase) GetTaskByID(userID, taskID uint) (*models.TaskResponse, error) {
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check access rights: user must be creator or assignee
	if !u.hasTaskAccess(userID, task) {
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
func (u *taskUsecase) UpdateTask(userID, taskID uint, req *models.UpdateTaskRequest) (*models.TaskResponse, error) {
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

	// Check permissions: only creator or assignee can update
	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Update fields if provided
	if req.Title != nil {
		task.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		task.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		task.Status = *req.Status
		task.LastStatusChangedBy = &userID
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.AssignedTo != nil {
		task.AssignedTo = req.AssignedTo
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return task.ToResponse(), nil
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

	// Check permissions: only creator or admin/super_admin can delete
	isCreator := task.CreatedBy == userID
	isAdmin := userRole == sharedmodels.RoleAdmin || userRole == sharedmodels.RoleSuperAdmin

	if !isCreator && !isAdmin {
		return fmt.Errorf("access denied: only task creator or administrator can delete the task")
	}

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

	// Assign task
	task.AssignedTo = &req.AssignedTo

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to assign task: %w", err)
	}

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
	task.Status = req.Status
	task.LastStatusChangedBy = &userID

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
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
func (u *taskUsecase) GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.TaskResponse, int64, error) {
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
	tasks, total, err := u.taskRepo.GetUserTasks(userID, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user tasks: %w", err)
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
	case models.TaskStatusNew, models.TaskStatusInProgress, models.TaskStatusReview,
		models.TaskStatusDone, models.TaskStatusCancelled:
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
