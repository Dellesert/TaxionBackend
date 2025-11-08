package repository

import (
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/shared/database"
)

// AnalyticsRepository handles analytics data access
type AnalyticsRepository struct {
	db *database.DB
}

// NewAnalyticsRepository creates a new analytics repository
func NewAnalyticsRepository(db *database.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// GetUserActivity gets user activity for a date range
func (r *AnalyticsRepository) GetUserActivity(start, end time.Time) ([]*models.UserActivity, error) {
	var activities []*models.UserActivity
	err := r.db.DB.
		Where("date >= ? AND date <= ?", start, end).
		Order("date DESC").
		Find(&activities).Error
	return activities, err
}

// UpsertUserActivity creates or updates user activity
func (r *AnalyticsRepository) UpsertUserActivity(activity *models.UserActivity) error {
	var existing models.UserActivity
	result := r.db.DB.Where("user_id = ? AND date = ?", activity.UserID, activity.Date).First(&existing)

	if result.Error == nil {
		// Update existing
		activity.ID = existing.ID
		return r.db.DB.Save(activity).Error
	}

	// Create new
	return r.db.DB.Create(activity).Error
}

// GetDepartmentStats gets department stats for a date range
func (r *AnalyticsRepository) GetDepartmentStats(start, end time.Time) ([]*models.DepartmentStats, error) {
	var stats []*models.DepartmentStats
	err := r.db.DB.
		Where("date >= ? AND date <= ?", start, end).
		Order("date DESC").
		Find(&stats).Error
	return stats, err
}

// UpsertDepartmentStats creates or updates department stats
func (r *AnalyticsRepository) UpsertDepartmentStats(stats *models.DepartmentStats) error {
	var existing models.DepartmentStats
	result := r.db.DB.Where("department_id = ? AND date = ?", stats.DepartmentID, stats.Date).First(&existing)

	if result.Error == nil {
		// Update existing
		stats.ID = existing.ID
		return r.db.DB.Save(stats).Error
	}

	// Create new
	return r.db.DB.Create(stats).Error
}

// GetTopActiveUsers gets top N active users
func (r *AnalyticsRepository) GetTopActiveUsers(start, end time.Time, limit int) ([]*models.UserActivity, error) {
	var activities []*models.UserActivity
	err := r.db.DB.
		Where("date >= ? AND date <= ?", start, end).
		Order("messages_sent DESC, tasks_completed DESC").
		Limit(limit).
		Find(&activities).Error
	return activities, err
}

// GetDepartmentStatsForPeriod gets aggregated stats for a department
func (r *AnalyticsRepository) GetDepartmentStatsForPeriod(departmentID uint64, start, end time.Time) (*models.DepartmentStats, error) {
	var stats models.DepartmentStats
	err := r.db.DB.Model(&models.DepartmentStats{}).
		Select("SUM(active_users) as active_users, SUM(total_messages) as total_messages, SUM(total_tasks) as total_tasks, SUM(completed_tasks) as completed_tasks, SUM(total_events) as total_events").
		Where("department_id = ? AND date >= ? AND date <= ?", departmentID, start, end).
		Scan(&stats).Error
	return &stats, err
}
