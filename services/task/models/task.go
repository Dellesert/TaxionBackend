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
	Status      TaskStatus   `gorm:"not null;default:'new';size:20" json:"status" validate:"required,oneof=new in_progress review done cancelled"`
	Priority    TaskPriority `gorm:"not null;default:'medium';size:20" json:"priority" validate:"required,oneof=low medium high critical"`
	AssignedTo  *uint        `gorm:"index" json:"assigned_to,omitempty" validate:"omitempty,min=1"`
	CreatedBy   uint         `gorm:"not null;index" json:"created_by" validate:"required,min=1"`
	DueDate     *time.Time   `json:"due_date,omitempty"`

	// Associations
	Comments []TaskComment `gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE" json:"comments,omitempty"`

	// Computed fields (not stored in DB)
	CommentCount int `gorm:"-" json:"comment_count,omitempty"`
}

// TableName returns the table name for Task model
func (Task) TableName() string {
	return "tasks"
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
	return nil
}

// Request/Response Models

// CreateTaskRequest represents request for creating a task
type CreateTaskRequest struct {
	Title       string        `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description string        `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Priority    *TaskPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	AssignedTo  *uint         `json:"assigned_to,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	DueDate     *time.Time    `json:"due_date,omitempty"`
}

// UpdateTaskRequest represents request for updating a task
type UpdateTaskRequest struct {
	Title       *string       `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description *string       `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Status      *TaskStatus   `json:"status,omitempty" binding:"omitempty,oneof=new in_progress review done cancelled" validate:"omitempty,oneof=new in_progress review done cancelled"`
	Priority    *TaskPriority `json:"priority,omitempty" binding:"omitempty,oneof=low medium high critical" validate:"omitempty,oneof=low medium high critical"`
	AssignedTo  *uint         `json:"assigned_to,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	DueDate     *time.Time    `json:"due_date,omitempty"`
}

// UpdateTaskStatusRequest represents request for updating task status only
type UpdateTaskStatusRequest struct {
	Status TaskStatus `json:"status" binding:"required,oneof=new in_progress review done cancelled" validate:"required,oneof=new in_progress review done cancelled"`
}

// AssignTaskRequest represents request for assigning a task to a user
type AssignTaskRequest struct {
	AssignedTo uint `json:"assigned_to" binding:"required,min=1" validate:"required,min=1"`
}

// Response Models

// TaskResponse represents a task in API responses
type TaskResponse struct {
	ID           uint         `json:"id"`
	Title        string       `json:"title"`
	Description  string       `json:"description,omitempty"`
	Status       TaskStatus   `json:"status"`
	Priority     TaskPriority `json:"priority"`
	AssignedTo   *uint        `json:"assigned_to,omitempty"`
	CreatedBy    uint         `json:"created_by"`
	DueDate      *time.Time   `json:"due_date,omitempty"`
	CommentCount int          `json:"comment_count"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ToResponse converts Task model to TaskResponse
func (t *Task) ToResponse() *TaskResponse {
	return &TaskResponse{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Status:       t.Status,
		Priority:     t.Priority,
		AssignedTo:   t.AssignedTo,
		CreatedBy:    t.CreatedBy,
		DueDate:      t.DueDate,
		CommentCount: t.CommentCount,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
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

// TaskFilterRequest represents filtering parameters for tasks
type TaskFilterRequest struct {
	Status     *TaskStatus   `form:"status" binding:"omitempty,oneof=new in_progress review done cancelled"`
	Priority   *TaskPriority `form:"priority" binding:"omitempty,oneof=low medium high critical"`
	AssignedTo *uint         `form:"assigned_to" binding:"omitempty,min=1"`
	CreatedBy  *uint         `form:"created_by" binding:"omitempty,min=1"`
	DueBefore  *time.Time    `form:"due_before" time_format:"2006-01-02"`
	DueAfter   *time.Time    `form:"due_after" time_format:"2006-01-02"`
	Limit      int           `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset     int           `form:"offset" binding:"omitempty,min=0"`
	SortBy     string        `form:"sort_by" binding:"omitempty,oneof=created_at updated_at due_date priority title"`
	SortOrder  string        `form:"sort_order" binding:"omitempty,oneof=asc desc"`
}
