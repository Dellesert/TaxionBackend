package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MessageRepository defines the interface for message data operations
type MessageRepository interface {
	Create(message *models.Message) error
	GetByID(id uint) (*models.Message, error)
	GetByChatID(chatID uint, limit, offset int) ([]*models.Message, error)
	GetByChatIDWithPagination(chatID uint, limit, offset int) ([]*models.Message, int64, error)
	GetByChatIDWithPaginationForUser(chatID, userID uint, limit, offset int) ([]*models.Message, int64, error)
	Update(message *models.Message) error
	Delete(id uint) error
	Count() (int64, error)
	CountByChatID(chatID uint) (int64, error)
	GetWithReactions(id uint) (*models.Message, error)
	GetMessagesAfter(chatID, userID, after uint, limit int) ([]*models.Message, error)
	GetMessagesBefore(chatID, userID, before uint, limit int) ([]*models.Message, error)
	GetMessagesByTimeRange(chatID uint, startTime, endTime time.Time, limit, offset int) ([]*models.Message, error)
	GetLatestMessage(chatID uint) (*models.Message, error)
	GetLatestMessageForUser(chatID, userID uint) (*models.Message, error)

	// Message reaction operations
	AddReaction(reaction *models.MessageReaction) error
	RemoveReaction(messageID, userID uint, emoji string) error
	GetReactions(messageID uint) ([]*models.MessageReaction, error)

	// Read receipt operations
	MarkAsRead(receipt *models.MessageReadReceipt) error
	GetReadReceipts(messageID uint) ([]*models.MessageReadReceipt, error)
	GetUnreadCount(chatID, userID uint) (int64, error)
	GetTotalUnreadCount(userID uint) (int64, error)
	MarkAllAsRead(chatID, userID uint) ([]uint, error)

	// Search and filtering
	SearchMessages(chatID, userID uint, query string, limit, offset int) ([]*models.Message, int64, error)
	GetMessagesByType(chatID uint, messageType models.MessageType, limit, offset int) ([]*models.Message, error)

	// Personal message deletion operations ("delete for me")
	AddMessageDeletion(messageID, userID uint) error
	RemoveMessageDeletion(messageID, userID uint) error
	GetUserDeletedMessages(chatID, userID uint) ([]uint, error)
	IsMessageDeletedForUser(messageID, userID uint) (bool, error)
	ClearChatHistoryForUser(chatID, userID uint) error

	// Attachment operations
	CreateAttachment(attachment *models.MessageAttachment) error
	GetChatAttachments(chatID uint, limit, offset int) ([]*models.MessageAttachment, int64, error)

	// New cursor-based pagination methods for refactored API
	GetLatestMessages(chatID, userID uint, limit int) ([]*models.Message, int64, error)
	GetMessagesBeforeID(chatID, userID, beforeID uint, limit int) ([]*models.Message, error)
	GetMessagesAfterID(chatID, userID, afterID uint, limit int) ([]*models.Message, error)
	GetMessageContext(chatID, userID, targetMessageID uint, before, after int) ([]*models.Message, error)
	GetFirstUnreadMessage(chatID, userID uint) (*models.Message, int64, error)
	HasOlderMessages(chatID, userID, oldestID uint) (bool, error)
	HasNewerMessages(chatID, userID, newestID uint) (bool, error)

	// Pinned messages
	GetPinnedMessages(chatID, userID uint) ([]*models.Message, error)
}

// messageRepository implements MessageRepository interface
type messageRepository struct {
	db *database.DB
}

// NewMessageRepository creates a new message repository
func NewMessageRepository(db *database.DB) MessageRepository {
	return &messageRepository{
		db: db,
	}
}

