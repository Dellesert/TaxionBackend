package repository

import (
	"errors"
	"fmt"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// CommentRepository defines the interface for task comment data operations
type CommentRepository interface {
	Create(comment *models.TaskComment) error
	GetByID(id uint) (*models.TaskComment, error)
	GetByTaskID(taskID uint, filter *models.CommentFilterRequest) ([]*models.TaskComment, int64, error)
	Update(comment *models.TaskComment) error
	Delete(id uint) error
	GetCommentsWithReplies(taskID uint, filter *models.CommentFilterRequest) ([]*models.TaskComment, int64, error)
	GetCommentsByUser(userID uint, limit, offset int) ([]*models.TaskComment, error)
	CountByTaskID(taskID uint) (int64, error)
}

// commentRepository implements CommentRepository interface
type commentRepository struct {
	db *database.DB
}

// NewCommentRepository creates a new comment repository
func NewCommentRepository(db *database.DB) CommentRepository {
	return &commentRepository{
		db: db,
	}
}

// Create creates a new task comment
func (r *commentRepository) Create(comment *models.TaskComment) error {
	if err := r.db.Create(comment).Error; err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return fmt.Errorf("invalid task or parent comment reference")
		}
		return fmt.Errorf("failed to create task comment: %w", err)
	}
	return nil
}

// GetByID retrieves a task comment by ID
func (r *commentRepository) GetByID(id uint) (*models.TaskComment, error) {
	var comment models.TaskComment
	err := r.db.First(&comment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task comment not found")
		}
		return nil, fmt.Errorf("failed to get task comment: %w", err)
	}
	return &comment, nil
}

// GetByTaskID retrieves all comments for a task with pagination
func (r *commentRepository) GetByTaskID(taskID uint, filter *models.CommentFilterRequest) ([]*models.TaskComment, int64, error) {
	query := r.db.Model(&models.TaskComment{}).Where("task_id = ?", taskID)

	// Apply filters
	if filter != nil && filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count task comments: %w", err)
	}

	// Apply pagination
	limit, offset := r.getPaginationParams(filter)
	query = query.Limit(limit).Offset(offset)

	var comments []*models.TaskComment
	err := query.Order("created_at ASC").Find(&comments).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get task comments: %w", err)
	}

	return comments, total, nil
}

// Update updates an existing task comment
func (r *commentRepository) Update(comment *models.TaskComment) error {
	result := r.db.Save(comment)
	if result.Error != nil {
		return fmt.Errorf("failed to update task comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task comment not found")
	}
	return nil
}

// Delete soft deletes a task comment by ID
func (r *commentRepository) Delete(id uint) error {
	result := r.db.Delete(&models.TaskComment{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete task comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task comment not found")
	}
	return nil
}

// GetCommentsWithReplies retrieves all comments for a task with their replies
func (r *commentRepository) GetCommentsWithReplies(taskID uint, filter *models.CommentFilterRequest) ([]*models.TaskComment, int64, error) {
	// First get root comments (comments without parent)
	query := r.db.Model(&models.TaskComment{}).Where("task_id = ? AND parent_id IS NULL", taskID)

	// Apply filters
	if filter != nil && filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}

	// Get total count of root comments
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count root comments: %w", err)
	}

	// Apply pagination
	limit, offset := r.getPaginationParams(filter)
	query = query.Limit(limit).Offset(offset)

	var comments []*models.TaskComment
	err := query.Preload("Replies", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Order("created_at ASC").Find(&comments).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get task comments with replies: %w", err)
	}

	return comments, total, nil
}

// GetCommentsByUser retrieves comments created by a specific user
func (r *commentRepository) GetCommentsByUser(userID uint, limit, offset int) ([]*models.TaskComment, error) {
	var comments []*models.TaskComment
	err := r.db.Where("user_id = ?", userID).
		Limit(limit).Offset(offset).
		Order("created_at DESC").
		Find(&comments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user comments: %w", err)
	}

	return comments, nil
}

// CountByTaskID returns the number of comments for a specific task
func (r *commentRepository) CountByTaskID(taskID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskComment{}).Where("task_id = ?", taskID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count comments for task: %w", err)
	}
	return count, nil
}

// Helper methods

// getPaginationParams extracts and validates pagination parameters
func (r *commentRepository) getPaginationParams(filter *models.CommentFilterRequest) (limit, offset int) {
	limit = 20 // default
	offset = 0 // default

	if filter != nil {
		if filter.Limit > 0 {
			limit = filter.Limit
		}
		if limit > 100 {
			limit = 100 // max limit
		}

		if filter.Offset > 0 {
			offset = filter.Offset
		}
	}

	return limit, offset
}
