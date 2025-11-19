package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MessageHandler handles HTTP requests for message operations
type MessageHandler struct {
	messageUsecase  usecase.MessageUsecase
	analyticsClient *analytics.Client
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(messageUsecase usecase.MessageUsecase, analyticsClient *analytics.Client) *MessageHandler {
	return &MessageHandler{
		messageUsecase:  messageUsecase,
		analyticsClient: analyticsClient,
	}
}

// Message Handler Methods

// GetMessages handles getting messages
func (h *MessageHandler) GetMessages(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse query parameters
	var req models.GetMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid query parameters for get messages")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid query parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	messages, err := h.messageUsecase.GetMessages(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    req.ChatID,
			"error":      err.Error(),
		}).Error("Failed to get messages")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get messages"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		} else if strings.Contains(err.Error(), "chat_id is required") {
			statusCode = http.StatusBadRequest
			errorMessage = "chat_id is required"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"chat_id":       req.ChatID,
		"message_count": len(messages.Messages),
	}).Info("Messages retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"messages":   messages.Messages,
		"total":      messages.Total,
		"limit":      messages.Limit,
		"offset":     messages.Offset,
		"has_more":   messages.HasMore,
		"request_id": requestID,
	})
}

// SendMessage handles sending a message
func (h *MessageHandler) SendMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	var req models.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for send message")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	message, err := h.messageUsecase.SendMessage(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    req.ChatID,
			"error":      err.Error(),
		}).Error("Failed to send message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to send message"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		} else if strings.Contains(err.Error(), "validation failed") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"chat_id":    req.ChatID,
		"message_id": message.ID,
	}).Info("Message sent successfully")

	// Send analytics event
	h.analyticsClient.SendEvent(
		analytics.EventMessageSent,
		analytics.CategoryMessage,
		uint64(userID),
		map[string]interface{}{
			"chat_id":    req.ChatID,
			"message_id": message.ID,
		},
	)

	c.JSON(http.StatusCreated, gin.H{
		"message":    message,
		"request_id": requestID,
	})
}

// GetMessage handles getting a specific message
func (h *MessageHandler) GetMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	message, err := h.messageUsecase.GetMessage(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to get message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    message,
		"request_id": requestID,
	})
}

// UpdateMessage handles updating a message
func (h *MessageHandler) UpdateMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update message")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	message, err := h.messageUsecase.UpdateMessage(userID, uint(messageID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to update message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "only message sender") {
			statusCode = http.StatusForbidden
			errorMessage = "Only message sender can edit the message"
		} else if strings.Contains(err.Error(), "deleted message") {
			statusCode = http.StatusBadRequest
			errorMessage = "Cannot edit deleted message"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    message,
		"request_id": requestID,
	})
}

// DeleteMessage handles deleting a message
func (h *MessageHandler) DeleteMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	// Parse request body to get delete_for parameter
	var req struct {
		DeleteFor string `json:"delete_for"` // "everyone" or "me"
	}

	// Try to bind JSON, but don't fail if body is empty (default to old behavior)
	_ = c.ShouldBindJSON(&req)

	// Default to "everyone" if not specified (for backward compatibility)
	deleteFor := req.DeleteFor
	if deleteFor == "" {
		deleteFor = "everyone"
	}

	// Use new DeleteMessageForUser method
	err = h.messageUsecase.DeleteMessageForUser(userID, uint(messageID), deleteFor)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"delete_for": deleteFor,
			"error":      err.Error(),
		}).Error("Failed to delete message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = "Insufficient permissions"
		} else if strings.Contains(err.Error(), "invalid delete_for") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
		"delete_for": deleteFor,
	}).Info("Message deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Message deleted successfully",
		"delete_for": deleteFor,
		"request_id": requestID,
	})
}

// BulkDeleteMessages handles deleting multiple messages at once
func (h *MessageHandler) BulkDeleteMessages(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Parse request body
	var req models.BulkDeleteMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"request_id": requestID,
		})
		return
	}

	// Default to "everyone" if not specified
	if req.DeleteFor == "" {
		req.DeleteFor = "everyone"
	}

	// Delete messages
	err = h.messageUsecase.BulkDeleteMessages(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"message_ids": req.MessageIDs,
			"delete_for":  req.DeleteFor,
			"error":       err.Error(),
		}).Error("Failed to bulk delete messages")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete messages"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "One or more messages not found"
		} else if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		} else if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = "Insufficient permissions"
		} else if strings.Contains(err.Error(), "invalid delete_for") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":  requestID,
		"user_id":     userID,
		"message_ids": req.MessageIDs,
		"delete_for":  req.DeleteFor,
	}).Info("Messages deleted successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":     "Messages deleted successfully",
		"count":       len(req.MessageIDs),
		"delete_for":  req.DeleteFor,
		"request_id":  requestID,
	})
}