// Create creates a new message
func (r *messageRepository) Create(message *models.Message) error {
	if err := r.db.Create(message).Error; err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

// GetByID retrieves a message by ID with reply-to message
func (r *messageRepository) GetByID(id uint) (*models.Message, error) {
	var message models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		First(&message, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	return &message, nil
}

// GetByChatID retrieves messages by chat ID with pagination, sorted by time (newest first)
func (r *messageRepository) GetByChatID(chatID uint, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
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

// GetByChatIDWithPagination retrieves messages with total count for proper pagination
func (r *messageRepository) GetByChatIDWithPagination(chatID uint, limit, offset int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	var total int64

	// Get total count
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Count(&total).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get messages with preloaded data, sorted by time (newest first)
	err = r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, total, nil
}

// GetByChatIDWithPaginationForUser retrieves messages with total count, excluding personally deleted messages
func (r *messageRepository) GetByChatIDWithPaginationForUser(chatID, userID uint, limit, offset int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	var total int64

	// Subquery to get message IDs deleted by this user
	// IMPORTANT: Use Unscoped() because MessageDeletion has a DeletedAt field that should not trigger soft delete behavior
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	// Get total count (excluding personally deleted)
	// ВАЖНО: НЕ фильтруем is_deleted, чтобы админы могли видеть удалённые сообщения
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Count(&total).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get messages with preloaded data, sorted by time (newest first)
	// ВАЖНО: НЕ фильтруем is_deleted, чтобы админы могли видеть удалённые сообщения
	err = r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get messages: %w", err)
	}

	return messages, total, nil
}

// Update updates an existing message
// Uses Select to only update specific fields and preserve associations (like read_receipts)
func (r *messageRepository) Update(message *models.Message) error {
	// Update only specific fields, not associations
	// This prevents clearing read_receipts, reactions, etc.
	// Note: updated_at will be automatically updated by GORM
	result := r.db.Model(message).
		Select("content", "is_edited", "edited_at", "is_pinned", "is_deleted", "status").
		Updates(message)

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

// Count returns the total number of non-deleted messages
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

// GetWithReactions retrieves a message with all related data
func (r *messageRepository) GetWithReactions(id uint) (*models.Message, error) {
	var message models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		First(&message, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message with reactions: %w", err)
	}
	return &message, nil
}

// GetMessagesAfter retrieves messages after a specific message ID (for real-time updates)
func (r *messageRepository) GetMessagesAfter(chatID, userID, after uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message

	// Subquery to get message IDs deleted by this user
	// IMPORTANT: Use Unscoped() because MessageDeletion has a DeletedAt field that should not trigger soft delete behavior
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND id > ? AND is_deleted = ?", chatID, after, false).
		Where("id NOT IN (?)", deletedSubquery).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages after: %w", err)
	}
	return messages, nil
}

// GetMessagesBefore retrieves messages before a specific message ID (for loading history)
func (r *messageRepository) GetMessagesBefore(chatID, userID, before uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message

	// Subquery to get message IDs deleted by this user
	// IMPORTANT: Use Unscoped() because MessageDeletion has a DeletedAt field that should not trigger soft delete behavior
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND id < ? AND is_deleted = ?", chatID, before, false).
		Where("id NOT IN (?)", deletedSubquery).
		Limit(limit).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages before: %w", err)
	}
	return messages, nil
}

// GetMessagesByTimeRange retrieves messages within a time range
func (r *messageRepository) GetMessagesByTimeRange(chatID uint, startTime, endTime time.Time, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND created_at BETWEEN ? AND ? AND is_deleted = ?", chatID, startTime, endTime, false).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages by time range: %w", err)
	}
	return messages, nil
}

// Message reaction operations

// AddReaction adds a reaction to a message
func (r *messageRepository) AddReaction(reaction *models.MessageReaction) error {
	// Check if reaction already exists
	var existing models.MessageReaction
	err := r.db.Where("message_id = ? AND user_id = ? AND emoji = ?",
		reaction.MessageID, reaction.UserID, reaction.Emoji).First(&existing).Error

	if err == nil {
		// Reaction already exists, don't add duplicate
		return fmt.Errorf("reaction already exists")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing reaction: %w", err)
	}

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
	if result.RowsAffected == 0 {
		return fmt.Errorf("reaction not found")
	}
	return nil
}

// GetReactions retrieves all reactions for a message, grouped by emoji
func (r *messageRepository) GetReactions(messageID uint) ([]*models.MessageReaction, error) {
	var reactions []*models.MessageReaction
	err := r.db.Where("message_id = ?", messageID).
		Order("emoji ASC, created_at ASC").
		Find(&reactions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get reactions: %w", err)
	}
	return reactions, nil
}

