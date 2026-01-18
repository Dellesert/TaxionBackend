package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/logger"
)

// FileClient handles communication with file service
type FileClient struct {
	baseURL    string
	httpClient *http.Client
}

// FileMetadata represents file metadata from file service
type FileMetadata struct {
	ID          string    `json:"id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	MimeType    string    `json:"mime_type"`
	UploadedBy  uint      `json:"uploaded_by"`
	UploadedAt  time.Time `json:"uploaded_at"`
	IsPublic    bool      `json:"is_public"`
	URL         string    `json:"url,omitempty"`
}

// NewFileClient creates a new file service client
func NewFileClient() *FileClient {
	baseURL := os.Getenv("FILE_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8087" // Default file service URL
	}

	return &FileClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetFileMetadata retrieves file metadata by ID
func (c *FileClient) GetFileMetadata(fileID string, userID uint) (*FileMetadata, error) {
	url := fmt.Sprintf("%s/api/v1/files/%s/metadata", c.baseURL, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		File *FileMetadata `json:"file"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.File, nil
}

// DownloadFile downloads file content by ID
func (c *FileClient) DownloadFile(fileID string, userID uint) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/files/%s/download", c.baseURL, fileID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file service returned status %d: %s", resp.StatusCode, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}

	return content, nil
}

// ValidateFileType checks if file has valid mime type for schedule import
func (c *FileClient) ValidateFileType(metadata *FileMetadata) error {
	validTypes := map[string]bool{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true, // .docx
		"application/msword": true, // .doc (legacy)
	}

	if !validTypes[metadata.MimeType] {
		return fmt.Errorf("invalid file type: %s. Only Word documents (.docx, .doc) are supported", metadata.MimeType)
	}

	return nil
}

// DownloadAndValidate downloads file and validates it's a Word document
func (c *FileClient) DownloadAndValidate(fileID string, userID uint) ([]byte, *FileMetadata, error) {
	// Get metadata
	metadata, err := c.GetFileMetadata(fileID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// Validate file type
	if err := c.ValidateFileType(metadata); err != nil {
		return nil, nil, err
	}

	// Download file
	content, err := c.DownloadFile(fileID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download file: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"file_id":   fileID,
		"file_name": metadata.FileName,
		"file_size": len(content),
	}).Info("File downloaded successfully for schedule import")

	return content, metadata, nil
}

// UploadFile uploads file to file service (for testing or exporting)
func (c *FileClient) UploadFile(fileName string, content []byte, userID uint) (*FileMetadata, error) {
	url := fmt.Sprintf("%s/api/v1/files/upload", c.baseURL)

	// Create multipart form data
	body := &bytes.Buffer{}
	// Note: In real implementation, you would use multipart/form-data
	// For simplicity, this is a basic implementation

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))
	req.Header.Set("Content-Type", "multipart/form-data")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		File *FileMetadata `json:"file"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.File, nil
}
