package database

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// SyncRepository provides methods for incremental sync operations
type SyncRepository struct {
	db *gorm.DB
}

// NewSyncRepository creates a new sync repository
func NewSyncRepository(db *DB) *SyncRepository {
	return &SyncRepository{db: db.DB}
}

// RecordDeletion records a deleted entity for sync tracking
func (r *SyncRepository) RecordDeletion(entityType string, entityID uint, deletedBy *uint) error {
	record := models.DeletedRecord{
		EntityType: entityType,
		EntityID:   entityID,
		DeletedAt:  time.Now(),
		DeletedBy:  deletedBy,
	}
	return r.db.Create(&record).Error
}

// GetDeletedIDsSince returns IDs of deleted entities since the given timestamp
func (r *SyncRepository) GetDeletedIDsSince(entityType string, since time.Time) ([]uint, error) {
	var records []models.DeletedRecord
	err := r.db.
		Where("entity_type = ? AND deleted_at > ?", entityType, since).
		Select("entity_id").
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(records))
	for i, record := range records {
		ids[i] = record.EntityID
	}
	return ids, nil
}

// CleanupOldRecords removes deleted records older than the given duration
// This should be called periodically to prevent the table from growing indefinitely
func (r *SyncRepository) CleanupOldRecords(olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)
	result := r.db.
		Where("deleted_at < ?", cutoffTime).
		Delete(&models.DeletedRecord{})
	return result.RowsAffected, result.Error
}

// MigrateDeletedRecords runs migration for the deleted_records table
func (r *SyncRepository) MigrateDeletedRecords() error {
	return r.db.AutoMigrate(&models.DeletedRecord{})
}

// Entity type constants for consistency across services
const (
	EntityTypeTask    = "task"
	EntityTypeChat    = "chat"
	EntityTypePoll    = "poll"
	EntityTypeEvent   = "event"
	EntityTypeMessage = "message"
)
