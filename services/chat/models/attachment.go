package models

import (
	"strings"

	"tachyon-messenger/shared/models"
)

// MessageAttachment represents a file attachment in a message
type MessageAttachment struct {
	models.BaseModel
	MessageID          uint    `gorm:"not null;index" json:"message_id" validate:"required"`
	FileID             uint    `gorm:"not null;index" json:"file_id" validate:"required"` // Reference to file-service
	FileName           string  `gorm:"not null;size:255" json:"file_name" validate:"required,max=255"`
	FileSize           int64   `gorm:"not null" json:"file_size" validate:"required,min=1"`
	FileURL            string  `gorm:"not null;size:500" json:"file_url" validate:"required,url,max=500"`
	ThumbnailURL       string  `gorm:"size:500" json:"thumbnail_url,omitempty" validate:"omitempty,url,max=500"`       // Legacy = medium
	ThumbnailSmallURL  string  `gorm:"size:500" json:"thumbnail_small_url,omitempty" validate:"omitempty,url,max=500"`  // ~100x100
	ThumbnailMediumURL string  `gorm:"size:500" json:"thumbnail_medium_url,omitempty" validate:"omitempty,url,max=500"` // ~400x300
	ThumbnailLargeURL  string  `gorm:"size:500" json:"thumbnail_large_url,omitempty" validate:"omitempty,url,max=500"`  // ~800x600
	MimeType           string  `gorm:"not null;size:100" json:"mime_type" validate:"required,max=100"`
	FileType           string  `gorm:"not null;size:20" json:"file_type" validate:"required,oneof=image video audio document other"`
	Duration           float64 `json:"duration,omitempty"` // Video/audio duration in seconds
	Width              int     `json:"width,omitempty"`    // Media width in pixels
	Height             int     `json:"height,omitempty"`   // Media height in pixels

	// Associations
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName returns the table name for MessageAttachment model
func (MessageAttachment) TableName() string {
	return "message_attachments"
}

// MessageAttachmentResponse represents message attachment response
type MessageAttachmentResponse struct {
	ID                 uint    `json:"id"`
	MessageID          uint    `json:"message_id"`
	FileID             uint    `json:"file_id"`
	FileName           string  `json:"file_name"`
	FileSize           int64   `json:"file_size"`
	FileURL            string  `json:"file_url"`
	ThumbnailURL       string  `json:"thumbnail_url,omitempty"`        // Legacy = medium
	ThumbnailSmallURL  string  `json:"thumbnail_small_url,omitempty"`  // ~100x100
	ThumbnailMediumURL string  `json:"thumbnail_medium_url,omitempty"` // ~400x300
	ThumbnailLargeURL  string  `json:"thumbnail_large_url,omitempty"`  // ~800x600
	MimeType           string  `json:"mime_type"`
	FileType           string  `json:"file_type"`
	Duration           float64 `json:"duration,omitempty"`
	Width              int     `json:"width,omitempty"`
	Height             int     `json:"height,omitempty"`
}

// reconstructURL reconstructs a URL using baseURL if provided, otherwise returns original
func reconstructURL(originalURL string, baseURL string) string {
	if baseURL == "" || originalURL == "" {
		return originalURL
	}
	parts := strings.Split(originalURL, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		return baseURL + "/api/v1/files/public/" + filename
	}
	return originalURL
}

// ToResponse converts MessageAttachment to MessageAttachmentResponse
// If baseURL is provided, it will be used to construct the file URL
// Otherwise, the stored FileURL will be used
func (ma *MessageAttachment) ToResponse(baseURL ...string) *MessageAttachmentResponse {
	base := ""
	if len(baseURL) > 0 {
		base = baseURL[0]
	}

	return &MessageAttachmentResponse{
		ID:                 ma.ID,
		MessageID:          ma.MessageID,
		FileID:             ma.FileID,
		FileName:           ma.FileName,
		FileSize:           ma.FileSize,
		FileURL:            reconstructURL(ma.FileURL, base),
		ThumbnailURL:       reconstructURL(ma.ThumbnailURL, base),
		ThumbnailSmallURL:  reconstructURL(ma.ThumbnailSmallURL, base),
		ThumbnailMediumURL: reconstructURL(ma.ThumbnailMediumURL, base),
		ThumbnailLargeURL:  reconstructURL(ma.ThumbnailLargeURL, base),
		MimeType:           ma.MimeType,
		FileType:           ma.FileType,
		Duration:           ma.Duration,
		Width:              ma.Width,
		Height:             ma.Height,
	}
}
