package models

import (
	"tachyon-messenger/shared/models"
)

// MessageAttachment represents a file attachment in a message
type MessageAttachment struct {
	models.BaseModel
	MessageID    uint   `gorm:"not null;index" json:"message_id" validate:"required"`
	FileID       uint   `gorm:"not null;index" json:"file_id" validate:"required"` // Reference to file-service
	FileName     string `gorm:"not null;size:255" json:"file_name" validate:"required,max=255"`
	FileSize     int64  `gorm:"not null" json:"file_size" validate:"required,min=1"`
	FileURL      string `gorm:"not null;size:500" json:"file_url" validate:"required,url,max=500"`
	ThumbnailURL string `gorm:"size:500" json:"thumbnail_url,omitempty" validate:"omitempty,url,max=500"`
	MimeType     string `gorm:"not null;size:100" json:"mime_type" validate:"required,max=100"`
	FileType     string `gorm:"not null;size:20" json:"file_type" validate:"required,oneof=image video audio document other"`

	// Associations
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName returns the table name for MessageAttachment model
func (MessageAttachment) TableName() string {
	return "message_attachments"
}

// MessageAttachmentResponse represents message attachment response
type MessageAttachmentResponse struct {
	ID           uint   `json:"id"`
	MessageID    uint   `json:"message_id"`
	FileID       uint   `json:"file_id"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	FileURL      string `json:"file_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	MimeType     string `json:"mime_type"`
	FileType     string `json:"file_type"`
}

// ToResponse converts MessageAttachment to MessageAttachmentResponse
// If baseURL is provided, it will be used to construct the file URL
// Otherwise, the stored FileURL will be used
func (ma *MessageAttachment) ToResponse(baseURL ...string) *MessageAttachmentResponse {
	fileURL := ma.FileURL

	// If baseURL is provided, construct the URL dynamically
	// This ensures the URL is always current (important after domain changes)
	if len(baseURL) > 0 && baseURL[0] != "" {
		// Extract filename from the stored URL or use FileID
		// Format: {baseURL}/api/v1/files/public/{filename}
		fileURL = baseURL[0] + "/api/v1/files/public/" + ma.FileName
	}

	return &MessageAttachmentResponse{
		ID:           ma.ID,
		MessageID:    ma.MessageID,
		FileID:       ma.FileID,
		FileName:     ma.FileName,
		FileSize:     ma.FileSize,
		FileURL:      fileURL,
		ThumbnailURL: ma.ThumbnailURL,
		MimeType:     ma.MimeType,
		FileType:     ma.FileType,
	}
}
