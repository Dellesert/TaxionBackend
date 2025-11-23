// File: services/poll/clients/user_client.go
package clients

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// UserClient handles communication with user service
type UserClient struct {
	baseURL string
	client  *http.Client
}

// NewUserClient creates a new user client
func NewUserClient() *UserClient {
	baseURL := os.Getenv("USER_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://tachyon-user-service:8081"
	}

	return &UserClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// UserInfo represents basic user information
type UserInfo struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// GetUserByID retrieves user information by ID
func (c *UserClient) GetUserByID(userID uint) (*UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/internal/users/%d", c.baseURL, userID)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user service returned status %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// GetUsersByIDs retrieves multiple users by their IDs
func (c *UserClient) GetUsersByIDs(userIDs []uint) (map[uint]*UserInfo, error) {
	// For now, fetch users one by one
	// TODO: Implement bulk endpoint in user service
	users := make(map[uint]*UserInfo)

	for _, userID := range userIDs {
		user, err := c.GetUserByID(userID)
		if err != nil {
			// Log error but continue with other users
			continue
		}
		users[userID] = user
	}

	return users, nil
}

// GetUsersByDepartment retrieves all users in a department
func (c *UserClient) GetUsersByDepartment(departmentID uint) ([]uint, error) {
	url := fmt.Sprintf("%s/internal/users/department/%d", c.baseURL, departmentID)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get department users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user service returned status %d", resp.StatusCode)
	}

	var response struct {
		UserIDs []uint `json:"user_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode department users: %w", err)
	}

	return response.UserIDs, nil
}
