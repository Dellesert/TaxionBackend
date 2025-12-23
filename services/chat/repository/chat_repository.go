package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/shared/database"
	sharedmodels "tachyon-messenger/shared/models"

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
	GetUserChats(userID uint, limit, offset int, chatType string, isFavorite, isPinned *bool) ([]*models.Chat, int64, error)
	GetUserChatsWithSync(userID uint, limit, offset int, chatType string, isFavorite, isPinned *bool, updatedSince *time.Time) ([]*models.Chat, int64, error)
	GetPinnedChats(userID uint, chatType string) ([]*models.Chat, error)
	GetDirectChatBetweenUsers(user1ID, user2ID uint) (*models.Chat, error)
	GetChatByTaskID(taskID uint) (*models.Chat, error)

	// Chat member operations
	AddMember(member *models.ChatMember) error
	RemoveMember(chatID, userID uint) error
	GetChatMembers(chatID uint) ([]*models.ChatMember, error)
	GetChatMemberIDs(chatID uint) ([]uint, error)
	GetUserChatIDs(userID uint) ([]uint, error) // Get all chat IDs where user is a member
	IsMember(chatID, userID uint) (bool, error)
	GetMemberRole(chatID, userID uint) (models.ChatMemberRole, error)
	UpdateMemberRole(chatID, userID uint, role models.ChatMemberRole) error
	UpdateFavoriteStatus(chatID, userID uint, isFavorite bool) error
	UpdatePinnedStatus(chatID, userID uint, isPinned bool) error
	UpdateHiddenStatus(chatID, userID uint, isHidden bool) error

	// Access control methods
	HasReadAccess(chatID, userID uint) (bool, error)
	HasWriteAccess(chatID, userID uint) (bool, error)
	HasAdminAccess(chatID, userID uint) (bool, error)
	HasOwnerAccess(chatID, userID uint) (bool, error)

	// Sync methods
	GetDeletedChatIDsSince(since time.Time) ([]uint, error)
	RecordDeletion(chatID uint, deletedBy *uint) error
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
	// Add secondary sort by id to ensure deterministic ordering for pagination
	err := r.db.
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ?", userID, true).
		Where("chats.is_active = ?", true).
		Limit(limit).
		Offset(offset).
		Order("chats.last_message_at DESC NULLS LAST, chats.updated_at DESC, chats.id DESC").
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
		Preload("Members.User"). // Загружаем User для каждого Member
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

// GetUserChats retrieves all chats for a user with pagination, sorting and optional filters
func (r *chatRepository) GetUserChats(userID uint, limit, offset int, chatType string, isFavorite, isPinned *bool) ([]*models.Chat, int64, error) {
	var chats []*models.Chat
	var total int64

	// Build base query for count
	countQuery := r.db.Model(&models.Chat{}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.is_active = ?", true)

	// Apply filters
	if chatType != "" {
		countQuery = countQuery.Where("chats.type = ?", chatType)
	}
	if isFavorite != nil {
		countQuery = countQuery.Where("chat_members.is_favorite = ?", *isFavorite)
	}
	if isPinned != nil {
		countQuery = countQuery.Where("chat_members.is_pinned = ?", *isPinned)
	}

	// Get total count (exclude hidden chats)
	err := countQuery.Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user chats: %w", err)
	}

	// Build query for getting chats
	query := r.db.
		Preload("Members.User"). // Загружаем User для каждого Member
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.is_active = ?", true)

	// Apply filters
	if chatType != "" {
		query = query.Where("chats.type = ?", chatType)
	}
	if isFavorite != nil {
		query = query.Where("chat_members.is_favorite = ?", *isFavorite)
	}
	if isPinned != nil {
		query = query.Where("chat_members.is_pinned = ?", *isPinned)
	}

	// Get chats with members, sorted by pinned status first, then by last activity (exclude hidden chats)
	// Pinned chats always come first, then sorted by last message time
	// Add secondary sort by id to ensure deterministic ordering for pagination
	err = query.
		Limit(limit).
		Offset(offset).
		Order("chat_members.is_pinned DESC, chats.last_message_at DESC NULLS LAST, chats.updated_at DESC, chats.id DESC").
		Find(&chats).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user chats: %w", err)
	}

	return chats, total, nil
}

