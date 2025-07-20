package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MessageHandler handles HTTP requests for message operations
type MessageHandler struct {
	messageUsecase usecase.MessageUsecase
}

// NewMessageHandler creates a new message handler
func NewMessageHandler(messageUsecase usecase.MessageUsecase) *MessageHandler {
	return &MessageHandler{
		messageUsecase: messageUsecase,
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

	err = h.messageUsecase.DeleteMessage(userID, uint(messageID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"message_id": messageID,
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
	}).Info("Message deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"message":    "Message deleted successfully",
		"request_id": requestID,
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

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	messages, err := h.messageUsecase.GetMessagesByChat(userID, uint(chatID), limit, offset)
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

	logger.WithFields(map[string]interface{}{
		"request_id":    requestID,
		"user_id":       userID,
		"chat_id":       chatID,
		"message_count": len(messages.Messages),
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
