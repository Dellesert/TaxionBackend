package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// MessageType represents the type of message
type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeImage    MessageType = "image"
	MessageTypeFile     MessageType = "file"
	MessageTypeVideo    MessageType = "video"
	MessageTypeAudio    MessageType = "audio"
	MessageTypeLocation MessageType = "location"
	MessageTypeSystem   MessageType = "system"
)

// MessageStatus represents the status of message delivery
type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

// Message represents a message in a chat
type Message struct {
	models.BaseModel
	ChatID    uint          `gorm:"not null;index" json:"chat_id" validate:"required"`
	SenderID  uint          `gorm:"not null;index" json:"sender_id" validate:"required"`
	Content   string        `gorm:"type:text" json:"content" validate:"required,max=10000"`
	Type      MessageType   `gorm:"not null;default:'text';size:20" json:"type" validate:"oneof=text image file video audio location system"`
	Status    MessageStatus `gorm:"not null;default:'sent';size:20" json:"status" validate:"oneof=sent delivered read failed"`
	ReplyToID *uint         `gorm:"index" json:"reply_to_id,omitempty"`
	EditedAt  *time.Time    `json:"edited_at,omitempty"`
	IsEdited  bool          `gorm:"not null;default:false" json:"is_edited"`
	IsDeleted bool          `gorm:"not null;default:false" json:"is_deleted"`

	// File-related fields for non-text messages
	FileName     string `gorm:"size:255" json:"file_name,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	FileURL      string `gorm:"size:500" json:"file_url,omitempty"`
	ThumbnailURL string `gorm:"size:500" json:"thumbnail_url,omitempty"`
	MimeType     string `gorm:"size:100" json:"mime_type,omitempty"`

	// Location-related fields
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`

	// System message metadata
	SystemData string `gorm:"type:text" json:"system_data,omitempty"`

	// Associations
	Chat    *Chat    `gorm:"foreignKey:ChatID" json:"chat,omitempty"`
	ReplyTo *Message `gorm:"foreignKey:ReplyToID" json:"reply_to,omitempty"`

	// Message reactions and read receipts
	Reactions    []MessageReaction    `gorm:"foreignKey:MessageID" json:"reactions,omitempty"`
	ReadReceipts []MessageReadReceipt `gorm:"foreignKey:MessageID" json:"read_receipts,omitempty"`
}

// TableName returns the table name for Message model
func (Message) TableName() string {
	return "messages"
}