// Read receipt operations

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
	err := r.db.Where("message_id = ?", messageID).
		Order("read_at DESC").
		Find(&receipts).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get read receipts: %w", err)
	}
	return receipts, nil
}

// GetUnreadCount returns the number of unread messages for a user in a chat
func (r *messageRepository) GetUnreadCount(chatID, userID uint) (int64, error) {
	var count int64

	// Get all messages in chat that don't have read receipts from this user
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND sender_id != ? AND is_deleted = ?", chatID, userID, false).
		Where("id NOT IN (?)",
			r.db.Table("message_read_receipts").
				Select("message_id").
				Where("user_id = ?", userID),
		).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count unread messages: %w", err)
	}
	return count, nil
}

// GetTotalUnreadCount returns the total number of unread messages for a user across all their chats
// For private chats, counts all unread messages
// For group/channel chats, counts as 1 if there are any unread messages
func (r *messageRepository) GetTotalUnreadCount(userID uint) (int64, error) {
	var count int64

	// Count unread messages in private chats
	var privateUnreadCount int64
	err := r.db.Model(&models.Message{}).
		Joins("JOIN chat_members ON messages.chat_id = chat_members.chat_id").
		Joins("JOIN chats ON chat_members.chat_id = chats.id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.type = ?", models.ChatTypePrivate).
		Where("messages.sender_id != ? AND messages.is_deleted = ?", userID, false).
		Where("messages.id NOT IN (?)",
			r.db.Table("message_read_receipts").
				Select("message_id").
				Where("user_id = ?", userID),
		).
		Count(&privateUnreadCount).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count private unread messages: %w", err)
	}

	// Count number of group/channel chats with unread messages
	var groupChatsWithUnread int64
	err = r.db.Table("chats").
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.type IN (?)", []models.ChatType{models.ChatTypeGroup, models.ChatTypeChannel}).
		Where("EXISTS (?)",
			r.db.Table("messages").
				Select("1").
				Where("messages.chat_id = chats.id").
				Where("messages.sender_id != ? AND messages.is_deleted = ?", userID, false).
				Where("messages.id NOT IN (?)",
					r.db.Table("message_read_receipts").
						Select("message_id").
						Where("user_id = ?", userID),
				),
		).
		Count(&groupChatsWithUnread).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count group chats with unread messages: %w", err)
	}

	count = privateUnreadCount + groupChatsWithUnread
	return count, nil
}

// Search and filtering operations

// SearchMessages searches for messages containing a query string in content or file names
func (r *messageRepository) SearchMessages(chatID, userID uint, query string, limit, offset int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	var total int64

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	// Build search condition for content and file names (case-insensitive)
	searchTerm := strings.TrimSpace(query)

	// Count total matching messages using raw SQL with ~* for case-insensitive regex
	// PostgreSQL ~* operator performs case-insensitive regex matching for all Unicode characters
	countQuery := r.db.Model(&models.Message{}).
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Where(
			"content ~* ? OR file_name ~* ? OR id IN (SELECT message_id FROM message_attachments WHERE file_name ~* ?)",
			searchTerm, searchTerm, searchTerm,
		)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Search for messages matching in content, file_name, or attachments (case-insensitive)
	// PostgreSQL ~* operator performs case-insensitive regex matching for all Unicode characters
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Where(
			"content ~* ? OR file_name ~* ? OR id IN (SELECT message_id FROM message_attachments WHERE file_name ~* ?)",
			searchTerm, searchTerm, searchTerm,
		).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to search messages: %w", err)
	}

	fmt.Printf("🔍 Search for '%s' in chat %d: found %d/%d messages\n", query, chatID, len(messages), total)

	return messages, total, nil
}

// GetMessagesByType retrieves messages of a specific type
func (r *messageRepository) GetMessagesByType(chatID uint, messageType models.MessageType, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND type = ? AND is_deleted = ?", chatID, messageType, false).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages by type: %w", err)
	}
	return messages, nil
}

// Additional helper methods for message management

