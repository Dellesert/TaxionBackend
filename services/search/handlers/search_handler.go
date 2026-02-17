package handlers

import (
	"fmt"
	"net/http"

	"tachyon-messenger/services/search/models"
	"tachyon-messenger/services/search/usecase"
	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SearchHandler handles public search endpoints
type SearchHandler struct {
	searchUsecase usecase.SearchUsecase
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchUsecase usecase.SearchUsecase) *SearchHandler {
	return &SearchHandler{searchUsecase: searchUsecase}
}

// Search handles GET /api/v1/search?q=...&type=task&type=chat&limit=5&category=task&offset=0
func (h *SearchHandler) Search(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user info from auth context
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Unauthorized",
			"request_id": requestID,
		})
		return
	}
	userID := userIDRaw.(uint)

	// Get user role (for admin bypass)
	userRole := ""
	if roleRaw, exists := c.Get("user_role"); exists {
		userRole = fmt.Sprintf("%v", roleRaw)
	}

	// Bind query parameters
	var req models.SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid search parameters",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Set defaults
	if req.Limit <= 0 || req.Limit > 50 {
		req.Limit = 5
	}

	// Validate types if provided
	for _, t := range req.Types {
		if !t.IsValid() {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid entity type: " + string(t),
				"request_id": requestID,
			})
			return
		}
	}

	result, err := h.searchUsecase.Search(req.Query, userID, userRole, &req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"query":      req.Query,
			"error":      err.Error(),
		}).Error("Search failed")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Search failed",
			"request_id": requestID,
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
