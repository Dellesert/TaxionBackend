package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusNew        TaskStatus = "new"
	TaskStatusViewed     TaskStatus = "viewed" // When task is first viewed by assignee
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskPriority represents the priority level of a task
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// Task represents a task in the system
type Task struct {
	models.BaseModel
	Title       string       `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Description string       `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=2000"`
	Status      TaskStatus   `gorm:"not null;default:'new';size:20" json:"status" validate:"required,oneof=new viewed in_progress review done cancelled"`
	Priority    TaskPriority `gorm:"not null;default:'medium';size:20" json:"priority" validate:"required,oneof=low medium high critical"`

	// Hierarchy support
	ParentTaskID *uint `gorm:"index" json:"parent_task_id,omitempty"`

	// Assignment and delegation
	CreatedByUserID       uint  `gorm:"not null;index;column:created_by_user_id" json:"created_by_user_id" validate:"required,min=1"`
	AssignedToUserID      *uint `gorm:"index;column:assigned_to_user_id" json:"assigned_to_user_id,omitempty"`
	DelegatedFromUserID   *uint `gorm:"index;column:delegated_from_user_id" json:"delegated_from_user_id,omitempty"`
	OriginalAssigneeID    *uint `gorm:"column:original_assignee_id" json:"original_assignee_id,omitempty"`
	AssignedToDepartment  *uint `gorm:"index;column:assigned_to_department_id" json:"assigned_to_department_id,omitempty"`

	// Progress tracking
	ProgressPercentage        int        `gorm:"default:0" json:"progress_percentage" validate:"min=0,max=100"`
	FirstViewedAt             *time.Time `json:"first_viewed_at,omitempty"`
	FirstViewedByUserID       *uint      `json:"first_viewed_by_user_id,omitempty"`
	LastStatusChangedByUserID *uint      `gorm:"index;column:last_status_changed_by_user_id" json:"last_status_changed_by_user_id,omitempty"`

	// Dates
	DueDate     *time.Time `json:"due_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Notification tracking
	LastDeadlineNotificationSentAt *time.Time `json:"last_deadline_notification_sent_at,omitempty"`

	// Associations
	ParentTask  *Task              `gorm:"foreignKey:ParentTaskID" json:"parent_task,omitempty"`
	Subtasks    []Task             `gorm:"foreignKey:ParentTaskID;constraint:OnDelete:CASCADE" json:"subtasks,omitempty"`
	Comments    []TaskComment      `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"comments,omitempty"`
	Assignees   []TaskAssignee     `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"assignees,omitempty"`
	Activities  []TaskActivity     `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"activities,omitempty"`
	Attachments []TaskAttachment   `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"attachments,omitempty"`
	Checklists  []TaskChecklist    `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"checklists,omitempty"`

	// Computed fields (not stored in DB)
	CommentCount    int `gorm:"-" json:"comment_count,omitempty"`
	SubtaskCount    int `gorm:"-" json:"subtask_count,omitempty"`
	AttachmentCount int `gorm:"-" json:"attachment_count,omitempty"`

	// Backward compatibility (deprecated fields)
	CreatedBy           uint    `gorm:"column:created_by" json:"created_by,omitempty"`
	AssignedTo          *uint   `gorm:"column:assigned_to" json:"assigned_to,omitempty"`
	LastStatusChangedBy *uint   `gorm:"column:last_status_changed_by" json:"last_status_changed_by,omitempty"`
	AssignedToDept      *string `gorm:"-" json:"assigned_to_department,omitempty"`
}

// TaskAssignee represents a user assigned to a task (many-to-many relationship)
type TaskAssignee struct {
	models.BaseModel
	TaskID         uint       `gorm:"not null;index" json:"task_id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	AssignedByUserID *uint    `json:"assigned_by_user_id,omitempty"`
	AssignedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"assigned_at"`
}

// TaskActivity represents an activity/action performed on a task
type TaskActivity struct {
	models.BaseModel
	TaskID     uint    `gorm:"not null;index" json:"task_id"`
	UserID     uint    `gorm:"not null;index" json:"user_id"`
	ActionType string  `gorm:"not null;size:50;index" json:"action_type"`
	OldValue   string  `gorm:"type:text" json:"old_value,omitempty"`
	NewValue   string  `gorm:"type:text" json:"new_value,omitempty"`
	Details    *string `gorm:"type:jsonb" json:"details,omitempty"` // JSONB field for PostgreSQL (nullable)
}

// TaskAttachment represents a file attached to a task
type TaskAttachment struct {
	models.BaseModel
	TaskID            uint   `gorm:"not null;index" json:"task_id"`
	UploadedByUserID  uint   `gorm:"not null;index" json:"uploaded_by_user_id"`
	FileName          string `gorm:"not null;size:255" json:"file_name"`
	FilePath          string `gorm:"not null;size:500" json:"file_path"`
	FileType          string `gorm:"size:50" json:"file_type,omitempty"`
	FileSize          int64  `json:"file_size,omitempty"`
}

// TaskChecklist represents a checklist within a task
type TaskChecklist struct {
	models.BaseModel
	TaskID      uint              `gorm:"not null;index" json:"task_id"`
	Title       string            `gorm:"not null;size:255" json:"title"`
	Description string            `gorm:"type:text" json:"description,omitempty"`
	Position    int               `gorm:"default:0;index" json:"position"`
	Items       []TaskChecklistItem `gorm:"foreignKey:ChecklistID;constraint:OnDelete:CASCADE" json:"items,omitempty"`
}

// TaskChecklistItem represents an item within a checklist
type TaskChecklistItem struct {
	models.BaseModel
	ChecklistID       uint       `gorm:"not null;index" json:"checklist_id"`
	Title             string     `gorm:"not null;size:500" json:"title"`
	IsCompleted       bool       `gorm:"default:false;index" json:"is_completed"`
	CompletedByUserID *uint      `json:"completed_by_user_id,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Position          int        `gorm:"default:0;index" json:"position"`
}

