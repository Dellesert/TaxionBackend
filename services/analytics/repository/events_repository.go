package repository

import (
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/shared/database"
)

// EventsRepository handles event data access
type EventsRepository struct {
	db *database.DB
}

// NewEventsRepository creates a new events repository
func NewEventsRepository(db *database.DB) *EventsRepository {
	return &EventsRepository{db: db}
}

// CreateEvent creates a new analytics event
func (r *EventsRepository) CreateEvent(event *models.AnalyticsEvent) error {
	return r.db.DB.Create(event).Error
}

// CreateEventsBatch creates multiple events in a single transaction
func (r *EventsRepository) CreateEventsBatch(events []*models.AnalyticsEvent) error {
	return r.db.DB.CreateInBatches(events, 100).Error
}

// GetEventsByType gets events by type within a time range
func (r *EventsRepository) GetEventsByType(eventType string, start, end time.Time) ([]*models.AnalyticsEvent, error) {
	var events []*models.AnalyticsEvent
	err := r.db.DB.
		Where("event_type = ? AND timestamp >= ? AND timestamp <= ?", eventType, start, end).
		Order("timestamp DESC").
		Find(&events).Error
	return events, err
}

// GetEventsByCategory gets events by category within a time range
func (r *EventsRepository) GetEventsByCategory(category string, start, end time.Time) ([]*models.AnalyticsEvent, error) {
	var events []*models.AnalyticsEvent
	err := r.db.DB.
		Where("event_category = ? AND timestamp >= ? AND timestamp <= ?", category, start, end).
		Order("timestamp DESC").
		Find(&events).Error
	return events, err
}

// GetEventsByUser gets events for a specific user
func (r *EventsRepository) GetEventsByUser(userID uint64, start, end time.Time) ([]*models.AnalyticsEvent, error) {
	var events []*models.AnalyticsEvent
	err := r.db.DB.
		Where("user_id = ? AND timestamp >= ? AND timestamp <= ?", userID, start, end).
		Order("timestamp DESC").
		Find(&events).Error
	return events, err
}

// CountEventsByType counts events by type
func (r *EventsRepository) CountEventsByType(eventType string, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.AnalyticsEvent{}).
		Where("event_type = ? AND timestamp >= ? AND timestamp <= ?", eventType, start, end).
		Count(&count).Error
	return count, err
}

// CountEventsByCategory counts events by category
func (r *EventsRepository) CountEventsByCategory(category string, start, end time.Time) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.AnalyticsEvent{}).
		Where("event_category = ? AND timestamp >= ? AND timestamp <= ?", category, start, end).
		Count(&count).Error
	return count, err
}

// DeleteOldEvents deletes events older than the specified number of days
func (r *EventsRepository) DeleteOldEvents(days int) error {
	cutoffDate := time.Now().AddDate(0, 0, -days)
	return r.db.DB.
		Where("timestamp < ?", cutoffDate).
		Delete(&models.AnalyticsEvent{}).Error
}

// GetUniqueUsersCount returns count of unique users in time range
func (r *EventsRepository) GetUniqueUsersCount(start, end time.Time) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.AnalyticsEvent{}).
		Where("user_id IS NOT NULL AND timestamp >= ? AND timestamp <= ?", start, end).
		Distinct("user_id").
		Count(&count).Error
	return count, err
}
