package repository

import (
	"fmt"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"
)

// ActivityRepository defines the interface for task activity data operations
type ActivityRepository interface {
	Create(activity *models.TaskActivity) error
	GetByTaskID(taskID uint, limit, offset int) ([]*models.TaskActivity, int64, error)
	GetByID(id uint) (*models.TaskActivity, error)
	Delete(id uint) error
	Count(taskID uint) (int64, error)
}

// activityRepository implements ActivityRepository interface
type activityRepository struct {
	db *database.DB
}

// NewActivityRepository creates a new activity repository
func NewActivityRepository(db *database.DB) ActivityRepository {
	return &activityRepository{
		db: db,
	}
}

// Create creates a new task activity record
func (r *activityRepository) Create(activity *models.TaskActivity) error {
	if err := r.db.Create(activity).Error; err != nil {
		return fmt.Errorf("failed to create task activity: %w", err)
	}
	return nil
}

// GetByTaskID retrieves all activities for a task with pagination
func (r *activityRepository) GetByTaskID(taskID uint, limit, offset int) ([]*models.TaskActivity, int64, error) {
	var activities []*models.TaskActivity
	var total int64

	// Count total activities
	if err := r.db.Model(&models.TaskActivity{}).Where("task_id = ?", taskID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count task activities: %w", err)
	}

	// Set default pagination
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Get activities ordered by created_at DESC (newest first)
	query := r.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&activities).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get task activities: %w", err)
	}

	return activities, total, nil
}

// GetByID retrieves a task activity by ID
func (r *activityRepository) GetByID(id uint) (*models.TaskActivity, error) {
	var activity models.TaskActivity
	if err := r.db.First(&activity, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get task activity: %w", err)
	}
	return &activity, nil
}

// Delete deletes a task activity by ID
func (r *activityRepository) Delete(id uint) error {
	result := r.db.Delete(&models.TaskActivity{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete task activity: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task activity not found")
	}
	return nil
}

// Count returns the total number of activities for a task
func (r *activityRepository) Count(taskID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskActivity{}).Where("task_id = ?", taskID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count task activities: %w", err)
	}
	return count, nil
}