// GetMessagesByChat handles getting messages for a specific chat
func (h *MessageHandler) GetMessagesByChat(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get chat ID from URL parameter
	chatIDStr := c.Param("chatId")
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatIDStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	// Parse pagination parameters including before/after for cursor-based pagination
	var req models.GetMessagesRequest
	req.ChatID = uint(chatID)

	if err := c.ShouldBindQuery(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Warn("Invalid query parameters")
	}

	// Use GetMessages which supports before/after for pagination
	messages, err := h.messageUsecase.GetMessages(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to get messages by chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get messages"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	// Mark messages as read if requested via mark_as_read parameter
	if req.MarkAsRead {
		err := h.messageUsecase.MarkChatAsRead(userID, uint(chatID))
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"chat_id":    chatID,
				"error":      err.Error(),
			}).Warn("Failed to mark chat as read after retrieving messages")
			// Don't fail the request if marking as read fails, just log the warning
		} else {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"chat_id":    chatID,
			}).Info("Chat messages marked as read")
		}
	}

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"chat_id":       chatID,
		"message_count": len(messages.Messages),
		"mark_as_read":  req.MarkAsRead,
	}).Info("Messages by chat retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"messages":   messages.Messages,
		"total":      messages.Total,
		"limit":      messages.Limit,
		"offset":     messages.Offset,
		"has_more":   messages.HasMore,
		"request_id": requestID,
	})
}

// AddReaction handles adding a reaction to a message
func (h *MessageHandler) AddReaction(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AddReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Warn("Invalid request body for add reaction")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.AddReaction(userID, uint(messageID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"emoji":      req.Emoji,
			"error":      err.Error(),
		}).Error("Failed to add reaction")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to add reaction"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
		"emoji":      req.Emoji,
	}).Info("Reaction added successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Reaction added successfully",
		"request_id": requestID,
	})
}

// RemoveReaction handles removing a reaction from a message
func (h *MessageHandler) RemoveReaction(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	// Get emoji from query parameter
	emoji := c.Query("emoji")
	if emoji == "" {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
		}).Warn("Emoji is required for remove reaction")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Emoji is required",
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.RemoveReaction(userID, uint(messageID), emoji)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"emoji":      emoji,
			"error":      err.Error(),
		}).Error("Failed to remove reaction")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to remove reaction"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
		"emoji":      emoji,
	}).Info("Reaction removed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Reaction removed successfully",
		"request_id": requestID,
	})
}

// MarkAsRead handles marking a message as read
func (h *MessageHandler) MarkAsRead(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.MarkAsRead(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to mark message as read")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to mark message as read"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message marked as read successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Message marked as read successfully",
		"request_id": requestID,
	})
}

// MarkChatAsRead handles marking all messages in a chat as read
func (h *MessageHandler) MarkChatAsRead(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get chat ID from URL parameter
	chatIDStr := c.Param("chatId")
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatIDStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.MarkChatAsRead(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to mark chat messages as read")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to mark chat messages as read"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"chat_id":    chatID,
	}).Info("Chat messages marked as read successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Chat messages marked as read successfully",
		"request_id": requestID,
	})
}

// ClearChatHistory handles clearing chat history for the current user
func (h *MessageHandler) ClearChatHistory(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get chat ID from URL parameter
	chatIDStr := c.Param("id")
	chatID, err := strconv.ParseUint(chatIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatIDStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.ClearChatHistory(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to clear chat history")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to clear chat history"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "Access denied"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"chat_id":    chatID,
	}).Info("Chat history cleared successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Chat history cleared successfully",
		"request_id": requestID,
	})
}

// RestoreMessage handles restoring a deleted message (admin only)
func (h *MessageHandler) RestoreMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	err = h.messageUsecase.RestoreMessage(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to restore message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to restore message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "only administrators") {
			statusCode = http.StatusForbidden
			errorMessage = "Only administrators can restore messages"
		} else if strings.Contains(err.Error(), "not deleted") {
			statusCode = http.StatusBadRequest
			errorMessage = "Message is not deleted"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message restored successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Message restored successfully",
		"request_id": requestID,
	})
}

// PinMessage handles pinning a message in chat
func (h *MessageHandler) PinMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	message, err := h.messageUsecase.PinMessage(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to pin message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to pin message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "only administrators") {
			statusCode = http.StatusForbidden
			errorMessage = "Only administrators can pin messages"
		} else if strings.Contains(err.Error(), "already pinned") {
			statusCode = http.StatusBadRequest
			errorMessage = "Message is already pinned"
		} else if strings.Contains(err.Error(), "deleted message") {
			statusCode = http.StatusBadRequest
			errorMessage = "Cannot pin deleted message"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message pinned successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    message,
		"request_id": requestID,
	})
}

// UnpinMessage handles unpinning a message in chat
func (h *MessageHandler) UnpinMessage(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get user ID from context")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get message ID from URL parameter
	idStr := c.Param("id")
	messageID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": idStr,
			"error":      err.Error(),
		}).Warn("Invalid message ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid message ID",
			"request_id": requestID,
		})
		return
	}

	message, err := h.messageUsecase.UnpinMessage(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
			"error":      err.Error(),
		}).Error("Failed to unpin message")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to unpin message"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Message not found"
		} else if strings.Contains(err.Error(), "only administrators") {
			statusCode = http.StatusForbidden
			errorMessage = "Only administrators can unpin messages"
		} else if strings.Contains(err.Error(), "not pinned") {
			statusCode = http.StatusBadRequest
			errorMessage = "Message is not pinned"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"message_id": messageID,
	}).Info("Message unpinned successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    message,
		"request_id": requestID,
	})
}
