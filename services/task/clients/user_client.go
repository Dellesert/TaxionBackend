package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"tachyon-messenger/shared/logger"
)

// UserInfo represents basic user information from user-service
type UserInfo struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar,omitempty"`
	Position string `json:"position,omitempty"`
}

// UserClient is HTTP client for user-service
type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewUserClient creates a new user service client
func NewUserClient() *UserClient {
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8080"
	}

	return &UserClient{
		baseURL: userServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetUsersByIDs retrieves multiple users by their IDs
func (c *UserClient) GetUsersByIDs(ids []uint) (map[uint]*UserInfo, error) {
	if len(ids) == 0 {
		return make(map[uint]*UserInfo), nil
	}

	// Convert IDs to comma-separated string
	idsStr := make([]string, len(ids))
	for i, id := range ids {
		idsStr[i] = fmt.Sprintf("%d", id)
	}
	idsParam := strings.Join(idsStr, ",")

	// Make request to internal endpoint
	url := fmt.Sprintf("%s/internal/users?ids=%s", c.baseURL, idsParam)

	logger.WithFields(map[string]interface{}{
		"url":      url,
		"ids_count": len(ids),
	}).Debug("Requesting users from user-service")

	resp, err := c.httpClient.Get(url)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Error("Failed to request users from user-service")
		return nil, fmt.Errorf("failed to request users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.WithFields(map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("User service returned non-OK status")
		return nil, fmt.Errorf("user service returned status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Users []*UserInfo `json:"users"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to decode user service response")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to map for easy lookup
	userMap := make(map[uint]*UserInfo)
	for _, user := range response.Users {
		userMap[user.ID] = user
	}

	logger.WithFields(map[string]interface{}{
		"requested_count": len(ids),
		"received_count":  len(userMap),
	}).Debug("Successfully retrieved users from user-service")

	return userMap, nil
}

// GetUserByID retrieves a single user by ID
func (c *UserClient) GetUserByID(id uint) (*UserInfo, error) {
	users, err := c.GetUsersByIDs([]uint{id})
	if err != nil {
		return nil, err
	}

	user, exists := users[id]
	if !exists {
		return nil, fmt.Errorf("user %d not found", id)
	}

	return user, nil
}
