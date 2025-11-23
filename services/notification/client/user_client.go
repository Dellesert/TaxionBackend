// File: services/notification/client/user_client.go
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// UserInfo represents basic user information
type UserInfo struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// userServiceResponse wraps the user service API response
type userServiceResponse struct {
	User struct {
		ID     uint   `json:"id"`
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	} `json:"user"`
}

// UserClient handles communication with user service
type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewUserClient creates a new user service client
func NewUserClient() *UserClient {
	baseURL := os.Getenv("USER_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tachyon-user-service:8081"
	}

	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetUserInfo retrieves user information by ID
func (c *UserClient) GetUserInfo(userID uint) (*UserInfo, error) {
	url := fmt.Sprintf("%s/internal/users/%d", c.baseURL, userID)

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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user service returned status %d: %s", resp.StatusCode, string(body))
	}

	var response userServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &UserInfo{
		ID:        response.User.ID,
		Name:      response.User.Name,
		AvatarURL: response.User.Avatar,
	}, nil
}

// GetMultipleUsers retrieves multiple users by IDs
func (c *UserClient) GetMultipleUsers(userIDs []uint) (map[uint]*UserInfo, error) {
	result := make(map[uint]*UserInfo)

	// For simplicity, fetch users one by one
	// In production, you might want to implement a batch endpoint
	for _, userID := range userIDs {
		userInfo, err := c.GetUserInfo(userID)
		if err != nil {
			// Log error but continue with other users
			fmt.Printf("Failed to get user info for ID %d: %v\n", userID, err)
			continue
		}
		result[userID] = userInfo
	}

	return result, nil
}
