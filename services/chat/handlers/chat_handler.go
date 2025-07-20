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

// ChatHandler handles HTTP requests for chat operations
type ChatHandler struct {
	chatUsecase usecase.ChatUsecase
}

// NewChatHandler creates a new chat handler
func NewChatHandler(chatUsecase usecase.ChatUsecase) *ChatHandler {
	return &ChatHandler{
		chatUsecase: chatUsecase,
	}
}

// Chat Handler Methods

// GetChats handles getting all chats for a user
func (h *ChatHandler) GetChats(c *gin.Context) {
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

	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	chats, err := h.chatUsecase.GetUserChats(userID, limit, offset)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get user chats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get chats",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"count":      len(chats.Chats),
	}).Info("User chats retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"chats":      chats.Chats,
		"total":      chats.Total,
		"limit":      chats.Limit,
		"offset":     chats.Offset,
		"request_id": requestID,
	})
}

// CreateChat handles chat creation
func (h *ChatHandler) CreateChat(c *gin.Context) {
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

	var req models.CreateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Warn("Invalid request body for create chat")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	chat, err := h.chatUsecase.CreateChat(userID, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_type":  req.Type,
			"error":      err.Error(),
		}).Error("Failed to create chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to create chat"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = err.Error()
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
		"chat_id":    chat.ID,
		"chat_type":  chat.Type,
	}).Info("Chat created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Chat created successfully",
		"chat":       chat,
		"request_id": requestID,
	})
}

// GetChat handles getting a specific chat
func (h *ChatHandler) GetChat(c *gin.Context) {
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
	idStr := c.Param("id")
	chatID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	chat, err := h.chatUsecase.GetChat(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to get chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get chat"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
		"chat_id":    chatID,
	}).Info("Chat retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"chat":       chat,
		"request_id": requestID,
	})
}

// UpdateChat handles chat update
func (h *ChatHandler) UpdateChat(c *gin.Context) {
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
	idStr := c.Param("id")
	chatID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	var req models.UpdateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Warn("Invalid request body for update chat")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	chat, err := h.chatUsecase.UpdateChat(userID, uint(chatID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to update chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update chat"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
		"chat_id":    chatID,
	}).Info("Chat updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Chat updated successfully",
		"chat":       chat,
		"request_id": requestID,
	})
}

// DeleteChat handles chat deletion
func (h *ChatHandler) DeleteChat(c *gin.Context) {
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
	idStr := c.Param("id")
	chatID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.DeleteChat(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to delete chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to delete chat"

		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
		} else if strings.Contains(err.Error(), "only chat owner") {
			statusCode = http.StatusForbidden
			errorMessage = "Only chat owner can delete the chat"
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
	}).Info("Chat deleted successfully")

	c.JSON(http.StatusNoContent, gin.H{
		"message":    "Chat deleted successfully",
		"request_id": requestID,
	})
}

// GetChatMembers handles getting chat members
func (h *ChatHandler) GetChatMembers(c *gin.Context) {
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
	idStr := c.Param("id")
	chatID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	members, err := h.chatUsecase.GetChatMembers(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to get chat members")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get chat members"

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
		"request_id":   requestID,
		"user_id":      userID,
		"chat_id":      chatID,
		"member_count": len(members),
	}).Info("Chat members retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"members":    members,
		"count":      len(members),
		"request_id": requestID,
	})
}

// AddChatMember handles adding a member to chat
func (h *ChatHandler) AddChatMember(c *gin.Context) {
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
	idStr := c.Param("id")
	chatID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    idStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid chat ID",
			"request_id": requestID,
		})
		return
	}

	var req models.AddChatMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Warn("Invalid request body for add chat member")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.AddMember(userID, uint(chatID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"chat_id":     chatID,
			"target_user": req.UserID,
			"error":       err.Error(),
		}).Error("Failed to add chat member")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to add member"

		if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = "Insufficient permissions"
		} else if strings.Contains(err.Error(), "already a member") {
			statusCode = http.StatusConflict
			errorMessage = "User is already a member"
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
		"chat_id":     chatID,
		"target_user": req.UserID,
	}).Info("Chat member added successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Member added successfully",
		"request_id": requestID,
	})
}

// RemoveChatMember handles removing a member from chat
func (h *ChatHandler) RemoveChatMember(c *gin.Context) {
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

	// Get user ID from URL parameter
	userIDStr := c.Param("userId")
	targetUserID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"chat_id":     chatID,
			"target_user": userIDStr,
			"error":       err.Error(),
		}).Warn("Invalid user ID")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid user ID",
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.RemoveMember(userID, uint(chatID), uint(targetUserID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"chat_id":     chatID,
			"target_user": targetUserID,
			"error":       err.Error(),
		}).Error("Failed to remove chat member")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to remove member"

		if strings.Contains(err.Error(), "insufficient permissions") {
			statusCode = http.StatusForbidden
			errorMessage = "Insufficient permissions"
		} else if strings.Contains(err.Error(), "cannot remove") {
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
		"chat_id":     chatID,
		"target_user": targetUserID,
	}).Info("Chat member removed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Member removed successfully",
		"request_id": requestID,
	})
}
