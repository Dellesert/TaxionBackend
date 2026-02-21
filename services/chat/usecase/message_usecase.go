package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"tachyon-messenger/services/chat/client"
	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/repository"
	chatutils "tachyon-messenger/services/chat/utils"
	searchclient "tachyon-messenger/services/search/client"

	"gorm.io/gorm"
)

// WebSocketHub interface to avoid circular dependency
type WebSocketHub interface {
	BroadcastToChat(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint)
	BroadcastToChatExcludeSender(chatID uint, data interface{}, msgType models.WSMessageType, senderID uint)
	GetChatRoomUsers(chatID uint) []uint
}

type MessageUsecase interface {
	SendMessage(userID uint, req *models.SendMessageRequest) (*models.MessageResponse, error)
	GetMessages(userID uint, req *models.GetMessagesRequest) (*models.MessageListResponse, error)
	GetMessage(userID, messageID uint) (*models.MessageResponse, error)
	UpdateMessage(userID, messageID uint, req *models.UpdateMessageRequest) (*models.MessageResponse, error)
	DeleteMessage(userID, messageID uint) error
	DeleteMessageForUser(userID, messageID uint, deleteFor string) error
	DeleteAttachment(userID, messageID, attachmentID uint) error
	BulkDeleteMessages(userID uint, req *models.BulkDeleteMessagesRequest) error
	BulkForwardMessages(userID uint, req *models.BulkForwardMessagesRequest) (*models.BulkForwardMessagesResponse, error)
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

	// New refactored API methods
	GetLatestMessages(userID, chatID uint, req *models.GetLatestMessagesRequest) (*models.GetLatestMessagesResponse, error)
	GetMessagesBeforeID(userID, chatID, beforeID uint, req *models.GetMessagesBeforeRequest) (*models.GetMessagesBeforeResponse, error)
	GetMessagesAfterID(userID, chatID, afterID uint, req *models.GetMessagesAfterRequest) (*models.GetMessagesAfterResponse, error)
	GetMessageContext(userID, chatID, targetMessageID uint, req *models.GetMessageContextRequest) (*models.GetMessageContextResponse, error)

	// Search messages
	SearchMessages(userID, chatID uint, req *models.SearchMessagesRequest) (*models.SearchMessagesResponse, error)

	// Thread operations (for channel comments)
	GetThreadMessages(userID, chatID, threadRootID uint, limit int, afterID uint) (*models.GetThreadMessagesResponse, error)
}

// messageUsecase implements MessageUsecase interface
type messageUsecase struct {
	messageRepo        repository.MessageRepository
	chatRepo           repository.ChatRepository
	wsHub              WebSocketHub
	fileClient         *client.FileClient
	notificationClient *client.NotificationClient
	searchClient       *searchclient.SearchClient
	baseURL            string
}

