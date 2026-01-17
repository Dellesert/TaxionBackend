package models

// DashboardRequest represents request parameters for dashboard endpoint
type DashboardRequest struct {
	Limit int `form:"limit" binding:"omitempty,min=1,max=20"`
}

// DashboardCounts represents counts for each category
type DashboardCounts struct {
	NewTasksCount     int64 `json:"new_tasks_count"`
	ActiveTasksCount  int64 `json:"active_tasks_count"`
	OverdueTasksCount int64 `json:"overdue_tasks_count"`
	PendingPollsCount int64 `json:"pending_polls_count"`
}

// PendingPollResponse represents a poll that user hasn't voted on yet
type PendingPollResponse struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	CreatedBy   uint   `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	EndTime     string `json:"end_time,omitempty"`
}

// DashboardResponse represents the complete dashboard data
type DashboardResponse struct {
	NewTasks     []*TaskResponse        `json:"new_tasks"`
	ActiveTasks  []*TaskResponse        `json:"active_tasks"`
	OverdueTasks []*TaskResponse        `json:"overdue_tasks"`
	PendingPolls []*PendingPollResponse `json:"pending_polls"`
	Counts       DashboardCounts        `json:"counts"`
}