// GetLatestMessage retrieves the most recent message in a chat
func (r *messageRepository) GetLatestMessage(chatID uint) (*models.Message, error) {
	var message models.Message
	err := r.db.
		Preload("Sender"). // Load sender information
		Preload("OriginalSender").
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments"). // Load attachments for last_message
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Order("created_at DESC").
		First(&message).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No messages found, not an error
		}
		return nil, fmt.Errorf("failed to get latest message: %w", err)
	}
	return &message, nil
}

// GetLatestMessageForUser retrieves the most recent message in a chat, excluding messages deleted by the user
func (r *messageRepository) GetLatestMessageForUser(chatID, userID uint) (*models.Message, error) {
	var message models.Message

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Where("id NOT IN (?)", deletedSubquery).
		Order("created_at DESC").
		First(&message).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No messages found, not an error
		}
		return nil, fmt.Errorf("failed to get latest message for user: %w", err)
	}
	return &message, nil
}

// GetMessagesForUser retrieves messages that a user can see (respects chat access)
func (r *messageRepository) GetMessagesForUser(chatID, userID uint, limit, offset int) ([]*models.Message, error) {
	// First verify user has access to the chat through a join with chat_members
	// Also exclude messages that were deleted by this user (from message_deletions table)
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Joins("JOIN chat_members ON chat_members.chat_id = messages.chat_id").
		Joins("LEFT JOIN message_deletions ON message_deletions.message_id = messages.id AND message_deletions.user_id = ?", userID).
		Where("messages.chat_id = ? AND messages.is_deleted = ?", chatID, false).
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("message_deletions.id IS NULL"). // Exclude messages deleted by this user
		Limit(limit).
		Offset(offset).
		Order("messages.created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages for user: %w", err)
	}
	return messages, nil
}

// GetMessagesSince retrieves messages since a specific timestamp
func (r *messageRepository) GetMessagesSince(chatID uint, since time.Time, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND created_at > ? AND is_deleted = ?", chatID, since, false).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages since: %w", err)
	}
	return messages, nil
}

// MarkAllAsRead marks all messages in a chat as read by a user
// Returns the list of message IDs that were marked as read
func (r *messageRepository) MarkAllAsRead(chatID, userID uint) ([]uint, error) {
	// Get all unread message IDs
	var messageIDs []uint
	err := r.db.Model(&models.Message{}).
		Select("id").
		Where("chat_id = ? AND sender_id != ? AND is_deleted = ?", chatID, userID, false).
		Where("id NOT IN (?)",
			r.db.Table("message_read_receipts").
				Select("message_id").
				Where("user_id = ?", userID),
		).
		Pluck("id", &messageIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get unread message IDs: %w", err)
	}

	if len(messageIDs) == 0 {
		return []uint{}, nil // No unread messages
	}

	// Create read receipts for all unread messages
	var receipts []models.MessageReadReceipt
	now := time.Now()
	for _, messageID := range messageIDs {
		receipts = append(receipts, models.MessageReadReceipt{
			MessageID: messageID,
			UserID:    userID,
			ReadAt:    now,
		})
	}

	if err := r.db.CreateInBatches(receipts, 100).Error; err != nil {
		return nil, fmt.Errorf("failed to create read receipts: %w", err)
	}

	return messageIDs, nil
}

// GetThreadMessages retrieves messages in a thread (replies to a specific message)
func (r *messageRepository) GetThreadMessages(replyToID uint, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("reply_to_id = ? AND is_deleted = ?", replyToID, false).
		Limit(limit).
		Offset(offset).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}
	return messages, nil
}