// NewMessageUsecase creates a new message usecase
func NewMessageUsecase(messageRepo repository.MessageRepository, chatRepo repository.ChatRepository, notificationClient *client.NotificationClient) MessageUsecase {
	// Get base URL from environment
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &messageUsecase{
		messageRepo:        messageRepo,
		chatRepo:           chatRepo,
		wsHub:              nil, // Will be set later to avoid circular dependency
		fileClient:         client.NewFileClient(),
		notificationClient: notificationClient,
		searchClient:       searchclient.NewSearchClient(),
		baseURL:            baseURL,
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

	// Check channel posting permissions
	chat, err := uc.chatRepo.GetByID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}
	if chat.Type == models.ChatTypeChannel {
		if req.ThreadRootID == nil {
			// Root-level post in channel: only admin/owner
			canPost, err := uc.chatRepo.HasChannelPostAccess(req.ChatID, userID)
			if err != nil {
				return nil, fmt.Errorf("failed to check channel post access: %w", err)
			}
			if !canPost {
				return nil, fmt.Errorf("only admins can post in channels")
			}
		} else {
			// Thread reply: any member can comment
			rootMsg, err := uc.messageRepo.GetByID(*req.ThreadRootID)
			if err != nil {
				return nil, fmt.Errorf("thread root message not found")
			}
			if rootMsg.ChatID != req.ChatID {
				return nil, fmt.Errorf("thread root message is not in this chat")
			}
			if rootMsg.ThreadRootID != nil {
				return nil, fmt.Errorf("cannot create nested threads")
			}
		}
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

	// Handle forward message
	var forwardedFromMessageID *uint
	var originalSenderID *uint
	var isForwarded bool

	if req.ForwardFromMessageID != nil {
		// Get the original message being forwarded
		originalMsg, err := uc.messageRepo.GetByID(*req.ForwardFromMessageID)
		if err != nil {
			return nil, fmt.Errorf("forwarded message not found")
		}

		forwardedFromMessageID = req.ForwardFromMessageID
		originalSenderID = &originalMsg.SenderID
		isForwarded = true

		// If content is not provided, copy from original message
		if strings.TrimSpace(req.Content) == "" {
			req.Content = originalMsg.Content
		}

		// Copy message type from original if not specified
		if req.Type == "" {
			req.Type = originalMsg.Type
		}

		// Copy file-related fields from original message if not provided
		if req.FileName == "" {
			req.FileName = originalMsg.FileName
		}
		if req.FileSize == 0 {
			req.FileSize = originalMsg.FileSize
		}
		if req.FileURL == "" {
			req.FileURL = originalMsg.FileURL
		}
		if req.ThumbnailURL == "" {
			req.ThumbnailURL = originalMsg.ThumbnailURL
		}
		if req.MimeType == "" {
			req.MimeType = originalMsg.MimeType
		}
		if req.Latitude == nil {
			req.Latitude = originalMsg.Latitude
		}
		if req.Longitude == nil {
			req.Longitude = originalMsg.Longitude
		}
	}

	// Create message
	message := &models.Message{
		ChatID:                 req.ChatID,
		SenderID:               userID,
		Content:                strings.TrimSpace(req.Content),
		Type:                   req.Type,
		Status:                 models.MessageStatusSent,
		ReplyToID:              req.ReplyToID,
		FileName:               req.FileName,
		FileSize:               req.FileSize,
		FileURL:                req.FileURL,
		ThumbnailURL:           req.ThumbnailURL,
		MimeType:               req.MimeType,
		Latitude:               req.Latitude,
		Longitude:              req.Longitude,
		ForwardedFromMessageID: forwardedFromMessageID,
		OriginalSenderID:       originalSenderID,
		IsForwarded:            isForwarded,
		ThreadRootID:           req.ThreadRootID,
	}

	// Set default type if not provided
	if message.Type == "" {
		message.Type = models.MessageTypeText
	}

	// Handle poll_data if provided
	if req.PollData != nil && len(req.PollData) > 0 {
		pollDataJSON, err := json.Marshal(req.PollData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal poll_data: %w", err)
		}
		message.PollData = string(pollDataJSON)
		fmt.Printf("📊 Poll data attached to message: %s\n", string(pollDataJSON))
	}

	if err := uc.messageRepo.Create(message); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Unhide the chat for all members when a new regular message is sent (not thread comments)
	// This ensures that hidden/deleted chats reappear when receiving new messages
	// Thread comments should not unhide/bump chats in the chat list
	var memberIDs []uint
	if req.ThreadRootID == nil {
		var mErr error
		memberIDs, mErr = uc.chatRepo.GetChatMemberIDs(req.ChatID)
		if mErr != nil {
			fmt.Printf("⚠️ Failed to get chat members to unhide chat: %v\n", mErr)
		} else {
			for _, memberID := range memberIDs {
				if err := uc.chatRepo.UpdateHiddenStatus(req.ChatID, memberID, false); err != nil {
					fmt.Printf("⚠️ Failed to unhide chat %d for user %d: %v\n", req.ChatID, memberID, err)
				}
			}
			fmt.Printf("✅ Chat %d unhidden for %d members\n", req.ChatID, len(memberIDs))
		}
	} else {
		// For thread comments, still get member IDs for search indexing
		var mErr error
		memberIDs, mErr = uc.chatRepo.GetChatMemberIDs(req.ChatID)
		if mErr != nil {
			fmt.Printf("⚠️ Failed to get chat members for thread comment: %v\n", mErr)
		}
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
				MessageID:          message.ID,
				FileID:             fileInfo.ID,
				FileName:           fileInfo.OriginalName,
				FileSize:           fileInfo.FileSize,
				FileURL:            fileInfo.FileURL,
				ThumbnailURL:       fileInfo.ThumbnailURL,
				ThumbnailSmallURL:  fileInfo.ThumbnailSmallURL,
				ThumbnailMediumURL: fileInfo.ThumbnailMediumURL,
				ThumbnailLargeURL:  fileInfo.ThumbnailLargeURL,
				MimeType:           fileInfo.MimeType,
				FileType:           fileInfo.FileType,
				Duration:           fileInfo.Duration,
				Width:              fileInfo.Width,
				Height:             fileInfo.Height,
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
		return message.ToResponseForUser(userID, uc.baseURL), nil // Return what we have
	}

	// Use ToResponse() for API response (shows content for sender)
	response := createdMessage.ToResponseForUser(userID, uc.baseURL)

	// Debug: Check message content
	fmt.Printf("🔍 Message created - ID: %d, Content: %q, IsDeleted: %v, SenderID: %d, ViewerID: %d\n",
		createdMessage.ID, createdMessage.Content, createdMessage.IsDeleted, createdMessage.SenderID, userID)
	fmt.Printf("🔍 Response content: %q\n", response.Content)

	// Debug: Check wsHub status before broadcast
	fmt.Printf("🔍 About to check wsHub - wsHub is nil: %v\n", uc.wsHub == nil)

	// Broadcast message to WebSocket clients with is_latest flag
	if uc.wsHub != nil {
		// IMPORTANT: Use ToResponse() without user filtering for WebSocket broadcast
		// Each client will decide what to show based on their own permissions
		broadcastResponse := createdMessage.ToResponse(uc.baseURL)
		fmt.Printf("📢 Broadcasting message ID %d to chat %d from user %d (content: %q)\n",
			broadcastResponse.ID, req.ChatID, userID, broadcastResponse.Content)

		if message.ThreadRootID != nil {
			// Thread message: broadcast as thread event
			wsData := models.WSNewMessageData{
				Message:  *broadcastResponse,
				IsLatest: true,
			}
			uc.wsHub.BroadcastToChat(req.ChatID, wsData, models.WSMessageTypeNewThreadMessage, userID)

			// Also broadcast updated root message with new thread_reply_count
			rootMsg, rootErr := uc.messageRepo.GetWithReactions(*message.ThreadRootID)
			if rootErr == nil && rootMsg != nil {
				rootResponse := rootMsg.ToResponse(uc.baseURL)
				uc.wsHub.BroadcastToChat(req.ChatID, rootResponse, models.WSMessageTypeThreadUpdate, userID)
			}
		} else {
			// Regular message: broadcast as new_message
			wsData := models.WSNewMessageData{
				Message:  *broadcastResponse,
				IsLatest: true,
			}
			uc.wsHub.BroadcastToChat(req.ChatID, wsData, models.WSMessageTypeNewMessage, userID)
		}
		fmt.Printf("✅ BroadcastToChat call completed for message %d\n", response.ID)
	} else {
		fmt.Println("❌ wsHub is nil - cannot broadcast!")
	}

	// Send notifications
	go func() {
		if err := uc.sendMessageNotifications(userID, req.ChatID, message, response); err != nil {
			fmt.Printf("Failed to send message notifications: %v\n", err)
		}
	}()

	// Index message in search service (only text messages)
	if message.Type == models.MessageTypeText && strings.TrimSpace(message.Content) != "" {
		uc.indexMessageInSearch(message, memberIDs)
	}

	// Async link preview fetch for text messages with URLs
	if message.Type == models.MessageTypeText && strings.TrimSpace(message.Content) != "" {
		if firstURL := chatutils.ExtractFirstURL(message.Content); firstURL != "" {
			go uc.fetchAndSaveLinkPreview(message.ID, req.ChatID, userID, firstURL)
		}
	}

	return response, nil
}

// fetchAndSaveLinkPreview fetches Open Graph metadata for a URL and updates the message
func (uc *messageUsecase) fetchAndSaveLinkPreview(messageID, chatID, senderID uint, url string) {
	preview, err := chatutils.FetchLinkPreview(url)
	if err != nil {
		fmt.Printf("🔗 Link preview fetch failed for %s: %v\n", url, err)
		return
	}

	previewJSON, err := json.Marshal(preview)
	if err != nil {
		fmt.Printf("🔗 Failed to marshal link preview: %v\n", err)
		return
	}

	// Update message with link preview data
	msg, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		fmt.Printf("🔗 Failed to get message %d for link preview update: %v\n", messageID, err)
		return
	}

	msg.LinkPreviewData = string(previewJSON)
	if err := uc.messageRepo.Update(msg); err != nil {
		fmt.Printf("🔗 Failed to save link preview for message %d: %v\n", messageID, err)
		return
	}

	fmt.Printf("🔗 Link preview saved for message %d: %s\n", messageID, preview.Title)

	// Notify clients via WebSocket about the updated message
	if uc.wsHub != nil {
		updatedMsg, err := uc.messageRepo.GetWithReactions(messageID)
		if err == nil {
			broadcastResponse := updatedMsg.ToResponse(uc.baseURL)
			uc.wsHub.BroadcastToChat(chatID, broadcastResponse, models.WSMessageTypeMessageEdit, senderID)
		}
	}
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
			messages, err = uc.messageRepo.GetMessagesAfter(req.ChatID, userID, req.After, req.Limit)
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
			messages, err = uc.messageRepo.GetMessagesBefore(req.ChatID, userID, req.Before, req.Limit)
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
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
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

	return message.ToResponseForUser(userID, uc.baseURL), nil
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

	// Check if message is forwarded
	if message.IsForwarded {
		return nil, fmt.Errorf("cannot edit forwarded message")
	}

	// Update message
	newContent := strings.TrimSpace(req.Content)
	message.Content = newContent
	message.IsEdited = true
	now := time.Now()
	message.EditedAt = &now

	// Clear old link preview — will be re-fetched if new content has a URL
	message.LinkPreviewData = ""

	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to update message: %w", err)
	}

	// Async link preview fetch if edited content has a URL
	if message.Type == models.MessageTypeText && newContent != "" {
		if firstURL := chatutils.ExtractFirstURL(newContent); firstURL != "" {
			go uc.fetchAndSaveLinkPreview(message.ID, message.ChatID, userID, firstURL)
		}
	}

	// Get updated message with relations
	updatedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponseForUser(userID, uc.baseURL), nil // Return what we have
	}

	response := updatedMessage.ToResponseForUser(userID, uc.baseURL)

	// Re-index message in search service
	if message.Type == models.MessageTypeText {
		chatMemberIDs, _ := uc.chatRepo.GetChatMemberIDs(message.ChatID)
		uc.indexMessageInSearch(message, chatMemberIDs)
	}

	// Broadcast message edit to WebSocket clients
	if uc.wsHub != nil {
		// For WebSocket, send version without user-specific filtering (viewerID=0)
		// so all clients see the same data. Deleted message content will be empty.
		broadcastResponse := updatedMessage.ToResponse(uc.baseURL)
		uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageEdit, userID)
	}

	return response, nil
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

	// Remove message from search index
	uc.searchClient.DeleteDocument("message", messageID)

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

		// Broadcast deletion to WebSocket clients with full message (content will be hidden)
		if uc.wsHub != nil {
			// Get updated message with is_deleted=true
			deletedMessage, err := uc.messageRepo.GetWithReactions(messageID)
			if err == nil {
				// Use ToResponse() to hide content for everyone
				broadcastResponse := deletedMessage.ToResponse(uc.baseURL)
				uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageDelete, userID)
			} else {
				// Fallback to old behavior if we can't get the message
				uc.wsHub.BroadcastToChat(message.ChatID, map[string]interface{}{
					"message_id": messageID,
				}, models.WSMessageTypeMessageDelete, userID)
			}
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

