package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// BackupClient handles communication with Backup Service
type BackupClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewBackupClient creates a new backup service client
func NewBackupClient() *BackupClient {
	baseURL := os.Getenv("BACKUP_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://backup-service:8089"
	}

	return &BackupClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// BackupStats represents backup statistics from the service
type BackupStats struct {
	TotalBackups      int       `json:"total_backups"`
	SuccessfulBackups int       `json:"successful_backups"`
	FailedBackups     int       `json:"failed_backups"`
	PendingBackups    int       `json:"pending_backups"`
	InProgressBackups int       `json:"in_progress_backups"`
	LastBackup        *LastBackupInfo `json:"last_backup,omitempty"`
	TotalSize         int64     `json:"total_size"`
}

// LastBackupInfo represents information about the last backup
type LastBackupInfo struct {
	ID          uint      `json:"id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// GetStats fetches backup statistics
func (c *BackupClient) GetStats() (*BackupStats, error) {
	url := fmt.Sprintf("%s/api/v1/backups/stats", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch backup stats: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backup service returned status %d: %s", resp.StatusCode, string(body))
	}

	var stats BackupStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stats, nil
}
