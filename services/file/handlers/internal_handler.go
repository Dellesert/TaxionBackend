package handlers

import (
	"net/http"

	"tachyon-messenger/services/file/usecase"

	"github.com/gin-gonic/gin"
)

// InternalHandler handles internal API requests (for inter-service communication)
type InternalHandler struct {
	fileUsecase *usecase.FileUsecase
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(fileUsecase *usecase.FileUsecase) *InternalHandler {
	return &InternalHandler{
		fileUsecase: fileUsecase,
	}
}

// GetFileStats returns file statistics for analytics service
// @Summary Get file statistics
// @Description Gets file count and size statistics for analytics (internal use only)
// @Tags internal
// @Produce json
// @Success 200 {object} FileStatsResponse
// @Failure 500 {object} gin.H
// @Router /internal/files/stats [get]
func (h *InternalHandler) GetFileStats(c *gin.Context) {
	stats, err := h.fileUsecase.GetFileStatsInternal()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить статистику файлов"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
