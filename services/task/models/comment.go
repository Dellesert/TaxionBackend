package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// TaskComment represents a comment on a task (moved from task.go for better organization)
type TaskComment struct {
	models.BaseModel
	TaskID   uint   `gorm:"not null;index" json:"task_id" validate:"required"`
	UserID   uint   `gorm:"not null;index" json:"user_id" validate:"required"`
	Content  string `gorm:"not null;type:text" json:"content" validate:"required,min=1,max=1000"`
	ParentID *uint  `gorm:"index" json:"parent_id,omitempty" validate:"omitempty,min=1"`

	// Associations
	Task    *Task         `gorm:"foreignKey:TaskID" json:"task,omitempty"`
	Parent  *TaskComment  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies []TaskComment `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE" json:"replies,omitempty"`
}

// TableName returns the table name for TaskComment model
func (TaskComment) TableName() string {
	return "task_comments"
}

// BeforeCreate hook is called before creating a task comment
func (tc *TaskComment) BeforeCreate(tx *gorm.DB) error {
	// Validate that parent comment belongs to the same task if ParentID is set
	if tc.ParentID != nil {
		var parentComment TaskComment
		if err := tx.Where("id = ? AND task_id = ?", *tc.ParentID, tc.TaskID).First(&parentComment).Error; err != nil {
			return gorm.ErrForeignKeyViolated
		}
	}
	return nil
}

// Request Models for Comments

// CreateTaskCommentRequest represents request for creating a task comment
type CreateTaskCommentRequest struct {
	Content  string `json:"content" binding:"required,min=1,max=1000" validate:"required,min=1,max=1000"`
	ParentID *uint  `json:"parent_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
}

// UpdateTaskCommentRequest represents request for updating a task comment
type UpdateTaskCommentRequest struct {
	Content string `json:"content" binding:"required,min=1,max=1000" validate:"required,min=1,max=1000"`
}

// Response Models for Comments

// TaskCommentResponse represents a task comment in API responses
type TaskCommentResponse struct {
	ID        uint                   `json:"id"`
	TaskID    uint                   `json:"task_id"`
	UserID    uint                   `json:"user_id"`
	Content   string                 `json:"content"`
	ParentID  *uint                  `json:"parent_id,omitempty"`
	Replies   []*TaskCommentResponse `json:"replies,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// ToResponse converts TaskComment model to TaskCommentResponse
func (tc *TaskComment) ToResponse() *TaskCommentResponse {
	response := &TaskCommentResponse{
		ID:        tc.ID,
		TaskID:    tc.TaskID,
		UserID:    tc.UserID,
		Content:   tc.Content,
		ParentID:  tc.ParentID,
		CreatedAt: tc.CreatedAt,
		UpdatedAt: tc.UpdatedAt,
	}

	// Convert replies if they exist
	if len(tc.Replies) > 0 {
		response.Replies = make([]*TaskCommentResponse, len(tc.Replies))
		for i, reply := range tc.Replies {
			response.Replies[i] = reply.ToResponse()
		}
	}

	return response
}

// CommentListResponse represents a paginated list of comments
type CommentListResponse struct {
	Comments []*TaskCommentResponse `json:"comments"`
	Total    int64                  `json:"total"`
	Limit    int                    `json:"limit"`
	Offset   int                    `json:"offset"`
}

// CommentFilterRequest represents filtering parameters for comments
type CommentFilterRequest struct {
	UserID *uint `form:"user_id" binding:"omitempty,min=1"`
	Limit  int   `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset int   `form:"offset" binding:"omitempty,min=0"`
}
