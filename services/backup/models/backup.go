package models

import (
	"time"

	sharedmodels "tachyon-messenger/shared/models"
)

// BackupStatus represents the status of a backup
type BackupStatus string

const (
	BackupStatusPending    BackupStatus = "pending"
	BackupStatusInProgress BackupStatus = "in_progress"
	BackupStatusCompleted  BackupStatus = "completed"
	BackupStatusFailed     BackupStatus = "failed"
)

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeManual    BackupType = "manual"
	BackupTypeScheduled BackupType = "scheduled"
	BackupTypeAutomatic BackupType = "automatic"
)

// Backup represents a database backup
type Backup struct {
	sharedmodels.BaseModel
	FileName     string       `gorm:"not null;size:255;uniqueIndex" json:"file_name"`
	FilePath     string       `gorm:"not null;size:512" json:"file_path"`
	FileSize     int64        `gorm:"not null;default:0" json:"file_size"`
	Status       BackupStatus `gorm:"not null;default:'pending';size:20;index" json:"status"`
	Type         BackupType   `gorm:"not null;default:'manual';size:20;index" json:"type"`
	CreatedBy    uint         `gorm:"not null;index" json:"created_by"`
	Description  string       `gorm:"size:500" json:"description,omitempty"`
	ErrorMessage string       `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
}

// TableName returns the table name for Backup model
func (Backup) TableName() string {
	return "backups"
}

// CreateBackupRequest represents request for creating a backup
type CreateBackupRequest struct {
	Description string `json:"description,omitempty" binding:"omitempty,max=500"`
}

// BackupResponse represents a backup in API responses
type BackupResponse struct {
	ID           uint         `json:"id"`
	FileName     string       `json:"file_name"`
	FilePath     string       `json:"file_path"`
	FileSize     int64        `json:"file_size"`
	Status       BackupStatus `json:"status"`
	Type         BackupType   `json:"type"`
	CreatedBy    uint         `json:"created_by"`
	Description  string       `json:"description,omitempty"`
	ErrorMessage string       `json:"error_message,omitempty"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ToResponse converts Backup model to BackupResponse
func (b *Backup) ToResponse() *BackupResponse {
	return &BackupResponse{
		ID:           b.ID,
		FileName:     b.FileName,
		FilePath:     b.FilePath,
		FileSize:     b.FileSize,
		Status:       b.Status,
		Type:         b.Type,
		CreatedBy:    b.CreatedBy,
		Description:  b.Description,
		ErrorMessage: b.ErrorMessage,
		StartedAt:    b.StartedAt,
		CompletedAt:  b.CompletedAt,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
}

// BackupListResponse represents a list of backups
type BackupListResponse struct {
	Backups    []*BackupResponse `json:"backups"`
	Total      int64             `json:"total"`
	Page       int               `json:"page"`
	PageSize   int               `json:"page_size"`
	TotalPages int               `json:"total_pages"`
}

// BackupStatsResponse represents backup statistics
type BackupStatsResponse struct {
	TotalBackups     int   `json:"total_backups"`
	SuccessfulBackups int   `json:"successful_backups"`
	FailedBackups    int   `json:"failed_backups"`
	TotalSize        int64 `json:"total_size"`
	LatestBackup     *BackupResponse `json:"latest_backup,omitempty"`
}
