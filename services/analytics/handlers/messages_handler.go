package handlers

import (
	"net/http"

	"tachyon-messenger/services/analytics/usecase"

	"github.com/gin-gonic/gin"
)

// MessagesHandler handles message analytics requests
type MessagesHandler struct {
	analyticsUsecase *usecase.AnalyticsUsecase
}

// NewMessagesHandler creates a new messages handler
func NewMessagesHandler(analyticsUsecase *usecase.AnalyticsUsecase) *MessagesHandler {
	return &MessagesHandler{analyticsUsecase: analyticsUsecase}
}

// GetStats returns message statistics
func (h *MessagesHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{}})
}

// GetTimeline returns message timeline data
func (h *MessagesHandler) GetTimeline(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}

// GetTopChats returns most active chats
func (h *MessagesHandler) GetTopChats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}
