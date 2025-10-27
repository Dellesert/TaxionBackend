package handlers

import (
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"

	"github.com/gin-gonic/gin"
)

// AttachmentHandler handles HTTP requests for task attachments
type AttachmentHandler struct {
	attachmentUsecase usecase.AttachmentUsecase
}

// NewAttachmentHandler creates a new attachment handler
func NewAttachmentHandler(attachmentUsecase usecase.AttachmentUsecase) *AttachmentHandler {
	return &AttachmentHandler{
		attachmentUsecase: attachmentUsecase,
	}
}

// UploadAttachment uploads a file attachment
// POST /api/v1/tasks/:id/attachments
func (h *AttachmentHandler) UploadAttachment(c *gin.Context) {
	// Get user ID from context (set by JWT middleware)
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

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Upload attachment
	attachment, err := h.attachmentUsecase.UploadAttachment(uint(taskID), userID.(uint), file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, attachment)
}

// GetTaskAttachments retrieves all attachments for a task
// GET /api/v1/tasks/:id/attachments
func (h *AttachmentHandler) GetTaskAttachments(c *gin.Context) {
	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// Get attachments
	attachments, err := h.attachmentUsecase.GetTaskAttachments(uint(taskID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, attachments)
}

// DeleteAttachment deletes an attachment
// DELETE /api/v1/attachments/:id
func (h *AttachmentHandler) DeleteAttachment(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get attachment ID from path
	attachmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid attachment ID"})
		return
	}

	// Delete attachment
	if err := h.attachmentUsecase.DeleteAttachment(uint(attachmentID), userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
}