// GetUserChatsWithSync retrieves user chats with optional incremental sync filtering
func (r *chatRepository) GetUserChatsWithSync(userID uint, limit, offset int, chatType string, isFavorite, isPinned *bool, updatedSince *time.Time) ([]*models.Chat, int64, error) {
	var chats []*models.Chat
	var total int64

	// Build base query for count
	countQuery := r.db.Model(&models.Chat{}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.is_active = ?", true)

	// Apply filters
	if chatType != "" {
		countQuery = countQuery.Where("chats.type = ?", chatType)
	}
	if isFavorite != nil {
		countQuery = countQuery.Where("chat_members.is_favorite = ?", *isFavorite)
	}
	if isPinned != nil {
		countQuery = countQuery.Where("chat_members.is_pinned = ?", *isPinned)
	}
	// Apply updated_since filter for incremental sync
	if updatedSince != nil {
		countQuery = countQuery.Where("chats.updated_at > ?", *updatedSince)
	}

	// Get total count
	err := countQuery.Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count user chats: %w", err)
	}

	// Build query for getting chats
	query := r.db.
		Preload("Members.User").
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chats.is_active = ?", true)

	// Apply filters
	if chatType != "" {
		query = query.Where("chats.type = ?", chatType)
	}
	if isFavorite != nil {
		query = query.Where("chat_members.is_favorite = ?", *isFavorite)
	}
	if isPinned != nil {
		query = query.Where("chat_members.is_pinned = ?", *isPinned)
	}
	// Apply updated_since filter for incremental sync
	if updatedSince != nil {
		query = query.Where("chats.updated_at > ?", *updatedSince)
	}

	// Get chats with members, sorted by pinned status first, then by last activity
	err = query.
		Limit(limit).
		Offset(offset).
		Order("chat_members.is_pinned DESC, chats.last_message_at DESC NULLS LAST, chats.updated_at DESC, chats.id DESC").
		Find(&chats).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user chats: %w", err)
	}

	return chats, total, nil
}

// GetDeletedChatIDsSince returns IDs of deleted chats since the given timestamp
func (r *chatRepository) GetDeletedChatIDsSince(since time.Time) ([]uint, error) {
	var records []sharedmodels.DeletedRecord
	err := r.db.
		Where("entity_type = ? AND deleted_at > ?", database.EntityTypeChat, since).
		Select("entity_id").
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(records))
	for i, record := range records {
		ids[i] = record.EntityID
	}
	return ids, nil
}

// RecordDeletion records a deleted chat for sync tracking
func (r *chatRepository) RecordDeletion(chatID uint, deletedBy *uint) error {
	record := sharedmodels.DeletedRecord{
		EntityType: database.EntityTypeChat,
		EntityID:   chatID,
		DeletedAt:  time.Now(),
		DeletedBy:  deletedBy,
	}
	return r.db.Create(&record).Error
}