// TableName returns the table name for Task model
func (Task) TableName() string {
	return "tasks"
}

// TableName returns the table name for TaskAssignee model
func (TaskAssignee) TableName() string {
	return "task_assignees"
}

// TableName returns the table name for TaskActivity model
func (TaskActivity) TableName() string {
	return "task_activities"
}

// TableName returns the table name for TaskAttachment model
func (TaskAttachment) TableName() string {
	return "task_attachments"
}

// TableName returns the table name for TaskChecklist model
func (TaskChecklist) TableName() string {
	return "task_checklists"
}

// TableName returns the table name for TaskChecklistItem model
func (TaskChecklistItem) TableName() string {
	return "task_checklist_items"
}

// BeforeCreate hook is called before creating a task
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if t.Status == "" {
		t.Status = TaskStatusNew
	}
	if t.Priority == "" {
		t.Priority = TaskPriorityMedium
	}

	// Initialize progress to 0 if not set
	if t.ProgressPercentage < 0 || t.ProgressPercentage > 100 {
		t.ProgressPercentage = 0
	}

	// Set backward compatibility fields
	t.CreatedBy = t.CreatedByUserID
	t.AssignedTo = t.AssignedToUserID
	t.LastStatusChangedBy = t.LastStatusChangedByUserID

	return nil
}

// AfterFind hook is called after loading a task from database
func (t *Task) AfterFind(tx *gorm.DB) error {
	// Set backward compatibility fields
	t.CreatedBy = t.CreatedByUserID
	t.AssignedTo = t.AssignedToUserID
	t.LastStatusChangedBy = t.LastStatusChangedByUserID

	return nil
}

// Request/Response Models

