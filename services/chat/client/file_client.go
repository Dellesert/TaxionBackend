package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// FileInfo represents file information from file-service
type FileInfo struct {
	ID           uint   `json:"id"`
	FileName     string `json:"file_name"`
	OriginalName string `json:"original_name"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	FileType     string `json:"file_type"`
	FileURL      string `json:"url"` // Changed from file_url to url to match FileResponse
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	UploadedBy   uint   `json:"uploaded_by"`
}

// FileClient handles communication with file-service
type FileClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewFileClient creates a new file service client
func NewFileClient() *FileClient {
	baseURL := os.Getenv("FILE_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://file-service:8088"
	}

	return &FileClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetFileByID retrieves file information by ID from file-service
func (c *FileClient) GetFileByID(fileID uint, userID uint) (*FileInfo, error) {
	// Use internal endpoint which doesn't require authentication
	url := fmt.Sprintf("%s/api/v1/internal/files/%d", c.baseURL, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add user_id to context (file-service needs this for access control)
	// In a real scenario, you'd pass JWT token here
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call file-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found")
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("access denied to file")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var fileInfo FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&fileInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &fileInfo, nil
}

// GetMultipleFiles retrieves information for multiple files
func (c *FileClient) GetMultipleFiles(fileIDs []uint, userID uint) ([]*FileInfo, error) {
	files := make([]*FileInfo, 0, len(fileIDs))

	for _, fileID := range fileIDs {
		fileInfo, err := c.GetFileByID(fileID, userID)
		if err != nil {
			// Log error but continue with other files
			fmt.Printf("⚠️ Failed to fetch file %d: %v\n", fileID, err)
			continue
		}
		files = append(files, fileInfo)
	}

	return files, nil
}
