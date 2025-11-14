package permissions

import (
	"context"
	"fmt"
	"time"

	"tachyon-messenger/services/task/models"
)

// TaskUserRole represents the role of a user in relation to a task
type TaskUserRole string

const (
	RoleCreator            TaskUserRole = "creator"               // Task creator
	RoleCurrentAssignee    TaskUserRole = "current_assignee"     // Current task assignee
	RoleSubtaskAssignee    TaskUserRole = "subtask_assignee"     // Subtask assignee
	RoleParentTaskCreator  TaskUserRole = "parent_task_creator"  // Creator of parent task viewing subtask of another user
	RoleDelegationChain    TaskUserRole = "delegation_chain"     // In delegation chain
	RoleNone               TaskUserRole = "none"                 // No access
)

// TaskPermissions represents user permissions for a task
type TaskPermissions struct {
	CanView              bool `json:"can_view"`               // View task
	CanViewSubtasks      bool `json:"can_view_subtasks"`      // View all subtasks
	CanEdit              bool `json:"can_edit"`               // Edit task
	CanChangeStatus      bool `json:"can_change_status"`      // Change status
	CanCheckItems        bool `json:"can_check_items"`        // Check checklist items
	CanCreateSubtasks    bool `json:"can_create_subtasks"`    // Create subtasks
	CanDelegate          bool `json:"can_delegate"`           // Delegate task
	CanEmergencyComplete bool `json:"can_emergency_complete"` // Emergency complete
	CanAssignUsers       bool `json:"can_assign_users"`       // Assign users
	CanDelete            bool `json:"can_delete"`             // Delete task
}

// DelegationInfo represents a user in the delegation chain
type DelegationInfo struct {
	DelegatorID uint      `json:"delegator_id"`
	DelegatedAt time.Time `json:"delegated_at"`
}

// GetUserTaskRole determines the role of a user in a task
func GetUserTaskRole(ctx context.Context, task *models.Task, userID uint) (TaskUserRole, error) {
	// 1. Check if user is a current assignee of this task
	isCurrentAssignee := false
	for _, assignee := range task.Assignees {
		if assignee.UserID == userID {
			isCurrentAssignee = true
			break
		}
	}

	// If user is assignee of a subtask, return SubtaskAssignee role
	if isCurrentAssignee && task.ParentTaskID != nil {
		return RoleSubtaskAssignee, nil
	}

	// If user is assignee of main task, return CurrentAssignee role
	if isCurrentAssignee {
		return RoleCurrentAssignee, nil
	}

	// 2. Check if user is the creator of this task
	if task.CreatedByUserID == userID {
		// If this is a subtask, check if user is also assignee
		// If not, user is ParentTaskCreator viewing someone else's subtask
		if task.ParentTaskID != nil {
			// User created the subtask but is not assignee = viewing others' subtask
			return RoleParentTaskCreator, nil
		}
		// User is creator of main task
		return RoleCreator, nil
	}

	// 3. Check delegation chain
	// User is in delegation chain if they delegated the task to someone else
	if task.DelegatedFromUserID != nil && *task.DelegatedFromUserID == userID {
		return RoleDelegationChain, nil
	}

	// Also check if user was the original assignee
	if task.OriginalAssigneeID != nil && *task.OriginalAssigneeID == userID {
		return RoleDelegationChain, nil
	}

	return RoleNone, nil
}

// GetTaskPermissions calculates user permissions for a task
func GetTaskPermissions(ctx context.Context, task *models.Task, userID uint) (*TaskPermissions, error) {
	role, err := GetUserTaskRole(ctx, task, userID)
	if err != nil {
		return nil, err
	}

	// Debug logging
	fmt.Printf("[GetTaskPermissions] Task ID: %d, User ID: %d, Role: %s\n", task.ID, userID, role)

	switch role {
	case RoleCreator:
		return getCreatorPermissions(task), nil

	case RoleCurrentAssignee:
		return getCurrentAssigneePermissions(task), nil

	case RoleSubtaskAssignee:
		return getSubtaskAssigneePermissions(task), nil

	case RoleParentTaskCreator:
		return getParentTaskCreatorPermissions(task), nil

	case RoleDelegationChain:
		return getDelegationChainPermissions(task), nil

	case RoleNone:
		return &TaskPermissions{}, nil

	default:
		return &TaskPermissions{}, nil
	}
}

// getCreatorPermissions returns permissions for task creator (full access)
func getCreatorPermissions(task *models.Task) *TaskPermissions {
	return &TaskPermissions{
		CanView:              true,
		CanViewSubtasks:      true,
		CanEdit:              true,
		CanChangeStatus:      true,
		CanCheckItems:        true, // Only in main task, not in others' subtasks
		CanCreateSubtasks:    true,
		CanDelegate:          true,
		CanEmergencyComplete: true,
		CanAssignUsers:       true,
		CanDelete:            true,
	}
}

// getCurrentAssigneePermissions returns permissions for current assignee of main task
func getCurrentAssigneePermissions(task *models.Task) *TaskPermissions {
	return &TaskPermissions{
		CanView:              true,
		CanViewSubtasks:      true,
		CanEdit:              false, // Current assignee cannot edit task details (title, description, etc.)
		CanChangeStatus:      true,
		CanCheckItems:        true,
		CanCreateSubtasks:    true,
		CanDelegate:          true,
		CanEmergencyComplete: false,
		CanAssignUsers:       false, // Only creator can assign users
		CanDelete:            false,
	}
}

