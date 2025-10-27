package repository

import (
	"fmt"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// AttachmentRepository defines the interface for task attachment data operations
type AttachmentRepository interface {
	Create(attachment *models.TaskAttachment) error
	GetByTaskID(taskID uint) ([]*models.TaskAttachment, error)
	GetByID(id uint) (*models.TaskAttachment, error)
	Delete(id uint) error
	Count(taskID uint) (int64, error)
}

// attachmentRepository implements AttachmentRepository interface
type attachmentRepository struct {
	db *database.DB
}

// NewAttachmentRepository creates a new attachment repository
func NewAttachmentRepository(db *database.DB) AttachmentRepository {
	return &attachmentRepository{
		db: db,
	}
}

// Create creates a new task attachment
func (r *attachmentRepository) Create(attachment *models.TaskAttachment) error {
	if err := r.db.Create(attachment).Error; err != nil {
		return fmt.Errorf("failed to create task attachment: %w", err)
	}
	return nil
}

// GetByTaskID retrieves all attachments for a task
func (r *attachmentRepository) GetByTaskID(taskID uint) ([]*models.TaskAttachment, error) {
	var attachments []*models.TaskAttachment

	// Get attachments ordered by created_at DESC (newest first)
	if err := r.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&attachments).Error; err != nil {
		return nil, fmt.Errorf("failed to get task attachments: %w", err)
	}

	return attachments, nil
}

// GetByID retrieves a task attachment by ID
func (r *attachmentRepository) GetByID(id uint) (*models.TaskAttachment, error) {
	var attachment models.TaskAttachment
	if err := r.db.First(&attachment, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("attachment not found")
		}
		return nil, fmt.Errorf("failed to get task attachment: %w", err)
	}
	return &attachment, nil
}

// Delete soft deletes a task attachment by ID
func (r *attachmentRepository) Delete(id uint) error {
	result := r.db.Delete(&models.TaskAttachment{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete task attachment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("attachment not found")
	}
	return nil
}

// Count returns the total number of attachments for a task
func (r *attachmentRepository) Count(taskID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.TaskAttachment{}).Where("task_id = ?", taskID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count task attachments: %w", err)
	}
	return count, nil
}
