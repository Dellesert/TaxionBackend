package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/task/models"

	"gorm.io/gorm"
)

// Comment methods

// AddComment adds a comment to a task
func (u *taskUsecase) AddComment(userID, taskID uint, req *models.CreateTaskCommentRequest) (*models.TaskCommentResponse, error) {
	// Validate request
	if err := u.validateCreateCommentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if task exists and user has access to it
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check access rights: user must be creator or assignee to comment
	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions to comment on this task")
	}

	// Validate parent comment if provided
	if req.ParentID != nil {
		parentComment, err := u.commentRepo.GetByID(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent comment not found")
		}
		if parentComment.TaskID != taskID {
			return nil, fmt.Errorf("parent comment does not belong to this task")
		}
	}

	// Create comment
	comment := &models.TaskComment{
		TaskID:   taskID,
		UserID:   userID,
		Content:  strings.TrimSpace(req.Content),
		ParentID: req.ParentID,
	}

	if err := u.commentRepo.Create(comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment.ToResponse(), nil
}

// GetTaskComments retrieves comments for a task
func (u *taskUsecase) GetTaskComments(userID, taskID uint, filter *models.CommentFilterRequest) (*models.CommentListResponse, error) {
	// Check if task exists and user has access to it
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Check access rights: user must be creator or assignee to view comments
	if !u.hasTaskAccess(userID, task) {
		return nil, fmt.Errorf("access denied: insufficient permissions to view comments on this task")
	}

	// Set default filter if not provided
	if filter == nil {
		filter = &models.CommentFilterRequest{
			Limit:  20,
			Offset: 0,
		}
	}

	// Get comments with replies
	comments, total, err := u.commentRepo.GetCommentsWithReplies(taskID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get task comments: %w", err)
	}

	// Convert to response format
	responses := make([]*models.TaskCommentResponse, len(comments))
	for i, comment := range comments {
		responses[i] = comment.ToResponse()
	}

	return &models.CommentListResponse{
		Comments: responses,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	}, nil
}

// UpdateComment updates a task comment
func (u *taskUsecase) UpdateComment(userID, commentID uint, req *models.UpdateTaskCommentRequest) (*models.TaskCommentResponse, error) {
	// Validate request
	if err := u.validateUpdateCommentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing comment
	comment, err := u.commentRepo.GetByID(commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("comment not found")
		}
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	// Check permissions: only comment author can update
	if comment.UserID != userID {
		return nil, fmt.Errorf("access denied: only comment author can update the comment")
	}

	// Update comment content
	comment.Content = strings.TrimSpace(req.Content)

	if err := u.commentRepo.Update(comment); err != nil {
		return nil, fmt.Errorf("failed to update comment: %w", err)
	}

	return comment.ToResponse(), nil
}

// DeleteComment deletes a task comment
func (u *taskUsecase) DeleteComment(userID, commentID uint) error {
	// Get existing comment
	comment, err := u.commentRepo.GetByID(commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("comment not found")
		}
		return fmt.Errorf("failed to get comment: %w", err)
	}

	// Check permissions: only comment author can delete
	if comment.UserID != userID {
		return fmt.Errorf("access denied: only comment author can delete the comment")
	}

	// Delete comment
	if err := u.commentRepo.Delete(commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
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
