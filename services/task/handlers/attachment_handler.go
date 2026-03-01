package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/models"
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
	userClient        *clients.UserClient
}

// NewAttachmentHandler creates a new attachment handler
func NewAttachmentHandler(attachmentUsecase usecase.AttachmentUsecase, taskUsecase TaskUsecaseNotifications, userClient *clients.UserClient) *AttachmentHandler {
	return &AttachmentHandler{
		attachmentUsecase: attachmentUsecase,
		taskUsecase:       taskUsecase,
		userClient:        userClient,
	}
}

// UploadAttachment uploads a file attachment or attaches existing file
// POST /api/v1/tasks/:id/attachments
func (h *AttachmentHandler) UploadAttachment(c *gin.Context) {
	// Get user ID from context (set by JWT middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Не авторизован"})
		return
	}

	// Get task ID from path
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID задачи"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не загружен и file_id не указан"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID задачи"})
		return
	}

	// Get attachments
	attachments, err := h.attachmentUsecase.GetTaskAttachments(uint(taskID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to response models
	responses := make([]*models.TaskAttachmentResponse, len(attachments))
	for i, a := range attachments {
		responses[i] = a.ToResponse()
	}

	// Batch-fetch user info for all uploaders
	userIDs := make([]uint, 0, len(attachments))
	seen := make(map[uint]bool)
	for _, a := range attachments {
		if !seen[a.UploadedByUserID] {
			userIDs = append(userIDs, a.UploadedByUserID)
			seen[a.UploadedByUserID] = true
		}
	}

	if len(userIDs) > 0 {
		users, err := h.userClient.GetUsersByIDs(userIDs)
		if err == nil {
			for _, resp := range responses {
				if u, exists := users[resp.UploadedByUserID]; exists {
					resp.UploadedBy = &models.UserInfo{
						ID:       u.ID,
						Name:     u.Name,
						Email:    u.Email,
						Avatar:   u.Avatar,
						Position: u.Position,
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, responses)
}

// DeleteAttachment deletes an attachment
// DELETE /api/v1/attachments/:id
func (h *AttachmentHandler) DeleteAttachment(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Не авторизован"})
		return
	}

	// Get attachment ID from path
	attachmentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID вложения"})
		return
	}

	// Delete attachment
	if err := h.attachmentUsecase.DeleteAttachment(uint(attachmentID), userID.(uint)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Attachment deleted successfully"})
}
