package usecase

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"tachyon-messenger/services/backup/models"
	"tachyon-messenger/services/backup/repository"
	"tachyon-messenger/shared/logger"
)

// BackupUsecase handles backup business logic
type BackupUsecase struct {
	repo       *repository.BackupRepository
	backupDir  string
	dbHost     string
	dbPort     string
	dbName     string
	dbUser     string
	dbPassword string
	log        *logger.Logger
}

// NewBackupUsecase creates a new backup usecase
func NewBackupUsecase(
	repo *repository.BackupRepository,
	backupDir string,
	dbHost string,
	dbPort string,
	dbName string,
	dbUser string,
	dbPassword string,
	log *logger.Logger,
) *BackupUsecase {
	return &BackupUsecase{
		repo:       repo,
		backupDir:  backupDir,
		dbHost:     dbHost,
		dbPort:     dbPort,
		dbName:     dbName,
		dbUser:     dbUser,
		dbPassword: dbPassword,
		log:        log,
	}
}

// CreateBackup creates a new database backup
func (u *BackupUsecase) CreateBackup(userID uint, backupType models.BackupType, description string) (*models.Backup, error) {
	// Generate unique filename
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("backup_%s.sql", timestamp)
	filePath := filepath.Join(u.backupDir, fileName)

	// Create backup record
	backup := &models.Backup{
		FileName:    fileName,
		FilePath:    filePath,
		Status:      models.BackupStatusPending,
		Type:        backupType,
		CreatedBy:   userID,
		Description: description,
	}

	if err := u.repo.Create(backup); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	// Start backup process in background
	go u.performBackup(backup)

	return backup, nil
}

// performBackup executes the actual backup process
func (u *BackupUsecase) performBackup(backup *models.Backup) {
	startTime := time.Now()
	backup.StartedAt = &startTime
	backup.Status = models.BackupStatusInProgress
	u.repo.Update(backup)

	u.log.WithFields(map[string]interface{}{
		"backup_id": backup.ID,
		"file_name": backup.FileName,
	}).Info("Starting backup process")

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(u.backupDir, 0755); err != nil {
		u.failBackup(backup, fmt.Sprintf("failed to create backup directory: %v", err))
		return
	}

	// Execute pg_dump command
	cmd := exec.Command("pg_dump",
		"-h", u.dbHost,
		"-p", u.dbPort,
		"-U", u.dbUser,
		"-d", u.dbName,
		"-F", "p", // Plain text SQL format
		"-f", backup.FilePath,
	)

	// Set PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", u.dbPassword))

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		u.failBackup(backup, fmt.Sprintf("pg_dump failed: %v, output: %s", err, string(output)))
		return
	}

	// Get file size
	fileInfo, err := os.Stat(backup.FilePath)
	if err != nil {
		u.failBackup(backup, fmt.Sprintf("failed to get file info: %v", err))
		return
	}

	// Update backup record with success
	completedTime := time.Now()
	backup.CompletedAt = &completedTime
	backup.Status = models.BackupStatusCompleted
	backup.FileSize = fileInfo.Size()

	if err := u.repo.Update(backup); err != nil {
		u.log.WithFields(map[string]interface{}{
			"backup_id": backup.ID,
			"error":     err.Error(),
		}).Error("Failed to update backup record after success")
		return
	}

	u.log.WithFields(map[string]interface{}{
		"backup_id": backup.ID,
		"file_name": backup.FileName,
		"file_size": backup.FileSize,
		"duration":  time.Since(startTime).Seconds(),
	}).Info("Backup completed successfully")
}

// failBackup marks a backup as failed
func (u *BackupUsecase) failBackup(backup *models.Backup, errorMsg string) {
	u.log.WithFields(map[string]interface{}{
		"backup_id": backup.ID,
		"error":     errorMsg,
	}).Error("Backup failed")

	backup.Status = models.BackupStatusFailed
	backup.ErrorMessage = errorMsg
	completedTime := time.Now()
	backup.CompletedAt = &completedTime

	if err := u.repo.Update(backup); err != nil {
		u.log.WithFields(map[string]interface{}{
			"backup_id": backup.ID,
			"error":     err.Error(),
		}).Error("Failed to update backup record after failure")
	}
}

// RestoreBackup restores database from a backup
func (u *BackupUsecase) RestoreBackup(backupID uint) error {
	backup, err := u.repo.GetByID(backupID)
	if err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}

	if backup.Status != models.BackupStatusCompleted {
		return fmt.Errorf("cannot restore from backup with status: %s", backup.Status)
	}

	// Check if backup file exists
	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backup.FilePath)
	}

	u.log.WithFields(map[string]interface{}{
		"backup_id": backup.ID,
		"file_name": backup.FileName,
	}).Info("Starting database restore")

	// Execute psql command to restore
	cmd := exec.Command("psql",
		"-h", u.dbHost,
		"-p", u.dbPort,
		"-U", u.dbUser,
		"-d", u.dbName,
		"-f", backup.FilePath,
	)

	// Set PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", u.dbPassword))

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		u.log.WithFields(map[string]interface{}{
			"backup_id": backup.ID,
			"error":     err.Error(),
			"output":    string(output),
		}).Error("Database restore failed")
		return fmt.Errorf("psql restore failed: %v, output: %s", err, string(output))
	}

	u.log.WithFields(map[string]interface{}{
		"backup_id": backup.ID,
		"file_name": backup.FileName,
	}).Info("Database restored successfully")

	return nil
}

// GetBackup retrieves a backup by ID
func (u *BackupUsecase) GetBackup(id uint) (*models.Backup, error) {
	return u.repo.GetByID(id)
}

// ListBackups retrieves backups with pagination
func (u *BackupUsecase) ListBackups(page, pageSize int) (*models.BackupListResponse, error) {
	backups, total, err := u.repo.List(page, pageSize)
	if err != nil {
		return nil, err
	}

	// Convert to responses
	responses := make([]*models.BackupResponse, len(backups))
	for i, backup := range backups {
		responses[i] = backup.ToResponse()
	}

	// Calculate total pages
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &models.BackupListResponse{
		Backups:    responses,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// DeleteBackup deletes a backup and its file
func (u *BackupUsecase) DeleteBackup(id uint) error {
	backup, err := u.repo.GetByID(id)
	if err != nil {
		return err
	}

	// Delete physical file
	if err := os.Remove(backup.FilePath); err != nil && !os.IsNotExist(err) {
		u.log.WithFields(map[string]interface{}{
			"backup_id": backup.ID,
			"file_path": backup.FilePath,
			"error":     err.Error(),
		}).Warn("Failed to delete backup file")
	}

	// Delete database record
	return u.repo.Delete(id)
}

// GetStats retrieves backup statistics
func (u *BackupUsecase) GetStats() (*models.BackupStatsResponse, error) {
	return u.repo.GetStats()
}

// DownloadBackup returns the file path for downloading a backup
func (u *BackupUsecase) DownloadBackup(id uint) (string, error) {
	backup, err := u.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	if backup.Status != models.BackupStatusCompleted {
		return "", fmt.Errorf("cannot download backup with status: %s", backup.Status)
	}

	// Check if file exists
	if _, err := os.Stat(backup.FilePath); os.IsNotExist(err) {
		return "", fmt.Errorf("backup file not found")
	}

	return backup.FilePath, nil
}
