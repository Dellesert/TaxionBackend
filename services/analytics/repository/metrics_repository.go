package repository

import (
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/shared/database"
)

// MetricsRepository handles metrics data access
type MetricsRepository struct {
	db *database.DB
}

// NewMetricsRepository creates a new metrics repository
func NewMetricsRepository(db *database.DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

// UpsertMetric creates or updates a metric
func (r *MetricsRepository) UpsertMetric(metric *models.AggregatedMetric) error {
	// Check if metric exists
	var existing models.AggregatedMetric
	result := r.db.DB.Where(
		"metric_name = ? AND period_type = ? AND period_start = ? AND COALESCE(department_id, 0) = COALESCE(?, 0)",
		metric.MetricName, metric.PeriodType, metric.PeriodStart, metric.DepartmentID,
	).First(&existing)

	if result.Error == nil {
		// Update existing
		metric.ID = existing.ID
		return r.db.DB.Save(metric).Error
	}

	// Create new
	return r.db.DB.Create(metric).Error
}

// GetMetric gets a specific metric
func (r *MetricsRepository) GetMetric(name, periodType string, periodStart time.Time, departmentID *uint64) (*models.AggregatedMetric, error) {
	var metric models.AggregatedMetric
	err := r.db.DB.Where(
		"metric_name = ? AND period_type = ? AND period_start = ? AND COALESCE(department_id, 0) = COALESCE(?, 0)",
		name, periodType, periodStart, departmentID,
	).First(&metric).Error
	return &metric, err
}

// GetMetricsByName gets all metrics by name within time range
func (r *MetricsRepository) GetMetricsByName(name string, start, end time.Time) ([]*models.AggregatedMetric, error) {
	var metrics []*models.AggregatedMetric
	err := r.db.DB.
		Where("metric_name = ? AND period_start >= ? AND period_start <= ?", name, start, end).
		Order("period_start ASC").
		Find(&metrics).Error
	return metrics, err
}

// GetMetricsByCategory gets metrics by category
func (r *MetricsRepository) GetMetricsByCategory(category string, start, end time.Time) ([]*models.AggregatedMetric, error) {
	var metrics []*models.AggregatedMetric
	err := r.db.DB.
		Where("metric_category = ? AND period_start >= ? AND period_start <= ?", category, start, end).
		Order("period_start ASC").
		Find(&metrics).Error
	return metrics, err
}

// GetLatestMetric gets the most recent metric value
func (r *MetricsRepository) GetLatestMetric(name string, departmentID *uint64) (*models.AggregatedMetric, error) {
	var metric models.AggregatedMetric
	err := r.db.DB.
		Where("metric_name = ? AND COALESCE(department_id, 0) = COALESCE(?, 0)", name, departmentID).
		Order("period_start DESC").
		First(&metric).Error
	return &metric, err
}
