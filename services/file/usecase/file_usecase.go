package usecase

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"tachyon-messenger/services/file/models"
	"tachyon-messenger/services/file/repository"
)

// FileUsecase handles file business logic
type FileUsecase struct {
	repo           *repository.FileRepository
	uploadDir      string
	maxFileSize    int64
	allowedTypes   map[string]bool
	baseURL        string
}

// NewFileUsecase creates a new file usecase
func NewFileUsecase(repo *repository.FileRepository, uploadDir, baseURL string) *FileUsecase {
	// Allowed MIME types
	allowedTypes := map[string]bool{
		// Images
		"image/jpeg":               true,
		"image/png":                true,
		"image/gif":                true,
		"image/webp":               true,
		"image/svg+xml":            true,
		"image/heic":               true,
		"image/heif":               true,
		"image/bmp":                true,
		"image/tiff":               true,
		"image/avif":               true,
		"image/x-icon":             true,
		"image/vnd.microsoft.icon": true,
		// Documents
		"application/pdf":                                                               true,
		"application/msword":                                                            true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":       true,
		"application/vnd.ms-excel":                                                      true,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":             true,
		"application/vnd.ms-powerpoint":                                                 true,
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":     true,
		"text/plain":                                                                    true,
		"text/csv":                                                                      true,
		"application/rtf":                                                               true,
		"text/markdown":                                                                 true,
		"application/json":                                                              true,
		"text/xml":                                                                      true,
		"application/xml":                                                               true,
		// Video
		"video/mp4":          true,
		"video/mpeg":         true,
		"video/webm":         true,
		"video/quicktime":    true, // MOV (iPhone)
		"video/x-msvideo":    true, // AVI
		"video/3gpp":         true, // 3GP
		"video/x-matroska":   true, // MKV
		// Audio
		"audio/mpeg":         true,
		"audio/wav":          true,
		"audio/x-wav":        true,
		"audio/ogg":          true,
		"audio/webm":         true,
		"audio/mp4":          true, // M4A
		"audio/x-m4a":        true, // M4A alternative
		"audio/aac":          true,
		"audio/flac":         true,
		// Archives
		"application/zip":              true,
		"application/x-rar-compressed": true,
		"application/vnd.rar":          true,
		"application/x-7z-compressed":  true,
		"application/gzip":             true,
		"application/x-gzip":           true,
		"application/x-tar":            true,
	}

	return &FileUsecase{
		repo:         repo,
		uploadDir:    uploadDir,
		maxFileSize:  50 * 1024 * 1024, // 50 MB default
		allowedTypes: allowedTypes,
		baseURL:      baseURL,
	}
}

// isHEIC checks if the file is a HEIC/HEIF image
func (u *FileUsecase) isHEIC(mimeType string) bool {
	return mimeType == "image/heic" || mimeType == "image/heif"
}

// convertHEICtoJPEG converts a HEIC file to JPEG using heif-convert
func (u *FileUsecase) convertHEICtoJPEG(heicPath string) (string, error) {
	// Generate output path with .jpg extension
	jpegPath := strings.TrimSuffix(heicPath, filepath.Ext(heicPath)) + ".jpg"

	// Run heif-convert command
	cmd := exec.Command("heif-convert", "-q", "90", heicPath, jpegPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("heif-convert failed: %v, stderr: %s", err, stderr.String())
	}

	// Remove original HEIC file
	os.Remove(heicPath)

	return jpegPath, nil
}

// UploadFile uploads a file and creates a record in the database
func (u *FileUsecase) UploadFile(
	file *multipart.FileHeader,
	fileType models.FileType,
	uploadedBy uint,
	entityType *string,
	entityID *uint,
	isPublic bool,
) (*models.File, error) {
	// Validate file size
	if file.Size > u.maxFileSize {
		return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", u.maxFileSize)
	}

	// Get MIME type
	mimeType := file.Header.Get("Content-Type")

	// Validate MIME type
	if !u.allowedTypes[mimeType] {
		return nil, errors.New("file type not allowed")
	}

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	// Generate unique filename
	fileName, err := u.generateUniqueFileName(file.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to generate filename: %w", err)
	}

	// Create upload directory if it doesn't exist
	uploadPath := filepath.Join(u.uploadDir, string(fileType))
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Full file path
	filePath := filepath.Join(uploadPath, fileName)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	written, err := io.Copy(dst, src)
	if err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Close destination file before conversion
	dst.Close()

	// Convert HEIC to JPEG if necessary
	originalName := file.Filename
	finalMimeType := mimeType
	if u.isHEIC(mimeType) {
		convertedPath, err := u.convertHEICtoJPEG(filePath)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to convert HEIC to JPEG: %w", err)
		}
		filePath = convertedPath
		fileName = filepath.Base(convertedPath)
		finalMimeType = "image/jpeg"
		// Update original name extension
		originalName = strings.TrimSuffix(originalName, filepath.Ext(originalName)) + ".jpg"

		// Get new file size after conversion
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to get converted file info: %w", err)
		}
		written = fileInfo.Size()
	}

	// Create file record
	fileRecord := &models.File{
		FileName:     fileName,
		OriginalName: originalName,
		FilePath:     filePath,
		FileSize:     written,
		MimeType:     finalMimeType,
		FileType:     fileType,
		UploadedBy:   uploadedBy,
		EntityType:   entityType,
		EntityID:     entityID,
		IsPublic:     isPublic,
	}

	// Save to database
	if err := u.repo.Create(fileRecord); err != nil {
		os.Remove(filePath) // Clean up on error
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	return fileRecord, nil
}

