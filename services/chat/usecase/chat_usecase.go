package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/repository"

	"gorm.io/gorm"
)

// ChatUsecase defines the interface for chat business logic
type ChatUsecase interface {
	CreateChat(userID uint, req *models.CreateChatRequest) (*models.ChatResponse, error)
	GetUserChats(userID uint, limit, offset int) (*models.ChatListResponse, error)
	GetChat(userID, chatID uint) (*models.ChatResponse, error)
	UpdateChat(userID, chatID uint, req *models.UpdateChatRequest) (*models.ChatResponse, error)
	DeleteChat(userID, chatID uint) error
	AddMember(userID, chatID uint, req *models.AddChatMemberRequest) error
	RemoveMember(userID, chatID, targetUserID uint) error
	GetChatMembers(userID, chatID uint) ([]models.ChatMemberResponse, error)
	LeaveChat(userID, chatID uint) error
}

// MessageUsecase defines the interface for message business logic
type MessageUsecase interface {
	SendMessage(userID uint, req *models.SendMessageRequest) (*models.MessageResponse, error)
	GetMessages(userID uint, req *models.GetMessagesRequest) (*models.MessageListResponse, error)
	GetMessage(userID, messageID uint) (*models.MessageResponse, error)
	UpdateMessage(userID, messageID uint, req *models.UpdateMessageRequest) (*models.MessageResponse, error)
	DeleteMessage(userID, messageID uint) error
	AddReaction(userID, messageID uint, req *models.AddReactionRequest) error
	RemoveReaction(userID, messageID uint, emoji string) error
	MarkAsRead(userID, messageID uint) error
	GetMessagesByChat(userID, chatID uint, limit, offset int) (*models.MessageListResponse, error)
}

// chatUsecase implements ChatUsecase interface
type chatUsecase struct {
	chatRepo    repository.ChatRepository
	messageRepo repository.MessageRepository
}

// messageUsecase implements MessageUsecase interface
type messageUsecase struct {
	messageRepo repository.MessageRepository
	chatRepo    repository.ChatRepository
}

// NewChatUsecase creates a new chat usecase
func NewChatUsecase(chatRepo repository.ChatRepository, messageRepo repository.MessageRepository) ChatUsecase {
	return &chatUsecase{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
	}
}

// NewMessageUsecase creates a new message usecase
func NewMessageUsecase(messageRepo repository.MessageRepository, chatRepo repository.ChatRepository) MessageUsecase {
	return &messageUsecase{
		messageRepo: messageRepo,
		chatRepo:    chatRepo,
	}
}

// Chat Usecase Methods

// CreateChat creates a new chat
func (uc *chatUsecase) CreateChat(userID uint, req *models.CreateChatRequest) (*models.ChatResponse, error) {
	// Validate request
	if err := uc.validateCreateChatRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// For private chats, ensure only 2 members (including creator)
	if req.Type == models.ChatTypePrivate {
		if len(req.MemberIDs) != 1 {
			return nil, fmt.Errorf("private chat must have exactly 2 members")
		}
		// Check if private chat already exists between these users
		existingChats, err := uc.chatRepo.GetByUserID(userID, 100, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing chats: %w", err)
		}

		for _, chat := range existingChats {
			if chat.Type == models.ChatTypePrivate {
				members, err := uc.chatRepo.GetChatMembers(chat.ID)
				if err != nil {
					continue
				}
				if len(members) == 2 {
					for _, member := range members {
						if member.UserID == req.MemberIDs[0] {
							return nil, fmt.Errorf("private chat already exists between these users")
						}
					}
				}
			}
		}
	}

	// Create chat
	chat := &models.Chat{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		CreatorID:   userID,
		Avatar:      req.Avatar,
		IsActive:    true,
	}

	// For private chats, set name to empty (will be handled by client)
	if req.Type == models.ChatTypePrivate {
		chat.Name = ""
	}

	if err := uc.chatRepo.Create(chat); err != nil {
		return nil, fmt.Errorf("failed to create chat: %w", err)
	}

	// Add additional members
	for _, memberID := range req.MemberIDs {
		member := &models.ChatMember{
			ChatID:   chat.ID,
			UserID:   memberID,
			Role:     models.ChatMemberRoleMember,
			JoinedAt: time.Now(),
			IsActive: true,
		}
		if err := uc.chatRepo.AddMember(member); err != nil {
			return nil, fmt.Errorf("failed to add member %d: %w", memberID, err)
		}
	}

	// Get chat with members for response
	chatWithMembers, err := uc.chatRepo.GetWithMembers(chat.ID)
	if err != nil {
		return chat.ToResponse(), nil // Return what we have
	}

	return chatWithMembers.ToResponse(), nil
}

