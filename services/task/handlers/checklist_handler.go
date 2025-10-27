package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"

	"github.com/gin-gonic/gin"
)

// ChecklistHandler handles HTTP requests for task checklists
type ChecklistHandler struct {
	checklistUsecase usecase.ChecklistUsecase
}

// NewChecklistHandler creates a new checklist handler
func NewChecklistHandler(checklistUsecase usecase.ChecklistUsecase) *ChecklistHandler {
	return &ChecklistHandler{
		checklistUsecase: checklistUsecase,
	}
}

// CreateChecklist creates a new checklist
// POST /api/v1/tasks/:id/checklists
func (h *ChecklistHandler) CreateChecklist(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Parse request body
	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create checklist
	checklist, err := h.checklistUsecase.CreateChecklist(uint(taskID), userID.(uint), req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, checklist)
}

// GetTaskChecklists retrieves all checklists for a task
// GET /api/v1/tasks/:id/checklists
func (h *ChecklistHandler) GetTaskChecklists(c *gin.Context) {
	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Get checklists with items
	checklists, err := h.checklistUsecase.GetTaskChecklistsWithItems(uint(taskID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, checklists)
}

// UpdateChecklist updates a checklist
// PUT /api/v1/checklists/:id
func (h *ChecklistHandler) UpdateChecklist(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get checklist ID from path
	checklistID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid checklist ID"})
		return
	}

	// Parse request body
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update checklist
	checklist, err := h.checklistUsecase.UpdateChecklist(uint(checklistID), userID.(uint), req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, checklist)
}

// DeleteChecklist deletes a checklist
// DELETE /api/v1/checklists/:id
func (h *ChecklistHandler) DeleteChecklist(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get checklist ID from path
	checklistID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid checklist ID"})
		return
	}

	// Delete checklist
	if err := h.checklistUsecase.DeleteChecklist(uint(checklistID), userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Checklist deleted successfully"})
}

// CreateChecklistItem creates a new checklist item
// POST /api/v1/checklists/:id/items
func (h *ChecklistHandler) CreateChecklistItem(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get checklist ID from path
	checklistID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid checklist ID"})
		return
	}

	// Parse request body
	var req struct {
		Title    string `json:"title" binding:"required"`
		Position int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create item
	item, err := h.checklistUsecase.CreateChecklistItem(uint(checklistID), userID.(uint), req.Title, req.Position)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// UpdateChecklistItem updates a checklist item
// PUT /api/v1/checklist-items/:id
func (h *ChecklistHandler) UpdateChecklistItem(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get item ID from path
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	// Parse request body
	var req struct {
		Title       *string `json:"title"`
		IsCompleted *bool   `json:"is_completed"`
		Position    *int    `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update item
	title := ""
	if req.Title != nil {
		title = *req.Title
	}
	item, err := h.checklistUsecase.UpdateChecklistItem(uint(itemID), userID.(uint), title, req.IsCompleted, req.Position)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

// ToggleChecklistItem toggles completion status
// PATCH /api/v1/checklist-items/:id/toggle
func (h *ChecklistHandler) ToggleChecklistItem(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get item ID from path
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	// Toggle item
	item, err := h.checklistUsecase.ToggleChecklistItem(uint(itemID), userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, item)
}

// DeleteChecklistItem deletes a checklist item
// DELETE /api/v1/checklist-items/:id
func (h *ChecklistHandler) DeleteChecklistItem(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get item ID from path
	itemID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	// Delete item
	if err := h.checklistUsecase.DeleteChecklistItem(uint(itemID), userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Checklist item deleted successfully"})
}