// CreateTaskRequest represents request for creating a task
type CreateTaskRequest struct {
	Title                string                    `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description          string                    `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Priority             *TaskPriority             `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	AssignedToUserID     *uint                     `json:"assigned_to_user_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	AssigneeIDs          []uint                    `json:"assignee_ids,omitempty" validate:"omitempty,dive,min=1"`
	AssignedToDepartment *uint                     `json:"assigned_to_department_id,omitempty" validate:"omitempty,min=1"`
	ParentTaskID         *uint                     `json:"parent_task_id,omitempty" validate:"omitempty,min=1"`
	DueDate              *time.Time                `json:"due_date,omitempty"`
	Checklists           []CreateChecklistRequest  `json:"checklists,omitempty" validate:"omitempty,dive"`
	ParentAttachmentIDs  []uint                    `json:"parent_attachment_ids,omitempty" validate:"omitempty,dive,min=1"` // IDs of attachments from parent task to copy

	// Backward compatibility
	AssignedTo           *uint   `json:"assigned_to,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"` // Deprecated
	AssignedToDept       *string `json:"assigned_to_department,omitempty" validate:"omitempty,max=100"` // Deprecated
}

// UpdateTaskRequest represents request for updating a task
type UpdateTaskRequest struct {
	Title                *string       `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description          *string       `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Status               *TaskStatus   `json:"status,omitempty" binding:"omitempty,oneof=new viewed in_progress review done cancelled" validate:"omitempty,oneof=new viewed in_progress review done cancelled"`
	Priority             *TaskPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	AssignedToUserID     *uint         `json:"assigned_to_user_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	AssigneeIDs          []uint        `json:"assignee_ids,omitempty" validate:"omitempty,dive,min=1"`
	AssignedToDepartment *uint         `json:"assigned_to_department_id,omitempty" validate:"omitempty,min=1"`
	DueDate              *time.Time    `json:"due_date,omitempty"`

	// Backward compatibility
	AssignedTo           *uint   `json:"assigned_to,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"` // Deprecated
	AssignedToDept       *string `json:"assigned_to_department,omitempty" validate:"omitempty,max=100"` // Deprecated
}

// UpdateTaskStatusRequest represents request for updating task status only
type UpdateTaskStatusRequest struct {
	Status TaskStatus `json:"status" binding:"required,oneof=new viewed in_progress review done cancelled" validate:"required,oneof=new viewed in_progress review done cancelled"`
}

// DelegateTaskRequest represents request for delegating a task
type DelegateTaskRequest struct {
	DelegateToUserID uint   `json:"delegate_to_user_id" binding:"required,min=1" validate:"required,min=1"`
	Comment          string `json:"comment,omitempty" validate:"omitempty,max=500"`
}

// AssignTaskRequest represents request for assigning a task to a user
type AssignTaskRequest struct {
	AssignedTo uint `json:"assigned_to" binding:"required,min=1" validate:"required,min=1"`
}

// Response Models

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

// TaskResponse represents a task in API responses
type TaskResponse struct {
	ID                   uint         `json:"id"`
	Title                string       `json:"title"`
	Description          string       `json:"description,omitempty"`
	Status               TaskStatus   `json:"status"`
	Priority             TaskPriority `json:"priority"`

	// Hierarchy
	ParentTaskID         *uint        `json:"parent_task_id,omitempty"`

	// Assignment and delegation
	CreatedByUserID      uint         `json:"created_by_user_id"`
	AssignedToUserID     *uint        `json:"assigned_to_user_id,omitempty"`
	DelegatedFromUserID  *uint        `json:"delegated_from_user_id,omitempty"`
	OriginalAssigneeID   *uint        `json:"original_assignee_id,omitempty"`
	AssignedToDepartment *uint        `json:"assigned_to_department_id,omitempty"`
	AssigneeIDs          []uint       `json:"assignee_ids,omitempty"`

	// User info
	Creator              *UserInfo    `json:"creator,omitempty"`
	AssignedToUser       *UserInfo    `json:"assigned_to_user,omitempty"`
	DelegatedFromUser    *UserInfo    `json:"delegated_from_user,omitempty"`
	OriginalAssignee     *UserInfo    `json:"original_assignee,omitempty"`
	Assignees            []UserInfo   `json:"assignees,omitempty"`

	// Progress tracking
	ProgressPercentage        int        `json:"progress_percentage"`
	FirstViewedAt             *time.Time `json:"first_viewed_at,omitempty"`
	FirstViewedByUser         *UserInfo  `json:"first_viewed_by_user,omitempty"`
	LastStatusChangedByUserID *uint      `json:"last_status_changed_by_user_id,omitempty"`
	LastStatusChanger         *UserInfo  `json:"last_status_changer,omitempty"`

	// Dates
	DueDate              *time.Time   `json:"due_date,omitempty"`
	CompletedAt          *time.Time   `json:"completed_at,omitempty"`

	// Counts
	CommentCount         int          `json:"comment_count"`
	SubtaskCount         int          `json:"subtask_count"`
	AttachmentCount      int          `json:"attachment_count"`

	// Delegation chain (list of users from top to current assignee)
	DelegationChain      []UserInfo   `json:"delegation_chain,omitempty"`

	// Permissions - user's permissions for this task
	Permissions          *TaskPermissions `json:"permissions,omitempty"`

	// Timestamps
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`

	// Backward compatibility
	AssignedTo           *uint        `json:"assigned_to,omitempty"` // Deprecated
	CreatedBy            uint         `json:"created_by"` // Deprecated
	LastStatusChangedBy  *uint        `json:"last_status_changed_by,omitempty"` // Deprecated
	AssignedToDept       *string      `json:"assigned_to_department,omitempty"` // Deprecated
}

