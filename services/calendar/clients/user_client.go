package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sharedmodels "tachyon-messenger/shared/models"
)

// UserInfo represents basic user information from user-service
type UserInfo struct {
	ID           uint              `json:"id"`
	Name         string            `json:"name"`
	Email        string            `json:"email"`
	Role         sharedmodels.Role `json:"role"`
	Avatar       string            `json:"avatar,omitempty"`
	Position     string            `json:"position,omitempty"`
	DepartmentID *uint             `json:"department_id,omitempty"`
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
		userServiceURL = "http://user-service:8081"
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

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to request users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response struct {
		Users []*UserInfo `json:"users"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to map for easy lookup
	userMap := make(map[uint]*UserInfo, len(response.Users))
	for _, user := range response.Users {
		userMap[user.ID] = user
	}

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
		return nil, fmt.Errorf("user not found: %d", id)
	}

	return user, nil
}

// UserForMatching represents user data needed for schedule import name matching
type UserForMatching struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email"`
}

// GetAllUsers retrieves all users from user service for name matching
func (c *UserClient) GetAllUsers() ([]*sharedmodels.User, error) {
	url := fmt.Sprintf("%s/internal/users/all", c.baseURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to request users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user service returned status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Users []*UserForMatching `json:"users"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to sharedmodels.User for compatibility with existing code
	users := make([]*sharedmodels.User, len(response.Users))
	for i, u := range response.Users {
		users[i] = &sharedmodels.User{
			BaseModel: sharedmodels.BaseModel{ID: u.ID},
			Name:      u.Name,
			Email:     u.Email,
		}
	}

	return users, nil
}
