package repository

import (
	"fmt"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// ChecklistRepository defines the interface for task checklist data operations
type ChecklistRepository interface {
	// Checklist operations
	CreateChecklist(checklist *models.TaskChecklist) error
	GetChecklistByID(id uint) (*models.TaskChecklist, error)
	GetChecklistsByTaskID(taskID uint) ([]*models.TaskChecklist, error)
	UpdateChecklist(checklist *models.TaskChecklist) error
	DeleteChecklist(id uint) error

	// Checklist item operations
	CreateChecklistItem(item *models.TaskChecklistItem) error
	GetChecklistItemByID(id uint) (*models.TaskChecklistItem, error)
	GetChecklistItemsByChecklistID(checklistID uint) ([]*models.TaskChecklistItem, error)
	UpdateChecklistItem(item *models.TaskChecklistItem) error
	DeleteChecklistItem(id uint) error

	// Batch operations
	CreateChecklistItems(items []*models.TaskChecklistItem) error
	GetChecklistsWithItems(taskID uint) ([]*models.TaskChecklist, error)

	// Counting operations for progress calculation
	CountChecklistItemsByTaskID(taskID uint) (int64, error)
	CountCompletedChecklistItemsByTaskID(taskID uint) (int64, error)
}

// checklistRepository implements ChecklistRepository interface
type checklistRepository struct {
	db *database.DB
}

// NewChecklistRepository creates a new checklist repository
func NewChecklistRepository(db *database.DB) ChecklistRepository {
	return &checklistRepository{
		db: db,
	}
}

// Checklist operations

// CreateChecklist creates a new task checklist
func (r *checklistRepository) CreateChecklist(checklist *models.TaskChecklist) error {
	if err := r.db.Create(checklist).Error; err != nil {
		return fmt.Errorf("failed to create task checklist: %w", err)
	}
	return nil
}

// GetChecklistByID retrieves a checklist by ID with its items
func (r *checklistRepository) GetChecklistByID(id uint) (*models.TaskChecklist, error) {
	var checklist models.TaskChecklist
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	}).First(&checklist, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("checklist not found")
		}
		return nil, fmt.Errorf("failed to get checklist: %w", err)
	}
	return &checklist, nil
}

// GetChecklistsByTaskID retrieves all checklists for a task
func (r *checklistRepository) GetChecklistsByTaskID(taskID uint) ([]*models.TaskChecklist, error) {
	var checklists []*models.TaskChecklist
	if err := r.db.Where("task_id = ?", taskID).
		Order("position ASC").
		Find(&checklists).Error; err != nil {
		return nil, fmt.Errorf("failed to get task checklists: %w", err)
	}
	return checklists, nil
}

// GetChecklistsWithItems retrieves all checklists for a task with their items
func (r *checklistRepository) GetChecklistsWithItems(taskID uint) ([]*models.TaskChecklist, error) {
	var checklists []*models.TaskChecklist
	if err := r.db.Preload("Items", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	}).Where("task_id = ?", taskID).
		Order("position ASC").
		Find(&checklists).Error; err != nil {
		return nil, fmt.Errorf("failed to get task checklists with items: %w", err)
	}
	return checklists, nil
}

// UpdateChecklist updates a checklist
func (r *checklistRepository) UpdateChecklist(checklist *models.TaskChecklist) error {
	result := r.db.Save(checklist)
	if result.Error != nil {
		return fmt.Errorf("failed to update checklist: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("checklist not found")
	}
	return nil
}

// DeleteChecklist soft deletes a checklist and its items
func (r *checklistRepository) DeleteChecklist(id uint) error {
	result := r.db.Delete(&models.TaskChecklist{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete checklist: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("checklist not found")
	}
	return nil
}

// Checklist item operations

// CreateChecklistItem creates a new checklist item
func (r *checklistRepository) CreateChecklistItem(item *models.TaskChecklistItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("failed to create checklist item: %w", err)
	}
	return nil
}

// CreateChecklistItems creates multiple checklist items
func (r *checklistRepository) CreateChecklistItems(items []*models.TaskChecklistItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.Create(&items).Error; err != nil {
		return fmt.Errorf("failed to create checklist items: %w", err)
	}
	return nil
}

// GetChecklistItemByID retrieves a checklist item by ID
func (r *checklistRepository) GetChecklistItemByID(id uint) (*models.TaskChecklistItem, error) {
	var item models.TaskChecklistItem
	if err := r.db.First(&item, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("checklist item not found")
		}
		return nil, fmt.Errorf("failed to get checklist item: %w", err)
	}
	return &item, nil
}

// GetChecklistItemsByChecklistID retrieves all items for a checklist
func (r *checklistRepository) GetChecklistItemsByChecklistID(checklistID uint) ([]*models.TaskChecklistItem, error) {
	var items []*models.TaskChecklistItem
	if err := r.db.Where("checklist_id = ?", checklistID).
		Order("position ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to get checklist items: %w", err)
	}
	return items, nil
}

// UpdateChecklistItem updates a checklist item
func (r *checklistRepository) UpdateChecklistItem(item *models.TaskChecklistItem) error {
	result := r.db.Save(item)
	if result.Error != nil {
		return fmt.Errorf("failed to update checklist item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("checklist item not found")
	}
	return nil
}

// DeleteChecklistItem soft deletes a checklist item
func (r *checklistRepository) DeleteChecklistItem(id uint) error {
	result := r.db.Delete(&models.TaskChecklistItem{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete checklist item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("checklist item not found")
	}
	return nil
}

// Counting operations for progress calculation

// CountChecklistItemsByTaskID counts total checklist items for a task
func (r *checklistRepository) CountChecklistItemsByTaskID(taskID uint) (int64, error) {
	var count int64
	err := r.db.Table("task_checklist_items").
		Joins("INNER JOIN task_checklists ON task_checklist_items.checklist_id = task_checklists.id").
		Where("task_checklists.task_id = ? AND task_checklist_items.deleted_at IS NULL AND task_checklists.deleted_at IS NULL", taskID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count checklist items: %w", err)
	}
	return count, nil
}

// CountCompletedChecklistItemsByTaskID counts completed checklist items for a task
func (r *checklistRepository) CountCompletedChecklistItemsByTaskID(taskID uint) (int64, error) {
	var count int64
	err := r.db.Table("task_checklist_items").
		Joins("INNER JOIN task_checklists ON task_checklist_items.checklist_id = task_checklists.id").
		Where("task_checklists.task_id = ? AND task_checklist_items.is_completed = ? AND task_checklist_items.deleted_at IS NULL AND task_checklists.deleted_at IS NULL", taskID, true).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count completed checklist items: %w", err)
	}
	return count, nil
}
