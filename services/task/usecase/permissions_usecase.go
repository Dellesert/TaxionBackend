package usecase

import (
	"context"
	"fmt"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/permissions"
)

// GetTaskPermissions returns permissions for a user on a specific task
func (u *taskUsecase) GetTaskPermissions(ctx context.Context, taskID uint, userID uint) (*models.TaskPermissions, error) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Get permissions from permissions package
	perms, err := permissions.GetTaskPermissions(ctx, task, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get permissions: %w", err)
	}

	// Convert from permissions.TaskPermissions to models.TaskPermissions
	return &models.TaskPermissions{
		CanView:              perms.CanView,
		CanViewSubtasks:      perms.CanViewSubtasks,
		CanEdit:              perms.CanEdit,
		CanChangeStatus:      perms.CanChangeStatus,
		CanCheckItems:        perms.CanCheckItems,
		CanCreateSubtasks:    perms.CanCreateSubtasks,
		CanDelegate:          perms.CanDelegate,
		CanEmergencyComplete: perms.CanEmergencyComplete,
		CanAssignUsers:       perms.CanAssignUsers,
		CanDelete:            perms.CanDelete,
	}, nil
}

// enrichTaskResponseWithPermissions adds permissions to task response
func (u *taskUsecase) enrichTaskResponseWithPermissions(ctx context.Context, response *models.TaskResponse, userID uint) error {
	// Get task
	task, err := u.taskRepo.GetByID(response.ID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Get permissions
	perms, err := permissions.GetTaskPermissions(ctx, task, userID)
	if err != nil {
		return fmt.Errorf("failed to get permissions: %w", err)
	}

	// Convert and attach to response
	response.Permissions = &models.TaskPermissions{
		CanView:              perms.CanView,
		CanViewSubtasks:      perms.CanViewSubtasks,
		CanEdit:              perms.CanEdit,
		CanChangeStatus:      perms.CanChangeStatus,
		CanCheckItems:        perms.CanCheckItems,
		CanCreateSubtasks:    perms.CanCreateSubtasks,
		CanDelegate:          perms.CanDelegate,
		CanEmergencyComplete: perms.CanEmergencyComplete,
		CanAssignUsers:       perms.CanAssignUsers,
		CanDelete:            perms.CanDelete,
	}

	return nil
}

// enrichTasksWithPermissions adds permissions to multiple task responses
func (u *taskUsecase) enrichTasksWithPermissions(ctx context.Context, responses []*models.TaskResponse, userID uint) error {
	if len(responses) == 0 {
		return nil
	}

	// For each task, get permissions and attach them
	for _, response := range responses {
		// Get task from cache/repository
		task, err := u.taskRepo.GetByID(response.ID)
		if err != nil {
			// Log error but continue with other tasks
			fmt.Printf("Failed to get task %d for permissions enrichment: %v\n", response.ID, err)
			continue
		}

		// Get permissions
		perms, err := permissions.GetTaskPermissions(ctx, task, userID)
		if err != nil {
			// Log error but continue with other tasks
			fmt.Printf("Failed to get permissions for task %d: %v\n", response.ID, err)
			continue
		}

		// Attach to response
		response.Permissions = &models.TaskPermissions{
			CanView:              perms.CanView,
			CanViewSubtasks:      perms.CanViewSubtasks,
			CanEdit:              perms.CanEdit,
			CanChangeStatus:      perms.CanChangeStatus,
			CanCheckItems:        perms.CanCheckItems,
			CanCreateSubtasks:    perms.CanCreateSubtasks,
			CanDelegate:          perms.CanDelegate,
			CanEmergencyComplete: perms.CanEmergencyComplete,
			CanAssignUsers:       perms.CanAssignUsers,
			CanDelete:            perms.CanDelete,
		}
	}

	return nil
}

// EmergencyCompleteTask completes a task in emergency mode (for users in delegation chain when task is overdue)
func (u *taskUsecase) EmergencyCompleteTask(ctx context.Context, taskID uint, userID uint) error {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Check permissions
	perms, err := permissions.GetTaskPermissions(ctx, task, userID)
	if err != nil {
		return fmt.Errorf("failed to get permissions: %w", err)
	}

	if !perms.CanEmergencyComplete {
		return fmt.Errorf("emergency completion is only allowed for overdue tasks (>3 days) by users in delegation chain")
	}

	// Update task status to done
	now := time.Now()
	task.Status = models.TaskStatusDone
	task.CompletedAt = &now
	task.LastStatusChangedByUserID = &userID

	if err := u.taskRepo.Update(task); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	// Log activity with emergency flag
	u.logActivity(taskID, userID, "task_emergency_completed", string(task.Status), string(models.TaskStatusDone), map[string]interface{}{
		"emergency":     true,
		"completed_at":  now,
		"completed_by":  userID,
		"original_status": task.Status,
	})

	// Recalculate progress
	u.RecalculateTaskProgress(taskID)

	// If task has parent, recalculate parent progress
	if task.ParentTaskID != nil {
		u.RecalculateTaskProgress(*task.ParentTaskID)
	}

	return nil
}

// CheckPermission checks if user has a specific permission for a task
func (u *taskUsecase) CheckPermission(ctx context.Context, taskID uint, userID uint, action string) (bool, error) {
	// Get task
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return false, fmt.Errorf("task not found: %w", err)
	}

	// Check permission
	hasPermission, err := permissions.HasPermission(ctx, task, userID, action)
	if err != nil {
		return false, fmt.Errorf("failed to check permission: %w", err)
	}

	return hasPermission, nil
}