// MessageReaction represents a reaction to a message
type MessageReaction struct {
	models.BaseModel
	MessageID uint   `gorm:"not null;index" json:"message_id" validate:"required"`
	UserID    uint   `gorm:"not null;index" json:"user_id" validate:"required"`
	Emoji     string `gorm:"not null;size:10" json:"emoji" validate:"required,max=10"`

	// Associations
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName returns the table name for MessageReaction model
func (MessageReaction) TableName() string {
	return "message_reactions"
}

// MessageReadReceipt represents when a message was read by a user
type MessageReadReceipt struct {
	models.BaseModel
	MessageID uint      `gorm:"not null;index" json:"message_id" validate:"required"`
	UserID    uint      `gorm:"not null;index" json:"user_id" validate:"required"`
	ReadAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"read_at"`

	// Associations
	Message *Message `gorm:"foreignKey:MessageID" json:"message,omitempty"`
}

// TableName returns the table name for MessageReadReceipt model
func (MessageReadReceipt) TableName() string {
	return "message_read_receipts"
}

// BeforeCreate hook is called before creating a message
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if m.Type == "" {
		m.Type = MessageTypeText
	}
	if m.Status == "" {
		m.Status = MessageStatusSent
	}
	return nil
}

// AfterCreate hook is called after creating a message
func (m *Message) AfterCreate(tx *gorm.DB) error {
	// Update chat's last_message_at
	now := time.Now()
	return tx.Model(&Chat{}).
		Where("id = ?", m.ChatID).
		Update("last_message_at", now).Error
}

// Request/Response structures

// SendMessageRequest represents request for sending a message
type SendMessageRequest struct {
	ChatID    uint        `json:"chat_id" binding:"required,min=1" validate:"required,min=1"`
	Content   string      `json:"content" binding:"required,max=10000" validate:"required,max=10000"`
	Type      MessageType `json:"type,omitempty" binding:"omitempty,oneof=text image file video audio location system" validate:"omitempty,oneof=text image file video audio location system"`
	ReplyToID *uint       `json:"reply_to_id,omitempty" validate:"omitempty,min=1"`

	// File-related fields
	FileName     string `json:"file_name,omitempty" validate:"omitempty,max=255"`
	FileSize     int64  `json:"file_size,omitempty" validate:"omitempty,min=0"`
	FileURL      string `json:"file_url,omitempty" validate:"omitempty,url,max=500"`
	ThumbnailURL string `json:"thumbnail_url,omitempty" validate:"omitempty,url,max=500"`
	MimeType     string `json:"mime_type,omitempty" validate:"omitempty,max=100"`

	// Location-related fields
	Latitude  *float64 `json:"latitude,omitempty" validate:"omitempty,min=-90,max=90"`
	Longitude *float64 `json:"longitude,omitempty" validate:"omitempty,min=-180,max=180"`
}

// UpdateMessageRequest represents request for updating a message
type UpdateMessageRequest struct {
	Content string `json:"content" binding:"required,max=10000" validate:"required,max=10000"`
}

// AddReactionRequest represents request for adding a reaction
type AddReactionRequest struct {
	Emoji string `json:"emoji" binding:"required,max=10" validate:"required,max=10"`
}

// GetMessagesRequest represents request parameters for getting messages
type GetMessagesRequest struct {
	ChatID uint `form:"chat_id" validate:"omitempty,min=1"`
	Limit  int  `form:"limit" validate:"omitempty,min=1,max=100"`
	Offset int  `form:"offset" validate:"omitempty,min=0"`
	Before uint `form:"before" validate:"omitempty,min=1"` // Get messages before this message ID
	After  uint `form:"after" validate:"omitempty,min=1"`  // Get messages after this message ID
}

// MessageResponse represents message response
type MessageResponse struct {
	ID           uint                         `json:"id"`
	ChatID       uint                         `json:"chat_id"`
	SenderID     uint                         `json:"sender_id"`
	Content      string                       `json:"content"`
	Type         MessageType                  `json:"type"`
	Status       MessageStatus                `json:"status"`
	ReplyToID    *uint                        `json:"reply_to_id,omitempty"`
	EditedAt     *time.Time                   `json:"edited_at,omitempty"`
	IsEdited     bool                         `json:"is_edited"`
	IsDeleted    bool                         `json:"is_deleted"`
	FileName     string                       `json:"file_name,omitempty"`
	FileSize     int64                        `json:"file_size,omitempty"`
	FileURL      string                       `json:"file_url,omitempty"`
	ThumbnailURL string                       `json:"thumbnail_url,omitempty"`
	MimeType     string                       `json:"mime_type,omitempty"`
	Latitude     *float64                     `json:"latitude,omitempty"`
	Longitude    *float64                     `json:"longitude,omitempty"`
	SystemData   string                       `json:"system_data,omitempty"`
	Reactions    []MessageReactionResponse    `json:"reactions,omitempty"`
	ReadReceipts []MessageReadReceiptResponse `json:"read_receipts,omitempty"`
	ReplyTo      *MessageResponse             `json:"reply_to,omitempty"`
	CreatedAt    time.Time                    `json:"created_at"`
	UpdatedAt    time.Time                    `json:"updated_at"`
}

// MessageReactionResponse represents message reaction response
type MessageReactionResponse struct {
	ID        uint      `json:"id"`
	MessageID uint      `json:"message_id"`
	UserID    uint      `json:"user_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

// MessageReadReceiptResponse represents message read receipt response
type MessageReadReceiptResponse struct {
	ID        uint      `json:"id"`
	MessageID uint      `json:"message_id"`
	UserID    uint      `json:"user_id"`
	ReadAt    time.Time `json:"read_at"`
}

// ToResponse converts Message to MessageResponse
func (m *Message) ToResponse() *MessageResponse {
	response := &MessageResponse{
		ID:           m.ID,
		ChatID:       m.ChatID,
		SenderID:     m.SenderID,
		Content:      m.Content,
		Type:         m.Type,
		Status:       m.Status,
		ReplyToID:    m.ReplyToID,
		EditedAt:     m.EditedAt,
		IsEdited:     m.IsEdited,
		IsDeleted:    m.IsDeleted,
		FileName:     m.FileName,
		FileSize:     m.FileSize,
		FileURL:      m.FileURL,
		ThumbnailURL: m.ThumbnailURL,
		MimeType:     m.MimeType,
		Latitude:     m.Latitude,
		Longitude:    m.Longitude,
		SystemData:   m.SystemData,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}

	// Include reply-to message if loaded
	if m.ReplyTo != nil {
		response.ReplyTo = m.ReplyTo.ToResponse()
	}

	// Include reactions if loaded
	if len(m.Reactions) > 0 {
		response.Reactions = make([]MessageReactionResponse, len(m.Reactions))
		for i, reaction := range m.Reactions {
			response.Reactions[i] = MessageReactionResponse{
				ID:        reaction.ID,
				MessageID: reaction.MessageID,
				UserID:    reaction.UserID,
				Emoji:     reaction.Emoji,
				CreatedAt: reaction.CreatedAt,
			}
		}
	}

	// Include read receipts if loaded
	if len(m.ReadReceipts) > 0 {
		response.ReadReceipts = make([]MessageReadReceiptResponse, len(m.ReadReceipts))
		for i, receipt := range m.ReadReceipts {
			response.ReadReceipts[i] = MessageReadReceiptResponse{
				ID:        receipt.ID,
				MessageID: receipt.MessageID,
				UserID:    receipt.UserID,
				ReadAt:    receipt.ReadAt,
			}
		}
	}

	return response
}

// MessageListResponse represents paginated message list response
type MessageListResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
	HasMore  bool              `json:"has_more"`
}

// WebSocket message types for real-time communication
type WSMessageType string

const (
	WSMessageTypeNewMessage    WSMessageType = "new_message"
	WSMessageTypeMessageEdit   WSMessageType = "message_edit"
	WSMessageTypeMessageDelete WSMessageType = "message_delete"
	WSMessageTypeTyping        WSMessageType = "typing"
	WSMessageTypeRead          WSMessageType = "message_read"
	WSMessageTypeReaction      WSMessageType = "reaction"
	WSMessageTypeUserJoin      WSMessageType = "user_join"
	WSMessageTypeUserLeave     WSMessageType = "user_leave"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type   WSMessageType `json:"type"`
	ChatID uint          `json:"chat_id"`
	UserID uint          `json:"user_id"`
	Data   interface{}   `json:"data"`
	SentAt time.Time     `json:"sent_at"`
}