// GetMessageStats returns statistics about messages in a chat
func (r *messageRepository) GetMessageStats(chatID uint) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total message count
	var totalCount int64
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Count(&totalCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count total messages: %w", err)
	}
	stats["total_messages"] = totalCount

	// Message count by type
	var typeCounts []struct {
		Type  models.MessageType `json:"type"`
		Count int64              `json:"count"`
	}
	err = r.db.Model(&models.Message{}).
		Select("type, COUNT(*) as count").
		Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Group("type").
		Scan(&typeCounts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count messages by type: %w", err)
	}
	stats["by_type"] = typeCounts

	// Messages with reactions count
	var reactedCount int64
	err = r.db.Model(&models.Message{}).
		Joins("JOIN message_reactions ON messages.id = message_reactions.message_id").
		Where("messages.chat_id = ? AND messages.is_deleted = ?", chatID, false).
		Distinct("messages.id").
		Count(&reactedCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count reacted messages: %w", err)
	}
	stats["messages_with_reactions"] = reactedCount

	// Get latest message timestamp
	var latestMessage models.Message
	err = r.db.Where("chat_id = ? AND is_deleted = ?", chatID, false).
		Order("created_at DESC").
		First(&latestMessage).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to get latest message: %w", err)
	}
	if err == nil {
		stats["latest_message_at"] = latestMessage.CreatedAt
	}

	return stats, nil
}

// CleanupOldMessages removes messages older than specified duration (hard delete)
func (r *messageRepository) CleanupOldMessages(olderThan time.Time) (int64, error) {
	result := r.db.Unscoped().
		Where("created_at < ? AND is_deleted = ?", olderThan, true).
		Delete(&models.Message{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old messages: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// Personal message deletion operations ("delete for me")

// AddMessageDeletion adds a personal deletion record for a user
func (r *messageRepository) AddMessageDeletion(messageID, userID uint) error {
	// Check if deletion already exists (use Unscoped to bypass soft delete filter)
	var existing models.MessageDeletion
	err := r.db.Unscoped().Where("message_id = ? AND user_id = ?", messageID, userID).
		First(&existing).Error

	if err == nil {
		// Already deleted for this user - just return success
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing deletion: %w", err)
	}

	// Create new deletion record
	deletion := &models.MessageDeletion{
		MessageID: messageID,
		UserID:    userID,
		DeletedAt: time.Now(),
	}

	if err := r.db.Create(deletion).Error; err != nil {
		// If error is duplicate key, just return success (race condition)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "idx_message_deletions_unique") {
			return nil
		}
		return fmt.Errorf("failed to add message deletion: %w", err)
	}

	return nil
}

// RemoveMessageDeletion removes a personal deletion record (for restore functionality)
func (r *messageRepository) RemoveMessageDeletion(messageID, userID uint) error {
	result := r.db.Where("message_id = ? AND user_id = ?", messageID, userID).
		Delete(&models.MessageDeletion{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove message deletion: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("deletion record not found")
	}

	return nil
}

// GetUserDeletedMessages returns list of message IDs that user has deleted for themselves
func (r *messageRepository) GetUserDeletedMessages(chatID, userID uint) ([]uint, error) {
	var messageIDs []uint

	err := r.db.Model(&models.MessageDeletion{}).
		Select("message_deletions.message_id").
		Joins("JOIN messages ON messages.id = message_deletions.message_id").
		Where("messages.chat_id = ? AND message_deletions.user_id = ?", chatID, userID).
		Pluck("message_id", &messageIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user deleted messages: %w", err)
	}

	return messageIDs, nil
}

// IsMessageDeletedForUser checks if a specific message is deleted for a user
func (r *messageRepository) IsMessageDeletedForUser(messageID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.MessageDeletion{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check message deletion: %w", err)
	}

	return count > 0, nil
}

// ClearChatHistoryForUser deletes all messages in a chat for a specific user
func (r *messageRepository) ClearChatHistoryForUser(chatID, userID uint) error {
	// Get all message IDs in the chat
	var messageIDs []uint
	err := r.db.Model(&models.Message{}).
		Select("id").
		Where("chat_id = ?", chatID).
		Pluck("id", &messageIDs).Error

	if err != nil {
		return fmt.Errorf("failed to get chat message IDs: %w", err)
	}

	if len(messageIDs) == 0 {
		return nil // No messages to delete
	}

	// Create deletion records for all messages (ignore duplicates)
	var deletions []models.MessageDeletion
	now := time.Now()
	for _, messageID := range messageIDs {
		deletions = append(deletions, models.MessageDeletion{
			MessageID: messageID,
			UserID:    userID,
			DeletedAt: now,
		})
	}

	// Use ON CONFLICT DO NOTHING to handle duplicates
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(deletions, 100).Error; err != nil {
		return fmt.Errorf("failed to clear chat history: %w", err)
	}

	return nil
}

// Attachment operations

// CreateAttachment creates a new message attachment record
func (r *messageRepository) CreateAttachment(attachment *models.MessageAttachment) error {
	if err := r.db.Create(attachment).Error; err != nil {
		return fmt.Errorf("failed to create attachment: %w", err)
	}
	return nil
}

// GetChatAttachments retrieves all attachments for a chat with pagination
func (r *messageRepository) GetChatAttachments(chatID uint, limit, offset int) ([]*models.MessageAttachment, int64, error) {
	var attachments []*models.MessageAttachment
	var total int64

	// First, count total attachments for this chat
	countQuery := r.db.Model(&models.MessageAttachment{}).
		Joins("JOIN messages ON message_attachments.message_id = messages.id").
		Where("messages.chat_id = ? AND messages.deleted_at IS NULL", chatID)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count attachments: %w", err)
	}

	// Then, get paginated attachments
	query := r.db.
		Joins("JOIN messages ON message_attachments.message_id = messages.id").
		Where("messages.chat_id = ? AND messages.deleted_at IS NULL", chatID).
		Order("message_attachments.created_at DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&attachments).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get attachments: %w", err)
	}

	return attachments, total, nil
}

// New cursor-based pagination methods for refactored API

// GetLatestMessages retrieves the latest N messages in chronological order (old to new)
func (r *messageRepository) GetLatestMessages(chatID, userID uint, limit int) ([]*models.Message, int64, error) {
	var messages []*models.Message
	var total int64

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	// Get total count (excluding personally deleted, but including soft-deleted for admins)
	err := r.db.Model(&models.Message{}).
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Count(&total).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	// Get latest messages (ORDER BY id DESC, then reverse in code for chronological order)
	err = r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ?", chatID).
		Where("id NOT IN (?)", deletedSubquery).
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get latest messages: %w", err)
	}

	// Reverse array to get chronological order (oldest to newest)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, total, nil
}