// DeleteAttachment deletes a single attachment from a message.
// If it's the last attachment and the message has no text content, the whole message is deleted.
func (uc *messageUsecase) DeleteAttachment(userID, messageID, attachmentID uint) error {
	// Get message
	message, err := uc.messageRepo.GetByID(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Check membership
	isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("user is not a member of this chat")
	}

	// Check permissions: sender or admin/owner
	if message.SenderID != userID {
		role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
		if err != nil {
			return fmt.Errorf("failed to get user role: %w", err)
		}
		if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
			return fmt.Errorf("insufficient permissions to delete attachment")
		}
	}

	// Verify attachment belongs to this message
	attachment, err := uc.messageRepo.GetAttachmentByID(attachmentID)
	if err != nil {
		return fmt.Errorf("attachment not found")
	}
	if attachment.MessageID != messageID {
		return fmt.Errorf("attachment does not belong to this message")
	}

	// Count remaining attachments
	count, err := uc.messageRepo.CountAttachmentsByMessageID(messageID)
	if err != nil {
		return fmt.Errorf("failed to count attachments: %w", err)
	}

	if count <= 1 && strings.TrimSpace(message.Content) == "" {
		// Last attachment and no text content — delete the whole message
		if err := uc.messageRepo.Delete(messageID); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		// Broadcast message deletion
		if uc.wsHub != nil {
			deletedMessage, err := uc.messageRepo.GetWithReactions(messageID)
			if err == nil {
				broadcastResponse := deletedMessage.ToResponse(uc.baseURL)
				uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageDelete, userID)
			}
		}
	} else {
		// Remove only this attachment
		if err := uc.messageRepo.DeleteAttachment(attachmentID); err != nil {
			return fmt.Errorf("failed to delete attachment: %w", err)
		}

		// Broadcast updated message so clients refresh
		if uc.wsHub != nil {
			updatedMessage, err := uc.messageRepo.GetWithReactions(messageID)
			if err == nil {
				broadcastResponse := updatedMessage.ToResponse(uc.baseURL)
				uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageEdit, userID)
			}
		}
	}

	return nil
}

