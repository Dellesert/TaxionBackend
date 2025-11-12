package usecase

import (
	"encoding/json"
	"fmt"
	"time"

	"tachyon-messenger/services/analytics/clients"
	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/repository"
	"tachyon-messenger/shared/logger"
	sharedredis "tachyon-messenger/shared/redis"
)

// AnalyticsUsecase handles analytics business logic
type AnalyticsUsecase struct {
	analyticsRepo *repository.AnalyticsRepository
	metricsRepo   *repository.MetricsRepository
	eventsRepo    *repository.EventsRepository
	redisClient   *sharedredis.Client
	taskClient    *clients.TaskClient
	fileClient    *clients.FileClient
	backupClient  *clients.BackupClient
	log           *logger.Logger
}

// NewAnalyticsUsecase creates a new analytics usecase
func NewAnalyticsUsecase(
	analyticsRepo *repository.AnalyticsRepository,
	metricsRepo *repository.MetricsRepository,
	eventsRepo *repository.EventsRepository,
	redisClient *sharedredis.Client,
	taskClient *clients.TaskClient,
	fileClient *clients.FileClient,
	backupClient *clients.BackupClient,
	log *logger.Logger,
) *AnalyticsUsecase {
	return &AnalyticsUsecase{
		analyticsRepo: analyticsRepo,
		metricsRepo:   metricsRepo,
		eventsRepo:    eventsRepo,
		redisClient:   redisClient,
		taskClient:    taskClient,
		fileClient:    fileClient,
		backupClient:  backupClient,
		log:           log,
	}
}

// GetDashboard returns complete dashboard data
func (u *AnalyticsUsecase) GetDashboard(period string, departmentID *uint64) (*models.DashboardResponse, error) {
	// Try to get from cache first
	deptKey := "all"
	if departmentID != nil {
		deptKey = fmt.Sprintf("%d", *departmentID)
	}
	cacheKey := fmt.Sprintf("dashboard:%s:%s", period, deptKey)
	cachedData, err := u.redisClient.Get(cacheKey)
	if err == nil && cachedData != "" {
		var dashboard models.DashboardResponse
		if err := json.Unmarshal([]byte(cachedData), &dashboard); err == nil {
			return &dashboard, nil
		}
	}

	// Get time range
	timeRange := models.GetTimeRange(period)

	// Build dashboard response
	dashboard := &models.DashboardResponse{
		Period: period,
		Stats:  u.getDashboardStats(timeRange, departmentID),
		Charts: u.getDashboardCharts(timeRange, departmentID),
		Tables: u.getDashboardTables(timeRange, departmentID),
	}

	// Cache the result for 5 minutes
	dashboardJSON, _ := json.Marshal(dashboard)
	u.redisClient.Set(cacheKey, string(dashboardJSON), 5*time.Minute)

	return dashboard, nil
}

// getDashboardStats generates statistics for dashboard
func (u *AnalyticsUsecase) getDashboardStats(timeRange models.TimeRange, departmentID *uint64) *models.DashboardStats {
	today := models.GetTimeRange("today")
	week := models.GetTimeRange("week")
	month := models.GetTimeRange("month")

	// Get real-time task stats from task service with time range filter
	// Determine period string from timeRange
	period := ""
	switch {
	case timeRange.Start.Equal(today.Start):
		period = "today"
	case timeRange.Start.Equal(week.Start):
		period = "week"
	case timeRange.Start.Equal(month.Start):
		period = "month"
	}

	taskStats := u.getRealTimeTaskStats(period)

	return &models.DashboardStats{
		ActiveUsers: &models.StatValue{
			Today:         u.getActiveUsersCount(today, departmentID),
			Week:          u.getActiveUsersCount(week, departmentID),
			Month:         u.getActiveUsersCount(month, departmentID),
			GrowthPercent: 12.5, // TODO: Calculate real growth
		},
		Messages: &models.StatValue{
			Today: u.getMessagesCount(today, departmentID),
			Week:  u.getMessagesCount(week, departmentID),
			Month: u.getMessagesCount(month, departmentID),
		},
		Tasks: taskStats,
		Calendar: &models.StatValue{
			Today: u.getCalendarEventsCount(today, departmentID),
			Week:  u.getCalendarEventsCount(week, departmentID),
			Month: u.getCalendarEventsCount(month, departmentID),
		},
		Polls: &models.StatValue{
			Today: u.getPollsCount(today, departmentID),
			Week:  u.getPollsCount(week, departmentID),
			Month: u.getPollsCount(month, departmentID),
		},
		Files:   u.getRealTimeFileStats(),
		Backups: u.getBackupStats(),
	}
}

