package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"tachyon-messenger/services/backup/models"
	"tachyon-messenger/services/backup/usecase"

	"github.com/gin-gonic/gin"
)

// BackupHandler handles backup-related HTTP requests
type BackupHandler struct {
	backupUsecase *usecase.BackupUsecase
}

// NewBackupHandler creates a new backup handler
func NewBackupHandler(backupUsecase *usecase.BackupUsecase) *BackupHandler {
	return &BackupHandler{
		backupUsecase: backupUsecase,
	}
}

// CreateBackup handles creating a new backup
// @Summary Create a new database backup
// @Description Creates a new backup of the database
// @Tags backups
// @Accept json
// @Produce json
// @Param body body models.CreateBackupRequest true "Backup description"
// @Success 202 {object} models.BackupResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /backups [post]
func (h *BackupHandler) CreateBackup(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req models.CreateBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	backup, err := h.backupUsecase.CreateBackup(userID.(uint), models.BackupTypeManual, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, backup.ToResponse())
}

// ListBackups handles listing backups
// @Summary List all backups
// @Description Retrieves a list of all backups with pagination
// @Tags backups
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Success 200 {object} models.BackupListResponse
// @Failure 401 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /backups [get]
func (h *BackupHandler) ListBackups(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	response, err := h.backupUsecase.ListBackups(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBackup handles getting a single backup
// @Summary Get backup by ID
// @Description Retrieves details of a specific backup
// @Tags backups
// @Produce json
// @Param id path int true "Backup ID"
// @Success 200 {object} models.BackupResponse
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /backups/{id} [get]
func (h *BackupHandler) GetBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	backup, err := h.backupUsecase.GetBackup(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Backup not found"})
		return
	}

	c.JSON(http.StatusOK, backup.ToResponse())
}

// RestoreBackup handles restoring from a backup
// @Summary Restore database from backup
// @Description Restores the database from a specified backup
// @Tags backups
// @Produce json
// @Param id path int true "Backup ID"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /backups/{id}/restore [post]
func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	if err := h.backupUsecase.RestoreBackup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Database restored successfully",
	})
}

// DeleteBackup handles deleting a backup
// @Summary Delete backup
// @Description Deletes a backup and its file
// @Tags backups
// @Produce json
// @Param id path int true "Backup ID"
// @Success 200 {object} gin.H
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 404 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /backups/{id} [delete]
func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	if err := h.backupUsecase.DeleteBackup(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Backup deleted successfully",
	})
}

// DownloadBackup handles downloading a backup file
// @Summary Download backup file
// @Description Downloads the backup SQL file
// @Tags backups
// @Produce application/octet-stream
// @Param id path int true "Backup ID"
// @Success 200 {file} binary
// @Failure 400 {object} gin.H
// @Failure 401 {object} gin.H
// @Failure 404 {object} gin.H
// @Router /backups/{id}/download [get]
func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
		return
	}

	filePath, err := h.backupUsecase.DownloadBackup(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filePath))
	c.Header("Content-Type", "application/sql")
	c.File(filePath)
}

// GetStats handles getting backup statistics
// @Summary Get backup statistics
// @Description Retrieves statistics about backups
// @Tags backups
// @Produce json
// @Success 200 {object} models.BackupStatsResponse
// @Failure 401 {object} gin.H
// @Failure 500 {object} gin.H
// @Router /backups/stats [get]
func (h *BackupHandler) GetStats(c *gin.Context) {
	stats, err := h.backupUsecase.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
