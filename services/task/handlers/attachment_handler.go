package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/usecase"

	"github.com/gin-gonic/gin"
)

// TaskUsecaseNotifications interface for sending notifications
type TaskUsecaseNotifications interface {
	SendAttachmentAddedNotification(taskID, userID uint, fileName string)
}

// AttachmentHandler handles HTTP requests for task attachments
type AttachmentHandler struct {
	attachmentUsecase usecase.AttachmentUsecase
	taskUsecase       TaskUsecaseNotifications
}

// NewAttachmentHandler creates a new attachment handler
func NewAttachmentHandler(attachmentUsecase usecase.AttachmentUsecase, taskUsecase TaskUsecaseNotifications) *AttachmentHandler {
	return &AttachmentHandler{
		attachmentUsecase: attachmentUsecase,
		taskUsecase:       taskUsecase,
	}
}

// UploadAttachment uploads a file attachment or attaches existing file
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

	// Check if file_id is provided in JSON body (new approach)
	var requestBody struct {
		FileID uint `json:"file_id"`
	}

	if err := c.ShouldBindJSON(&requestBody); err == nil && requestBody.FileID > 0 {
		// New approach: attach existing file by ID
		fmt.Printf("📎 Attaching existing file_id: %d to task: %d\n", requestBody.FileID, taskID)
		attachment, err := h.attachmentUsecase.AttachFileToTask(uint(taskID), userID.(uint), requestBody.FileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Send notification about attachment (async)
		go h.taskUsecase.SendAttachmentAddedNotification(uint(taskID), userID.(uint), attachment.FileName)

		c.JSON(http.StatusCreated, attachment)
		return
	}

	// Old approach: direct file upload
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded and no file_id provided"})
		return
	}

	// Upload attachment
	attachment, err := h.attachmentUsecase.UploadAttachment(uint(taskID), userID.(uint), file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Send notification about attachment (async)
	go h.taskUsecase.SendAttachmentAddedNotification(uint(taskID), userID.(uint), file.Filename)

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