// getSubtaskAssigneePermissions returns permissions for subtask assignee
func getSubtaskAssigneePermissions(task *models.Task) *TaskPermissions {
	return &TaskPermissions{
		CanView:              true,
		CanViewSubtasks:      false, // Cannot see other subtasks
		CanEdit:              false, // Cannot edit description
		CanChangeStatus:      true,  // Only their own subtask
		CanCheckItems:        true,  // Only in their own subtask
		CanCreateSubtasks:    false,
		CanDelegate:          false,
		CanEmergencyComplete: false,
		CanAssignUsers:       false,
		CanDelete:            false,
	}
}

// getParentTaskCreatorPermissions returns permissions for creator of parent task viewing subtask of another user
func getParentTaskCreatorPermissions(task *models.Task) *TaskPermissions {
	// Check for emergency completion (if task is overdue by more than 3 days)
	canEmergency := false
	if task.DueDate != nil {
		now := time.Now()
		threeDaysAgo := now.AddDate(0, 0, -3)
		// Task is overdue if DueDate is before threeDaysAgo
		if task.DueDate.Before(threeDaysAgo) {
			canEmergency = true
		}
		// Debug logging
		fmt.Printf("[ParentTaskCreator] Task ID: %d, DueDate: %v, Now: %v, ThreeDaysAgo: %v, CanEmergency: %v\n",
			task.ID, task.DueDate, now, threeDaysAgo, canEmergency)
	}

	return &TaskPermissions{
		CanView:              true,
		CanViewSubtasks:      true,  // Can view all subtasks
		CanEdit:              false, // Cannot edit subtask of another user
		CanChangeStatus:      false, // Cannot change status of subtask
		CanCheckItems:        false, // CANNOT check items in subtask of another user
		CanCreateSubtasks:    false, // Cannot create sub-subtasks
		CanDelegate:          false,
		CanEmergencyComplete: canEmergency, // Can emergency complete if overdue
		CanAssignUsers:       false,
		CanDelete:            true, // Can delete subtasks as parent task creator
	}
}

// getDelegationChainPermissions returns permissions for user in delegation chain (former assignee)
func getDelegationChainPermissions(task *models.Task) *TaskPermissions {
	// Check for emergency completion (if task is overdue by more than 3 days)
	canEmergency := false
	if task.DueDate != nil {
		now := time.Now()
		threeDaysAgo := now.AddDate(0, 0, -3)
		// Task is overdue if DueDate is before threeDaysAgo
		if task.DueDate.Before(threeDaysAgo) {
			canEmergency = true
		}
		// Debug logging
		fmt.Printf("[DelegationChain] Task ID: %d, DueDate: %v, Now: %v, ThreeDaysAgo: %v, CanEmergency: %v\n",
			task.ID, task.DueDate, now, threeDaysAgo, canEmergency)
	}

	return &TaskPermissions{
		CanView:              true,
		CanViewSubtasks:      true, // Can view all subtasks
		CanEdit:              false,
		CanChangeStatus:      false,
		CanCheckItems:        false, // Cannot check checklists
		CanCreateSubtasks:    false,
		CanDelegate:          false,
		CanEmergencyComplete: canEmergency, // Only if overdue by more than 3 days
		CanAssignUsers:       false,
		CanDelete:            false,
	}
}

// CanCheckChecklistItem checks if user can check a specific checklist item
// This ensures that only the assignee of the specific task (not subtasks of others) can check items
func CanCheckChecklistItem(ctx context.Context, task *models.Task, userID uint) (bool, error) {
	permissions, err := GetTaskPermissions(ctx, task, userID)
	if err != nil {
		return false, err
	}

	if !permissions.CanCheckItems {
		return false, nil
	}

	// Additional check: if this is a subtask, user must be its assignee
	if task.ParentTaskID != nil {
		isAssignee := false
		for _, assignee := range task.Assignees {
			if assignee.UserID == userID {
				isAssignee = true
				break
			}
		}
		return isAssignee, nil
	}

	return true, nil
}

// HasPermission checks if a user has a specific permission for a task
func HasPermission(ctx context.Context, task *models.Task, userID uint, action string) (bool, error) {
	perms, err := GetTaskPermissions(ctx, task, userID)
	if err != nil {
		return false, err
	}

	switch action {
	case "view":
		return perms.CanView, nil
	case "view_subtasks":
		return perms.CanViewSubtasks, nil
	case "edit":
		return perms.CanEdit, nil
	case "change_status":
		return perms.CanChangeStatus, nil
	case "check_items":
		return perms.CanCheckItems, nil
	case "create_subtasks":
		return perms.CanCreateSubtasks, nil
	case "delegate":
		return perms.CanDelegate, nil
	case "emergency_complete":
		return perms.CanEmergencyComplete, nil
	case "assign_users":
		return perms.CanAssignUsers, nil
	case "delete":
		return perms.CanDelete, nil
	default:
		return false, nil
	}
}
