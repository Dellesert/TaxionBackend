package repository

import (
	"errors"
	"fmt"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// ChatRepository defines the interface for chat data operations
type ChatRepository interface {
	Create(chat *models.Chat) error
	GetByID(id uint) (*models.Chat, error)
	GetByUserID(userID uint, limit, offset int) ([]*models.Chat, error)
	Update(chat *models.Chat) error
	Delete(id uint) error
	Count() (int64, error)
	GetWithMembers(id uint) (*models.Chat, error)
	GetUserChats(userID uint, limit, offset int) ([]*models.Chat, int64, error)

	// Chat member operations
	AddMember(member *models.ChatMember) error
	RemoveMember(chatID, userID uint) error
	GetChatMembers(chatID uint) ([]*models.ChatMember, error)
	IsMember(chatID, userID uint) (bool, error)
	GetMemberRole(chatID, userID uint) (models.ChatMemberRole, error)

	// Access control methods
	HasReadAccess(chatID, userID uint) (bool, error)
	HasWriteAccess(chatID, userID uint) (bool, error)
	HasAdminAccess(chatID, userID uint) (bool, error)
	HasOwnerAccess(chatID, userID uint) (bool, error)
}

// chatRepository implements ChatRepository interface
type chatRepository struct {
	db *database.DB
}

// NewChatRepository creates a new chat repository
func NewChatRepository(db *database.DB) ChatRepository {
	return &chatRepository{
		db: db,
	}
}

// Create creates a new chat
func (r *chatRepository) Create(chat *models.Chat) error {
	if err := r.db.Create(chat).Error; err != nil {
		return fmt.Errorf("failed to create chat: %w", err)
	}
	return nil
}

// GetByID retrieves a chat by ID
func (r *chatRepository) GetByID(id uint) (*models.Chat, error) {
	var chat models.Chat
	err := r.db.First(&chat, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}
	return &chat, nil
}

// GetByUserID retrieves chats by user ID with pagination and sorting
func (r *chatRepository) GetByUserID(userID uint, limit, offset int) ([]*models.Chat, error) {
	var chats []*models.Chat
	err := r.db.
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Order("chats.last_message_at DESC NULLS LAST, chats.updated_at DESC").
		Find(&chats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}
	return chats, nil
}

// Update updates an existing chat
func (r *chatRepository) Update(chat *models.Chat) error {
	result := r.db.Save(chat)
	if result.Error != nil {
		return fmt.Errorf("failed to update chat: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat not found")
	}
	return nil
}

// Delete soft deletes a chat by ID
func (r *chatRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Chat{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete chat: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat not found")
	}
	return nil
}

// Count returns the total number of chats
func (r *chatRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Chat{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count chats: %w", err)
	}
	return count, nil
}

// GetWithMembers retrieves a chat by ID with members preloaded and sorted
func (r *chatRepository) GetWithMembers(id uint) (*models.Chat, error) {
	var chat models.Chat
	err := r.db.
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		First(&chat, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, fmt.Errorf("failed to get chat with members: %w", err)
	}
	return &chat, nil
}

// GetUserChats retrieves all chats for a user with pagination and sorting
func (r *chatRepository) GetUserChats(userID uint, limit, offset int) ([]*models.Chat, int64, error) {
	var chats []*models.Chat
	var total int64

	// Get total count
	err := r.db.Model(&models.Chat{}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Count(&total).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user chats: %w", err)
	}

	// Get chats with members, sorted by last activity
	err = r.db.
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Order("chats.last_message_at DESC NULLS LAST, chats.updated_at DESC").
		Find(&chats).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user chats: %w", err)
	}

	return chats, total, nil
}

// AddMember adds a member to a chat
func (r *chatRepository) AddMember(member *models.ChatMember) error {
	// Check if member already exists
	var existing models.ChatMember
	err := r.db.Where("chat_id = ? AND user_id = ?", member.ChatID, member.UserID).
		First(&existing).Error

	if err == nil {
		// Member exists, check if inactive
		if !existing.IsActive {
			existing.IsActive = true
			existing.Role = member.Role
			existing.LeftAt = nil
			return r.db.Save(&existing).Error
		}
		return fmt.Errorf("user is already a member of this chat")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing member: %w", err)
	}

	if err := r.db.Create(member).Error; err != nil {
		return fmt.Errorf("failed to add chat member: %w", err)
	}
	return nil
}

