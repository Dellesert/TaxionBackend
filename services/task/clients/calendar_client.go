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

// TodayEventResponse represents a calendar event for today
type TodayEventResponse struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	AllDay      bool   `json:"all_day"`
	Location    string `json:"location,omitempty"`
	Type        string `json:"type"`
	Color       string `json:"color,omitempty"`
	IsPrivate   bool   `json:"is_private"`
}

// CalendarClient is HTTP client for calendar-service
type CalendarClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewCalendarClient creates a new calendar service client
func NewCalendarClient() *CalendarClient {
	calendarServiceURL := os.Getenv("CALENDAR_SERVICE_URL")
	if calendarServiceURL == "" {
		calendarServiceURL = "http://calendar-service:8084"
	}

	return &CalendarClient{
		baseURL: calendarServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTodayEventsForUser retrieves today's events for a user
func (c *CalendarClient) GetTodayEventsForUser(userID uint, limit int) ([]*TodayEventResponse, int64, error) {
	// Get today's date range in UTC
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	url := fmt.Sprintf("%s/internal/events/today?user_id=%d&limit=%d&start=%s&end=%s",
		c.baseURL, userID, limit,
		startOfDay.Format(time.RFC3339),
		endOfDay.Format(time.RFC3339))

	logger.WithFields(map[string]interface{}{
		"url":     url,
		"user_id": userID,
		"limit":   limit,
	}).Debug("Requesting today's events from calendar-service")

	resp, err := c.httpClient.Get(url)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Error("Failed to request today's events from calendar-service")
		return nil, 0, fmt.Errorf("failed to request today's events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.WithFields(map[string]interface{}{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("Calendar service returned non-OK status")
		return nil, 0, fmt.Errorf("calendar service returned status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		Events []*TodayEventResponse `json:"events"`
		Total  int64                 `json:"total"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to decode calendar service response")
		return nil, 0, fmt.Errorf("failed to decode response: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":        userID,
		"received_count": len(response.Events),
		"total":          response.Total,
	}).Debug("Successfully retrieved today's events from calendar-service")

	return response.Events, response.Total, nil
}
