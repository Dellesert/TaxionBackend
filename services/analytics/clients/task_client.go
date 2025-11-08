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
func (c *TaskClient) GetTaskStats() (*TaskStatsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/internal/tasks/stats", c.baseURL)

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
