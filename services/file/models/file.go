package models

import (
	"path/filepath"
	"time"

	"tachyon-messenger/shared/models"
)

// FileType represents the type/purpose of a file
type FileType string

const (
	FileTypeAvatar     FileType = "avatar"      // User avatar
	FileTypeAttachment FileType = "attachment"  // Message attachment
	FileTypeDocument   FileType = "document"    // Document file
	FileTypeImage      FileType = "image"       // Image file
	FileTypeVideo      FileType = "video"       // Video file
	FileTypeAudio      FileType = "audio"       // Audio file
	FileTypeOther      FileType = "other"       // Other file types
)

// File represents a file stored in the system
type File struct {
	models.BaseModel
	FileName         string    `gorm:"not null;size:255" json:"file_name" validate:"required,min=1,max=255"`
	OriginalName     string    `gorm:"not null;size:255" json:"original_name" validate:"required,min=1,max=255"`
	FilePath         string    `gorm:"not null;size:512" json:"file_path" validate:"required"`
	FileSize         int64     `gorm:"not null" json:"file_size" validate:"required,min=1"`
	ThumbnailPath    string    `gorm:"size:512" json:"thumbnail_path,omitempty"`
	ThumbnailSize    int64     `json:"thumbnail_size,omitempty"`
	MimeType         string    `gorm:"not null;size:100" json:"mime_type" validate:"required"`
	FileType         FileType  `gorm:"not null;size:20;index" json:"file_type" validate:"required,oneof=avatar attachment document image video audio other"`
	UploadedBy       uint      `gorm:"not null;index" json:"uploaded_by" validate:"required,min=1"`
	EntityType       *string   `gorm:"size:50;index" json:"entity_type,omitempty"` // e.g., "user", "message", "task"
	EntityID         *uint     `gorm:"index" json:"entity_id,omitempty"`           // ID of the related entity
	IsPublic         bool      `gorm:"not null;default:false" json:"is_public"`
	URL              string    `gorm:"-" json:"url,omitempty"`          // Computed field for public URL
	ThumbnailURL     string    `gorm:"-" json:"thumbnail_url,omitempty"` // Computed field for thumbnail URL
}

// TableName returns the table name for File model
func (File) TableName() string {
	return "files"
}

// Request/Response Models

// UploadFileRequest represents request for uploading a file
type UploadFileRequest struct {
	FileType   FileType `form:"file_type" binding:"required,oneof=avatar attachment document image video audio other" validate:"required"`
	EntityType *string  `form:"entity_type,omitempty" binding:"omitempty,max=50" validate:"omitempty,max=50"`
	EntityID   *uint    `form:"entity_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	IsPublic   bool     `form:"is_public"`
}

// FileResponse represents a file in API responses
type FileResponse struct {
	ID           uint      `json:"id"`
	FileName     string    `json:"file_name"`
	OriginalName string    `json:"original_name"`
	FilePath     string    `json:"file_path"`
	FileSize     int64     `json:"file_size"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	MimeType     string    `json:"mime_type"`
	FileType     FileType  `json:"file_type"`
	UploadedBy   uint      `json:"uploaded_by"`
	EntityType   *string   `json:"entity_type,omitempty"`
	EntityID     *uint     `json:"entity_id,omitempty"`
	IsPublic     bool      `json:"is_public"`
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ToResponse converts File model to FileResponse
func (f *File) ToResponse(baseURL string) *FileResponse {
	url := baseURL + "/api/v1/files/download/" + f.FileName
	if f.IsPublic {
		url = baseURL + "/api/v1/files/public/" + f.FileName
	}

	// Generate thumbnail URL if thumbnail exists
	thumbnailURL := ""
	if f.ThumbnailPath != "" {
		thumbnailFileName := filepath.Base(f.ThumbnailPath)
		if f.IsPublic {
			thumbnailURL = baseURL + "/api/v1/files/public/" + thumbnailFileName
		} else {
			thumbnailURL = baseURL + "/api/v1/files/download/" + thumbnailFileName
		}
	}

	return &FileResponse{
		ID:           f.ID,
		FileName:     f.FileName,
		OriginalName: f.OriginalName,
		FilePath:     f.FilePath,
		FileSize:     f.FileSize,
		ThumbnailURL: thumbnailURL,
		MimeType:     f.MimeType,
		FileType:     f.FileType,
		UploadedBy:   f.UploadedBy,
		EntityType:   f.EntityType,
		EntityID:     f.EntityID,
		IsPublic:     f.IsPublic,
		URL:          url,
		CreatedAt:    f.CreatedAt,
		UpdatedAt:    f.UpdatedAt,
	}
}

// FileFilterRequest represents filtering parameters for files
type FileFilterRequest struct {
	FileType   *FileType `form:"file_type" binding:"omitempty,oneof=avatar attachment document image video audio other"`
	EntityType *string   `form:"entity_type" binding:"omitempty,max=50"`
	EntityID   *uint     `form:"entity_id" binding:"omitempty,min=1"`
	UploadedBy *uint     `form:"uploaded_by" binding:"omitempty,min=1"`
	Limit      int       `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset     int       `form:"offset" binding:"omitempty,min=0"`
}
