package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/shared/middleware"

	"github.com/gin-gonic/gin"
)

// GetOrCreateDirectChat gets or creates a direct chat with another user
// @Summary Get or create direct chat
// @Description Gets existing or creates a new direct chat with specified user
// @Tags chats
// @Accept json
// @Produce json
// @Param userId path int true "Target User ID"
// @Security BearerAuth
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /chats/direct/{userId} [post]
func (h *ChatHandler) GetOrCreateDirectChat(c *gin.Context) {
	// Get current user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не аутентифицирован"})
		return
	}

	// Parse target user ID from URL parameter
	targetUserIDStr := c.Param("userId")
	targetUserID, err := strconv.ParseUint(targetUserIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID пользователя"})
		return
	}

	// Call usecase to get or create direct chat
	chat, err := h.chatUsecase.GetOrCreateDirectChat(userID, uint(targetUserID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat": chat})
}

// GetOrCreateTaskChat gets or creates a group chat for a task
// @Summary Get or create task chat
// @Description Gets existing or creates a new group chat for specified task
// @Tags chats
// @Accept json
// @Produce json
// @Param taskId path int true "Task ID"
// @Security BearerAuth
// @Success 200 {object} models.ChatResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /chats/task/{taskId} [post]
func (h *ChatHandler) GetOrCreateTaskChat(c *gin.Context) {
	// Get current user ID from JWT token
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Пользователь не аутентифицирован"})
		return
	}

	// Parse task ID from URL parameter
	taskIDStr := c.Param("taskId")
	taskID, err := strconv.ParseUint(taskIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID задачи"})
		return
	}

	// Call usecase to get or create task chat
	chat, err := h.chatUsecase.GetOrCreateTaskChat(userID, uint(taskID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat": chat})
}