// ToResponse converts Task model to TaskResponse
func (t *Task) ToResponse() *TaskResponse {
	// Extract assignee IDs from Assignees
	assigneeIDs := make([]uint, 0, len(t.Assignees))
	for _, assignee := range t.Assignees {
		assigneeIDs = append(assigneeIDs, assignee.UserID)
	}

	return &TaskResponse{
		ID:                        t.ID,
		Title:                     t.Title,
		Description:               t.Description,
		Status:                    t.Status,
		Priority:                  t.Priority,
		ParentTaskID:              t.ParentTaskID,
		CreatedByUserID:           t.CreatedByUserID,
		AssignedToUserID:          t.AssignedToUserID,
		DelegatedFromUserID:       t.DelegatedFromUserID,
		OriginalAssigneeID:        t.OriginalAssigneeID,
		AssignedToDepartment:      t.AssignedToDepartment,
		AssigneeIDs:               assigneeIDs,
		ProgressPercentage:        t.ProgressPercentage,
		FirstViewedAt:             t.FirstViewedAt,
		LastStatusChangedByUserID: t.LastStatusChangedByUserID,
		DueDate:                   t.DueDate,
		CompletedAt:               t.CompletedAt,
		CommentCount:              t.CommentCount,
		SubtaskCount:              t.SubtaskCount,
		AttachmentCount:           t.AttachmentCount,
		CreatedAt:                 t.CreatedAt,
		UpdatedAt:                 t.UpdatedAt,

		// Backward compatibility
		AssignedTo:          t.AssignedToUserID,
		CreatedBy:           t.CreatedByUserID,
		LastStatusChangedBy: t.LastStatusChangedByUserID,
	}
}

// TaskStatsResponse represents task statistics
type TaskStatsResponse struct {
	TotalTasks        int `json:"total_tasks"`
	NewTasks          int `json:"new_tasks"`
	InProgressTasks   int `json:"in_progress_tasks"`
	ReviewTasks       int `json:"review_tasks"`
	DoneTasks         int `json:"done_tasks"`
	CancelledTasks    int `json:"cancelled_tasks"`
	OverdueTasks      int `json:"overdue_tasks"`
	TasksAssignedToMe int `json:"tasks_assigned_to_me"`
	TasksCreatedByMe  int `json:"tasks_created_by_me"`
}