// BulkDeleteMessages deletes multiple messages at once
func (uc *messageUsecase) BulkDeleteMessages(userID uint, req *models.BulkDeleteMessagesRequest) error {
	if len(req.MessageIDs) == 0 {
		return fmt.Errorf("message_ids cannot be empty")
	}

	if len(req.MessageIDs) > 100 {
		return fmt.Errorf("cannot delete more than 100 messages at once")
	}

	// Default to "everyone" if not specified
	deleteFor := req.DeleteFor
	if deleteFor == "" {
		deleteFor = "everyone"
	}

	// Validate deleteFor parameter
	if deleteFor != "everyone" && deleteFor != "me" {
		return fmt.Errorf("invalid delete_for value: must be 'everyone' or 'me'")
	}

	fmt.Printf("🗑️ Bulk deleting %d messages for user %d (delete_for=%s)\n", len(req.MessageIDs), userID, deleteFor)

	// Track successful and failed deletions
	successCount := 0
	failedCount := 0
	var firstError error

	// Group messages by chat for efficient WebSocket broadcasting
	messagesByChatID := make(map[uint][]uint)

	for _, messageID := range req.MessageIDs {
		// Get message to check permissions and chat
		message, err := uc.messageRepo.GetByID(messageID)
		if err != nil {
			fmt.Printf("⚠️ Message %d not found: %v\n", messageID, err)
			failedCount++
			if firstError == nil {
				firstError = fmt.Errorf("message %d not found", messageID)
			}
			continue
		}

		// Check if user is a member of the chat
		isMember, err := uc.chatRepo.IsMember(message.ChatID, userID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check membership for message %d: %v\n", messageID, err)
			failedCount++
			if firstError == nil {
				firstError = fmt.Errorf("failed to check membership for message %d", messageID)
			}
			continue
		}
		if !isMember {
			fmt.Printf("⚠️ User %d is not a member of chat %d (message %d)\n", userID, message.ChatID, messageID)
			failedCount++
			if firstError == nil {
				firstError = fmt.Errorf("user is not a member of chat for message %d", messageID)
			}
			continue
		}

		if deleteFor == "everyone" {
			// Check if user has permission to delete for everyone
			if message.SenderID != userID {
				role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
				if err != nil {
					fmt.Printf("⚠️ Failed to get user role for message %d: %v\n", messageID, err)
					failedCount++
					if firstError == nil {
						firstError = fmt.Errorf("failed to get user role for message %d", messageID)
					}
					continue
				}
				if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
					fmt.Printf("⚠️ User %d has insufficient permissions to delete message %d for everyone\n", userID, messageID)
					failedCount++
					if firstError == nil {
						firstError = fmt.Errorf("insufficient permissions to delete message %d for everyone", messageID)
					}
					continue
				}
			}

			// Soft delete the message
			if err := uc.messageRepo.Delete(messageID); err != nil {
				fmt.Printf("⚠️ Failed to delete message %d: %v\n", messageID, err)
				failedCount++
				if firstError == nil {
					firstError = fmt.Errorf("failed to delete message %d", messageID)
				}
				continue
			}

			// Track for WebSocket broadcast
			messagesByChatID[message.ChatID] = append(messagesByChatID[message.ChatID], messageID)
		} else if deleteFor == "me" {
			// Delete for this user only (personal deletion)
			if err := uc.messageRepo.AddMessageDeletion(messageID, userID); err != nil {
				fmt.Printf("⚠️ Failed to delete message %d for user: %v\n", messageID, err)
				failedCount++
				if firstError == nil {
					firstError = fmt.Errorf("failed to delete message %d for user", messageID)
				}
				continue
			}
		}

		successCount++
	}

	// Broadcast deletions to WebSocket clients (only for "everyone" deletions)
	if deleteFor == "everyone" && uc.wsHub != nil {
		for chatID, messageIDs := range messagesByChatID {
			for _, messageID := range messageIDs {
				// Get updated message with is_deleted=true and send full message
				deletedMessage, err := uc.messageRepo.GetWithReactions(messageID)
				if err == nil {
					// Use ToResponse() to hide content for everyone
					broadcastResponse := deletedMessage.ToResponse(uc.baseURL)
					uc.wsHub.BroadcastToChat(chatID, broadcastResponse, models.WSMessageTypeMessageDelete, userID)
				} else {
					// Fallback to old behavior if we can't get the message
					uc.wsHub.BroadcastToChat(chatID, map[string]interface{}{
						"message_id": messageID,
					}, models.WSMessageTypeMessageDelete, userID)
				}
			}
			fmt.Printf("📢 Broadcasted deletion of %d messages in chat %d\n", len(messageIDs), chatID)
		}
	}

	fmt.Printf("✅ Bulk delete completed: %d succeeded, %d failed\n", successCount, failedCount)

	// Return error only if all deletions failed
	if successCount == 0 && failedCount > 0 {
		return fmt.Errorf("all message deletions failed: %w", firstError)
	}

	return nil
}