// GetFile retrieves a file by ID
func (u *FileUsecase) GetFile(id uint, userID uint) (*models.File, error) {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Check access permissions
	if !file.IsPublic && file.UploadedBy != userID {
		return nil, errors.New("access denied")
	}

	return file, nil
}

// GetFileByID retrieves a file by ID without access control (for internal service use)
func (u *FileUsecase) GetFileByID(id uint) (*models.File, error) {
	return u.repo.GetByID(id)
}

// GetFileByName retrieves a file by filename
func (u *FileUsecase) GetFileByName(fileName string, userID uint) (*models.File, error) {
	file, err := u.repo.GetByFileName(fileName)
	if err != nil {
		return nil, err
	}

	// Check access permissions
	if !file.IsPublic && file.UploadedBy != userID {
		return nil, errors.New("access denied")
	}

	return file, nil
}

// GetPublicFileByName retrieves a public file by filename (no auth required)
func (u *FileUsecase) GetPublicFileByName(fileName string) (*models.File, error) {
	file, err := u.repo.GetByFileName(fileName)
	if err != nil {
		return nil, err
	}

	if !file.IsPublic {
		return nil, errors.New("file is not public")
	}

	return file, nil
}

// GetFilesByEntity retrieves files for a specific entity
func (u *FileUsecase) GetFilesByEntity(entityType string, entityID uint) ([]models.File, error) {
	return u.repo.GetByEntity(entityType, entityID)
}

// GetUserFiles retrieves files uploaded by a user
func (u *FileUsecase) GetUserFiles(userID uint, limit, offset int) ([]models.File, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return u.repo.GetByUploader(userID, limit, offset)
}

// ListFiles retrieves files with filters
func (u *FileUsecase) ListFiles(filter *models.FileFilterRequest, userID uint) ([]models.File, int64, error) {
	// Users can only list their own files unless they're admin
	// For now, we'll restrict to user's own files
	filter.UploadedBy = &userID

	return u.repo.List(filter)
}

// DeleteFile deletes a file (both record and physical file)
func (u *FileUsecase) DeleteFile(id uint, userID uint) error {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Only the uploader can delete the file
	if file.UploadedBy != userID {
		return errors.New("access denied: only the uploader can delete this file")
	}

	// Delete physical file
	if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete physical file: %w", err)
	}

	// Delete database record
	if err := u.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete file record: %w", err)
	}

	return nil
}

// DeleteFilesByEntity deletes all files for an entity
func (u *FileUsecase) DeleteFilesByEntity(entityType string, entityID uint, userID uint) error {
	files, err := u.repo.GetByEntity(entityType, entityID)
	if err != nil {
		return err
	}

	for _, file := range files {
		// Check permission for each file
		if file.UploadedBy != userID {
			continue // Skip files not owned by the user
		}

		// Delete physical file
		if err := os.Remove(file.FilePath); err != nil && !os.IsNotExist(err) {
			// Log error but continue
			continue
		}
	}

	// Delete all records
	return u.repo.DeleteByEntity(entityType, entityID)
}

// GetUserAvatar retrieves user's avatar
func (u *FileUsecase) GetUserAvatar(userID uint) (*models.File, error) {
	return u.repo.GetUserAvatar(userID)
}

// UpdateFileEntity updates the entity association of a file
func (u *FileUsecase) UpdateFileEntity(id uint, entityType *string, entityID *uint, userID uint) error {
	file, err := u.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Only the uploader can update the file
	if file.UploadedBy != userID {
		return errors.New("access denied")
	}

	file.EntityType = entityType
	file.EntityID = entityID

	return u.repo.Update(file)
}

// generateUniqueFileName generates a unique filename based on timestamp and hash
func (u *FileUsecase) generateUniqueFileName(originalName string) (string, error) {
	// Get file extension
	ext := filepath.Ext(originalName)

	// Generate hash from filename and timestamp
	hash := md5.New()
	hash.Write([]byte(originalName + time.Now().String()))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	// Create unique filename: timestamp_hash.ext
	fileName := fmt.Sprintf("%d_%s%s", time.Now().Unix(), hashStr[:16], ext)

	// Sanitize filename (remove any potentially dangerous characters)
	fileName = strings.ReplaceAll(fileName, "..", "")
	fileName = strings.ReplaceAll(fileName, "/", "")
	fileName = strings.ReplaceAll(fileName, "\\", "")

	return fileName, nil
}

// GetBaseURL returns the base URL for file access
func (u *FileUsecase) GetBaseURL() string {
	return u.baseURL
}

// GetFileStatsInternal retrieves file statistics for analytics (no auth required - internal use)
func (u *FileUsecase) GetFileStatsInternal() (*repository.FileStatsInternal, error) {
	return u.repo.GetFileStatsInternal()
}