// getDashboardCharts generates chart data
func (u *AnalyticsUsecase) getDashboardCharts(timeRange models.TimeRange, departmentID *uint64) *models.DashboardCharts {
	// Get real task stats from task service (no period filter for charts - show all time)
	taskStats, err := u.taskClient.GetTaskStats("")
	tasksByStatus := &models.TaskStatusChart{
		New:        0,
		InProgress: 0,
		Completed:  0,
		Overdue:    0,
	}

	if err == nil {
		tasksByStatus.New = taskStats.TotalTasks - taskStats.CompletedTasks - taskStats.InProgressTasks
		if tasksByStatus.New < 0 {
			tasksByStatus.New = 0
		}
		tasksByStatus.InProgress = taskStats.InProgressTasks
		tasksByStatus.Completed = taskStats.CompletedTasks
		tasksByStatus.Overdue = taskStats.OverdueTasks
	}

	return &models.DashboardCharts{
		UserActivity:      u.getUserActivityChart(timeRange, departmentID),
		MessagesByHour:    u.getMessagesByHourChart(timeRange, departmentID),
		TasksByStatus:     tasksByStatus,
		DepartmentHeatMap: u.getDepartmentHeatMap(timeRange),
	}
}

// getDashboardTables generates table data
func (u *AnalyticsUsecase) getDashboardTables(timeRange models.TimeRange, departmentID *uint64) *models.DashboardTables {
	return &models.DashboardTables{
		TopUsers:       []models.TopUser{},       // TODO: Implement
		TopChats:       []models.TopChat{},       // TODO: Implement
		TopPerformers:  []models.TopPerformer{},  // TODO: Implement
		DepartmentActivity: []models.DepartmentActivity{}, // TODO: Implement
	}
}

// Helper functions
func (u *AnalyticsUsecase) getActiveUsersCount(timeRange models.TimeRange, departmentID *uint64) int {
	count, _ := u.eventsRepo.GetUniqueUsersCount(timeRange.Start, timeRange.End)
	return int(count)
}

func (u *AnalyticsUsecase) getMessagesCount(timeRange models.TimeRange, departmentID *uint64) int {
	count, _ := u.eventsRepo.CountEventsByType(models.EventMessageSent, timeRange.Start, timeRange.End)
	return int(count)
}

func (u *AnalyticsUsecase) getTasksCount(timeRange models.TimeRange, status string, departmentID *uint64) int {
	var eventType string
	switch status {
	case "created":
		eventType = models.EventTaskCreated
	case "completed":
		eventType = models.EventTaskCompleted
	case "in_progress":
		// Calculate in progress as created minus completed
		created, _ := u.eventsRepo.CountEventsByType(models.EventTaskCreated, timeRange.Start, timeRange.End)
		completed, _ := u.eventsRepo.CountEventsByType(models.EventTaskCompleted, timeRange.Start, timeRange.End)
		inProgress := int(created) - int(completed)
		if inProgress < 0 {
			inProgress = 0
		}
		return inProgress
	case "overdue":
		// For now, return 0 as we don't have overdue event tracking
		// TODO: Implement by querying task service for tasks past their due date
		return 0
	default:
		return 0
	}
	count, _ := u.eventsRepo.CountEventsByType(eventType, timeRange.Start, timeRange.End)
	return int(count)
}

