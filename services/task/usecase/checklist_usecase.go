package usecase

import (
	"fmt"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
)

// ChecklistUsecase defines the interface for task checklist business logic
type ChecklistUsecase interface {
	// Checklist operations
	CreateChecklist(taskID, userID uint, title, description string) (*models.TaskChecklist, error)
	GetChecklistByID(id uint) (*models.TaskChecklist, error)
	GetTaskChecklists(taskID uint) ([]*models.TaskChecklist, error)
	GetTaskChecklistsWithItems(taskID uint) ([]*models.TaskChecklist, error)
	UpdateChecklist(id, userID uint, title, description string) (*models.TaskChecklist, error)
	DeleteChecklist(id, userID uint) error

	// Checklist item operations
	CreateChecklistItem(checklistID, userID uint, title string, position int) (*models.TaskChecklistItem, error)
	UpdateChecklistItem(id, userID uint, title string, isCompleted *bool, position *int) (*models.TaskChecklistItem, error)
	DeleteChecklistItem(id, userID uint) error
	ToggleChecklistItem(id, userID uint) (*models.TaskChecklistItem, error)

	// Batch operations
	ReorderChecklistItems(checklistID, userID uint, itemPositions map[uint]int) error

	// Internal method to set TaskUsecase (to avoid circular dependency)
	SetTaskUsecase(taskUsecase TaskUsecase)
}

// checklistUsecase implements ChecklistUsecase interface
type checklistUsecase struct {
	checklistRepo repository.ChecklistRepository
	taskRepo      repository.TaskRepository
	taskUsecase   TaskUsecase
}

// NewChecklistUsecase creates a new checklist usecase
func NewChecklistUsecase(
	checklistRepo repository.ChecklistRepository,
	taskRepo repository.TaskRepository,
) ChecklistUsecase {
	return &checklistUsecase{
		checklistRepo: checklistRepo,
		taskRepo:      taskRepo,
	}
}

// CreateChecklist creates a new checklist for a task
func (u *checklistUsecase) CreateChecklist(taskID, userID uint, title, description string) (*models.TaskChecklist, error) {
	// Verify task exists
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	// Get current checklist count to set position
	checklists, err := u.checklistRepo.GetChecklistsByTaskID(taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing checklists: %w", err)
	}

	checklist := &models.TaskChecklist{
		TaskID:      taskID,
		Title:       title,
		Description: description,
		Position:    len(checklists), // Add at the end
	}

	if err := u.checklistRepo.CreateChecklist(checklist); err != nil {
		return nil, fmt.Errorf("failed to create checklist: %w", err)
	}

	return checklist, nil
}

// GetChecklistByID retrieves a checklist by ID with its items
func (u *checklistUsecase) GetChecklistByID(id uint) (*models.TaskChecklist, error) {
	return u.checklistRepo.GetChecklistByID(id)
}

// GetTaskChecklists retrieves all checklists for a task (without items)
func (u *checklistUsecase) GetTaskChecklists(taskID uint) ([]*models.TaskChecklist, error) {
	// Verify task exists
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	return u.checklistRepo.GetChecklistsByTaskID(taskID)
}

// GetTaskChecklistsWithItems retrieves all checklists for a task with their items
func (u *checklistUsecase) GetTaskChecklistsWithItems(taskID uint) ([]*models.TaskChecklist, error) {
	// Verify task exists
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	return u.checklistRepo.GetChecklistsWithItems(taskID)
}

