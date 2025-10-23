package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/chat/client"
	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/repository"

	"gorm.io/gorm"
)

// WebSocketHub interface to avoid circular dependency
type WebSocketHub interface {
	BroadcastToChat(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint)
	BroadcastToChatExcludeSender(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint)
}

type MessageUsecase interface {
	SendMessage(userID uint, req *models.SendMessageRequest) (*models.MessageResponse, error)
	GetMessages(userID uint, req *models.GetMessagesRequest) (*models.MessageListResponse, error)
	GetMessage(userID, messageID uint) (*models.MessageResponse, error)
	UpdateMessage(userID, messageID uint, req *models.UpdateMessageRequest) (*models.MessageResponse, error)
	DeleteMessage(userID, messageID uint) error
	DeleteMessageForUser(userID, messageID uint, deleteFor string) error
	ClearChatHistory(userID, chatID uint) error
	RestoreMessage(userID, messageID uint) error
	PinMessage(userID, messageID uint) (*models.MessageResponse, error)
	UnpinMessage(userID, messageID uint) (*models.MessageResponse, error)
	AddReaction(userID, messageID uint, req *models.AddReactionRequest) error
	RemoveReaction(userID, messageID uint, emoji string) error
	MarkAsRead(userID, messageID uint) error
	MarkChatAsRead(userID, chatID uint) error
	GetMessagesByChat(userID, chatID uint, limit, offset int) (*models.MessageListResponse, error)
	SetWebSocketHub(hub WebSocketHub)
}

// messageUsecase implements MessageUsecase interface
type messageUsecase struct {
	messageRepo repository.MessageRepository
	chatRepo    repository.ChatRepository
	wsHub       WebSocketHub
	fileClient  *client.FileClient
}

// NewMessageUsecase creates a new message usecase
func NewMessageUsecase(messageRepo repository.MessageRepository, chatRepo repository.ChatRepository) MessageUsecase {
	return &messageUsecase{
		messageRepo: messageRepo,
		chatRepo:    chatRepo,
		wsHub:       nil, // Will be set later to avoid circular dependency
		fileClient:  client.NewFileClient(),
	}
}

// SetWebSocketHub sets the WebSocket hub
func (uc *messageUsecase) SetWebSocketHub(hub WebSocketHub) {
	uc.wsHub = hub
	fmt.Println("✅ WebSocket hub set in MessageUsecase")
}

// Message Usecase Methods

