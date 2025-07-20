package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// MessageRepository defines the interface for message data operations
type MessageRepository interface {
	Create(message *models.Message) error
	GetByID(id uint) (*models.Message, error)
	GetByChatID(chatID uint, limit, offset int) ([]*models.Message, error)
	GetByChatIDWithPagination(chatID uint, limit, offset int) ([]*models.Message, int64, error)
	Update(message *models.Message) error
	Delete(id uint) error
	Count() (int64, error)
	CountByChatID(chatID uint) (int64, error)
	GetWithReactions(id uint) (*models.Message, error)
	GetMessagesAfter(chatID uint, after uint, limit int) ([]*models.Message, error)
	GetMessagesBefore(chatID uint, before uint, limit int) ([]*models.Message, error)
	GetMessagesByTimeRange(chatID uint, startTime, endTime time.Time, limit, offset int) ([]*models.Message, error)

	// Message reaction operations
	AddReaction(reaction *models.MessageReaction) error
	RemoveReaction(messageID, userID uint, emoji string) error
	GetReactions(messageID uint) ([]*models.MessageReaction, error)

	// Read receipt operations
	MarkAsRead(receipt *models.MessageReadReceipt) error
	GetReadReceipts(messageID uint) ([]*models.MessageReadReceipt, error)
	GetUnreadCount(chatID, userID uint) (int64, error)

	// Search and filtering
	SearchMessages(chatID uint, query string, limit, offset int) ([]*models.Message, error)
	GetMessagesByType(chatID uint, messageType models.MessageType, limit, offset int) ([]*models.Message, error)
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
	err := r.db.Preload("ReplyTo").First(&message, id).Error
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
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
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
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
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
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("ReadReceipts", func(db *gorm.DB) *gorm.DB {
			return db.Order("read_at DESC")
		}).
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
func (r *messageRepository) GetMessagesAfter(chatID uint, after uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Where("chat_id = ? AND id > ? AND is_deleted = ?", chatID, after, false).
		Limit(limit).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get messages after: %w", err)
	}
	return messages, nil
}

// GetMessagesBefore retrieves messages before a specific message ID (for loading history)
func (r *messageRepository) GetMessagesBefore(chatID uint, before uint, limit int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Where("chat_id = ? AND id < ? AND is_deleted = ?", chatID, before, false).
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
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
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

// Search and filtering operations

// SearchMessages searches for messages containing a query string
func (r *messageRepository) SearchMessages(chatID uint, query string, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Where("chat_id = ? AND content ILIKE ? AND is_deleted = ?", chatID, "%"+query+"%", false).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&messages).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}
	return messages, nil
}

// GetMessagesByType retrieves messages of a specific type
func (r *messageRepository) GetMessagesByType(chatID uint, messageType models.MessageType, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
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

// GetMessagesForUser retrieves messages that a user can see (respects chat access)
func (r *messageRepository) GetMessagesForUser(chatID, userID uint, limit, offset int) ([]*models.Message, error) {
	// First verify user has access to the chat through a join with chat_members
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Joins("JOIN chat_members ON chat_members.chat_id = messages.chat_id").
		Where("messages.chat_id = ? AND messages.is_deleted = ?", chatID, false).
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
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
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
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
func (r *messageRepository) MarkAllAsRead(chatID, userID uint) error {
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
		return fmt.Errorf("failed to get unread message IDs: %w", err)
	}

	if len(messageIDs) == 0 {
		return nil // No unread messages
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
		return fmt.Errorf("failed to create read receipts: %w", err)
	}

	return nil
}

// GetThreadMessages retrieves messages in a thread (replies to a specific message)
func (r *messageRepository) GetThreadMessages(replyToID uint, limit, offset int) ([]*models.Message, error) {
	var messages []*models.Message
	err := r.db.
		Preload("ReplyTo").
		Preload("Reactions", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
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
