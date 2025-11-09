package usecase

import (
	"errors"
	"fmt"
	"strings"

	"tachyon-messenger/services/task/models"
	sharedmodels "tachyon-messenger/shared/models"

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

	// Log activity
	u.logActivity(taskID, userID, "comment_added", "", fmt.Sprintf("Comment %d added", comment.ID), map[string]interface{}{
		"comment_id": comment.ID,
	})

	response := comment.ToResponse()

	// Enrich with user information
	responses := []*models.TaskCommentResponse{response}
	if err := u.enrichCommentsWithUsers(responses); err != nil {
		fmt.Printf("Warning: failed to enrich comment with user info: %v\n", err)
	}

	return response, nil
}

// GetTaskComments retrieves comments for a task
func (u *taskUsecase) GetTaskComments(userID uint, userRole sharedmodels.Role, taskID uint, filter *models.CommentFilterRequest) (*models.CommentListResponse, error) {
	// Check if task exists and user has access to it
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Only super_admin can access any task's comments
	isSuperAdmin := userRole == sharedmodels.RoleSuperAdmin

	// Check access rights: user must be creator, assignee, or super_admin to view comments
	if !isSuperAdmin && !u.hasTaskAccess(userID, task) {
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

	// Enrich comments with user information
	if err := u.enrichCommentsWithUsers(responses); err != nil {
		// Log error but don't fail the request
		// Comments will be returned without user info
		fmt.Printf("Warning: failed to enrich comments with user info: %v\n", err)
	}

	return &models.CommentListResponse{
		Comments: responses,
		Total:    total,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	}, nil
}

// enrichCommentsWithUsers enriches comment responses with user information from user-service
func (u *taskUsecase) enrichCommentsWithUsers(comments []*models.TaskCommentResponse) error {
	if len(comments) == 0 {
		return nil
	}

	// Collect unique user IDs
	userIDsMap := make(map[uint]bool)
	var collectUserIDs func([]*models.TaskCommentResponse)
	collectUserIDs = func(cmts []*models.TaskCommentResponse) {
		for _, comment := range cmts {
			userIDsMap[comment.UserID] = true
			// Recursively collect from replies
			if len(comment.Replies) > 0 {
				collectUserIDs(comment.Replies)
			}
		}
	}
	collectUserIDs(comments)

	// Convert map to slice
	userIDs := make([]uint, 0, len(userIDsMap))
	for userID := range userIDsMap {
		userIDs = append(userIDs, userID)
	}

	// Fetch user info from user-service
	userInfoMap, err := u.userClient.GetUsersByIDs(userIDs)
	if err != nil {
		return fmt.Errorf("failed to get users from user-service: %w", err)
	}

	// Enrich comments with user info
	var enrichComments func([]*models.TaskCommentResponse)
	enrichComments = func(cmts []*models.TaskCommentResponse) {
		for _, comment := range cmts {
			if clientUserInfo, ok := userInfoMap[comment.UserID]; ok {
				comment.User = &models.UserInfo{
					ID:       clientUserInfo.ID,
					Name:     clientUserInfo.Name,
					Email:    clientUserInfo.Email,
					Avatar:   clientUserInfo.Avatar,
					Position: clientUserInfo.Position,
				}
			}
			// Recursively enrich replies
			if len(comment.Replies) > 0 {
				enrichComments(comment.Replies)
			}
		}
	}
	enrichComments(comments)

	return nil
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

// Comment validation methods

// validateCreateCommentRequest validates comment creation request
func (u *taskUsecase) validateCreateCommentRequest(req *models.CreateTaskCommentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate content
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return fmt.Errorf("comment content is required")
	}
	if len(content) > 1000 {
		return fmt.Errorf("comment content must be less than 1000 characters")
	}

	// Validate parent ID if provided
	if req.ParentID != nil && *req.ParentID == 0 {
		return fmt.Errorf("invalid parent comment ID")
	}

	return nil
}

// validateUpdateCommentRequest validates comment update request
func (u *taskUsecase) validateUpdateCommentRequest(req *models.UpdateTaskCommentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate content
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return fmt.Errorf("comment content cannot be empty")
	}
	if len(content) > 1000 {
		return fmt.Errorf("comment content must be less than 1000 characters")
	}

	return nil
}
