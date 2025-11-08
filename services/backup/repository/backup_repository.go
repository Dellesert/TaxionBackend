package repository

import (
	"fmt"

	"tachyon-messenger/services/backup/models"
	"tachyon-messenger/shared/database"
)

// BackupRepository handles backup data persistence
type BackupRepository struct {
	db *database.DB
}

// NewBackupRepository creates a new backup repository
func NewBackupRepository(db *database.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

// Create creates a new backup record
func (r *BackupRepository) Create(backup *models.Backup) error {
	if err := r.db.DB.Create(backup).Error; err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return nil
}

// GetByID retrieves a backup by ID
func (r *BackupRepository) GetByID(id uint) (*models.Backup, error) {
	var backup models.Backup
	if err := r.db.DB.First(&backup, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get backup: %w", err)
	}
	return &backup, nil
}

// GetByFileName retrieves a backup by filename
func (r *BackupRepository) GetByFileName(fileName string) (*models.Backup, error) {
	var backup models.Backup
	if err := r.db.DB.Where("file_name = ?", fileName).First(&backup).Error; err != nil {
		return nil, fmt.Errorf("failed to get backup by filename: %w", err)
	}
	return &backup, nil
}

// List retrieves backups with pagination
func (r *BackupRepository) List(page, pageSize int) ([]*models.Backup, int64, error) {
	var backups []*models.Backup
	var total int64

	query := r.db.DB.Model(&models.Backup{})

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count backups: %w", err)
	}

	// Set pagination defaults
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize

	// Get paginated results
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&backups).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get backups: %w", err)
	}

	return backups, total, nil
}

// Update updates a backup record
func (r *BackupRepository) Update(backup *models.Backup) error {
	if err := r.db.DB.Save(backup).Error; err != nil {
		return fmt.Errorf("failed to update backup: %w", err)
	}
	return nil
}

// Delete deletes a backup record by ID
func (r *BackupRepository) Delete(id uint) error {
	result := r.db.DB.Delete(&models.Backup{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete backup: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("backup not found")
	}
	return nil
}

// GetLatest retrieves the latest successful backup
func (r *BackupRepository) GetLatest() (*models.Backup, error) {
	var backup models.Backup
	if err := r.db.DB.Where("status = ?", models.BackupStatusCompleted).
		Order("created_at DESC").
		First(&backup).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest backup: %w", err)
	}
	return &backup, nil
}

// GetStats retrieves backup statistics
func (r *BackupRepository) GetStats() (*models.BackupStatsResponse, error) {
	var stats models.BackupStatsResponse
	var totalBackups, successfulBackups, failedBackups int64
	var totalSize int64

	// Count total backups
	if err := r.db.DB.Model(&models.Backup{}).Count(&totalBackups).Error; err != nil {
		return nil, fmt.Errorf("failed to count total backups: %w", err)
	}

	// Count successful backups
	if err := r.db.DB.Model(&models.Backup{}).
		Where("status = ?", models.BackupStatusCompleted).
		Count(&successfulBackups).Error; err != nil {
		return nil, fmt.Errorf("failed to count successful backups: %w", err)
	}

	// Count failed backups
	if err := r.db.DB.Model(&models.Backup{}).
		Where("status = ?", models.BackupStatusFailed).
		Count(&failedBackups).Error; err != nil {
		return nil, fmt.Errorf("failed to count failed backups: %w", err)
	}

	// Sum total size
	row := r.db.DB.Model(&models.Backup{}).
		Where("status = ?", models.BackupStatusCompleted).
		Select("COALESCE(SUM(file_size), 0)").Row()
	if err := row.Scan(&totalSize); err != nil {
		return nil, fmt.Errorf("failed to sum backup sizes: %w", err)
	}

	stats.TotalBackups = int(totalBackups)
	stats.SuccessfulBackups = int(successfulBackups)
	stats.FailedBackups = int(failedBackups)
	stats.TotalSize = totalSize

	// Get latest backup
	latest, err := r.GetLatest()
	if err == nil {
		stats.LatestBackup = latest.ToResponse()
	}

	return &stats, nil
}