// GetMessagesBeforeID retrieves messages before a specific message ID in chronological order
func (r *messageRepository) GetMessagesBeforeID(chatID, userID, beforeID uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND id < ?", chatID, beforeID).
		Where("id NOT IN (?)", deletedSubquery).
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages before ID: %w", err)
	}

	// Reverse array to get chronological order (oldest to newest)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetMessageContext retrieves messages around a target message (for "jump to message" feature)
func (r *messageRepository) GetMessageContext(chatID, userID, targetMessageID uint, before, after int) ([]*models.Message, error) {
	var beforeMessages []*models.Message
	var targetMessage models.Message
	var afterMessages []*models.Message

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	// Get target message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("id = ? AND chat_id = ?", targetMessageID, chatID).
		Where("id NOT IN (?)", deletedSubquery).
		First(&targetMessage).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("target message not found")
		}
		return nil, fmt.Errorf("failed to get target message: %w", err)
	}

	// Get messages before target
	if before > 0 {
		err = r.db.
			Preload("Sender").
			Preload("ReplyTo").
			Preload("ReplyTo.Sender").
			Preload("Reactions", func(db *gorm.DB) *gorm.DB {
				return db.Order("created_at ASC")
			}).
			Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
				return db.Order("read_at DESC")
			}).
			Preload("Attachments").
			Where("chat_id = ? AND id < ?", chatID, targetMessageID).
			Where("id NOT IN (?)", deletedSubquery).
			Order("id DESC").
			Limit(before).
			Find(&beforeMessages).Error

		if err != nil {
			return nil, fmt.Errorf("failed to get messages before target: %w", err)
		}

		// Reverse to get chronological order
		for i, j := 0, len(beforeMessages)-1; i < j; i, j = i+1, j-1 {
			beforeMessages[i], beforeMessages[j] = beforeMessages[j], beforeMessages[i]
		}
	}

	// Get messages after target
	if after > 0 {
		err = r.db.
			Preload("Sender").
			Preload("ReplyTo").
			Preload("ReplyTo.Sender").
			Preload("Reactions", func(db *gorm.DB) *gorm.DB {
				return db.Order("created_at ASC")
			}).
			Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
				return db.Order("read_at DESC")
			}).
			Preload("Attachments").
			Where("chat_id = ? AND id > ?", chatID, targetMessageID).
			Where("id NOT IN (?)", deletedSubquery).
			Order("id ASC").
			Limit(after).
			Find(&afterMessages).Error

		if err != nil {
			return nil, fmt.Errorf("failed to get messages after target: %w", err)
		}
	}

	// Combine all messages in chronological order
	allMessages := make([]*models.Message, 0, len(beforeMessages)+1+len(afterMessages))
	allMessages = append(allMessages, beforeMessages...)
	allMessages = append(allMessages, &targetMessage)
	allMessages = append(allMessages, afterMessages...)

	return allMessages, nil
}

