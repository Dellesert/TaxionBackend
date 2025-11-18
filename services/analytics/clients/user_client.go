package clients

import (
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"
)

// UserClient is a client for the User Service
type UserClient struct {
	baseURL    string
	httpClient *http.Client
	log        *logger.Logger
}

// NewUserClient creates a new User Service client
func NewUserClient(baseURL string, log *logger.Logger) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		log: log,
	}
}

// TerminateSession terminates a user session by calling the user service
func (c *UserClient) TerminateSession(sessionID string) error {
	url := fmt.Sprintf("%s/internal/sessions/%s", c.baseURL, sessionID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
