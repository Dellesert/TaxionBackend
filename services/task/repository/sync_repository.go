package repository

import (
	"time"

	"tachyon-messenger/shared/database"
	sharedmodels "tachyon-messenger/shared/models"
)

// SyncRepository defines the interface for sync-related data operations
type SyncRepository interface {
	// RecordDeletion records a deleted task for sync tracking
	RecordDeletion(taskID uint, deletedBy *uint) error

	// GetDeletedIDsSince returns IDs of deleted tasks since the given timestamp
	GetDeletedIDsSince(since time.Time) ([]uint, error)
}

// syncRepository implements SyncRepository interface
type syncRepository struct {
	db *database.DB
}

// NewSyncRepository creates a new sync repository
func NewSyncRepository(db *database.DB) SyncRepository {
	return &syncRepository{db: db}
}

// RecordDeletion records a deleted task for sync tracking
func (r *syncRepository) RecordDeletion(taskID uint, deletedBy *uint) error {
	record := sharedmodels.DeletedRecord{
		EntityType: database.EntityTypeTask,
		EntityID:   taskID,
		DeletedAt:  time.Now(),
		DeletedBy:  deletedBy,
	}
	return r.db.Create(&record).Error
}

// GetDeletedIDsSince returns IDs of deleted tasks since the given timestamp
func (r *syncRepository) GetDeletedIDsSince(since time.Time) ([]uint, error) {
	var records []sharedmodels.DeletedRecord
	err := r.db.
		Where("entity_type = ? AND deleted_at > ?", database.EntityTypeTask, since).
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
