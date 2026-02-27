package handlers

import (
	"net/http"

	"tachyon-messenger/services/search/models"
	"tachyon-messenger/services/search/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// IndexHandler handles internal API requests for document indexing
type IndexHandler struct {
	searchUsecase usecase.SearchUsecase
}

// NewIndexHandler creates a new index handler
func NewIndexHandler(searchUsecase usecase.SearchUsecase) *IndexHandler {
	return &IndexHandler{searchUsecase: searchUsecase}
}

// IndexDocument handles POST /api/v1/internal/search/index
func (h *IndexHandler) IndexDocument(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.IndexDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if err := h.searchUsecase.IndexDocument(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"entity_type": req.EntityType,
			"entity_id":   req.EntityID,
			"error":       err.Error(),
		}).Error("Failed to index document")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось проиндексировать документ",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "indexed",
		"entity_type": req.EntityType,
		"entity_id":   req.EntityID,
	})
}

// BulkIndex handles POST /api/v1/internal/search/bulk-index
func (h *IndexHandler) BulkIndex(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.BulkIndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if err := h.searchUsecase.BulkIndexDocuments(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":     requestID,
			"document_count": len(req.Documents),
			"error":          err.Error(),
		}).Error("Failed to bulk index documents")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось массово проиндексировать документы",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "indexed",
		"count":  len(req.Documents),
	})
}

// DeleteDocument handles DELETE /api/v1/internal/search/index
func (h *IndexHandler) DeleteDocument(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.DeleteDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Неверное тело запроса",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	if err := h.searchUsecase.DeleteDocument(req.EntityType, req.EntityID); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"entity_type": req.EntityType,
			"entity_id":   req.EntityID,
			"error":       err.Error(),
		}).Error("Failed to delete document")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось удалить документ",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "deleted",
		"entity_type": req.EntityType,
		"entity_id":   req.EntityID,
	})
}