// BulkForwardMessages forwards multiple messages to another chat
func (uc *messageUsecase) BulkForwardMessages(userID uint, req *models.BulkForwardMessagesRequest) (*models.BulkForwardMessagesResponse, error) {
	if len(req.MessageIDs) == 0 {
		return nil, fmt.Errorf("message_ids cannot be empty")
	}

	if len(req.MessageIDs) > 100 {
		return nil, fmt.Errorf("cannot forward more than 100 messages at once")
	}

	// Check if user is a member of the target chat
	isMember, err := uc.chatRepo.IsMember(req.TargetChatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership in target chat: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of the target chat")
	}

	fmt.Printf("📤 Bulk forwarding %d messages to chat %d by user %d\n", len(req.MessageIDs), req.TargetChatID, userID)

	response := &models.BulkForwardMessagesResponse{
		ForwardedMessages: make([]models.MessageResponse, 0),
		FailedMessageIDs:  make([]uint, 0),
	}

	// Forward each message in order
	for _, messageID := range req.MessageIDs {
		// Get the original message with attachments
		originalMsg, err := uc.messageRepo.GetWithReactions(messageID)
		if err != nil {
			fmt.Printf("⚠️ Message %d not found: %v\n", messageID, err)
			response.FailedMessageIDs = append(response.FailedMessageIDs, messageID)
			response.TotalFailed++
			continue
		}

		// Check if user has access to the original message (is member of the source chat)
		isMemberSource, err := uc.chatRepo.IsMember(originalMsg.ChatID, userID)
		if err != nil || !isMemberSource {
			fmt.Printf("⚠️ User %d is not a member of source chat %d (message %d)\n", userID, originalMsg.ChatID, messageID)
			response.FailedMessageIDs = append(response.FailedMessageIDs, messageID)
			response.TotalFailed++
			continue
		}

		// Extract FileIDs from attachments
		var fileIDs []uint
		if len(originalMsg.Attachments) > 0 {
			fileIDs = make([]uint, len(originalMsg.Attachments))
			for i, att := range originalMsg.Attachments {
				fileIDs[i] = att.FileID
			}
			fmt.Printf("📎 Message %d has %d attachments with FileIDs: %v\n", messageID, len(fileIDs), fileIDs)
		}

		// Create forward request for SendMessage
		forwardReq := &models.SendMessageRequest{
			ChatID:               req.TargetChatID,
			ForwardFromMessageID: &messageID,
			FileIDs:              fileIDs,
		}

		// Send the forwarded message
		forwardedMsg, err := uc.SendMessage(userID, forwardReq)
		if err != nil {
			fmt.Printf("⚠️ Failed to forward message %d: %v\n", messageID, err)
			response.FailedMessageIDs = append(response.FailedMessageIDs, messageID)
			response.TotalFailed++
			continue
		}

		response.ForwardedMessages = append(response.ForwardedMessages, *forwardedMsg)
		response.TotalForwarded++
	}

	fmt.Printf("✅ Bulk forward completed: %d forwarded, %d failed\n", response.TotalForwarded, response.TotalFailed)

	// Return error only if all forwards failed
	if response.TotalForwarded == 0 && response.TotalFailed > 0 {
		return response, fmt.Errorf("all message forwards failed")
	}

	return response, nil
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

	// Clear last_message_at in the chat so it doesn't show stale data
	if err := uc.chatRepo.ClearLastMessage(chatID); err != nil {
		fmt.Printf("⚠️ Failed to clear last message for chat %d: %v\n", chatID, err)
		// Don't fail the operation if this fails
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
		// Re-fetch message with all preloads (attachments, reactions, etc.)
		fullMessage, err := uc.messageRepo.GetWithReactions(messageID)
		if err != nil {
			return fmt.Errorf("failed to get full message for broadcast: %w", err)
		}
		response := fullMessage.ToResponse(uc.baseURL)
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

	// Get chat to check type
	chat, err := uc.chatRepo.GetByID(message.ChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Check if user has permission to pin
	// In private chats, any member can pin messages
	// In group chats, only owner or admin can pin messages
	if chat.Type != models.ChatTypePrivate {
		role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user role: %w", err)
		}
		if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
			return nil, fmt.Errorf("only administrators can pin messages in group chats")
		}
	}

	// Check if message is deleted
	if message.IsDeleted {
		return nil, fmt.Errorf("cannot pin deleted message")
	}

	// Check if already pinned
	if message.IsPinned {
		return nil, fmt.Errorf("message is already pinned")
	}

	// Pin the message - only update IsPinned field to preserve read_receipts
	message.IsPinned = true
	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to pin message: %w", err)
	}

	// Get updated message with relations
	pinnedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponseForUser(userID, uc.baseURL), nil // Return what we have
	}

	response := pinnedMessage.ToResponseForUser(userID, uc.baseURL)

	// Broadcast pin to WebSocket clients
	if uc.wsHub != nil {
		// For WebSocket, use ToResponse() which hides deleted message content for everyone
		broadcastResponse := pinnedMessage.ToResponse(uc.baseURL)
		uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageEdit, userID)
	}

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

	// Get chat to check type
	chat, err := uc.chatRepo.GetByID(message.ChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Check if user has permission to unpin
	// In private chats, any member can unpin messages
	// In group chats, only owner or admin can unpin messages
	if chat.Type != models.ChatTypePrivate {
		role, err := uc.chatRepo.GetMemberRole(message.ChatID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user role: %w", err)
		}
		if role != models.ChatMemberRoleOwner && role != models.ChatMemberRoleAdmin {
			return nil, fmt.Errorf("only administrators can unpin messages in group chats")
		}
	}

	// Check if already unpinned
	if !message.IsPinned {
		return nil, fmt.Errorf("message is not pinned")
	}

	// Unpin the message - only update IsPinned field to preserve read_receipts
	message.IsPinned = false
	if err := uc.messageRepo.Update(message); err != nil {
		return nil, fmt.Errorf("failed to unpin message: %w", err)
	}

	// Get updated message with relations
	unpinnedMessage, err := uc.messageRepo.GetWithReactions(messageID)
	if err != nil {
		return message.ToResponseForUser(userID, uc.baseURL), nil // Return what we have
	}

	response := unpinnedMessage.ToResponseForUser(userID, uc.baseURL)

	// Broadcast unpin to WebSocket clients
	if uc.wsHub != nil {
		// For WebSocket, use ToResponse() which hides deleted message content for everyone
		broadcastResponse := unpinnedMessage.ToResponse(uc.baseURL)
		uc.wsHub.BroadcastToChat(message.ChatID, broadcastResponse, models.WSMessageTypeMessageEdit, userID)
	}

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

	// Broadcast reaction to WebSocket clients
	if uc.wsHub != nil {
		data := map[string]interface{}{
			"message_id": messageID,
			"emoji":      strings.TrimSpace(req.Emoji),
			"action":     "added",
		}
		if reaction.User != nil {
			data["user"] = reaction.User
		}
		uc.wsHub.BroadcastToChatExcludeSender(message.ChatID, data, models.WSMessageTypeReaction, userID)
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

	// Broadcast reaction removal to WebSocket clients
	if uc.wsHub != nil {
		uc.wsHub.BroadcastToChatExcludeSender(message.ChatID, map[string]interface{}{
			"message_id": messageID,
			"emoji":      emoji,
			"action":     "removed",
		}, models.WSMessageTypeReaction, userID)
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

	// Mark related message notifications as read (async, don't block on errors)
	if uc.notificationClient != nil {
		go func() {
			if err := uc.notificationClient.MarkNotificationsReadByChatID(userID, chatID); err != nil {
				fmt.Printf("⚠️ Failed to mark notifications as read for chat %d, user %d: %v\n", chatID, userID, err)
			}
		}()
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
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
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

	// Content is required unless files are attached or forwarding a message
	if strings.TrimSpace(req.Content) == "" && len(req.FileIDs) == 0 && req.ForwardFromMessageID == nil {
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
			models.MessageTypePoll,
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


// isUserActiveInChat checks if user is actively viewing the chat (connected via WebSocket and in the chat room)
func (uc *messageUsecase) isUserActiveInChat(userID, chatID uint) bool {
	if uc.wsHub == nil {
		fmt.Printf("🔍 [isUserActiveInChat] wsHub is nil for user %d in chat %d\n", userID, chatID)
		return false
	}

	// Get list of users currently in the chat room (actively viewing)
	activeUsers := uc.wsHub.GetChatRoomUsers(chatID)
	fmt.Printf("🔍 [isUserActiveInChat] Active users in chat %d: %v (checking for user %d)\n", chatID, activeUsers, userID)

	for _, id := range activeUsers {
		if id == userID {
			fmt.Printf("✅ [isUserActiveInChat] User %d IS active in chat %d\n", userID, chatID)
			return true
		}
	}
	fmt.Printf("❌ [isUserActiveInChat] User %d is NOT active in chat %d\n", userID, chatID)
	return false
}

// sendMessageNotifications sends notifications to chat members about new message
func (uc *messageUsecase) sendMessageNotifications(senderID, chatID uint, message *models.Message, response *models.MessageResponse) error {
	// Get chat information
	chat, err := uc.chatRepo.GetByID(chatID)
	if err != nil {
		return fmt.Errorf("failed to get chat: %w", err)
	}

	// Skip notifications for thread comments in channels (channel post comments)
	if chat.Type == models.ChatTypeChannel && message.ThreadRootID != nil {
		return nil
	}

	// Get all chat members
	memberIDs, err := uc.chatRepo.GetChatMemberIDs(chatID)
	if err != nil {
		return fmt.Errorf("failed to get chat members: %w", err)
	}

	// Get sender information for notification
	var senderName string
	if response.Sender != nil {
		senderName = response.Sender.Name
	} else {
		senderName = "Кто-то"
	}

	// Truncate message content for notification (max 100 characters)
	messageContent := message.Content
	if len(messageContent) > 100 {
		messageContent = messageContent[:97] + "..."
	}

	// Determine notification type based on chat type and message type
	var notificationTitle string
	var notificationMessage string
	priority := "medium"

	// Check if this is a reply to a message
	isReply := message.ReplyToID != nil
	var replyToAuthorID uint

	if isReply {
		// Get the original message to find its author
		originalMsg, err := uc.messageRepo.GetByID(*message.ReplyToID)
		if err == nil {
			replyToAuthorID = originalMsg.SenderID
			// Set high priority for replies
			priority = "high"
		}
	}

	switch chat.Type {
	case models.ChatTypePrivate:
		// Personal message
		notificationTitle = fmt.Sprintf("📩 %s", senderName)
		notificationMessage = messageContent
		priority = "high" // Personal messages are always high priority

	case models.ChatTypeGroup, models.ChatTypeChannel:
		// Group message
		chatName := chat.Name
		if chatName == "" {
			chatName = "Групповой чат"
		}

		if isReply {
			notificationTitle = fmt.Sprintf("💬 %s ответил в \"%s\"", senderName, chatName)
		} else {
			notificationTitle = fmt.Sprintf("👥 %s в \"%s\"", senderName, chatName)
		}
		notificationMessage = messageContent
	}

	// Send notifications to all members except sender
	for _, memberID := range memberIDs {
		// Skip sender
		if memberID == senderID {
			continue
		}

		// If this is a reply, only notify the original author (and skip others in group chats)
		if isReply && chat.Type != models.ChatTypePrivate {
			// In group chats, only notify the person being replied to
			if memberID != replyToAuthorID {
				continue
			}
			// Update title for reply notification
			notificationTitle = "💬 Ответ на ваше сообщение"
		}

		// Check per-chat mute
		memberMutedUntil, err := uc.chatRepo.GetMemberMutedUntil(chatID, memberID)
		if err == nil && models.IsMutedUntil(memberMutedUntil) {
			fmt.Printf("🔇 User %d has chat %d muted until %v - skipping notification\n", memberID, chatID, memberMutedUntil)
			continue
		}

		// Check global mute preferences (channels and groups)
		if chat.Type == models.ChatTypeChannel || chat.Type == models.ChatTypeGroup {
			globalPref, err := uc.chatRepo.GetUserMutePreference(memberID)
			if err == nil && globalPref != nil {
				if chat.Type == models.ChatTypeChannel && models.IsMutedUntil(globalPref.MuteAllChannelsUntil) {
					fmt.Printf("🔇 User %d has all channels muted - skipping notification\n", memberID)
					continue
				}
				if chat.Type == models.ChatTypeGroup && models.IsMutedUntil(globalPref.MuteAllGroupsUntil) {
					fmt.Printf("🔇 User %d has all groups muted - skipping notification\n", memberID)
					continue
				}
			}
		}

		// If user is actively viewing this chat, skip PUSH notification but keep in_app
		channels := []string{"in_app", "push"}
		if uc.isUserActiveInChat(memberID, chatID) {
			fmt.Printf("⏭️ User %d is actively viewing chat %d - sending in_app only (no push)\n", memberID, chatID)
			channels = []string{"in_app"} // Only in-app notification, no push
		}

		notificationReq := &client.NotificationRequest{
			UserID:      memberID,
			Type:        "message",
			Title:       notificationTitle,
			Message:     notificationMessage,
			Priority:    &priority,
			RelatedID:   &message.ID,
			RelatedType: "message",
			Data: map[string]interface{}{
				"chat_id":    chatID,
				"message_id": message.ID,
				"sender_id":  senderID, // Add sender_id for notification grouping
			},
			Channels: channels,
		}

		// Send notification async (don't block on errors)
		if err := uc.notificationClient.SendNotification(notificationReq); err != nil {
			fmt.Printf("Failed to send message notification to user %d: %v\n", memberID, err)
		}
	}

	return nil
}

// New refactored API methods

// GetLatestMessages retrieves the latest N messages in a chat
func (uc *messageUsecase) GetLatestMessages(userID, chatID uint, req *models.GetLatestMessagesRequest) (*models.GetLatestMessagesResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set default limit
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	// Get latest messages (for channels, exclude thread replies from main feed)
	var messages []*models.Message
	var total int64

	chat, err := uc.chatRepo.GetByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	if chat.Type == models.ChatTypeChannel {
		messages, total, err = uc.messageRepo.GetLatestMessagesExcludeThreads(chatID, userID, limit)
	} else {
		messages, total, err = uc.messageRepo.GetLatestMessages(chatID, userID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest messages: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
	}

	// Check if there are older messages
	hasOlder := false
	if len(messages) > 0 {
		hasOlder, err = uc.messageRepo.HasOlderMessages(chatID, userID, messages[0].ID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check for older messages: %v\n", err)
			// Don't fail the request, just assume there might be older messages
			hasOlder = total > int64(len(messages))
		}
	}

	response := &models.GetLatestMessagesResponse{
		Messages: messageResponses,
		Total:    total,
		HasOlder: hasOlder,
	}

	// Include unread info if requested (default: true)
	if req.IncludeUnreadMarker || (!req.IncludeUnreadMarker && req.Limit == 0) {
		// Get first unread message
		firstUnreadMsg, unreadCount, err := uc.messageRepo.GetFirstUnreadMessage(chatID, userID)
		if err != nil {
			fmt.Printf("⚠️ Failed to get first unread message: %v\n", err)
		} else if firstUnreadMsg != nil {
			response.UnreadInfo = &models.UnreadInfo{
				FirstUnreadID: &firstUnreadMsg.ID,
				UnreadCount:   unreadCount,
			}
		} else {
			// All messages are read
			response.UnreadInfo = &models.UnreadInfo{
				FirstUnreadID: nil,
				UnreadCount:   0,
			}
		}
	}

	// Get pinned messages for this chat
	pinnedMessages, err := uc.messageRepo.GetPinnedMessages(chatID, userID)
	if err != nil {
		fmt.Printf("⚠️ Failed to get pinned messages: %v\n", err)
		// Don't fail the request, just set empty array
		response.PinnedMessages = []models.MessageResponse{}
	} else {
		// Convert to response format
		pinnedResponses := make([]models.MessageResponse, len(pinnedMessages))
		for i, msg := range pinnedMessages {
			pinnedResponses[i] = *msg.ToResponseForUser(userID, uc.baseURL)
		}
		response.PinnedMessages = pinnedResponses
		fmt.Printf("📌 Retrieved %d pinned messages for chat %d\n", len(pinnedResponses), chatID)
	}

	fmt.Printf("✅ Retrieved %d latest messages for chat %d (total: %d, has_older: %v, pinned: %d)\n",
		len(messageResponses), chatID, total, hasOlder, len(response.PinnedMessages))

	return response, nil
}

// GetMessagesBeforeID retrieves messages before a specific message ID
func (uc *messageUsecase) GetMessagesBeforeID(userID, chatID, beforeID uint, req *models.GetMessagesBeforeRequest) (*models.GetMessagesBeforeResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set default limit
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	// Get messages before the specified ID (for channels, exclude thread replies)
	chat, err := uc.chatRepo.GetByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	var messages []*models.Message
	if chat.Type == models.ChatTypeChannel {
		messages, err = uc.messageRepo.GetMessagesBeforeIDExcludeThreads(chatID, userID, beforeID, limit)
	} else {
		messages, err = uc.messageRepo.GetMessagesBeforeID(chatID, userID, beforeID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get messages before ID: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
	}

	// Determine oldest message ID and check for older messages
	var oldestID *uint
	hasOlder := false
	if len(messages) > 0 {
		oldestID = &messages[0].ID
		hasOlder, err = uc.messageRepo.HasOlderMessages(chatID, userID, *oldestID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check for older messages: %v\n", err)
			// Don't fail the request, assume there might be more
			hasOlder = len(messages) == limit
		}
	}

	response := &models.GetMessagesBeforeResponse{
		Messages: messageResponses,
		HasOlder: hasOlder,
		OldestID: oldestID,
	}

	fmt.Printf("✅ Retrieved %d messages before ID %d for chat %d (has_older: %v)\n",
		len(messageResponses), beforeID, chatID, hasOlder)

	return response, nil
}

// GetMessagesAfterID retrieves messages after a specific message ID
func (uc *messageUsecase) GetMessagesAfterID(userID, chatID, afterID uint, req *models.GetMessagesAfterRequest) (*models.GetMessagesAfterResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set default limit
	limit := req.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	// Get messages after the specified ID (for channels, exclude thread replies)
	chat, err := uc.chatRepo.GetByID(chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	var messages []*models.Message
	if chat.Type == models.ChatTypeChannel {
		messages, err = uc.messageRepo.GetMessagesAfterIDExcludeThreads(chatID, userID, afterID, limit)
	} else {
		messages, err = uc.messageRepo.GetMessagesAfterID(chatID, userID, afterID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get messages after ID: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
	}

	// Determine newest message ID and check for newer messages
	var newestID *uint
	hasNewer := false
	if len(messages) > 0 {
		// Messages are in ascending order (oldest first), so newest is last
		newestID = &messages[len(messages)-1].ID
		hasNewer, err = uc.messageRepo.HasNewerMessages(chatID, userID, *newestID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check for newer messages: %v\n", err)
			// Don't fail the request, assume there might be more
			hasNewer = len(messages) == limit
		}
	}

	response := &models.GetMessagesAfterResponse{
		Messages: messageResponses,
		HasNewer: hasNewer,
		NewestID: newestID,
	}

	fmt.Printf("✅ Retrieved %d messages after ID %d for chat %d (has_newer: %v)\n",
		len(messageResponses), afterID, chatID, hasNewer)

	return response, nil
}

// GetMessageContext retrieves messages around a specific message (for "jump to message")
func (uc *messageUsecase) GetMessageContext(userID, chatID, targetMessageID uint, req *models.GetMessageContextRequest) (*models.GetMessageContextResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set defaults
	before := req.Before
	if before <= 0 {
		before = 15
	}
	if before > 50 {
		before = 50
	}

	after := req.After
	if after <= 0 {
		after = 15
	}
	if after > 50 {
		after = 50
	}

	// Get message context
	messages, err := uc.messageRepo.GetMessageContext(chatID, userID, targetMessageID, before, after)
	if err != nil {
		return nil, fmt.Errorf("failed to get message context: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
	}

	// Check for older and newer messages
	hasOlder := false
	hasNewer := false
	if len(messages) > 0 {
		oldestID := messages[0].ID
		newestID := messages[len(messages)-1].ID

		hasOlder, err = uc.messageRepo.HasOlderMessages(chatID, userID, oldestID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check for older messages: %v\n", err)
		}

		hasNewer, err = uc.messageRepo.HasNewerMessages(chatID, userID, newestID)
		if err != nil {
			fmt.Printf("⚠️ Failed to check for newer messages: %v\n", err)
		}
	}

	response := &models.GetMessageContextResponse{
		Messages:        messageResponses,
		TargetMessageID: targetMessageID,
		HasOlder:        hasOlder,
		HasNewer:        hasNewer,
	}

	fmt.Printf("✅ Retrieved %d messages around target ID %d for chat %d (has_older: %v, has_newer: %v)\n",
		len(messageResponses), targetMessageID, chatID, hasOlder, hasNewer)

	return response, nil
}

// SearchMessages searches for messages in a chat by content or file names
func (uc *messageUsecase) SearchMessages(userID, chatID uint, req *models.SearchMessagesRequest) (*models.SearchMessagesResponse, error) {
	// Check if user is a member of the chat
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Set default pagination
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	// Search messages
	messages, total, err := uc.messageRepo.SearchMessages(chatID, userID, req.Query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to search messages: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, message := range messages {
		messageResponses[i] = *message.ToResponseForUser(userID, uc.baseURL)
	}

	// Check if there are more results
	hasMore := int64(offset+len(messages)) < total

	response := &models.SearchMessagesResponse{
		Messages: messageResponses,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
		HasMore:  hasMore,
		Query:    req.Query,
	}

	fmt.Printf("🔍 Search results for '%s' in chat %d: %d/%d messages (offset: %d, has_more: %v)\n",
		req.Query, chatID, len(messageResponses), total, offset, hasMore)

	return response, nil
}

// GetThreadMessages retrieves comments in a thread (channel post)
// Uses forward pagination: afterID=0 loads from beginning, afterID>0 loads next page.
func (uc *messageUsecase) GetThreadMessages(userID, chatID, threadRootID uint, limit int, afterID uint) (*models.GetThreadMessagesResponse, error) {
	// Check membership
	isMember, err := uc.chatRepo.IsMember(chatID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if !isMember {
		return nil, fmt.Errorf("user is not a member of this chat")
	}

	// Validate thread root exists and belongs to this chat
	rootMsg, err := uc.messageRepo.GetWithReactions(threadRootID)
	if err != nil {
		return nil, fmt.Errorf("thread root message not found")
	}
	if rootMsg.ChatID != chatID {
		return nil, fmt.Errorf("thread root message is not in this chat")
	}

	// Set default limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	// Get thread messages (forward pagination)
	messages, total, err := uc.messageRepo.GetThreadMessages(threadRootID, userID, limit, afterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	// Convert to response format
	messageResponses := make([]models.MessageResponse, len(messages))
	for i, msg := range messages {
		messageResponses[i] = *msg.ToResponseForUser(userID, uc.baseURL)
	}

	// Check if there are more messages after the current page
	hasMore := int64(len(messages)) == int64(limit)

	rootResponse := rootMsg.ToResponseForUser(userID, uc.baseURL)

	return &models.GetThreadMessagesResponse{
		Messages:    messageResponses,
		Total:       total,
		HasMore:     hasMore,
		RootMessage: rootResponse,
	}, nil
}

// indexMessageInSearch sends a message to the search service for indexing
func (uc *messageUsecase) indexMessageInSearch(message *models.Message, chatMemberIDs []uint) {
	if uc.searchClient == nil {
		return
	}

	// Build search content: message content + file name if any
	content := message.Content
	if message.FileName != "" {
		content = content + " " + message.FileName
	}

	metadata := map[string]interface{}{
		"chat_id":   message.ChatID,
		"sender_id": message.SenderID,
		"type":      string(message.Type),
	}

	uc.searchClient.IndexDocument(&searchclient.IndexRequest{
		EntityType:   "message",
		EntityID:     message.ID,
		Title:        "",
		Content:      content,
		Metadata:     metadata,
		AccessibleBy: chatMemberIDs,
		IsPublic:     false,
		CreatorID:    message.SenderID,
	})
}
