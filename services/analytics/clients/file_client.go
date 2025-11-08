package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"
)

// FileClient is a client for the File Service
type FileClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

// FileStatsResponse represents the response from the File Service internal stats endpoint
type FileStatsResponse struct {
	TotalFiles  int   `json:"total_files"`
	TotalSize   int64 `json:"total_size"`
	AvgFileSize int64 `json:"avg_file_size"`
}

// NewFileClient creates a new File Service client
func NewFileClient(baseURL string, log *logger.Logger) *FileClient {
	return &FileClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		log: log,
	}
}

// GetFileStats fetches file statistics from the File Service
func (c *FileClient) GetFileStats() (*FileStatsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/internal/files/stats", c.baseURL)

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

	var stats FileStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &stats, nil
}