// TaskStatsInternalResponse represents task statistics for analytics (no user-specific data)
type TaskStatsInternalResponse struct {
	TotalTasks      int `json:"total_tasks"`
	NewTasks        int `json:"new_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	ReviewTasks     int `json:"review_tasks"`
	CompletedTasks  int `json:"completed_tasks"`
	CancelledTasks  int `json:"cancelled_tasks"`
	OverdueTasks    int `json:"overdue_tasks"`
}

// TaskFilterRequest represents filtering parameters for tasks
type TaskFilterRequest struct {
	Status           *TaskStatus   `form:"status" binding:"omitempty,oneof=new viewed in_progress review done cancelled"`
	Priority         *TaskPriority `form:"priority" binding:"omitempty,oneof=low medium high critical"`
	AssignedTo       *uint         `form:"assigned_to" binding:"omitempty,min=1"`
	AssignedToUserID *uint         `form:"assigned_to_user_id" binding:"omitempty,min=1"`
	CreatedBy        *uint         `form:"created_by" binding:"omitempty,min=1"`
	CreatedByUserID  *uint         `form:"created_by_user_id" binding:"omitempty,min=1"`
	ParentTaskID     *uint         `form:"parent_task_id" binding:"omitempty,min=1"`
	IsSubtask        *bool         `form:"is_subtask"` // true = only subtasks, false = only parent tasks
	DueBefore        *time.Time    `form:"due_before" time_format:"2006-01-02"`
	DueAfter         *time.Time    `form:"due_after" time_format:"2006-01-02"`
	Search           string        `form:"search" binding:"omitempty"` // Text search in title and description
	Limit            int           `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset           int           `form:"offset" binding:"omitempty,min=0"`
	SortBy           string        `form:"sort_by" binding:"omitempty,oneof=created_at updated_at due_date priority title progress_percentage"`
	SortOrder        string        `form:"sort_order" binding:"omitempty,oneof=asc desc"`

	// Incremental sync parameters
	UpdatedSince *time.Time `form:"updated_since" time_format:"2006-01-02T15:04:05Z07:00"` // For incremental sync: only records updated after this timestamp
}

// TaskSyncListResponse represents a sync-aware list response for tasks
type TaskSyncListResponse struct {
	Tasks      []*TaskResponse `json:"data"`                  // List of tasks (renamed to "data" for consistency)
	Total      int64           `json:"total"`                 // Total count matching filters
	DeletedIDs []uint          `json:"deleted_ids,omitempty"` // IDs of deleted tasks since updated_since
	ServerTime time.Time       `json:"server_time"`           // Server timestamp for next sync request
	Limit      int             `json:"limit"`
	Offset     int             `json:"offset"`
}

// Activity-related models

// TaskActivityResponse represents a task activity in API responses
type TaskActivityResponse struct {
	ID         uint        `json:"id"`
	TaskID     uint        `json:"task_id"`
	TaskTitle  string      `json:"task_title,omitempty"`
	UserID     uint        `json:"user_id"`
	User       *UserInfo   `json:"user,omitempty"`
	ActionType string      `json:"action_type"`
	OldValue   string      `json:"old_value,omitempty"`
	NewValue   string      `json:"new_value,omitempty"`
	Details    *string     `json:"details,omitempty"`
	Assignees  []*UserInfo `json:"assignees,omitempty"` // For subtask_created activities
	CreatedAt  time.Time   `json:"created_at"`
}

// ToActivityResponse converts TaskActivity to TaskActivityResponse
func (a *TaskActivity) ToResponse() *TaskActivityResponse {
	return &TaskActivityResponse{
		ID:         a.ID,
		TaskID:     a.TaskID,
		UserID:     a.UserID,
		ActionType: a.ActionType,
		OldValue:   a.OldValue,
		NewValue:   a.NewValue,
		Details:    a.Details,
		CreatedAt:  a.CreatedAt,
	}
}

// Attachment-related models

// CreateAttachmentRequest represents request for uploading an attachment
type CreateAttachmentRequest struct {
	FileName string `json:"file_name" binding:"required" validate:"required,max=255"`
	FilePath string `json:"file_path" binding:"required" validate:"required,max=500"`
	FileType string `json:"file_type,omitempty" validate:"omitempty,max=50"`
	FileSize int64  `json:"file_size,omitempty"`
}