// UpdateChecklist updates a checklist
func (u *checklistUsecase) UpdateChecklist(id, userID uint, title, description string) (*models.TaskChecklist, error) {
	// Get existing checklist
	checklist, err := u.checklistRepo.GetChecklistByID(id)
	if err != nil {
		return nil, fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists and user has permission
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// TODO: Add proper permission check
	// For now, allow task creator and assignees
	_ = task

	// Update fields
	if title != "" {
		checklist.Title = title
	}
	if description != "" {
		checklist.Description = description
	}

	if err := u.checklistRepo.UpdateChecklist(checklist); err != nil {
		return nil, fmt.Errorf("failed to update checklist: %w", err)
	}

	return checklist, nil
}

// DeleteChecklist deletes a checklist and all its items
func (u *checklistUsecase) DeleteChecklist(id, userID uint) error {
	// Get existing checklist
	checklist, err := u.checklistRepo.GetChecklistByID(id)
	if err != nil {
		return fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists and user has permission
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// TODO: Add proper permission check
	// For now, allow task creator
	if task.CreatedByUserID != userID {
		return fmt.Errorf("permission denied: only task creator can delete checklists")
	}

	if err := u.checklistRepo.DeleteChecklist(id); err != nil {
		return err
	}

	// Recalculate task progress
	if u.taskUsecase != nil {
		u.taskUsecase.RecalculateTaskProgress(task.ID)
	}

	return nil
}

// CreateChecklistItem creates a new checklist item
func (u *checklistUsecase) CreateChecklistItem(checklistID, userID uint, title string, position int) (*models.TaskChecklistItem, error) {
	// Verify checklist exists
	checklist, err := u.checklistRepo.GetChecklistByID(checklistID)
	if err != nil {
		return nil, fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// If position not provided, add at the end
	if position < 0 {
		items, err := u.checklistRepo.GetChecklistItemsByChecklistID(checklistID)
		if err != nil {
			return nil, fmt.Errorf("failed to get existing items: %w", err)
		}
		position = len(items)
	}

	item := &models.TaskChecklistItem{
		ChecklistID: checklistID,
		Title:       title,
		IsCompleted: false,
		Position:    position,
	}

	if err := u.checklistRepo.CreateChecklistItem(item); err != nil {
		return nil, fmt.Errorf("failed to create checklist item: %w", err)
	}

	// Recalculate task progress
	if u.taskUsecase != nil {
		u.taskUsecase.RecalculateTaskProgress(task.ID)
	}

	return item, nil
}

// UpdateChecklistItem updates a checklist item
func (u *checklistUsecase) UpdateChecklistItem(id, userID uint, title string, isCompleted *bool, position *int) (*models.TaskChecklistItem, error) {
	// Get existing item
	item, err := u.checklistRepo.GetChecklistItemByID(id)
	if err != nil {
		return nil, fmt.Errorf("checklist item not found: %w", err)
	}

	// Verify checklist exists
	checklist, err := u.checklistRepo.GetChecklistByID(item.ChecklistID)
	if err != nil {
		return nil, fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Track if completion status changed
	completionChanged := false
	if isCompleted != nil && item.IsCompleted != *isCompleted {
		completionChanged = true
	}

	// Update fields
	if title != "" {
		item.Title = title
	}
	if isCompleted != nil {
		item.IsCompleted = *isCompleted
	}
	if position != nil {
		item.Position = *position
	}

	if err := u.checklistRepo.UpdateChecklistItem(item); err != nil {
		return nil, fmt.Errorf("failed to update checklist item: %w", err)
	}

	// Recalculate task progress if completion status changed
	if completionChanged && u.taskUsecase != nil {
		u.taskUsecase.RecalculateTaskProgress(task.ID)
	}

	return item, nil
}

// DeleteChecklistItem deletes a checklist item
func (u *checklistUsecase) DeleteChecklistItem(id, userID uint) error {
	// Get existing item
	item, err := u.checklistRepo.GetChecklistItemByID(id)
	if err != nil {
		return fmt.Errorf("checklist item not found: %w", err)
	}

	// Verify checklist exists
	checklist, err := u.checklistRepo.GetChecklistByID(item.ChecklistID)
	if err != nil {
		return fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists and user has permission
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// TODO: Add proper permission check
	// For now, allow task creator and assignees

	if err := u.checklistRepo.DeleteChecklistItem(id); err != nil {
		return err
	}

	// Recalculate task progress
	if u.taskUsecase != nil {
		u.taskUsecase.RecalculateTaskProgress(task.ID)
	}

	return nil
}

// ToggleChecklistItem toggles the completion status of a checklist item
func (u *checklistUsecase) ToggleChecklistItem(id, userID uint) (*models.TaskChecklistItem, error) {
	// Get existing item
	item, err := u.checklistRepo.GetChecklistItemByID(id)
	if err != nil {
		return nil, fmt.Errorf("checklist item not found: %w", err)
	}

	// Verify checklist exists
	checklist, err := u.checklistRepo.GetChecklistByID(item.ChecklistID)
	if err != nil {
		return nil, fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists
	task, err := u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Toggle completion status
	item.IsCompleted = !item.IsCompleted

	if err := u.checklistRepo.UpdateChecklistItem(item); err != nil {
		return nil, fmt.Errorf("failed to toggle checklist item: %w", err)
	}

	// Recalculate task progress
	if u.taskUsecase != nil {
		u.taskUsecase.RecalculateTaskProgress(task.ID)
	}

	return item, nil
}

// ReorderChecklistItems reorders checklist items based on provided positions
func (u *checklistUsecase) ReorderChecklistItems(checklistID, userID uint, itemPositions map[uint]int) error {
	// Verify checklist exists
	checklist, err := u.checklistRepo.GetChecklistByID(checklistID)
	if err != nil {
		return fmt.Errorf("checklist not found: %w", err)
	}

	// Verify task exists
	_, err = u.taskRepo.GetByID(checklist.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Update each item's position
	for itemID, position := range itemPositions {
		item, err := u.checklistRepo.GetChecklistItemByID(itemID)
		if err != nil {
			return fmt.Errorf("checklist item %d not found: %w", itemID, err)
		}

		// Verify item belongs to this checklist
		if item.ChecklistID != checklistID {
			return fmt.Errorf("item %d does not belong to checklist %d", itemID, checklistID)
		}

		item.Position = position
		if err := u.checklistRepo.UpdateChecklistItem(item); err != nil {
			return fmt.Errorf("failed to update item %d position: %w", itemID, err)
		}
	}

	// Note: Reordering doesn't affect progress, so no need to recalculate

	return nil
}

// SetTaskUsecase sets the TaskUsecase to avoid circular dependency
func (u *checklistUsecase) SetTaskUsecase(taskUsecase TaskUsecase) {
	u.taskUsecase = taskUsecase
}
