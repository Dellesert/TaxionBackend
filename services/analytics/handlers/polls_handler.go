package handlers

import (
	"net/http"

	"tachyon-messenger/services/analytics/usecase"

	"github.com/gin-gonic/gin"
)

// PollsHandler handles poll analytics requests
type PollsHandler struct {
	analyticsUsecase *usecase.AnalyticsUsecase
}

// NewPollsHandler creates a new polls handler
func NewPollsHandler(analyticsUsecase *usecase.AnalyticsUsecase) *PollsHandler {
	return &PollsHandler{analyticsUsecase: analyticsUsecase}
}

// GetStats returns poll statistics
func (h *PollsHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{}})
}

// GetParticipation returns poll participation stats
func (h *PollsHandler) GetParticipation(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
}
