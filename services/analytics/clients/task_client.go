package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"
)

// TaskClient is a client for the Task Service
type TaskClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

// TaskStatsResponse represents the response from the Task Service internal stats endpoint
type TaskStatsResponse struct {
	TotalTasks      int `json:"total_tasks"`
	NewTasks        int `json:"new_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	ReviewTasks     int `json:"review_tasks"`
	CompletedTasks  int `json:"completed_tasks"`
	CancelledTasks  int `json:"cancelled_tasks"`
	OverdueTasks    int `json:"overdue_tasks"`
}

// DepartmentTaskStats represents task statistics by department
type DepartmentTaskStats struct {
	DepartmentID      uint    `json:"department_id"`
	DepartmentName    string  `json:"department_name"`
	TotalTasks        int     `json:"total_tasks"`
	CompletedTasks    int     `json:"completed_tasks"`
	InProgressTasks   int     `json:"in_progress_tasks"`
	OverdueTasks      int     `json:"overdue_tasks"`
	CompletionRate    float64 `json:"completion_rate"`
	AvgCompletionTime float64 `json:"avg_completion_time"`
	EmployeeCount     int     `json:"employee_count"`
}

// EmployeePerformance represents employee task performance
type EmployeePerformance struct {
	UserID            uint    `json:"user_id"`
	UserName          string  `json:"user_name"`
	DepartmentID      *uint   `json:"department_id,omitempty"`
	DepartmentName    string  `json:"department_name,omitempty"`
	TasksCreated      int     `json:"tasks_created"`
	TasksCompleted    int     `json:"tasks_completed"`
	TasksInProgress   int     `json:"tasks_in_progress"`
	TasksOverdue      int     `json:"tasks_overdue"`
	CompletionRate    float64 `json:"completion_rate"`
	AvgCompletionTime float64 `json:"avg_completion_time"`
	QualityScore      float64 `json:"quality_score"`
}

// TrendDataPoint represents a point in task trend data
type TrendDataPoint struct {
	Date      string `json:"date"`
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
	Overdue   int    `json:"overdue"`
}

// PriorityDistribution represents task distribution by priority
type PriorityDistribution struct {
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
}

// NewTaskClient creates a new Task Service client
func NewTaskClient(baseURL string, log *logger.Logger) *TaskClient {
	return &TaskClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		log: log,
	}
}

// GetTaskStats fetches task statistics from the Task Service
func (c *TaskClient) GetTaskStats(period string) (*TaskStatsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/stats", c.baseURL)
	if period != "" {
		url = fmt.Sprintf("%s?period=%s", url, period)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var stats TaskStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stats, nil
}

// GetDepartmentTaskStats fetches task statistics grouped by department
func (c *TaskClient) GetDepartmentTaskStats(period string) ([]*DepartmentTaskStats, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/analytics/departments?period=%s", c.baseURL, period)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data []*DepartmentTaskStats `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetTopPerformers fetches top performing employees
func (c *TaskClient) GetTopPerformers(limit int, period string, departmentID *uint) ([]*EmployeePerformance, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/analytics/top-performers?limit=%d&period=%s", c.baseURL, limit, period)
	if departmentID != nil {
		url = fmt.Sprintf("%s&department_id=%d", url, *departmentID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data []*EmployeePerformance `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetTaskTrends fetches task completion trends over time
func (c *TaskClient) GetTaskTrends(period string, interval string) ([]*TrendDataPoint, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/analytics/trends?period=%s&interval=%s", c.baseURL, period, interval)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data []*TrendDataPoint `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}

// GetPriorityDistribution fetches task distribution by priority
func (c *TaskClient) GetPriorityDistribution(period string) (*PriorityDistribution, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/analytics/priority-distribution?period=%s", c.baseURL, period)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data *PriorityDistribution `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data, nil
}