// SendMessage sends a new message
func (uc *messageUsecase) SendMessage(userID uint, req *models.SendMessageRequest) (*models.MessageResponse, error) {
	// Debug: log request details
	fmt.Printf("📥 SendMessage request: ChatID=%d, Content='%s', FileIDs=%v (len=%d)\n",
		req.ChatID, req.Content, req.FileIDs, len(req.FileIDs))

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

	// Process file attachments if FileIDs are provided
	if len(req.FileIDs) > 0 {
		fmt.Printf("📎 Processing %d file attachments for message %d\n", len(req.FileIDs), message.ID)

		// Fetch file information from file-service
		fileInfos, err := uc.fileClient.GetMultipleFiles(req.FileIDs, userID)
		if err != nil {
			fmt.Printf("⚠️ Error fetching file info: %v\n", err)
		}

		// Create attachment records
		for _, fileInfo := range fileInfos {
			attachment := &models.MessageAttachment{
				MessageID:    message.ID,
				FileID:       fileInfo.ID,
				FileName:     fileInfo.OriginalName,
				FileSize:     fileInfo.FileSize,
				FileURL:      fileInfo.FileURL,
				ThumbnailURL: fileInfo.ThumbnailURL,
				MimeType:     fileInfo.MimeType,
				FileType:     fileInfo.FileType,
			}

			if err := uc.messageRepo.CreateAttachment(attachment); err != nil {
				fmt.Printf("⚠️ Failed to create attachment record for file %d: %v\n", fileInfo.ID, err)
				continue
			}

			fmt.Printf("✅ Created attachment record: file_id=%d, message_id=%d\n", fileInfo.ID, message.ID)
		}
	}

	// Get message with relations for response
	createdMessage, err := uc.messageRepo.GetWithReactions(message.ID)
	if err != nil {
		return message.ToResponse(), nil // Return what we have
	}

	response := createdMessage.ToResponse()

	// Debug: Check wsHub status before broadcast
	fmt.Printf("🔍 About to check wsHub - wsHub is nil: %v\n", uc.wsHub == nil)

	// Broadcast message to WebSocket clients
	if uc.wsHub != nil {
		fmt.Printf("📢 Broadcasting message ID %d to chat %d from user %d\n", response.ID, req.ChatID, userID)
		uc.wsHub.BroadcastToChat(req.ChatID, response, models.WSMessageTypeNewMessage, userID)
		fmt.Printf("✅ BroadcastToChat call completed for message %d\n", response.ID)
	} else {
		fmt.Println("❌ wsHub is nil - cannot broadcast!")
	}

	return response, nil
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

	fmt.Printf("📚 GetMessages request: ChatID=%d, Before=%d, After=%d, Limit=%d, Offset=%d\n",
		req.ChatID, req.Before, req.After, req.Limit, req.Offset)

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
			fmt.Printf("📖 Loading messages AFTER ID %d\n", req.After)
			messages, err = uc.messageRepo.GetMessagesAfter(req.ChatID, req.After, req.Limit)
			if err != nil {
				return nil, fmt.Errorf("failed to get messages: %w", err)
			}
			// Get total count for pagination
			total, err = uc.messageRepo.CountByChatID(req.ChatID)
			if err != nil {
				total = 0 // Don't fail on count error
			}
		} else if req.Before > 0 {
			fmt.Printf("📖 Loading messages BEFORE ID %d\n", req.Before)
			messages, err = uc.messageRepo.GetMessagesBefore(req.ChatID, req.Before, req.Limit)
			if err != nil {
				return nil, fmt.Errorf("failed to get messages: %w", err)
			}
			// Get total count for pagination
			total, err = uc.messageRepo.CountByChatID(req.ChatID)
			if err != nil {
				total = 0 // Don't fail on count error
			}
		} else {
			fmt.Printf("📖 Loading latest messages (no before/after) for user %d\n", userID)
			// Use the new method that excludes personally deleted messages
			messages, total, err = uc.messageRepo.GetByChatIDWithPaginationForUser(req.ChatID, userID, req.Limit, req.Offset)
			if err != nil {
				return nil, fmt.Errorf("failed to get messages: %w", err)
			}
		}

		fmt.Printf("✅ Retrieved %d messages\n", len(messages))
		if len(messages) > 0 {
			fmt.Printf("   First message ID: %d, Last message ID: %d\n", messages[0].ID, messages[len(messages)-1].ID)
		}
	} else {
		return nil, fmt.Errorf("chat_id is required")
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponse()
	}

	// Debug: Check if sender is in response
	if len(messageResponses) > 0 {
		firstMsg := messageResponses[0]
		fmt.Printf("🔍 First message response: ID=%d, SenderID=%d, Sender=%v (nil=%v)\n",
			firstMsg.ID, firstMsg.SenderID, firstMsg.Sender, firstMsg.Sender == nil)
		if firstMsg.Sender != nil {
			fmt.Printf("   Sender details: ID=%d, Name=%s\n", firstMsg.Sender.ID, firstMsg.Sender.Name)
		}
	}

	hasMore := len(messages) == req.Limit

	fmt.Printf("📦 Returning %d messages, hasMore=%v\n", len(messageResponses), hasMore)

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

	// Check if message is forwarded (starts with forwarding prefix)
	if strings.HasPrefix(message.Content, "📩 Переслано от ") {
		return nil, fmt.Errorf("cannot edit forwarded message")
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

// DeleteMessageForUser deletes a message with "delete_for" parameter
func (uc *messageUsecase) DeleteMessageForUser(userID, messageID uint, deleteFor string) error {
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

	if deleteFor == "everyone" {
		// Delete for everyone (soft delete) - only sender or admin/owner can do this
		if message.SenderID != userID {
			role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
			if err != nil {
				return fmt.Errorf("failed to get user role: %w", err)
			}
			if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
				return fmt.Errorf("insufficient permissions to delete message for everyone")
			}
		}

		// Soft delete the message
		if err := uc.messageRepo.Delete(messageID); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		// Broadcast deletion to WebSocket clients
		if uc.wsHub != nil {
			uc.wsHub.BroadcastToChat(message.ChatID, map[string]interface{}{
				"message_id": messageID,
			}, models.WSMessageTypeMessageDelete, userID)
		}
	} else if deleteFor == "me" {
		// Delete for this user only (personal deletion)
		if err := uc.messageRepo.AddMessageDeletion(messageID, userID); err != nil {
			return fmt.Errorf("failed to delete message for user: %w", err)
		}
	} else {
		return fmt.Errorf("invalid delete_for value: must be 'everyone' or 'me'")
	}

	return nil
}

