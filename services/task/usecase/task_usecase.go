package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"

	"gorm.io/gorm"
)

// TaskUsecase defines the interface for task business logic
type TaskUsecase interface {
	CreateTask(userID uint, req *models.CreateTaskRequest) (*models.TaskResponse, error)
	GetTaskByID(userID, taskID uint) (*models.TaskResponse, error)
	UpdateTask(userID, taskID uint, req *models.UpdateTaskRequest) (*models.TaskResponse, error)
	DeleteTask(userID, taskID uint) error
	AssignTask(userID, taskID uint, req *models.AssignTaskRequest) (*models.TaskResponse, error)
	UnassignTask(userID, taskID uint) (*models.TaskResponse, error)
	UpdateTaskStatus(userID, taskID uint, req *models.UpdateTaskStatusRequest) (*models.TaskResponse, error)
	GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.TaskResponse, int64, error)
	GetTaskStats(userID uint) (*models.TaskStatsResponse, error)
}

// taskUsecase implements TaskUsecase interface
type taskUsecase struct {
	taskRepo repository.TaskRepository
}

// NewTaskUsecase creates a new task usecase
func NewTaskUsecase(taskRepo repository.TaskRepository) TaskUsecase {
	return &taskUsecase{
		taskRepo: taskRepo,
	}
}

// CreateTask creates a new task
func (u *taskUsecase) CreateTask(userID uint, req *models.CreateTaskRequest) (*models.TaskResponse, error) {
	// Validate request
	if err := u.validateCreateTaskRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create task model
	task := &models.Task{
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		CreatedBy:   userID,
		DueDate:     req.DueDate,
	}

	// Set priority (default to medium if not provided)
	if req.Priority != nil {
		task.Priority = *req.Priority
	} else {
		task.Priority = models.TaskPriorityMedium
	}

	// Set assigned user if provided
	if req.AssignedTo != nil {
		task.AssignedTo = req.AssignedTo
	}

	// Save task
	if err := u.taskRepo.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return task.ToResponse(), nil
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

	return task.ToResponse(), nil
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
func (u *taskUsecase) DeleteTask(userID, taskID uint) error {
	// Get existing task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("task not found")
		}
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Check permissions: only creator can delete
	if task.CreatedBy != userID {
		return fmt.Errorf("access denied: only task creator can delete the task")
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

	// Save updated task
	if err := u.taskRepo.Update(task); err != nil {
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	return task.ToResponse(), nil
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

	return responses, total, nil
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

	// User is assignee
	if task.AssignedTo != nil && *task.AssignedTo == userID {
		return true
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
