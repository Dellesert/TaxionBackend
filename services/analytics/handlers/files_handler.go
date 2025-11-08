package handlers

import (
	"net/http"

	"tachyon-messenger/services/analytics/usecase"

	"github.com/gin-gonic/gin"
)

// FilesHandler handles file analytics requests
type FilesHandler struct {
	analyticsUsecase *usecase.AnalyticsUsecase
}

// NewFilesHandler creates a new files handler
func NewFilesHandler(analyticsUsecase *usecase.AnalyticsUsecase) *FilesHandler {
	return &FilesHandler{analyticsUsecase: analyticsUsecase}
}

// GetStats returns file statistics
func (h *FilesHandler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{}})
}

// GetStorage returns storage usage stats
func (h *FilesHandler) GetStorage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"total_storage": 10000000000,
		"used_storage":  4500000000,
		"used_percent":  45.0,
	})
}