// ClearChatHistory deletes all messages in a chat for the current user
func (uc *messageUsecase) ClearChatHistory(userID, chatID uint) error {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Clear all messages for this user
	if err := uc.messageRepo.ClearChatHistoryForUser(chatID, userID); err != nil {
		return fmt.Errorf("failed to clear chat history: %w", err)
	}

	return nil
}

// RestoreMessage restores a deleted message (admin only)
func (uc *messageUsecase) RestoreMessage(userID, messageID uint) error {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check if user is admin or owner of the chat
	role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return fmt.Errorf("only administrators can restore messages")
	}

	// Check if message is actually deleted
	if !message.IsDeleted {
		return fmt.Errorf("message is not deleted")
	}

	// Restore the message
	message.IsDeleted = false
	if err := uc.messageRepo.Update(message); err != nil {
		return fmt.Errorf("failed to restore message: %w", err)
	}

	// Broadcast restore to WebSocket clients as message edit
	if uc.wsHub != nil {
		response := message.ToResponse()
		uc.wsHub.BroadcastToChat(message.ChatID, response, models.WSMessageTypeMessageEdit, userID)
	}

	return nil
}

// PinMessage pins a message in chat
func (uc *messageUsecase) PinMessage(userID, messageID uint) (*models.MessageResponse, error) {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
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

	// Check if user has permission to pin (owner or admin)
	role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return nil, fmt.Errorf("only administrators can pin messages")
	}

	// Check if message is deleted
	if message.IsDeleted {
		return nil, fmt.Errorf("cannot pin deleted message")
	}

	// Check if already pinned
	if message.IsPinned {
		return nil, fmt.Errorf("message is already pinned")
	}

	// Pin the message
	message.IsPinned = true
	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to pin message: %w", err)
	}

	// Get updated message with relations
	pinnedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponse(), nil // Return what we have
	}

	response := pinnedMessage.ToResponse()

	// Broadcast pin to WebSocket clients
	if uc.wsHub != nil {
		uc.wsHub.BroadcastToChat(message.ChatID, response, models.WSMessageTypeMessageEdit, userID)
	}

	fmt.Printf("📌 Message %d pinned in chat %d by user %d\n", messageID, message.ChatID, userID)
	return response, nil
}

// UnpinMessage unpins a message in chat
func (uc *messageUsecase) UnpinMessage(userID, messageID uint) (*models.MessageResponse, error) {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
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

	// Check if user has permission to unpin (owner or admin)
	role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %w", err)
	}
	if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
		return nil, fmt.Errorf("only administrators can unpin messages")
	}

	// Check if already unpinned
	if !message.IsPinned {
		return nil, fmt.Errorf("message is not pinned")
	}

	// Unpin the message
	message.IsPinned = false
	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to unpin message: %w", err)
	}

	// Get updated message with relations
	unpinnedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponse(), nil // Return what we have
	}

	response := unpinnedMessage.ToResponse()

	// Broadcast unpin to WebSocket clients
	if uc.wsHub != nil {
		uc.wsHub.BroadcastToChat(message.ChatID, response, models.WSMessageTypeMessageEdit, userID)
	}

	fmt.Printf("📌 Message %d unpinned in chat %d by user %d\n", messageID, message.ChatID, userID)
	return response, nil
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

	// Broadcast read event to WebSocket clients
	if uc.wsHub != nil {
		uc.wsHub.BroadcastToChat(message.ChatID, map[string]interface{}{
			"message_id": messageID,
		}, models.WSMessageTypeRead, userID)
		fmt.Printf("📬 Message %d marked as read by user %d, broadcast sent\n", messageID, userID)
	}

	return nil
}

// MarkChatAsRead marks all messages in a chat as read
func (uc *messageUsecase) MarkChatAsRead(userID, chatID uint) error {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Mark all messages as read and get the list of marked message IDs
	messageIDs, err := uc.messageRepo.MarkAllAsRead(chatID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark all messages as read: %w", err)
	}

	// Broadcast read events to WebSocket clients for each message
	if uc.wsHub != nil && len(messageIDs) > 0 {
		for _, messageID := range messageIDs {
			uc.wsHub.BroadcastToChat(chatID, map[string]interface{}{
				"message_id": messageID,
			}, models.WSMessageTypeRead, userID)
		}
		fmt.Printf("📬 Chat %d: %d messages marked as read by user %d, broadcasts sent\n", chatID, len(messageIDs), userID)
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

	// Use the new method that excludes personally deleted messages
	messages, total, err := uc.messageRepo.GetByChatIDWithPaginationForUser(chatID, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
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

	// Content is required unless files are attached
	if strings.TrimSpace(req.Content) == "" && len(req.FileIDs) == 0 {
		return fmt.Errorf("content or file attachments are required")
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