// TaskAttachmentResponse represents a task attachment in API responses
type TaskAttachmentResponse struct {
	ID               uint      `json:"id"`
	TaskID           uint      `json:"task_id"`
	UploadedByUserID uint      `json:"uploaded_by_user_id"`
	UploadedBy       *UserInfo `json:"uploaded_by,omitempty"`
	FileName         string    `json:"file_name"`
	FilePath         string    `json:"file_path"`
	FileType         string    `json:"file_type,omitempty"`
	FileSize         int64     `json:"file_size,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToAttachmentResponse converts TaskAttachment to TaskAttachmentResponse
func (a *TaskAttachment) ToResponse() *TaskAttachmentResponse {
	return &TaskAttachmentResponse{
		ID:               a.ID,
		TaskID:           a.TaskID,
		UploadedByUserID: a.UploadedByUserID,
		FileName:         a.FileName,
		FilePath:         a.FilePath,
		FileType:         a.FileType,
		FileSize:         a.FileSize,
		CreatedAt:        a.CreatedAt,
	}
}

// Checklist-related models

// CreateChecklistRequest represents request for creating a checklist
type CreateChecklistRequest struct {
	Title       string   `json:"title" binding:"required" validate:"required,max=255"`
	Description string   `json:"description,omitempty" validate:"omitempty,max=2000"`
	Items       []string `json:"items,omitempty" validate:"omitempty,dive,max=500"`
}

// UpdateChecklistRequest represents request for updating a checklist
type UpdateChecklistRequest struct {
	Title       *string `json:"title,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
}

// CreateChecklistItemRequest represents request for creating a checklist item
type CreateChecklistItemRequest struct {
	Title string `json:"title" binding:"required" validate:"required,max=500"`
}

// UpdateChecklistItemRequest represents request for updating a checklist item
type UpdateChecklistItemRequest struct {
	Title       *string `json:"title,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	IsCompleted *bool   `json:"is_completed,omitempty"`
}

// TaskChecklistResponse represents a task checklist in API responses
type TaskChecklistResponse struct {
	ID          uint                       `json:"id"`
	TaskID      uint                       `json:"task_id"`
	Title       string                     `json:"title"`
	Description string                     `json:"description,omitempty"`
	Position    int                        `json:"position"`
	Items       []TaskChecklistItemResponse `json:"items,omitempty"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}

// TaskChecklistItemResponse represents a checklist item in API responses
type TaskChecklistItemResponse struct {
	ID                uint      `json:"id"`
	ChecklistID       uint      `json:"checklist_id"`
	Title             string    `json:"title"`
	IsCompleted       bool      `json:"is_completed"`
	CompletedByUserID *uint     `json:"completed_by_user_id,omitempty"`
	CompletedBy       *UserInfo `json:"completed_by,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Position          int       `json:"position"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ToChecklistResponse converts TaskChecklist to TaskChecklistResponse
func (c *TaskChecklist) ToResponse() *TaskChecklistResponse {
	items := make([]TaskChecklistItemResponse, 0, len(c.Items))
	for _, item := range c.Items {
		items = append(items, *item.ToResponse())
	}

	return &TaskChecklistResponse{
		ID:          c.ID,
		TaskID:      c.TaskID,
		Title:       c.Title,
		Description: c.Description,
		Position:    c.Position,
		Items:       items,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// ToResponse converts TaskChecklistItem to TaskChecklistItemResponse
func (i *TaskChecklistItem) ToResponse() *TaskChecklistItemResponse {
	return &TaskChecklistItemResponse{
		ID:                i.ID,
		ChecklistID:       i.ChecklistID,
		Title:             i.Title,
		IsCompleted:       i.IsCompleted,
		CompletedByUserID: i.CompletedByUserID,
		CompletedAt:       i.CompletedAt,
		Position:          i.Position,
		CreatedAt:         i.CreatedAt,
		UpdatedAt:         i.UpdatedAt,
	}
}