// GetPinnedChats retrieves all pinned chats for a user with optional type filter
// This returns ALL pinned chats without pagination, sorted by last activity
func (r *chatRepository) GetPinnedChats(userID uint, chatType string) ([]*models.Chat, error) {
	var chats []*models.Chat

	// Build query for getting pinned chats
	query := r.db.
		Preload("Members.User"). // Load User for each Member
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		Joins("JOIN chat_members ON chats.id = chat_members.chat_id").
		Where("chat_members.user_id = ? AND chat_members.is_active = ? AND chat_members.is_hidden = ?", userID, true, false).
		Where("chat_members.is_pinned = ?", true).
		Where("chats.is_active = ?", true)

	// Apply type filter if specified
	if chatType != "" {
		query = query.Where("chats.type = ?", chatType)
	}

	// Get all pinned chats sorted by last activity
	err := query.
		Order("chats.last_message_at DESC NULLS LAST, chats.updated_at DESC, chats.id DESC").
		Find(&chats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get pinned chats: %w", err)
	}

	return chats, nil
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

// GetChatMemberIDs retrieves only the user IDs of active members in a chat
// This is optimized for WebSocket broadcasts to avoid loading full member data
func (r *chatRepository) GetChatMemberIDs(chatID uint) ([]uint, error) {
	var userIDs []uint
	err := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND is_active = ?", chatID, true).
		Pluck("user_id", &userIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get chat member IDs: %w", err)
	}
	return userIDs, nil
}

// GetUserChatIDs retrieves all chat IDs where user is an active member
// This is used for broadcasting user presence to all relevant chats
func (r *chatRepository) GetUserChatIDs(userID uint) ([]uint, error) {
	var chatIDs []uint
	err := r.db.Model(&models.ChatMember{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Pluck("chat_id", &chatIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user chat IDs: %w", err)
	}
	return chatIDs, nil
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

// UpdateMemberRole updates the role of a chat member
func (r *chatRepository) UpdateMemberRole(chatID, userID uint, role models.ChatMemberRole) error {
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Update("role", role)

	if result.Error != nil {
		return fmt.Errorf("failed to update member role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found")
	}
	return nil
}

// UpdateFavoriteStatus updates the favorite status of a chat for a user
func (r *chatRepository) UpdateFavoriteStatus(chatID, userID uint, isFavorite bool) error {
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Update("is_favorite", isFavorite)

	if result.Error != nil {
		return fmt.Errorf("failed to update favorite status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found")
	}
	return nil
}

// UpdatePinnedStatus updates the pinned status of a chat for a user
func (r *chatRepository) UpdatePinnedStatus(chatID, userID uint, isPinned bool) error {
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Update("is_pinned", isPinned)

	if result.Error != nil {
		return fmt.Errorf("failed to update pinned status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found")
	}
	return nil
}

// UpdateHiddenStatus updates the hidden status of a chat for a user
func (r *chatRepository) UpdateHiddenStatus(chatID, userID uint, isHidden bool) error {
	result := r.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ? AND is_active = ?", chatID, userID, true).
		Update("is_hidden", isHidden)

	if result.Error != nil {
		return fmt.Errorf("failed to update hidden status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("chat member not found")
	}
	return nil
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

// GetDirectChatBetweenUsers finds an existing private chat between two users
// Note: This includes hidden chats - they can be unhidden when a new message is sent
func (r *chatRepository) GetDirectChatBetweenUsers(user1ID, user2ID uint) (*models.Chat, error) {
	var chat models.Chat

	// Find a private chat where both users are members (including hidden chats)
	// We don't filter by is_hidden here because we want to find existing chats
	// even if they're hidden - they'll be unhidden when reopened
	err := r.db.
		Joins("JOIN chat_members cm1 ON chats.id = cm1.chat_id AND cm1.user_id = ? AND cm1.is_active = ?", user1ID, true).
		Joins("JOIN chat_members cm2 ON chats.id = cm2.chat_id AND cm2.user_id = ? AND cm2.is_active = ?", user2ID, true).
		Where("chats.type = ? AND chats.is_active = ? AND chats.task_id IS NULL", models.ChatTypePrivate, true).
		Preload("Members.User").
		Preload("Members", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("role ASC, joined_at ASC")
		}).
		First(&chat).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // No existing chat found
		}
		return nil, fmt.Errorf("failed to find direct chat: %w", err)
	}

	return &chat, nil
}

// GetChatByTaskID finds a chat linked to a specific task
func (r *chatRepository) GetChatByTaskID(taskID uint) (*models.Chat, error) {
	var chats []models.Chat

	err := r.db.
		Where("task_id = ? AND is_active = ?", taskID, true).
		Limit(1).
		Find(&chats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find chat by task ID: %w", err)
	}

	// If no chat found, return nil, nil (not an error)
	if len(chats) == 0 {
		return nil, nil
	}

	chat := &chats[0]

	// Load members separately if chat found
	err = r.db.
		Preload("User").
		Where("chat_id = ? AND is_active = ?", chat.ID, true).
		Order("role ASC, joined_at ASC").
		Find(&chat.Members).Error

	if err != nil {
		return nil, fmt.Errorf("failed to load chat members: %w", err)
	}

	return chat, nil
}
