package models

import "time"

// DashboardResponse represents the complete dashboard data
type DashboardResponse struct {
	Period string                 `json:"period"`
	Stats  *DashboardStats        `json:"stats"`
	Charts *DashboardCharts       `json:"charts"`
	Tables *DashboardTables       `json:"tables"`
}

// DashboardStats contains overview statistics
type DashboardStats struct {
	ActiveUsers *StatValue   `json:"active_users"`
	Messages    *StatValue   `json:"messages"`
	Tasks       *TaskStats   `json:"tasks"`
	Calendar    *StatValue   `json:"calendar"`
	Polls       *StatValue   `json:"polls"`
	Files       *FileStats   `json:"files"`
	Backups     *BackupStats `json:"backups"`
}

// StatValue represents a statistical value with comparisons
type StatValue struct {
	Today         int     `json:"today"`
	Week          int     `json:"week"`
	Month         int     `json:"month"`
	GrowthPercent float64 `json:"growth_percent"`
	AvgPerUser    float64 `json:"avg_per_user,omitempty"`
}

// TaskStats contains task-specific statistics
type TaskStats struct {
	Created        int     `json:"created"`
	Completed      int     `json:"completed"`
	InProgress     int     `json:"in_progress"`
	Overdue        int     `json:"overdue"`
	CompletionRate float64 `json:"completion_rate"`
}

// FileStats contains file storage statistics
type FileStats struct {
	TotalFiles    int     `json:"total_files"`
	TotalSize     int64   `json:"total_size"`
	AvgFileSize   float64 `json:"avg_file_size"`
	StorageUsed   float64 `json:"storage_used_percent"`
}

// BackupStats contains backup statistics
type BackupStats struct {
	TotalBackups     int     `json:"total_backups"`
	SuccessfulBackups int    `json:"successful_backups"`
	FailedBackups    int     `json:"failed_backups"`
	LastBackupStatus string  `json:"last_backup_status"`
	LastBackupTime   string  `json:"last_backup_time,omitempty"`
	TotalBackupSize  int64   `json:"total_backup_size"`
}

// DashboardCharts contains chart data
type DashboardCharts struct {
	UserActivity   []ChartDataPoint   `json:"user_activity"`
	MessagesByHour []ChartDataPoint   `json:"messages_by_hour"`
	TasksByStatus  *TaskStatusChart   `json:"tasks_by_status"`
	DepartmentHeatMap []DepartmentHeat `json:"department_heat_map"`
}

// ChartDataPoint represents a single point in a chart
type ChartDataPoint struct {
	Date  string  `json:"date"`
	Label string  `json:"label,omitempty"`
	Value float64 `json:"value"`
}

// TaskStatusChart represents task distribution by status
type TaskStatusChart struct {
	New        int `json:"new"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Overdue    int `json:"overdue"`
}

// DepartmentHeat represents activity heat for a department
type DepartmentHeat struct {
	DepartmentID   uint64  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	Activity       float64 `json:"activity"`
	ActiveUsers    int     `json:"active_users"`
}

// DashboardTables contains tabular data
type DashboardTables struct {
	TopUsers       []TopUser       `json:"top_users"`
	TopChats       []TopChat       `json:"top_chats"`
	TopPerformers  []TopPerformer  `json:"top_performers"`
	DepartmentActivity []DepartmentActivity `json:"department_activity"`
}

// TopUser represents a top active user
type TopUser struct {
	UserID       uint64 `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Messages     int    `json:"messages"`
	TasksCreated int    `json:"tasks_created"`
	TasksCompleted int  `json:"tasks_completed"`
	Department   string `json:"department"`
}

// TopChat represents a top active chat
type TopChat struct {
	ChatID       uint64 `json:"chat_id"`
	ChatName     string `json:"chat_name"`
	MessageCount int    `json:"message_count"`
	ActiveUsers  int    `json:"active_users"`
	LastActivity string `json:"last_activity"`
}

// TopPerformer represents a top task performer
type TopPerformer struct {
	UserID         uint64  `json:"user_id"`
	Name           string  `json:"name"`
	TasksCompleted int     `json:"tasks_completed"`
	AvgCompletionTime float64 `json:"avg_completion_time_hours"`
	OnTimeRate     float64 `json:"on_time_rate"`
}

// DepartmentActivity represents department activity stats
type DepartmentActivity struct {
	DepartmentID   uint64  `json:"department_id"`
	DepartmentName string  `json:"department_name"`
	ActiveUsers    int     `json:"active_users"`
	TotalUsers     int     `json:"total_users"`
	Messages       int     `json:"messages"`
	Tasks          int     `json:"tasks"`
	ActivityScore  float64 `json:"activity_score"`
}

// TimeRange represents a time period filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// GetTimeRange returns start and end time based on period
func GetTimeRange(period string) TimeRange {
	now := time.Now()

	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return TimeRange{Start: start, End: now}

	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, now.Location())
		end := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, now.Location())
		return TimeRange{Start: start, End: end}

	case "week":
		start := now.AddDate(0, 0, -7)
		return TimeRange{Start: start, End: now}

	case "month":
		start := now.AddDate(0, -1, 0)
		return TimeRange{Start: start, End: now}

	case "year":
		start := now.AddDate(-1, 0, 0)
		return TimeRange{Start: start, End: now}

	default: // default to week
		start := now.AddDate(0, 0, -7)
		return TimeRange{Start: start, End: now}
	}
}