// GetUserChats retrieves all chats for a user
func (uc *chatUsecase) GetUserChats(userID uint, limit, offset int) (*models.ChatListResponse, error) {
	// Set default pagination
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	chats, total, err := uc.chatRepo.GetUserChats(userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user chats: %w", err)
	}

	// Convert to response format
	chatResponses := make([]models.ChatResponse, len(chats))
	for i, chat := range chats {
		chatResponses[i] = *chat.ToResponse()
	}

	return &models.ChatListResponse{
		Chats:  chatResponses,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// GetChat retrieves a specific chat
func (uc *chatUsecase) GetChat(userID, chatID uint) (*models.ChatResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Get chat with members
	chat, err := uc.chatRepo.GetWithMembers(chatID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("chat not found")
		}
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	return chat.ToResponse(), nil
}

// UpdateChat updates a chat
func (uc *chatUsecase) UpdateChat(userID, chatID uint, req *models.UpdateChatRequest) (*models.ChatResponse, error) {
	// Check if user has permission to update chat
	role, err := uc.chatRepo.GetMemberRole(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return nil, fmt.Errorf("insufficient permissions to update chat")
	}

	// Get existing chat
	chat, err := uc.chatRepo.GetByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Update fields
	if req.Name != nil {
		chat.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		chat.Description = strings.TrimSpace(*req.Description)
	}
	if req.Avatar != nil {
		chat.Avatar = strings.TrimSpace(*req.Avatar)
	}

	if err := uc.chatRepo.Update(chat); err != nil {
		return nil, fmt.Errorf("failed to update chat: %w", err)
	}

	// Get updated chat with members
	updatedChat, err := uc.chatRepo.GetWithMembers(chatID)
	if err != nil {
		return chat.ToResponse(), nil // Return what we have
	}

	return updatedChat.ToResponse(), nil
}

// DeleteChat deletes a chat
func (uc *chatUsecase) DeleteChat(userID, chatID uint) error {
	// Check if user is the owner
	role, err := uc.chatRepo.GetMemberRole(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner {
		return fmt.Errorf("only chat owner can delete the chat")
	}

	if err := uc.chatRepo.Delete(chatID); err != nil {
		return fmt.Errorf("failed to delete chat: %w", err)
	}

	return nil
}

// AddMember adds a member to a chat
func (uc *chatUsecase) AddMember(userID, chatID uint, req *models.AddChatMemberRequest) error {
	// Check if user has permission to add members
	role, err := uc.chatRepo.GetMemberRole(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return fmt.Errorf("insufficient permissions to add members")
	}

	// Check if target user is already a member
	isMember, err := uc.chatRepo.IsMember(chatID, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if isMember {
		return fmt.Errorf("user is already a member of this chat")
	}

	// Set default role if not provided
	memberRole := req.Role
	if memberRole == "" {
		memberRole = models.ChatMemberRoleMember
	}

	// Create member
	member := &models.ChatMember{
		ChatID:   chatID,
		UserID:   req.UserID,
		Role:     memberRole,
		JoinedAt: time.Now(),
		IsActive: true,
	}

	if err := uc.chatRepo.AddMember(member); err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	return nil
}

// RemoveMember removes a member from a chat
func (uc *chatUsecase) RemoveMember(userID, chatID, targetUserID uint) error {
	// Check if user has permission to remove members
	role, err := uc.chatRepo.GetMemberRole(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}

	// Get target user role
	targetRole, err := uc.chatRepo.GetMemberRole(chatID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to get target user role: %w", err)
	}

	// Permission checks
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return fmt.Errorf("insufficient permissions to remove members")
	}

	// Admin cannot remove owner
	if role == models.ChatMemberRoleAdmin && targetRole == models.ChatMemberRoleOwner {
		return fmt.Errorf("admin cannot remove chat owner")
	}

	// Owner cannot be removed (must transfer ownership first)
	if targetRole == models.ChatMemberRoleOwner {
		return fmt.Errorf("cannot remove chat owner, transfer ownership first")
	}

	if err := uc.chatRepo.RemoveMember(chatID, targetUserID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	return nil
}

// GetChatMembers retrieves all members of a chat
func (uc *chatUsecase) GetChatMembers(userID, chatID uint) ([]models.ChatMemberResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	members, err := uc.chatRepo.GetChatMembers(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat members: %w", err)
	}

	// Convert to response format
	memberResponses := make([]models.ChatMemberResponse, len(members))
	for i, member := range members {
		memberResponses[i] = *member.ToResponse()
	}

	return memberResponses, nil
}

// LeaveChat allows a user to leave a chat
func (uc *chatUsecase) LeaveChat(userID, chatID uint) error {
	// Check if user is a member
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Get user role
	role, err := uc.chatRepo.GetMemberRole(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}

	// Owner cannot leave without transferring ownership
	if role == models.ChatMemberRoleOwner {
		return fmt.Errorf("chat owner cannot leave, transfer ownership first")
	}

	if err := uc.chatRepo.RemoveMember(chatID, userID); err != nil {
		return fmt.Errorf("failed to leave chat: %w", err)
	}

	return nil
}

// validateCreateChatRequest validates chat creation request
func (uc *chatUsecase) validateCreateChatRequest(req *models.CreateChatRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate chat type
	if req.Type != models.ChatTypePrivate && req.Type != models.ChatTypeGroup && req.Type != models.ChatTypeChannel {
		return fmt.Errorf("invalid chat type")
	}

	// For group and channel chats, name is required
	if (req.Type == models.ChatTypeGroup || req.Type == models.ChatTypeChannel) && strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required for group and channel chats")
	}

	// Validate member IDs
	if len(req.MemberIDs) == 0 && req.Type != models.ChatTypeChannel {
		return fmt.Errorf("at least one member is required")
	}

	return nil
}

// Message Usecase Methods

// SendMessage sends a new message
func (uc *messageUsecase) SendMessage(userID uint, req *models.SendMessageRequest) (*models.MessageResponse, error) {
	// Validate request
	if err := uc.validateSendMessageRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(req.ChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Validate reply-to message if provided
	if req.ReplyToID != nil {
		replyMsg, err := uc.messageRepo.GetByID(*req.ReplyToID)
		if err != nil {
			return nil, fmt.Errorf("reply-to message not found")
		}
		if replyMsg.ChatID != req.ChatID {
			return nil, fmt.Errorf("reply-to message is not in the same chat")
		}
	}

	// Create message
	message := &models.Message{
		ChatID:       req.ChatID,
		SenderID:     userID,
		Content:      strings.TrimSpace(req.Content),
		Type:         req.Type,
		Status:       models.MessageStatusSent,
		ReplyToID:    req.ReplyToID,
		FileName:     req.FileName,
		FileSize:     req.FileSize,
		FileURL:      req.FileURL,
		ThumbnailURL: req.ThumbnailURL,
		MimeType:     req.MimeType,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
	}

	// Set default type if not provided
	if message.Type == "" {
		message.Type = models.MessageTypeText
	}

	if err := uc.messageRepo.Create(message); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Get message with relations for response
	createdMessage, err := uc.messageRepo.GetWithReactions(message.ID)
	if err != nil {
		return message.ToResponse(), nil // Return what we have
	}

	return createdMessage.ToResponse(), nil
}

// GetMessages retrieves messages with filters
func (uc *messageUsecase) GetMessages(userID uint, req *models.GetMessagesRequest) (*models.MessageListResponse, error) {
	// Set default pagination
	if req.Limit <= 0 {
		req.Limit = 50
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	var messages []*models.Message
	var total int64

	if req.ChatID > 0 {
		// Check if user is a member of the chat
		isMember, err := uc.chatRepo.IsMember(req.ChatID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to check membership: %w", err)
		}
		if !isMember {
			return nil, fmt.Errorf("user is not a member of this chat")
		}

		// Get messages based on filters
		if req.After > 0 {
			messages, err = uc.messageRepo.GetMessagesAfter(req.ChatID, req.After, req.Limit)
		} else if req.Before > 0 {
			messages, err = uc.messageRepo.GetMessagesBefore(req.ChatID, req.Before, req.Limit)
		} else {
			messages, err = uc.messageRepo.GetByChatID(req.ChatID, req.Limit, req.Offset)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}

		// Get total count for pagination
		total, err = uc.messageRepo.CountByChatID(req.ChatID)
		if err != nil {
			total = 0 // Don't fail on count error
		}
	} else {
		return nil, fmt.Errorf("chat_id is required")
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponse()
	}

	hasMore := len(messages) == req.Limit

	return &models.MessageListResponse{
		Messages: messageResponses,
		Total:    total,
		Limit:    req.Limit,
		Offset:   req.Offset,
		HasMore:  hasMore,
	}, nil
}

// GetMessage retrieves a specific message
func (uc *messageUsecase) GetMessage(userID, messageID uint) (*models.MessageResponse, error) {
	message, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	return message.ToResponse(), nil
}

// UpdateMessage updates a message
func (uc *messageUsecase) UpdateMessage(userID, messageID uint, req *models.UpdateMessageRequest) (*models.MessageResponse, error) {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is the sender
	if message.SenderID != userID {
		return nil, fmt.Errorf("only message sender can edit the message")
	}

	// Check if message is already deleted
	if message.IsDeleted {
		return nil, fmt.Errorf("cannot edit deleted message")
	}

	// Update message
	message.Content = strings.TrimSpace(req.Content)
	message.IsEdited = true
	now := time.Now()
	message.EditedAt = &now

	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	// Get updated message with relations
	updatedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponse(), nil // Return what we have
	}

	return updatedMessage.ToResponse(), nil
}

// DeleteMessage deletes a message
func (uc *messageUsecase) DeleteMessage(userID, messageID uint) error {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is the sender or has admin/owner role in chat
	if message.SenderID != userID {
		role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
		if err != nil {
			return fmt.Errorf("failed to get user role: %w", err)
		}
		if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
			return fmt.Errorf("insufficient permissions to delete message")
		}
	}

	if err := uc.messageRepo.Delete(messageID); err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}

	return nil
}

// AddReaction adds a reaction to a message
func (uc *messageUsecase) AddReaction(userID, messageID uint, req *models.AddReactionRequest) error {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Create reaction
	reaction := &models.MessageReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     strings.TrimSpace(req.Emoji),
	}

	if err := uc.messageRepo.AddReaction(reaction); err != nil {
		return fmt.Errorf("failed to add reaction: %w", err)
	}

	return nil
}

// RemoveReaction removes a reaction from a message
func (uc *messageUsecase) RemoveReaction(userID, messageID uint, emoji string) error {
	// Get message to check chat membership
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	if err := uc.messageRepo.RemoveReaction(messageID, userID, emoji); err != nil {
		return fmt.Errorf("failed to remove reaction: %w", err)
	}

	return nil
}

// MarkAsRead marks a message as read
func (uc *messageUsecase) MarkAsRead(userID, messageID uint) error {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Create read receipt
	receipt := &models.MessageReadReceipt{
		MessageID: messageID,
		UserID:    userID,
		ReadAt:    time.Now(),
	}

	if err := uc.messageRepo.MarkAsRead(receipt); err != nil {
		return fmt.Errorf("failed to mark message as read: %w", err)
	}

	return nil
}

// GetMessagesByChat retrieves messages for a specific chat
func (uc *messageUsecase) GetMessagesByChat(userID, chatID uint, limit, offset int) (*models.MessageListResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set default pagination
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	messages, err := uc.messageRepo.GetByChatID(chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	// Get total count
	total, err := uc.messageRepo.CountByChatID(chatID)
	if err != nil {
		total = 0 // Don't fail on count error
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponse()
	}

	hasMore := len(messages) == limit

	return &models.MessageListResponse{
		Messages: messageResponses,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		HasMore:  hasMore,
	}, nil
}

// validateSendMessageRequest validates message sending request
func (uc *messageUsecase) validateSendMessageRequest(req *models.SendMessageRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.ChatID == 0 {
		return fmt.Errorf("chat_id is required")
	}

	if strings.TrimSpace(req.Content) == "" {
		return fmt.Errorf("content is required")
	}

	// Validate message type
	if req.Type != "" {
		validTypes := []models.MessageType{
			models.MessageTypeText,
			models.MessageTypeImage,
			models.MessageTypeFile,
			models.MessageTypeVideo,
			models.MessageTypeAudio,
			models.MessageTypeLocation,
			models.MessageTypeSystem,
		}

		valid := false
		for _, validType := range validTypes {
			if req.Type == validType {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("invalid message type")
		}
	}

	// Validate location data
	if req.Type == models.MessageTypeLocation {
		if req.Latitude == nil || req.Longitude == nil {
			return fmt.Errorf("latitude and longitude are required for location messages")
		}
	}

	// Validate file data for file types
	if req.Type == models.MessageTypeFile || req.Type == models.MessageTypeImage ||
		req.Type == models.MessageTypeVideo || req.Type == models.MessageTypeAudio {
		if req.FileURL == "" {
			return fmt.Errorf("file_url is required for file messages")
		}
	}

	return nil
}
