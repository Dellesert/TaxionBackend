package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/logger"
)

// SearchClient is the HTTP client for search-service
type SearchClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSearchClient creates a new search service client
func NewSearchClient() *SearchClient {
	searchServiceURL := os.Getenv("SEARCH_SERVICE_URL")
	if searchServiceURL == "" {
		searchServiceURL = "http://search-service:8090"
	}

	return &SearchClient{
		baseURL: searchServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IndexRequest represents a document to be indexed
type IndexRequest struct {
	EntityType   string                 `json:"entity_type"`
	EntityID     uint                   `json:"entity_id"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	Metadata     map[string]interface{} `json:"metadata"`
	AccessibleBy []uint                 `json:"accessible_by"`
	IsPublic     bool                   `json:"is_public"`
	CreatorID    uint                   `json:"creator_id"`
}

// BulkIndexRequest represents a bulk index request
type BulkIndexRequest struct {
	Documents []IndexRequest `json:"documents"`
}

// DeleteRequest represents a document to be deleted from the index
type DeleteRequest struct {
	EntityType string `json:"entity_type"`
	EntityID   uint   `json:"entity_id"`
}

// IndexDocument sends a document to the search service for indexing (async, fire-and-forget)
func (c *SearchClient) IndexDocument(req *IndexRequest) {
	go func() {
		if err := c.indexDocumentSync(req); err != nil {
			logger.WithFields(map[string]interface{}{
				"entity_type": req.EntityType,
				"entity_id":   req.EntityID,
				"error":       err.Error(),
			}).Warn("Failed to index document in search service")
		}
	}()
}

// BulkIndex sends multiple documents for indexing (async, fire-and-forget)
func (c *SearchClient) BulkIndex(req *BulkIndexRequest) {
	go func() {
		if err := c.bulkIndexSync(req); err != nil {
			logger.WithFields(map[string]interface{}{
				"document_count": len(req.Documents),
				"error":          err.Error(),
			}).Warn("Failed to bulk index documents in search service")
		}
	}()
}

// DeleteDocument removes a document from the search index (async, fire-and-forget)
func (c *SearchClient) DeleteDocument(entityType string, entityID uint) {
	go func() {
		if err := c.deleteDocumentSync(entityType, entityID); err != nil {
			logger.WithFields(map[string]interface{}{
				"entity_type": entityType,
				"entity_id":   entityID,
				"error":       err.Error(),
			}).Warn("Failed to delete document from search service")
		}
	}()
}

func (c *SearchClient) indexDocumentSync(req *IndexRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/search/index", c.baseURL)

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search service returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *SearchClient) bulkIndexSync(req *BulkIndexRequest) error {
	url := fmt.Sprintf("%s/api/v1/internal/search/bulk-index", c.baseURL)

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search service returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *SearchClient) deleteDocumentSync(entityType string, entityID uint) error {
	url := fmt.Sprintf("%s/api/v1/internal/search/index", c.baseURL)

	req := &DeleteRequest{
		EntityType: entityType,
		EntityID:   entityID,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("DELETE", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("search service returned status %d", resp.StatusCode)
	}
	return nil
}
