package models

import (
	"time"

	"gorm.io/gorm"
)

// UserActivity represents daily user activity statistics
type UserActivity struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         uint64    `gorm:"not null;uniqueIndex:idx_user_activity_unique" json:"user_id"`
	Date           time.Time `gorm:"type:date;not null;uniqueIndex:idx_user_activity_unique;index:idx_user_activity_date" json:"date"`
	SessionsCount  int       `gorm:"default:0" json:"sessions_count"`
	MessagesSent   int       `gorm:"default:0" json:"messages_sent"`
	TasksCreated   int       `gorm:"default:0" json:"tasks_created"`
	TasksCompleted int       `gorm:"default:0" json:"tasks_completed"`
	EventsCreated  int       `gorm:"default:0" json:"events_created"`
	PollsCreated   int       `gorm:"default:0" json:"polls_created"`
	FilesUploaded  int       `gorm:"default:0" json:"files_uploaded"`
	LastActiveAt   *time.Time `json:"last_active_at,omitempty"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for UserActivity
func (UserActivity) TableName() string {
	return "user_activity"
}

// BeforeCreate hook
func (u *UserActivity) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook
func (u *UserActivity) BeforeUpdate(tx *gorm.DB) error {
	u.UpdatedAt = time.Now()
	return nil
}

// DepartmentStats represents daily statistics per department
type DepartmentStats struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	DepartmentID   uint64    `gorm:"not null;uniqueIndex:idx_dept_stats_unique" json:"department_id"`
	Date           time.Time `gorm:"type:date;not null;uniqueIndex:idx_dept_stats_unique;index:idx_dept_stats_date" json:"date"`
	ActiveUsers    int       `gorm:"default:0" json:"active_users"`
	TotalMessages  int       `gorm:"default:0" json:"total_messages"`
	TotalTasks     int       `gorm:"default:0" json:"total_tasks"`
	CompletedTasks int       `gorm:"default:0" json:"completed_tasks"`
	TotalEvents    int       `gorm:"default:0" json:"total_events"`
	CreatedAt      time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for DepartmentStats
func (DepartmentStats) TableName() string {
	return "department_stats"
}

// BeforeCreate hook
func (d *DepartmentStats) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.UpdatedAt.IsZero() {
		d.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook
func (d *DepartmentStats) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = time.Now()
	return nil
}