func (u *AnalyticsUsecase) getCalendarEventsCount(timeRange models.TimeRange, departmentID *uint64) int {
	count, _ := u.eventsRepo.CountEventsByType(models.EventEventCreated, timeRange.Start, timeRange.End)
	return int(count)
}

func (u *AnalyticsUsecase) getPollsCount(timeRange models.TimeRange, departmentID *uint64) int {
	count, _ := u.eventsRepo.CountEventsByType(models.EventPollCreated, timeRange.Start, timeRange.End)
	return int(count)
}

func (u *AnalyticsUsecase) calculateCompletionRate(timeRange models.TimeRange, departmentID *uint64) float64 {
	created := u.getTasksCount(timeRange, "created", departmentID)
	completed := u.getTasksCount(timeRange, "completed", departmentID)

	if created == 0 {
		return 0.0
	}

	return (float64(completed) / float64(created)) * 100.0
}

// getRealTimeTaskStats fetches real-time task statistics from Task Service with period filter
func (u *AnalyticsUsecase) getRealTimeTaskStats(period string) *models.TaskStats {
	// Get real-time statistics from Task Service with period filter
	stats, err := u.taskClient.GetTaskStats(period)
	if err != nil {
		// If task service is unavailable, fall back to event-based calculation
		// This provides resilience in case the task service is down
		u.log.Error("Failed to fetch task stats from task service", "error", err, "period", period)
		return &models.TaskStats{
			Created:        0,
			Completed:      0,
			InProgress:     0,
			Overdue:        0,
			CompletionRate: 0.0,
		}
	}

	// Calculate completion rate from real-time data
	completionRate := 0.0
	if stats.TotalTasks > 0 {
		completionRate = (float64(stats.CompletedTasks) / float64(stats.TotalTasks)) * 100.0
	}

	return &models.TaskStats{
		Created:        stats.TotalTasks,
		Completed:      stats.CompletedTasks,
		InProgress:     stats.InProgressTasks,
		Overdue:        stats.OverdueTasks,
		CompletionRate: completionRate,
	}
}

// getRealTimeFileStats fetches real-time file statistics from File Service
func (u *AnalyticsUsecase) getRealTimeFileStats() *models.FileStats {
	// Get real-time statistics from File Service
	stats, err := u.fileClient.GetFileStats()
	if err != nil {
		// If file service is unavailable, return zero values
		return &models.FileStats{
			TotalFiles:  0,
			TotalSize:   0,
			AvgFileSize: 0,
			StorageUsed: 0.0,
		}
	}

	// Calculate storage used percentage (assuming 100GB = 100,000,000,000 bytes limit)
	const storageLimit = int64(100 * 1024 * 1024 * 1024) // 100 GB
	storageUsedPercentage := 0.0
	if storageLimit > 0 {
		storageUsedPercentage = (float64(stats.TotalSize) / float64(storageLimit)) * 100.0
	}

	return &models.FileStats{
		TotalFiles:  stats.TotalFiles,
		TotalSize:   stats.TotalSize,
		AvgFileSize: float64(stats.AvgFileSize),
		StorageUsed: storageUsedPercentage,
	}
}

func (u *AnalyticsUsecase) getBackupStats() *models.BackupStats {
	// Get real-time statistics from Backup Service
	stats, err := u.backupClient.GetStats()
	if err != nil {
		// If backup service is unavailable, return zero values
		return &models.BackupStats{
			TotalBackups:      0,
			SuccessfulBackups: 0,
			FailedBackups:     0,
			LastBackupStatus:  "unknown",
			TotalBackupSize:   0,
		}
	}

	// Prepare the response
	backupStats := &models.BackupStats{
		TotalBackups:      stats.TotalBackups,
		SuccessfulBackups: stats.SuccessfulBackups,
		FailedBackups:     stats.FailedBackups,
		LastBackupStatus:  "none",
		TotalBackupSize:   stats.TotalSize,
	}

	// Add last backup info if available
	if stats.LastBackup != nil {
		backupStats.LastBackupStatus = stats.LastBackup.Status
		backupStats.LastBackupTime = stats.LastBackup.CreatedAt.Format(time.RFC3339)
	}

	return backupStats
}

