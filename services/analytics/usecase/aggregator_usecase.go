package usecase

import (
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/repository"
	sharedredis "tachyon-messenger/shared/redis"
)

// AggregatorUsecase handles data aggregation
type AggregatorUsecase struct {
	analyticsRepo *repository.AnalyticsRepository
	metricsRepo   *repository.MetricsRepository
	eventsRepo    *repository.EventsRepository
	redisClient   *sharedredis.Client
}

// NewAggregatorUsecase creates a new aggregator usecase
func NewAggregatorUsecase(
	analyticsRepo *repository.AnalyticsRepository,
	metricsRepo   *repository.MetricsRepository,
	eventsRepo    *repository.EventsRepository,
	redisClient   *sharedredis.Client,
) *AggregatorUsecase {
	return &AggregatorUsecase{
		analyticsRepo: analyticsRepo,
		metricsRepo:   metricsRepo,
		eventsRepo:    eventsRepo,
		redisClient:   redisClient,
	}
}

// AggregateHourlyMetrics aggregates metrics for the last hour
func (u *AggregatorUsecase) AggregateHourlyMetrics() error {
	now := time.Now()
	hourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	hourEnd := hourStart.Add(time.Hour)

	// Aggregate various metrics
	u.aggregateUserMetrics(hourStart, hourEnd, models.PeriodHour)
	u.aggregateMessageMetrics(hourStart, hourEnd, models.PeriodHour)
	u.aggregateTaskMetrics(hourStart, hourEnd, models.PeriodHour)

	return nil
}

// AggregateDailyMetrics aggregates metrics for the previous day
func (u *AggregatorUsecase) AggregateDailyMetrics() error {
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -1)
	dayEnd := dayStart.Add(24 * time.Hour)

	// Aggregate various metrics
	u.aggregateUserMetrics(dayStart, dayEnd, models.PeriodDay)
	u.aggregateMessageMetrics(dayStart, dayEnd, models.PeriodDay)
	u.aggregateTaskMetrics(dayStart, dayEnd, models.PeriodDay)

	// Clear cache (TODO: implement pattern deletion)
	// u.redisClient.DeletePattern("dashboard:*")

	return nil
}

// aggregateUserMetrics aggregates user-related metrics
func (u *AggregatorUsecase) aggregateUserMetrics(start, end time.Time, periodType string) error {
	// Count active users
	count, err := u.eventsRepo.GetUniqueUsersCount(start, end)
	if err != nil {
		return err
	}

	metric := &models.AggregatedMetric{
		MetricName:     models.MetricDailyActiveUsers,
		MetricCategory: models.CategoryUser,
		PeriodType:     periodType,
		PeriodStart:    start,
		PeriodEnd:      end,
		Value:          float64(count),
	}

	return u.metricsRepo.UpsertMetric(metric)
}

// aggregateMessageMetrics aggregates message-related metrics
func (u *AggregatorUsecase) aggregateMessageMetrics(start, end time.Time, periodType string) error {
	count, err := u.eventsRepo.CountEventsByType(models.EventMessageSent, start, end)
	if err != nil {
		return err
	}

	metric := &models.AggregatedMetric{
		MetricName:     models.MetricMessagesSent,
		MetricCategory: models.CategoryMessage,
		PeriodType:     periodType,
		PeriodStart:    start,
		PeriodEnd:      end,
		Value:          float64(count),
	}

	return u.metricsRepo.UpsertMetric(metric)
}

// aggregateTaskMetrics aggregates task-related metrics
func (u *AggregatorUsecase) aggregateTaskMetrics(start, end time.Time, periodType string) error {
	// Tasks created
	createdCount, _ := u.eventsRepo.CountEventsByType(models.EventTaskCreated, start, end)
	metricCreated := &models.AggregatedMetric{
		MetricName:     models.MetricTasksCreated,
		MetricCategory: models.CategoryTask,
		PeriodType:     periodType,
		PeriodStart:    start,
		PeriodEnd:      end,
		Value:          float64(createdCount),
	}
	u.metricsRepo.UpsertMetric(metricCreated)

	// Tasks completed
	completedCount, _ := u.eventsRepo.CountEventsByType(models.EventTaskCompleted, start, end)
	metricCompleted := &models.AggregatedMetric{
		MetricName:     models.MetricTasksCompleted,
		MetricCategory: models.CategoryTask,
		PeriodType:     periodType,
		PeriodStart:    start,
		PeriodEnd:      end,
		Value:          float64(completedCount),
	}
	u.metricsRepo.UpsertMetric(metricCompleted)

	return nil
}

// CleanupOldEvents removes events older than specified days
func (u *AggregatorUsecase) CleanupOldEvents(retentionDays int) error {
	return u.eventsRepo.DeleteOldEvents(retentionDays)
}
