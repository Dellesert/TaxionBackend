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
}

// MessageRepository defines the interface for message data operations
type MessageRepository interface {
	Create(message *models.Message) error
	GetByID(id uint) (*models.Message, error)
	GetByChatID(chatID uint, limit, offset int) ([]*models.Message, error)
	Update(message *models.Message) error
	Delete(id uint) error
	Count() (int64, error)
	CountByChatID(chatID uint) (int64, error)
	GetWithReactions(id uint) (*models.Message, error)
	GetMessagesAfter(chatID uint, after uint, limit int) ([]*models.Message, error)
	GetMessagesBefore(chatID uint, before uint, limit int) ([]*models.Message, error)

	// Message reaction operations
	AddReaction(reaction *models.MessageReaction) error
	RemoveReaction(messageID, userID uint, emoji string) error
	GetReactions(messageID uint) ([]*models.MessageReaction, error)

	// Read receipt operations
	MarkAsRead(receipt *models.MessageReadReceipt) error
	GetReadReceipts(messageID uint) ([]*models.MessageReadReceipt, error)
}

// chatRepository implements ChatRepository interface
type chatRepository struct {
	db *database.DB
}

// messageRepository implements MessageRepository interface
type messageRepository struct {
	db *database.DB
}

// NewChatRepository creates a new chat repository
func NewChatRepository(db *database.DB) ChatRepository {
	return &chatRepository{
		db: db,
	}
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *database.DB) MessageRepository {
	return &messageRepository{
		db: db,
	}
}

// Chat Repository Methods

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

// GetByUserID retrieves chats by user ID
func (r *chatRepository) GetByUserID(userID uint, limit, offset int) ([]*models.Chat, error) {
	var chats []*models.Chat
	err := r.db.
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Order("chats.last_message_at DESC").
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

// GetWithMembers retrieves a chat by ID with members preloaded
func (r *chatRepository) GetWithMembers(id uint) (*models.Chat, error) {
	var chat models.Chat
	err := r.db.Preload("Members").First(&chat, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, fmt.Errorf("failed to get chat with members: %w", err)
	}
	return &chat, nil
}

// GetUserChats retrieves all chats for a user with pagination
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

	// Get chats with members
	err = r.db.
		Preload("Members").
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Order("chats.last_message_at DESC").
		Find(&chats).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user chats: %w", err)
	}

	return chats, total, nil
}

// AddMember adds a member to a chat
func (r *chatRepository) AddMember(member *models.ChatMember) error {
	if err := r.db.Create(member).Error; err != nil {
		return fmt.Errorf("failed to add chat member: %w", err)
	}
	return nil
}

// RemoveMember removes a member from a chat
func (r *chatRepository) RemoveMember(chatID, userID uint) error {
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to remove chat member: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found")
	}
	return nil
}

// GetChatMembers retrieves all members of a chat
func (r *chatRepository) GetChatMembers(chatID uint) ([]*models.ChatMember, error) {
	var members []*models.ChatMember
	err := r.db.Where("chat_id = ? AND is_active = ?", chatID, true).Find(&members).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get chat members: %w", err)
	}
	return members, nil
}

// IsMember checks if a user is a member of a chat
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

// Message Repository Methods

// Create creates a new message
func (r *messageRepository) Create(message *models.Message) error {
	if err := r.db.Create(message).Error; err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

// GetByID retrieves a message by ID
func (r *messageRepository) GetByID(id uint) (*models.Message, error) {
	var message models.Message
	err := r.db.Preload("ReplyTo").First(&message, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	return &message, nil
}

// GetByChatID retrieves messages by chat ID with pagination
func (r *messageRepository) GetByChatID(chatID uint, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions").
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

// Update updates an existing message
func (r *messageRepository) Update(message *models.Message) error {
	result := r.db.Save(message)
	if result.Error != nil {
		return fmt.Errorf("failed to update message: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("message not found")
	}
	return nil
}

// Delete soft deletes a message by ID
func (r *messageRepository) Delete(id uint) error {
	result := r.db.Model(&models.Message{}).Where("id = ?", id).Update("is_deleted", true)
	if result.Error != nil {
		return fmt.Errorf("failed to delete message: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("message not found")
	}
	return nil
}

// Count returns the total number of messages
func (r *messageRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).Where("is_deleted = ?", false).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}
	return count, nil
}

// CountByChatID returns the number of messages in a chat
func (r *messageRepository) CountByChatID(chatID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count chat messages: %w", err)
	}
	return count, nil
}

// GetWithReactions retrieves a message with reactions
func (r *messageRepository) GetWithReactions(id uint) (*models.Message, error) {
	var message models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions").
		Preload("ReadReceipts").
		First(&message, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message with reactions: %w", err)
	}
	return &message, nil
}

// GetMessagesAfter retrieves messages after a specific message ID
func (r *messageRepository) GetMessagesAfter(chatID uint, after uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions").
		Where("chat_id = ? AND id > ? AND is_deleted = ?", chatID, after, false).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages after: %w", err)
	}
	return messages, nil
}

// GetMessagesBefore retrieves messages before a specific message ID
func (r *messageRepository) GetMessagesBefore(chatID uint, before uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions").
		Where("chat_id = ? AND id < ? AND is_deleted = ?", chatID, before, false).
		Limit(limit).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages before: %w", err)
	}
	return messages, nil
}

// AddReaction adds a reaction to a message
func (r *messageRepository) AddReaction(reaction *models.MessageReaction) error {
	if err := r.db.Create(reaction).Error; err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}
	return nil
}

// RemoveReaction removes a reaction from a message
func (r *messageRepository) RemoveReaction(messageID, userID uint, emoji string) error {
	result := r.db.Where("message_id = ? AND user_id = ? AND emoji = ?", messageID, userID, emoji).
		Delete(&models.MessageReaction{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove reaction: %w", result.Error)
	}
	return nil
}

// GetReactions retrieves all reactions for a message
func (r *messageRepository) GetReactions(messageID uint) ([]*models.MessageReaction, error) {
	var reactions []*models.MessageReaction
	err := r.db.Where("message_id = ?", messageID).Find(&reactions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}
	return reactions, nil
}

// MarkAsRead marks a message as read by a user
func (r *messageRepository) MarkAsRead(receipt *models.MessageReadReceipt) error {
	// Check if already marked as read
	var existing models.MessageReadReceipt
	err := r.db.Where("message_id = ? AND user_id = ?", receipt.MessageID, receipt.UserID).
		First(&existing).Error

	if err == nil {
		// Already marked as read, update timestamp
		existing.ReadAt = receipt.ReadAt
		return r.db.Save(&existing).Error
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing read receipt: %w", err)
	}

	// Create new read receipt
	if err := r.db.Create(receipt).Error; err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}
	return nil
}

// GetReadReceipts retrieves all read receipts for a message
func (r *messageRepository) GetReadReceipts(messageID uint) ([]*models.MessageReadReceipt, error) {
	var receipts []*models.MessageReadReceipt
	err := r.db.Where("message_id = ?", messageID).Find(&receipts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get read receipts: %w", err)
	}
	return receipts, nil
}