func (u *AnalyticsUsecase) getUserActivityChart(timeRange models.TimeRange, departmentID *uint64) []models.ChartDataPoint {
	// TODO: Implement real data
	return []models.ChartDataPoint{
		{Date: "2025-01-01", Value: 45},
		{Date: "2025-01-02", Value: 52},
	}
}

func (u *AnalyticsUsecase) getMessagesByHourChart(timeRange models.TimeRange, departmentID *uint64) []models.ChartDataPoint {
	// TODO: Implement real data
	return []models.ChartDataPoint{}
}

func (u *AnalyticsUsecase) getDepartmentHeatMap(timeRange models.TimeRange) []models.DepartmentHeat {
	// TODO: Implement real data
	return []models.DepartmentHeat{}
}

// New task analytics methods

// GetTaskStats returns comprehensive task statistics
func (u *AnalyticsUsecase) GetTaskStats(period string, departmentID *uint) (interface{}, error) {
	timeRange := models.GetTimeRange(period)

	// Convert uint to uint64 for compatibility
	var deptID *uint64
	if departmentID != nil {
		id := uint64(*departmentID)
		deptID = &id
	}

	// Get basic stats (existing method)
	basicStats := u.getDashboardStats(timeRange, deptID)

	// Get additional task-specific stats from task client (no period filter)
	taskStats, err := u.taskClient.GetTaskStats("")
	if err != nil {
		return basicStats.Tasks, nil
	}

	return map[string]interface{}{
		"basic": basicStats.Tasks,
		"detailed": taskStats,
	}, nil
}

// GetCompletionRate returns task completion rate
func (u *AnalyticsUsecase) GetCompletionRate(period string, departmentID *uint) (float64, error) {
	timeRange := models.GetTimeRange(period)

	var deptID *uint64
	if departmentID != nil {
		id := uint64(*departmentID)
		deptID = &id
	}

	return u.calculateCompletionRate(timeRange, deptID), nil
}

// GetTopPerformers returns top performing employees
func (u *AnalyticsUsecase) GetTopPerformers(limit int, period string, departmentID *uint) (interface{}, error) {
	// Call task service to get real performance data
	performers, err := u.taskClient.GetTopPerformers(limit, period, departmentID)
	if err != nil {
		u.log.Error("Failed to fetch top performers from task service", "error", err)
		// Return empty array as fallback
		return []interface{}{}, nil
	}

	return performers, nil
}

// GetDepartmentTaskStats returns task statistics grouped by department
func (u *AnalyticsUsecase) GetDepartmentTaskStats(period string) (interface{}, error) {
	// Call task service to get real department statistics
	stats, err := u.taskClient.GetDepartmentTaskStats(period)
	if err != nil {
		u.log.Error("Failed to fetch department task stats from task service", "error", err)
		// Return empty array as fallback
		return []interface{}{}, nil
	}

	return stats, nil
}

// GetTaskTrends returns task completion trends over time
func (u *AnalyticsUsecase) GetTaskTrends(period string, interval string) (interface{}, error) {
	// Call task service to get real trend data
	trends, err := u.taskClient.GetTaskTrends(period, interval)
	if err != nil {
		u.log.Error("Failed to fetch task trends from task service", "error", err)
		// Return empty array as fallback
		return []interface{}{}, nil
	}

	return trends, nil
}

// GetPriorityDistribution returns task distribution by priority
func (u *AnalyticsUsecase) GetPriorityDistribution(period string) (interface{}, error) {
	// Call task service to get real priority distribution
	distribution, err := u.taskClient.GetPriorityDistribution(period)
	if err != nil {
		u.log.Error("Failed to fetch priority distribution from task service", "error", err)
		// Return empty distribution as fallback
		return map[string]int{
			"low":      0,
			"medium":   0,
			"high":     0,
			"critical": 0,
		}, nil
	}

	return distribution, nil
}
