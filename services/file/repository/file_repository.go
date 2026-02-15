package repository

import (
	"errors"
	"fmt"

	"tachyon-messenger/services/file/models"
	"tachyon-messenger/shared/database"
)

// FileRepository handles file data persistence
type FileRepository struct {
	db *database.DB
}

// NewFileRepository creates a new file repository
func NewFileRepository(db *database.DB) *FileRepository {
	return &FileRepository{db: db}
}

// Create creates a new file record
func (r *FileRepository) Create(file *models.File) error {
	if err := r.db.DB.Create(file).Error; err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	return nil
}

// GetByID retrieves a file by ID
func (r *FileRepository) GetByID(id uint) (*models.File, error) {
	var file models.File
	if err := r.db.DB.First(&file, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return &file, nil
}

// GetByFileName retrieves a file by filename
func (r *FileRepository) GetByFileName(fileName string) (*models.File, error) {
	var file models.File
	if err := r.db.DB.Where("file_name = ?", fileName).First(&file).Error; err != nil {
		return nil, fmt.Errorf("failed to get file by filename: %w", err)
	}
	return &file, nil
}

// GetByThumbnailFileName retrieves a file by its thumbnail filename
func (r *FileRepository) GetByThumbnailFileName(thumbnailFileName string) (*models.File, error) {
	var file models.File
	if err := r.db.DB.Where("thumbnail_path LIKE ?", "%/"+thumbnailFileName).First(&file).Error; err != nil {
		return nil, fmt.Errorf("failed to get file by thumbnail filename: %w", err)
	}
	return &file, nil
}

// GetByContentHash retrieves a file by its content hash (SHA-256)
func (r *FileRepository) GetByContentHash(hash string) (*models.File, error) {
	var file models.File
	if err := r.db.DB.Where("content_hash = ?", hash).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// GetByEntity retrieves files by entity type and ID
func (r *FileRepository) GetByEntity(entityType string, entityID uint) ([]models.File, error) {
	var files []models.File
	if err := r.db.DB.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC").
		Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to get files by entity: %w", err)
	}
	return files, nil
}

// GetByUploader retrieves files uploaded by a user
func (r *FileRepository) GetByUploader(userID uint, limit, offset int) ([]models.File, int64, error) {
	var files []models.File
	var total int64

	query := r.db.DB.Where("uploaded_by = ?", userID)

	// Count total
	if err := query.Model(&models.File{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count files: %w", err)
	}

	// Get paginated results
	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&files).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get files: %w", err)
	}

	return files, total, nil
}

// List retrieves files with filters
func (r *FileRepository) List(filter *models.FileFilterRequest) ([]models.File, int64, error) {
	var files []models.File
	var total int64

	query := r.db.DB.Model(&models.File{})

	// Apply filters
	if filter.FileType != nil {
		query = query.Where("file_type = ?", *filter.FileType)
	}
	if filter.EntityType != nil {
		query = query.Where("entity_type = ?", *filter.EntityType)
	}
	if filter.EntityID != nil {
		query = query.Where("entity_id = ?", *filter.EntityID)
	}
	if filter.UploadedBy != nil {
		query = query.Where("uploaded_by = ?", *filter.UploadedBy)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count files: %w", err)
	}

	// Set pagination defaults
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Get paginated results
	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&files).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get files: %w", err)
	}

	return files, total, nil
}

// Delete deletes a file record by ID
func (r *FileRepository) Delete(id uint) error {
	result := r.db.DB.Delete(&models.File{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete file: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("file not found")
	}
	return nil
}

// DeleteByEntity deletes all files for an entity
func (r *FileRepository) DeleteByEntity(entityType string, entityID uint) error {
	if err := r.db.DB.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Delete(&models.File{}).Error; err != nil {
		return fmt.Errorf("failed to delete files by entity: %w", err)
	}
	return nil
}

// Update updates a file record
func (r *FileRepository) Update(file *models.File) error {
	if err := r.db.DB.Save(file).Error; err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}
	return nil
}

// GetUserAvatar retrieves the latest avatar for a user
func (r *FileRepository) GetUserAvatar(userID uint) (*models.File, error) {
	var file models.File
	err := r.db.DB.Where("uploaded_by = ? AND file_type = ?", userID, models.FileTypeAvatar).
		Order("created_at DESC").
		First(&file).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user avatar: %w", err)
	}
	return &file, nil
}

// GetPublicFiles retrieves all public files
func (r *FileRepository) GetPublicFiles(limit, offset int) ([]models.File, int64, error) {
	var files []models.File
	var total int64

	query := r.db.DB.Where("is_public = ?", true)

	// Count total
	if err := query.Model(&models.File{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count public files: %w", err)
	}

	// Get paginated results
	if err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&files).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get public files: %w", err)
	}

	return files, total, nil
}

// FileStatsInternal represents file statistics for internal use
type FileStatsInternal struct {
	TotalFiles  int   `json:"total_files"`
	TotalSize   int64 `json:"total_size"`
	AvgFileSize int64 `json:"avg_file_size"`
}

// GetFileStatsInternal retrieves file statistics for analytics
func (r *FileRepository) GetFileStatsInternal() (*FileStatsInternal, error) {
	var stats FileStatsInternal
	var totalFiles int64
	var totalSize int64

	// Count total files
	if err := r.db.DB.Model(&models.File{}).Count(&totalFiles).Error; err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	// Sum total size
	row := r.db.DB.Model(&models.File{}).Select("COALESCE(SUM(file_size), 0)").Row()
	if err := row.Scan(&totalSize); err != nil {
		return nil, fmt.Errorf("failed to sum file sizes: %w", err)
	}

	stats.TotalFiles = int(totalFiles)
	stats.TotalSize = totalSize

	// Calculate average file size
	if totalFiles > 0 {
		stats.AvgFileSize = totalSize / totalFiles
	} else {
		stats.AvgFileSize = 0
	}

	return &stats, nil
}
