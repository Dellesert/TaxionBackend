package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"tachyon-messenger/services/file/models"
	"tachyon-messenger/services/file/usecase"

	"github.com/gin-gonic/gin"
)

// FileHandler handles file-related HTTP requests
type FileHandler struct {
	fileUsecase *usecase.FileUsecase
}

// NewFileHandler creates a new file handler
func NewFileHandler(fileUsecase *usecase.FileUsecase) *FileHandler {
	return &FileHandler{
		fileUsecase: fileUsecase,
	}
}

// UploadFile handles file upload
// @Summary Upload a file
// @Description Upload a file (avatar, attachment, document, etc.)
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Param file_type formData string true "File type" Enums(avatar, attachment, document, image, video, audio, other)
// @Param entity_type formData string false "Entity type (e.g., user, message, task)"
// @Param entity_id formData int false "Entity ID"
// @Param is_public formData bool false "Is file public"
// @Success 201 {object} models.FileResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /files/upload [post]
func (h *FileHandler) UploadFile(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get file from request
	file, err := c.FormFile("file")
	if err != nil {
		fmt.Printf("❌ Error getting file from form: %v\n", err)
		fmt.Printf("📋 Request headers: %v\n", c.Request.Header)
		fmt.Printf("📋 Content-Type: %s\n", c.ContentType())
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to get file from request: %v", err)})
		return
	}

	// Parse request
	var req models.UploadFileRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Upload file
	uploadedFile, err := h.fileUsecase.UploadFile(
		file,
		req.FileType,
		userID.(uint),
		req.EntityType,
		req.EntityID,
		req.IsPublic,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Return response
	response := uploadedFile.ToResponse(h.fileUsecase.GetBaseURL())
	c.JSON(http.StatusCreated, response)
}

// GetFile handles getting a file by ID
// @Summary Get file by ID
// @Description Get file details by ID
// @Tags files
// @Produce json
// @Param id path int true "File ID"
// @Success 200 {object} models.FileResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /files/{id} [get]
func (h *FileHandler) GetFile(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse file ID
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Get file
	file, err := h.fileUsecase.GetFile(uint(fileID), userID.(uint))
	if err != nil {
		if err.Error() == "access denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Return response
	response := file.ToResponse(h.fileUsecase.GetBaseURL())
	c.JSON(http.StatusOK, response)
}

// DownloadFile handles file download by filename
// @Summary Download file
// @Description Download file by filename (requires authentication). Supports thumbnails with _thumb suffix.
// @Tags files
// @Produce application/octet-stream
// @Param filename path string true "Filename"
// @Success 200 {file} binary
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /files/{filename} [get]
func (h *FileHandler) DownloadFile(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get filename from URL
	fileName := c.Param("filename")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	// Check if this is a thumbnail request
	isThumbnail := strings.Contains(fileName, "_thumb")

	// Get file record
	file, err := h.fileUsecase.GetFileByName(fileName, userID.(uint))
	if err != nil {
		if err.Error() == "access denied" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Determine which file to serve (original or thumbnail)
	filePath := file.FilePath
	if isThumbnail && file.ThumbnailPath != "" {
		filePath = file.ThumbnailPath
	} else if isThumbnail && file.ThumbnailPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail not available"})
		return
	}

	// Serve file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+file.OriginalName)
	c.Header("Content-Type", file.MimeType)
	c.File(filePath)
}

// DownloadPublicFile handles public file download (no auth required)
// @Summary Download public file
// @Description Download public file by filename (no authentication required). Supports thumbnails with _thumb suffix.
// @Tags files
// @Produce application/octet-stream
// @Param filename path string true "Filename"
// @Success 200 {file} binary
// @Failure 404 {object} map[string]interface{}
// @Router /files/public/{filename} [get]
func (h *FileHandler) DownloadPublicFile(c *gin.Context) {
	// Get filename from URL
	fileName := c.Param("filename")
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Filename is required"})
		return
	}

	// Check if this is a thumbnail request
	isThumbnail := strings.Contains(fileName, "_thumb")

	// Get file record
	file, err := h.fileUsecase.GetPublicFileByName(fileName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Determine which file to serve (original or thumbnail)
	filePath := file.FilePath
	if isThumbnail && file.ThumbnailPath != "" {
		filePath = file.ThumbnailPath
	} else if isThumbnail && file.ThumbnailPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Thumbnail not available"})
		return
	}

	// Serve file
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "inline; filename="+file.OriginalName)
	c.Header("Content-Type", file.MimeType)
	c.Header("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	// Add CORS headers for public files (needed for cross-origin requests)
	// This is safe for public files as they are meant to be accessible from anywhere
	origin := c.Request.Header.Get("Origin")
	if origin != "" {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type")
	}

	c.File(filePath)
}

// ListFiles handles listing files with filters
// @Summary List files
// @Description List files with optional filters
// @Tags files
// @Produce json
// @Param file_type query string false "File type"
// @Param entity_type query string false "Entity type"
// @Param entity_id query int false "Entity ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /files [get]
func (h *FileHandler) ListFiles(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse query parameters
	var filter models.FileFilterRequest
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get files
	files, total, err := h.fileUsecase.ListFiles(&filter, userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Convert to responses
	responses := make([]*models.FileResponse, len(files))
	for i, file := range files {
		responses[i] = file.ToResponse(h.fileUsecase.GetBaseURL())
	}

	c.JSON(http.StatusOK, gin.H{
		"files": responses,
		"total": total,
		"limit": filter.Limit,
		"offset": filter.Offset,
	})
}

// DeleteFile handles file deletion
// @Summary Delete file
// @Description Delete a file by ID
// @Tags files
// @Param id path int true "File ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /files/{id} [delete]
func (h *FileHandler) DeleteFile(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse file ID
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Delete file
	if err := h.fileUsecase.DeleteFile(uint(fileID), userID.(uint)); err != nil {
		if err.Error() == "access denied: only the uploader can delete this file" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

// GetUserAvatar handles getting user's avatar
// @Summary Get user avatar
// @Description Get current user's avatar
// @Tags files
// @Produce json
// @Success 200 {object} models.FileResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /files/avatar [get]
func (h *FileHandler) GetUserAvatar(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get avatar
	avatar, err := h.fileUsecase.GetUserAvatar(userID.(uint))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Avatar not found"})
		return
	}

	// Return response
	response := avatar.ToResponse(h.fileUsecase.GetBaseURL())
	c.JSON(http.StatusOK, response)
}

// GetFileInternal handles getting file by ID for internal service-to-service communication
// No authentication required - should only be accessible within Docker network
// @Summary Get file by ID (Internal)
// @Description Get file details by ID for inter-service communication (no auth required)
// @Tags internal
// @Produce json
// @Param id path int true "File ID"
// @Success 200 {object} models.FileResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /internal/files/{id} [get]
func (h *FileHandler) GetFileInternal(c *gin.Context) {
	// Parse file ID
	fileID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	// Get file without access control check
	file, err := h.fileUsecase.GetFileByID(uint(fileID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Return response
	response := file.ToResponse(h.fileUsecase.GetBaseURL())
	c.JSON(http.StatusOK, response)
}
