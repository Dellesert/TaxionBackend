package usecase

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
)

// AttachmentUsecase defines the interface for task attachment business logic
type AttachmentUsecase interface {
	// Upload and manage attachments
	UploadAttachment(taskID, userID uint, file *multipart.FileHeader) (*models.TaskAttachment, error)
	GetTaskAttachments(taskID uint) ([]*models.TaskAttachment, error)
	GetAttachmentByID(id uint) (*models.TaskAttachment, error)
	DeleteAttachment(id, userID uint) error

	// File validation
	ValidateFile(file *multipart.FileHeader) error
}

// attachmentUsecase implements AttachmentUsecase interface
type attachmentUsecase struct {
	attachmentRepo repository.AttachmentRepository
	taskRepo       repository.TaskRepository
	uploadDir      string
	maxFileSize    int64 // in bytes
	allowedTypes   []string
}

// NewAttachmentUsecase creates a new attachment usecase
func NewAttachmentUsecase(
	attachmentRepo repository.AttachmentRepository,
	taskRepo repository.TaskRepository,
) AttachmentUsecase {
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads/tasks"
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		fmt.Printf("Warning: failed to create upload directory: %v\n", err)
	}

	return &attachmentUsecase{
		attachmentRepo: attachmentRepo,
		taskRepo:       taskRepo,
		uploadDir:      uploadDir,
		maxFileSize:    50 * 1024 * 1024, // 50 MB default
		allowedTypes: []string{
			// Documents
			".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
			".txt", ".rtf", ".odt", ".ods", ".odp",
			// Images
			".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp",
			// Archives
			".zip", ".rar", ".7z", ".tar", ".gz",
			// Other
			".csv", ".json", ".xml",
		},
	}
}

// ValidateFile validates the uploaded file
func (u *attachmentUsecase) ValidateFile(file *multipart.FileHeader) error {
	// Check file size
	if file.Size > u.maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d MB", u.maxFileSize/(1024*1024))
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := false
	for _, allowedExt := range u.allowedTypes {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("file type %s is not allowed", ext)
	}

	return nil
}

// UploadAttachment uploads a file attachment for a task
func (u *attachmentUsecase) UploadAttachment(taskID, userID uint, file *multipart.FileHeader) (*models.TaskAttachment, error) {
	// Validate file
	if err := u.ValidateFile(file); err != nil {
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	// Check if task exists
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%d_%d_%s%s", taskID, timestamp, sanitizeFilename(file.Filename), ext)
	filePath := filepath.Join(u.uploadDir, filename)

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file
	if _, err := io.Copy(dst, src); err != nil {
		// Clean up on error
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Create attachment record
	attachment := &models.TaskAttachment{
		TaskID:           taskID,
		UploadedByUserID: userID,
		FileName:         file.Filename,
		FilePath:         filePath,
		FileType:         ext,
		FileSize:         file.Size,
	}

	if err := u.attachmentRepo.Create(attachment); err != nil {
		// Clean up file on database error
		os.Remove(filePath)
		return nil, fmt.Errorf("failed to create attachment record: %w", err)
	}

	return attachment, nil
}

// GetTaskAttachments retrieves all attachments for a task
func (u *attachmentUsecase) GetTaskAttachments(taskID uint) ([]*models.TaskAttachment, error) {
	// Check if task exists
	task, err := u.taskRepo.GetByID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	return u.attachmentRepo.GetByTaskID(taskID)
}

// GetAttachmentByID retrieves an attachment by ID
func (u *attachmentUsecase) GetAttachmentByID(id uint) (*models.TaskAttachment, error) {
	return u.attachmentRepo.GetByID(id)
}

// DeleteAttachment deletes an attachment
func (u *attachmentUsecase) DeleteAttachment(id, userID uint) error {
	// Get attachment
	attachment, err := u.attachmentRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("attachment not found: %w", err)
	}

	// Check if task exists
	task, err := u.taskRepo.GetByID(attachment.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// TODO: Add permission check - only uploader, task creator, or admin can delete
	// For now, we'll allow the uploader or task creator
	if attachment.UploadedByUserID != userID && task.CreatedByUserID != userID {
		return fmt.Errorf("permission denied: only the uploader or task creator can delete this attachment")
	}

	// Delete file from filesystem
	if err := os.Remove(attachment.FilePath); err != nil {
		// Log error but continue with database deletion
		fmt.Printf("Warning: failed to delete file from filesystem: %v\n", err)
	}

	// Delete from database
	if err := u.attachmentRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete attachment: %w", err)
	}

	return nil
}

// sanitizeFilename removes potentially dangerous characters from filename
func sanitizeFilename(filename string) string {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Replace spaces and special characters
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)

	// Limit length
	if len(name) > 50 {
		name = name[:50]
	}

	return name
}
