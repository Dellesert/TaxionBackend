package models

import (
	"fmt"
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
	ChatTypeSaved   ChatType = "saved" // Personal "Saved Messages" chat for each user
)

// Chat represents a chat conversation
type Chat struct {
	models.BaseModel
	Name          string     `gorm:"size:255" json:"name" validate:"omitempty,max=255"`
	Description   string     `gorm:"size:500" json:"description,omitempty" validate:"omitempty,max=500"`
	Type          ChatType   `gorm:"not null;default:'private';size:20" json:"type" validate:"required,oneof=private group channel saved"`
	CreatorID     uint       `gorm:"not null;index" json:"creator_id" validate:"required"`
	Avatar        string     `gorm:"size:500" json:"avatar,omitempty" validate:"omitempty,url,max=500"`
	IsActive      bool       `gorm:"not null;default:true" json:"is_active"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	TaskID        *uint      `gorm:"index" json:"task_id,omitempty"` // Link to task for task-related chats

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
	ChatID     uint           `gorm:"not null;index" json:"chat_id" validate:"required"`
	UserID     uint           `gorm:"not null;index" json:"user_id" validate:"required"`
	Role       ChatMemberRole `gorm:"not null;default:'member';size:20" json:"role" validate:"oneof=owner admin member"`
	JoinedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"joined_at"`
	LeftAt     *time.Time     `json:"left_at,omitempty"`
	IsActive   bool           `gorm:"not null;default:true" json:"is_active"`
	IsFavorite bool           `gorm:"not null;default:false" json:"is_favorite"`
	IsPinned   bool           `gorm:"not null;default:false" json:"is_pinned"`
	IsHidden   bool           `gorm:"not null;default:false" json:"is_hidden"`   // Allows user to hide chat without leaving
	MutedUntil *time.Time     `gorm:"type:timestamptz" json:"muted_until,omitempty"` // NULL = not muted, timestamp = muted until

	// Associations
	Chat *Chat        `gorm:"foreignKey:ChatID" json:"chat,omitempty"`
	User *models.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
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
	IsFavorite    bool                 `json:"is_favorite"`
	IsPinned      bool                 `json:"is_pinned"`
	IsMuted       bool                 `json:"is_muted"`
	MutedUntil    *time.Time           `json:"muted_until,omitempty"`
	LastMessageAt *time.Time           `json:"last_message_at,omitempty"`
	LastMessage   *MessageResponse     `json:"last_message,omitempty"`
	UnreadCount   int64                `json:"unread_count"`
	MemberCount   int                  `json:"member_count"`
	Members       []ChatMemberResponse `json:"members,omitempty"`
	TaskID        *uint                `json:"task_id,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

// MemberUserResponse represents user info embedded in chat member response
type MemberUserResponse struct {
	ID              uint               `json:"id"`
	Name            string             `json:"name"`
	Email           string             `json:"email"`
	Avatar          string             `json:"avatar,omitempty"`
	AvatarThumbnail string             `json:"avatar_thumbnail,omitempty"`
	Status          models.UserStatus  `json:"status"`
	Department      string             `json:"department,omitempty"`
	Position        string             `json:"position,omitempty"`
	LastActiveAt    *time.Time         `json:"last_active_at,omitempty"`
}

// ChatMemberResponse represents chat member response
type ChatMemberResponse struct {
	ID         uint                `json:"id"`
	ChatID     uint                `json:"chat_id"`
	UserID     uint                `json:"user_id"`
	User       *MemberUserResponse `json:"user,omitempty"`
	Role       ChatMemberRole      `json:"role"`
	JoinedAt   time.Time           `json:"joined_at"`
	LeftAt     *time.Time          `json:"left_at,omitempty"`
	IsActive   bool                `json:"is_active"`
	IsFavorite bool                `json:"is_favorite"`
	IsPinned   bool                `json:"is_pinned"`
	MutedUntil *time.Time          `json:"muted_until,omitempty"`
}

// ToResponse converts Chat to ChatResponse
// If baseURL is provided, it will be used to construct file URLs for message attachments
// currentUserID can be optionally provided as second parameter to personalize private chat names
func (c *Chat) ToResponse(params ...interface{}) *ChatResponse {
	// Parse optional parameters
	var baseURL string
	var currentUserID uint

	for i, param := range params {
		switch v := param.(type) {
		case string:
			if i == 0 {
				baseURL = v
			}
		case uint:
			currentUserID = v
		}
	}

	response := &ChatResponse{
		ID:            c.ID,
		Name:          c.Name,
		Description:   c.Description,
		Type:          c.Type,
		CreatorID:     c.CreatorID,
		Avatar:        c.Avatar,
		IsActive:      c.IsActive,
		LastMessageAt: c.LastMessageAt,
		TaskID:        c.TaskID,
		MemberCount:   len(c.Members),
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}

	// For private chats, set name to the other user's name
	if c.Type == ChatTypePrivate && currentUserID > 0 && len(c.Members) > 0 {
		for _, member := range c.Members {
			if member.UserID != currentUserID && member.User != nil {
				response.Name = member.User.Name
				response.Avatar = member.User.Avatar
				break
			}
		}
	}

	// For saved chats, set default name
	if c.Type == ChatTypeSaved {
		response.Name = "Избранное"
		response.IsPinned = true // Saved chat is always pinned
	}

	// Include members if loaded
	if len(c.Members) > 0 {
		response.Members = make([]ChatMemberResponse, len(c.Members))
		for i, member := range c.Members {
			memberResp := ChatMemberResponse{
				ID:       member.ID,
				ChatID:   member.ChatID,
				UserID:   member.UserID,
				Role:     member.Role,
				JoinedAt: member.JoinedAt,
				LeftAt:   member.LeftAt,
				IsActive: member.IsActive,
			}
			// Include user info if loaded (via Preload)
			if member.User != nil {
				memberResp.User = &MemberUserResponse{
					ID:              member.User.ID,
					Name:            member.User.Name,
					Email:           member.User.Email,
					Avatar:          member.User.Avatar,
					AvatarThumbnail: member.User.AvatarThumbnail,
					Status:          member.User.Status,
					Department:      member.User.Department,
					Position:        member.User.Position,
					LastActiveAt:    member.User.LastActiveAt,
				}
			}
			response.Members[i] = memberResp
		}
	}

	// Include last message if loaded
	if len(c.Messages) > 0 {
		// Pass baseURL to message response
		if baseURL != "" {
			response.LastMessage = c.Messages[0].ToResponse(baseURL)
		} else {
			response.LastMessage = c.Messages[0].ToResponse()
		}
	}

	return response
}

// ToResponse converts ChatMember to ChatMemberResponse
func (cm *ChatMember) ToResponse() *ChatMemberResponse {
	resp := &ChatMemberResponse{
		ID:         cm.ID,
		ChatID:     cm.ChatID,
		UserID:     cm.UserID,
		Role:       cm.Role,
		JoinedAt:   cm.JoinedAt,
		LeftAt:     cm.LeftAt,
		IsActive:   cm.IsActive,
		IsFavorite: cm.IsFavorite,
		IsPinned:   cm.IsPinned,
		MutedUntil: cm.MutedUntil,
	}
	// Include user info if loaded
	if cm.User != nil {
		resp.User = &MemberUserResponse{
			ID:              cm.User.ID,
			Name:            cm.User.Name,
			Email:           cm.User.Email,
			Avatar:          cm.User.Avatar,
			AvatarThumbnail: cm.User.AvatarThumbnail,
			Status:          cm.User.Status,
			Department:      cm.User.Department,
			Position:        cm.User.Position,
			LastActiveAt:    cm.User.LastActiveAt,
		}
	}
	return resp
}

// ChatListResponse represents paginated chat list response
type ChatListResponse struct {
	Chats  []ChatResponse `json:"chats"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// ChatSyncListResponse represents a sync-aware list response for chats
type ChatSyncListResponse struct {
	Chats      []ChatResponse `json:"data"`                  // List of chats (renamed to "data" for consistency)
	Total      int64          `json:"total"`                 // Total count matching filters
	DeletedIDs []uint         `json:"deleted_ids,omitempty"` // IDs of deleted chats since updated_since
	ServerTime time.Time      `json:"server_time"`           // Server timestamp for next sync request
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}

type CreateGroupChatRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description string `json:"description,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
	MemberIDs   []uint `json:"member_ids" binding:"required,min=1" validate:"required,min=1,dive,min=1"`
}

// UserMutePreference stores global mute preferences for a user
type UserMutePreference struct {
	models.BaseModel
	UserID               uint       `gorm:"not null" json:"user_id"`
	MuteAllChannelsUntil *time.Time `gorm:"type:timestamptz" json:"mute_all_channels_until,omitempty"`
	MuteAllGroupsUntil   *time.Time `gorm:"type:timestamptz" json:"mute_all_groups_until,omitempty"`
}

// TableName returns the table name for UserMutePreference model
func (UserMutePreference) TableName() string {
	return "user_mute_preferences"
}

// MuteChatRequest represents the request body for muting a chat
type MuteChatRequest struct {
	Duration string `json:"duration" binding:"required,oneof=1h 12h forever"`
}

// UpdateGlobalMuteRequest represents the request for updating global mute settings
type UpdateGlobalMuteRequest struct {
	MuteAllChannels *string `json:"mute_all_channels,omitempty" binding:"omitempty,oneof=1h 12h forever off"`
	MuteAllGroups   *string `json:"mute_all_groups,omitempty" binding:"omitempty,oneof=1h 12h forever off"`
}

// GlobalMuteResponse represents the response for global mute settings
type GlobalMuteResponse struct {
	MuteAllChannelsUntil *time.Time `json:"mute_all_channels_until,omitempty"`
	MuteAllGroupsUntil   *time.Time `json:"mute_all_groups_until,omitempty"`
}

// ComputeMutedUntil calculates the muted_until timestamp from a duration string
func ComputeMutedUntil(duration string) (*time.Time, error) {
	var mutedUntil time.Time
	switch duration {
	case "1h":
		mutedUntil = time.Now().Add(1 * time.Hour)
	case "12h":
		mutedUntil = time.Now().Add(12 * time.Hour)
	case "forever":
		mutedUntil = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	case "off":
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid duration: %s", duration)
	}
	return &mutedUntil, nil
}

// IsMutedUntil checks if a muted_until timestamp means the user is currently muted
func IsMutedUntil(mutedUntil *time.Time) bool {
	return mutedUntil != nil && mutedUntil.After(time.Now())
}
