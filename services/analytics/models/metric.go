package models

import (
	"time"

	"gorm.io/gorm"
)

// AggregatedMetric represents aggregated analytics metrics
type AggregatedMetric struct {
	ID             uint64                 `gorm:"primaryKey;autoIncrement" json:"id"`
	MetricName     string                 `gorm:"type:varchar(100);not null;uniqueIndex:idx_unique_metric" json:"metric_name"`
	MetricCategory string                 `gorm:"type:varchar(50);not null;index:idx_metrics_category_period" json:"metric_category"`
	PeriodType     string                 `gorm:"type:varchar(20);not null;uniqueIndex:idx_unique_metric" json:"period_type"`
	PeriodStart    time.Time              `gorm:"not null;uniqueIndex:idx_unique_metric;index:idx_metrics_name_period,idx_metrics_category_period" json:"period_start"`
	PeriodEnd      time.Time              `gorm:"not null" json:"period_end"`
	Value          float64                `gorm:"not null" json:"value"`
	DepartmentID   *uint64                `gorm:"uniqueIndex:idx_unique_metric" json:"department_id,omitempty"`
	Metadata       map[string]interface{} `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt      time.Time              `gorm:"not null" json:"created_at"`
	UpdatedAt      time.Time              `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for AggregatedMetric
func (AggregatedMetric) TableName() string {
	return "aggregated_metrics"
}

// BeforeCreate hook
func (m *AggregatedMetric) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook
func (m *AggregatedMetric) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// Period types
const (
	PeriodHour  = "hour"
	PeriodDay   = "day"
	PeriodWeek  = "week"
	PeriodMonth = "month"
	PeriodYear  = "year"
)

// Metric names
const (
	// User metrics
	MetricDailyActiveUsers   = "daily_active_users"
	MetricWeeklyActiveUsers  = "weekly_active_users"
	MetricMonthlyActiveUsers = "monthly_active_users"
	MetricNewRegistrations   = "new_registrations"
	MetricTotalLogins        = "total_logins"
	MetricAverageSessionTime = "average_session_time"

	// Message metrics
	MetricMessagesSent       = "messages_sent"
	MetricMessagesRead       = "messages_read"
	MetricAverageMessageLength = "average_message_length"
	MetricActiveChats        = "active_chats"
	MetricNewChannels        = "new_channels"

	// Task metrics
	MetricTasksCreated      = "tasks_created"
	MetricTasksCompleted    = "tasks_completed"
	MetricTasksInProgress   = "tasks_in_progress"
	MetricTasksOverdue      = "tasks_overdue"
	MetricAverageTaskTime   = "average_task_completion_time"
	MetricTaskCompletionRate = "task_completion_rate"

	// Calendar metrics
	MetricEventsCreated     = "events_created"
	MetricEventsAttended    = "events_attended"
	MetricAverageAttendance = "average_attendance"
	MetricUpcomingEvents    = "upcoming_events"

	// Poll metrics
	MetricPollsCreated      = "polls_created"
	MetricPollsCompleted    = "polls_completed"
	MetricAverageVotes      = "average_votes"
	MetricPollParticipation = "poll_participation_rate"

	// File metrics
	MetricFilesUploaded     = "files_uploaded"
	MetricFilesDownloaded   = "files_downloaded"
	MetricStorageUsed       = "storage_used"
	MetricAverageFileSize   = "average_file_size"

	// System metrics
	MetricAPIRequests       = "api_requests"
	MetricAPIErrors         = "api_errors"
	MetricAverageResponseTime = "average_response_time"
	MetricErrorRate         = "error_rate"

	// Security metrics
	MetricFailedLogins              = "failed_logins"
	MetricSuccessfulLogins          = "successful_logins"
	MetricLoginSuccessRate          = "login_success_rate"
	Metric2FAUsage                  = "2fa_usage_rate"
	MetricPasskeyUsage              = "passkey_usage_rate"
	MetricPasswordResets            = "password_resets"
	MetricSuspiciousActivities      = "suspicious_activities"
	MetricAccountLockouts           = "account_lockouts"
	MetricNewDeviceLogins           = "new_device_logins"
	MetricAverageSessionDuration    = "average_session_duration"
	MetricConcurrentSessions        = "concurrent_sessions"
	MetricUniqueIPsPerUser          = "unique_ips_per_user"
	MetricBruteForceAttempts        = "brute_force_attempts"
	MetricPasswordChanges           = "password_changes"
	MetricSessionExpirations        = "session_expirations"
)
