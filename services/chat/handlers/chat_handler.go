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

	// Parse filter parameters
	chatType := c.Query("type")        // "private", "group", "channel"
	isFavoriteStr := c.Query("is_favorite") // "true", "false"
	isPinnedStr := c.Query("is_pinned")     // "true", "false"

	var isFavorite, isPinned *bool
	if isFavoriteStr != "" {
		val := isFavoriteStr == "true"
		isFavorite = &val
	}
	if isPinnedStr != "" {
		val := isPinnedStr == "true"
		isPinned = &val
	}

	chats, err := h.chatUsecase.GetUserChats(userID, limit, offset, chatType, isFavorite, isPinned)
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

// GetPinnedChats handles getting all pinned chats for a user
func (h *ChatHandler) GetPinnedChats(c *gin.Context) {
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

	// Parse filter parameters
	chatType := c.Query("type") // "private", "group", "channel"

	pinnedChats, err := h.chatUsecase.GetPinnedChats(userID, chatType)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get pinned chats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get pinned chats",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
		"count":      len(pinnedChats),
	}).Info("Pinned chats retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"chats":      pinnedChats,
		"count":      len(pinnedChats),
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

	// Parse optional query parameter for clearing history
	clearHistory := c.Query("clear_history") == "true"

	err = h.chatUsecase.DeleteChat(userID, uint(chatID), clearHistory)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":    requestID,
			"user_id":       userID,
			"chat_id":       chatID,
			"clear_history": clearHistory,
			"error":         err.Error(),
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

// UpdateChatMemberRole handles updating a member's role in chat
func (h *ChatHandler) UpdateChatMemberRole(c *gin.Context) {
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

	var req models.UpdateChatMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"chat_id":     chatID,
			"target_user": targetUserID,
			"error":       err.Error(),
		}).Warn("Invalid request body for update chat member role")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.UpdateMemberRole(userID, uint(chatID), uint(targetUserID), &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"user_id":     userID,
			"chat_id":     chatID,
			"target_user": targetUserID,
			"new_role":    req.Role,
			"error":       err.Error(),
		}).Error("Failed to update chat member role")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update member role"

		if strings.Contains(err.Error(), "only chat owner or admin") {
			statusCode = http.StatusForbidden
			errorMessage = "Only chat owner or admin can change member roles"
		} else if strings.Contains(err.Error(), "only chat owner can change admin") {
			statusCode = http.StatusForbidden
			errorMessage = "Only chat owner can change admin roles"
		} else if strings.Contains(err.Error(), "cannot change owner role") {
			statusCode = http.StatusBadRequest
			errorMessage = "Cannot change owner role"
		} else if strings.Contains(err.Error(), "cannot promote to owner") {
			statusCode = http.StatusBadRequest
			errorMessage = "Cannot promote to owner"
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
		"new_role":    req.Role,
	}).Info("Chat member role updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Member role updated successfully",
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

// JoinChat handles joining a chat
func (h *ChatHandler) JoinChat(c *gin.Context) {
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

	err = h.chatUsecase.JoinChat(userID, uint(chatID))
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to join chat")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to join chat"

		if strings.Contains(err.Error(), "not active") {
			statusCode = http.StatusBadRequest
			errorMessage = "Chat is not active"
		} else if strings.Contains(err.Error(), "already a member") {
			statusCode = http.StatusConflict
			errorMessage = "User is already a member of this chat"
		} else if strings.Contains(err.Error(), "private chat") {
			statusCode = http.StatusForbidden
			errorMessage = "Cannot join private chat"
		} else if strings.Contains(err.Error(), "maximum member limit") {
			statusCode = http.StatusForbidden
			errorMessage = "Chat has reached maximum member limit"
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
	}).Info("User joined chat successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Successfully joined chat",
		"request_id": requestID,
	})
}

// ToggleFavorite handles toggling favorite status for a chat
func (h *ChatHandler) ToggleFavorite(c *gin.Context) {
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

	// Parse request body
	var req struct {
		IsFavorite bool `json:"is_favorite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Warn("Invalid request body for toggle favorite")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.ToggleFavorite(userID, uint(chatID), req.IsFavorite)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"is_favorite": req.IsFavorite,
			"error":      err.Error(),
		}).Error("Failed to toggle favorite status")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update favorite status"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "User is not a member of this chat"
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
		"is_favorite": req.IsFavorite,
	}).Info("Chat favorite status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Favorite status updated successfully",
		"is_favorite": req.IsFavorite,
		"request_id": requestID,
	})
}

// TogglePinned handles toggling pinned status for a chat
func (h *ChatHandler) TogglePinned(c *gin.Context) {
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

	// Parse request body
	var req struct {
		IsPinned bool `json:"is_pinned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Warn("Invalid request body for toggle pinned")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	err = h.chatUsecase.TogglePinned(userID, uint(chatID), req.IsPinned)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"is_pinned":  req.IsPinned,
			"error":      err.Error(),
		}).Error("Failed to toggle pinned status")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to update pinned status"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "User is not a member of this chat"
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
		"is_pinned":  req.IsPinned,
	}).Info("Chat pinned status updated successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":   "Pinned status updated successfully",
		"is_pinned": req.IsPinned,
		"request_id": requestID,
	})
}

// GetChatAttachments handles getting all attachments for a chat
func (h *ChatHandler) GetChatAttachments(c *gin.Context) {
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
			"chat_id":    chatIDStr,
			"error":      err.Error(),
		}).Warn("Invalid chat ID format")

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

	// Get attachments from usecase
	attachments, total, err := h.chatUsecase.GetChatAttachments(userID, uint(chatID), limit, offset)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"chat_id":    chatID,
			"error":      err.Error(),
		}).Error("Failed to get chat attachments")

		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to get attachments"

		if strings.Contains(err.Error(), "not a member") {
			statusCode = http.StatusForbidden
			errorMessage = "You are not a member of this chat"
		} else if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorMessage = "Chat not found"
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
		"count":      len(attachments),
		"total":      total,
	}).Info("Chat attachments retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"attachments": attachments,
		"total":       total,
		"limit":       limit,
		"offset":      offset,
		"request_id":  requestID,
	})
}

// GetTotalUnreadCount handles getting total unread messages count for the user across all chats
func (h *ChatHandler) GetTotalUnreadCount(c *gin.Context) {
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

	// Get total unread count from usecase
	unreadCount, err := h.chatUsecase.GetTotalUnreadCount(userID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to get total unread count")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to get unread count",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id":   requestID,
		"user_id":      userID,
		"unread_count": unreadCount,
	}).Info("Total unread count retrieved successfully")

	c.JSON(http.StatusOK, gin.H{
		"unread_count": unreadCount,
		"request_id":   requestID,
	})
}