// RemoveMember removes a member from a chat (soft removal)
func (r *chatRepository) RemoveMember(chatID, userID uint) error {
	now := gorm.Expr("CURRENT_TIMESTAMP")
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Updates(map[string]interface{}{
			"is_active": false,
			"left_at":   now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to remove chat member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found or already inactive")
	}
	return nil
}

// GetChatMembers retrieves all active members of a chat, sorted by role and join time
func (r *chatRepository) GetChatMembers(chatID uint) ([]*models.ChatMember, error) {
	var members []*models.ChatMember
	err := r.db.
		Where("chat_id = ? AND is_active = ?", chatID, true).
		Order("CASE role WHEN 'owner' THEN 1 WHEN 'admin' THEN 2 ELSE 3 END, joined_at ASC").
		Find(&members).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get chat members: %w", err)
	}
	return members, nil
}

// IsMember checks if a user is an active member of a chat
func (r *chatRepository) IsMember(chatID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check chat membership: %w", err)
	}
	return count > 0, nil
}

// GetMemberRole retrieves the role of a user in a chat
func (r *chatRepository) GetMemberRole(chatID, userID uint) (models.ChatMemberRole, error) {
	var member models.ChatMember
	err := r.db.Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		First(&member).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("user is not a member of this chat")
		}
		return "", fmt.Errorf("failed to get member role: %w", err)
	}
	return member.Role, nil
}

// Access control methods

// HasReadAccess checks if user can read messages in chat
func (r *chatRepository) HasReadAccess(chatID, userID uint) (bool, error) {
	// Any active member can read
	return r.IsMember(chatID, userID)
}

// HasWriteAccess checks if user can send messages in chat
func (r *chatRepository) HasWriteAccess(chatID, userID uint) (bool, error) {
	// Check if user is active member
	isMember, err := r.IsMember(chatID, userID)
	if err != nil {
		return false, err
	}

	if !isMember {
		return false, nil
	}

	// Check if chat is active
	chat, err := r.GetByID(chatID)
	if err != nil {
		return false, err
	}

	return chat.IsActive, nil
}

// HasAdminAccess checks if user has admin privileges in chat
func (r *chatRepository) HasAdminAccess(chatID, userID uint) (bool, error) {
	role, err := r.GetMemberRole(chatID, userID)
	if err != nil {
		return false, err
	}

	return role == models.ChatMemberRoleOwner || role == models.ChatMemberRoleAdmin, nil
}

// HasOwnerAccess checks if user is the owner of the chat
func (r *chatRepository) HasOwnerAccess(chatID, userID uint) (bool, error) {
	role, err := r.GetMemberRole(chatID, userID)
	if err != nil {
		return false, err
	}

	return role == models.ChatMemberRoleOwner, nil
}

// Additional helper methods for complex access control

// CanModifyChat checks if user can modify chat settings
func (r *chatRepository) CanModifyChat(chatID, userID uint) (bool, error) {
	return r.HasAdminAccess(chatID, userID)
}

// CanDeleteChat checks if user can delete the chat
func (r *chatRepository) CanDeleteChat(chatID, userID uint) (bool, error) {
	return r.HasOwnerAccess(chatID, userID)
}

// CanAddMembers checks if user can add new members
func (r *chatRepository) CanAddMembers(chatID, userID uint) (bool, error) {
	return r.HasAdminAccess(chatID, userID)
}

// CanRemoveMembers checks if user can remove other members
func (r *chatRepository) CanRemoveMembers(chatID, userID, targetUserID uint) (bool, error) {
	// Users can always remove themselves
	if userID == targetUserID {
		return r.IsMember(chatID, userID)
	}

	// Get requester role
	requesterRole, err := r.GetMemberRole(chatID, userID)
	if err != nil {
		return false, err
	}

	// Get target role
	targetRole, err := r.GetMemberRole(chatID, targetUserID)
	if err != nil {
		return false, err
	}

	// Owner can remove anyone except other owners
	if requesterRole == models.ChatMemberRoleOwner {
		return targetRole != models.ChatMemberRoleOwner, nil
	}

	// Admin can remove only members
	if requesterRole == models.ChatMemberRoleAdmin {
		return targetRole == models.ChatMemberRoleMember, nil
	}

	// Members cannot remove others
	return false, nil
}

// CanPromoteMembers checks if user can change member roles
func (r *chatRepository) CanPromoteMembers(chatID, userID uint) (bool, error) {
	return r.HasOwnerAccess(chatID, userID)
}