// GetFirstUnreadMessage retrieves the first unread message and unread count for a user in a chat
func (r *messageRepository) GetFirstUnreadMessage(chatID, userID uint) (*models.Message, int64, error) {
	var message models.Message
	var unreadCount int64

	// Subquery for read messages
	readSubquery := r.db.Table("message_read_receipts").
		Select("message_id").
		Where("user_id = ?", userID)

	// Subquery for personally deleted messages
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	// Get first unread message
	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND sender_id != ?", chatID, userID).
		Where("id NOT IN (?)", readSubquery).
		Where("id NOT IN (?)", deletedSubquery).
		Order("id ASC").
		First(&message).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, nil // No unread messages
		}
		return nil, 0, fmt.Errorf("failed to get first unread message: %w", err)
	}

	// Count unread messages from this message onwards
	err = r.db.Model(&models.Message{}).
		Where("chat_id = ? AND id >= ? AND sender_id != ?", chatID, message.ID, userID).
		Where("id NOT IN (?)", readSubquery).
		Where("id NOT IN (?)", deletedSubquery).
		Count(&unreadCount).Error

	if err != nil {
		return &message, 0, fmt.Errorf("failed to count unread messages: %w", err)
	}

	return &message, unreadCount, nil
}

// HasOlderMessages checks if there are messages older than the given ID
func (r *messageRepository) HasOlderMessages(chatID, userID, oldestID uint) (bool, error) {
	var count int64

	// Subquery for personally deleted messages
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND id < ?", chatID, oldestID).
		Where("id NOT IN (?)", deletedSubquery).
		Limit(1).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check for older messages: %w", err)
	}

	return count > 0, nil
}

// HasNewerMessages checks if there are messages newer than the given ID
func (r *messageRepository) HasNewerMessages(chatID, userID, newestID uint) (bool, error) {
	var count int64

	// Subquery for personally deleted messages
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.Model(&models.Message{}).
		Where("chat_id = ? AND id > ?", chatID, newestID).
		Where("id NOT IN (?)", deletedSubquery).
		Limit(1).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check for newer messages: %w", err)
	}

	return count > 0, nil
}

// GetMessagesAfterID retrieves messages after a specific message ID in chronological order
func (r *messageRepository) GetMessagesAfterID(chatID, userID, afterID uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND id > ?", chatID, afterID).
		Where("id NOT IN (?)", deletedSubquery).
		Order("id ASC"). // Chronological order (oldest to newest)
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages after ID: %w", err)
	}

	return messages, nil
}

// GetPinnedMessages retrieves all pinned messages in a chat
func (r *messageRepository) GetPinnedMessages(chatID, userID uint) ([]*models.Message, error) {
	var messages []*models.Message

	// Subquery to get message IDs deleted by this user
	deletedSubquery := r.db.Model(&models.MessageDeletion{}).
		Unscoped().
		Select("message_id").
		Where("user_id = ?", userID)

	err := r.db.
		Preload("Sender").
		Preload("OriginalSender").
		Preload("ReplyTo").
		Preload("ReplyTo.Sender").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
		Preload("Attachments").
		Where("chat_id = ? AND is_pinned = ?", chatID, true).
		Where("id NOT IN (?)", deletedSubquery).
		Order("id DESC"). // Most recently pinned first
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get pinned messages: %w", err)
	}

	return messages, nil
}
