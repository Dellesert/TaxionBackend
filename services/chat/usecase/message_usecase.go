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

// messageUsecase implements MessageUsecase interface
type messageUsecase struct {
	messageRepo repository.MessageRepository
	chatRepo    repository.ChatRepository
}

// NewMessageUsecase creates a new message usecase
func NewMessageUsecase(messageRepo repository.MessageRepository, chatRepo repository.ChatRepository) MessageUsecase {
	return &messageUsecase{
		messageRepo: messageRepo,
		chatRepo:    chatRepo,
	}
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
