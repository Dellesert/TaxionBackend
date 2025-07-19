package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// ChatType represents the type of chat
type ChatType string

const (
	ChatTypePrivate ChatType = "private"
	ChatTypeGroup   ChatType = "group"
	ChatTypeChannel ChatType = "channel"
)

// Chat represents a chat conversation
type Chat struct {
	models.BaseModel
	Name          string     `gorm:"size:255" json:"name" validate:"omitempty,max=255"`
	Description   string     `gorm:"size:500" json:"description,omitempty" validate:"omitempty,max=500"`
	Type          ChatType   `gorm:"not null;default:'private';size:20" json:"type" validate:"required,oneof=private group channel"`
	CreatorID     uint       `gorm:"not null;index" json:"creator_id" validate:"required"`
	Avatar        string     `gorm:"size:500" json:"avatar,omitempty" validate:"omitempty,url,max=500"`
	IsActive      bool       `gorm:"not null;default:true" json:"is_active"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`

	// Associations
	Members  []ChatMember `gorm:"foreignKey:ChatID" json:"members,omitempty"`
	Messages []Message    `gorm:"foreignKey:ChatID" json:"messages,omitempty"`
}

// TableName returns the table name for Chat model
func (Chat) TableName() string {
	return "chats"
}

// ChatMember represents a member of a chat
type ChatMember struct {
	models.BaseModel
	ChatID   uint           `gorm:"not null;index" json:"chat_id" validate:"required"`
	UserID   uint           `gorm:"not null;index" json:"user_id" validate:"required"`
	Role     ChatMemberRole `gorm:"not null;default:'member';size:20" json:"role" validate:"oneof=owner admin member"`
	JoinedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"joined_at"`
	LeftAt   *time.Time     `json:"left_at,omitempty"`
	IsActive bool           `gorm:"not null;default:true" json:"is_active"`

	// Associations
	Chat *Chat `gorm:"foreignKey:ChatID" json:"chat,omitempty"`
}

// TableName returns the table name for ChatMember model
func (ChatMember) TableName() string {
	return "chat_members"
}

// ChatMemberRole represents the role of a chat member
type ChatMemberRole string

const (
	ChatMemberRoleOwner  ChatMemberRole = "owner"
	ChatMemberRoleAdmin  ChatMemberRole = "admin"
	ChatMemberRoleMember ChatMemberRole = "member"
)

// BeforeCreate hook is called before creating a chat
func (c *Chat) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if c.Type == "" {
		c.Type = ChatTypePrivate
	}
	return nil
}

// AfterCreate hook is called after creating a chat
func (c *Chat) AfterCreate(tx *gorm.DB) error {
	// Add creator as owner
	member := ChatMember{
		ChatID:   c.ID,
		UserID:   c.CreatorID,
		Role:     ChatMemberRoleOwner,
		JoinedAt: time.Now(),
		IsActive: true,
	}
	return tx.Create(&member).Error
}

// Request/Response structures

// CreateChatRequest represents request for creating a chat
type CreateChatRequest struct {
	Name        string   `json:"name" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Description string   `json:"description,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	Type        ChatType `json:"type" binding:"required,oneof=private group channel" validate:"required,oneof=private group channel"`
	Avatar      string   `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	MemberIDs   []uint   `json:"member_ids,omitempty" validate:"omitempty,dive,min=1"`
}

// UpdateChatRequest represents request for updating a chat
type UpdateChatRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	Avatar      *string `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
}

// AddChatMemberRequest represents request for adding a member to chat
type AddChatMemberRequest struct {
	UserID uint           `json:"user_id" binding:"required,min=1" validate:"required,min=1"`
	Role   ChatMemberRole `json:"role,omitempty" binding:"omitempty,oneof=admin member" validate:"omitempty,oneof=admin member"`
}

// UpdateChatMemberRequest represents request for updating a chat member
type UpdateChatMemberRequest struct {
	Role ChatMemberRole `json:"role" binding:"required,oneof=owner admin member" validate:"required,oneof=owner admin member"`
}

// ChatResponse represents chat response (without sensitive data)
type ChatResponse struct {
	ID            uint                 `json:"id"`
	Name          string               `json:"name"`
	Description   string               `json:"description,omitempty"`
	Type          ChatType             `json:"type"`
	CreatorID     uint                 `json:"creator_id"`
	Avatar        string               `json:"avatar,omitempty"`
	IsActive      bool                 `json:"is_active"`
	LastMessageAt *time.Time           `json:"last_message_at,omitempty"`
	MemberCount   int                  `json:"member_count"`
	Members       []ChatMemberResponse `json:"members,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// ChatMemberResponse represents chat member response
type ChatMemberResponse struct {
	ID       uint           `json:"id"`
	ChatID   uint           `json:"chat_id"`
	UserID   uint           `json:"user_id"`
	Role     ChatMemberRole `json:"role"`
	JoinedAt time.Time      `json:"joined_at"`
	LeftAt   *time.Time     `json:"left_at,omitempty"`
	IsActive bool           `json:"is_active"`
}

// ToResponse converts Chat to ChatResponse
func (c *Chat) ToResponse() *ChatResponse {
	response := &ChatResponse{
		ID:            c.ID,
		Name:          c.Name,
		Description:   c.Description,
		Type:          c.Type,
		CreatorID:     c.CreatorID,
		Avatar:        c.Avatar,
		IsActive:      c.IsActive,
		LastMessageAt: c.LastMessageAt,
		MemberCount:   len(c.Members),
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}

	// Include members if loaded
	if len(c.Members) > 0 {
		response.Members = make([]ChatMemberResponse, len(c.Members))
		for i, member := range c.Members {
			response.Members[i] = ChatMemberResponse{
				ID:       member.ID,
				ChatID:   member.ChatID,
				UserID:   member.UserID,
				Role:     member.Role,
				JoinedAt: member.JoinedAt,
				LeftAt:   member.LeftAt,
				IsActive: member.IsActive,
			}
		}
	}

	return response
}

// ToResponse converts ChatMember to ChatMemberResponse
func (cm *ChatMember) ToResponse() *ChatMemberResponse {
	return &ChatMemberResponse{
		ID:       cm.ID,
		ChatID:   cm.ChatID,
		UserID:   cm.UserID,
		Role:     cm.Role,
		JoinedAt: cm.JoinedAt,
		LeftAt:   cm.LeftAt,
		IsActive: cm.IsActive,
	}
}

// ChatListResponse represents paginated chat list response
type ChatListResponse struct {
	Chats  []ChatResponse `json:"chats"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}
