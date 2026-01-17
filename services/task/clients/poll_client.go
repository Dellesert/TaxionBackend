package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"tachyon-messenger/shared/logger"
)

// PendingPollResponse represents a poll that user hasn't voted on yet
type PendingPollResponse struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	CreatedBy   uint    `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	EndTime     *string `json:"end_time,omitempty"`
}

// PollClient is HTTP client for poll-service
type PollClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPollClient creates a new poll service client
func NewPollClient() *PollClient {
	pollServiceURL := os.Getenv("POLL_SERVICE_URL")
	if pollServiceURL == "" {
		pollServiceURL = "http://poll-service:8085"
	}

	return &PollClient{
		baseURL: pollServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetPendingPollsForUser retrieves active polls where user hasn't voted
func (c *PollClient) GetPendingPollsForUser(userID uint, limit int) ([]*PendingPollResponse, int64, error) {
	url := fmt.Sprintf("%s/internal/polls/pending?user_id=%d&limit=%d", c.baseURL, userID, limit)

	logger.WithFields(map[string]interface{}{
		"url":     url,
		"user_id": userID,
		"limit":   limit,
	}).Debug("Requesting pending polls from poll-service")

	resp, err := c.httpClient.Get(url)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Error("Failed to request pending polls from poll-service")
		return nil, 0, fmt.Errorf("failed to request pending polls: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.WithFields(map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("Poll service returned non-OK status")
		return nil, 0, fmt.Errorf("poll service returned status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Polls []*PendingPollResponse `json:"polls"`
		Total int64                  `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to decode poll service response")
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":        userID,
		"received_count": len(response.Polls),
		"total":          response.Total,
	}).Debug("Successfully retrieved pending polls from poll-service")

	return response.Polls, response.Total, nil
}
