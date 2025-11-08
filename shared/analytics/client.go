package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"
)

// Client represents an analytics client for sending events
type Client struct {
	analyticsURL string
	httpClient   *http.Client
	log          *logger.Logger
}

// Event represents an analytics event
type Event struct {
	EventType     string                 `json:"event_type"`
	EventCategory string                 `json:"event_category"`
	UserID        uint64                 `json:"user_id"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewClient creates a new analytics client
func NewClient(analyticsURL string, log *logger.Logger) *Client {
	return &Client{
		analyticsURL: analyticsURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		log: log,
	}
}

// SendEvent sends an analytics event to the analytics service
// This is non-blocking and errors are logged but don't fail the main operation
func (c *Client) SendEvent(eventType, eventCategory string, userID uint64, metadata map[string]interface{}) {
	// Log that we're attempting to send an event
	c.log.WithFields(map[string]interface{}{
		"event_type":     eventType,
		"event_category": eventCategory,
		"user_id":        userID,
	}).Debug("Sending analytics event")

	// Send event asynchronously
	go func() {
		if err := c.sendEventSync(eventType, eventCategory, userID, metadata); err != nil {
			c.log.WithFields(map[string]interface{}{
				"event_type":     eventType,
				"event_category": eventCategory,
				"user_id":        userID,
				"error":          err,
			}).Warn("Failed to send analytics event")
		} else {
			c.log.WithFields(map[string]interface{}{
				"event_type":     eventType,
				"event_category": eventCategory,
				"user_id":        userID,
			}).Debug("Analytics event sent successfully")
		}
	}()
}

// sendEventSync sends an analytics event synchronously
func (c *Client) sendEventSync(eventType, eventCategory string, userID uint64, metadata map[string]interface{}) error {
	event := Event{
		EventType:     eventType,
		EventCategory: eventCategory,
		UserID:        userID,
		Metadata:      metadata,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/analytics/events", c.analyticsURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Event type constants
const (
	// User events
	EventUserLogin        = "user_login"
	EventUserLogout       = "user_logout"
	EventUserRegistration = "user_registration"
	EventUserUpdated      = "user_updated"

	// Message events
	EventMessageSent = "message_sent"
	EventMessageRead = "message_read"

	// Task events
	EventTaskCreated   = "task_created"
	EventTaskUpdated   = "task_updated"
	EventTaskCompleted = "task_completed"
	EventTaskDeleted   = "task_deleted"

	// Calendar events
	EventCalendarEventCreated = "event_created"
	EventCalendarEventUpdated = "event_updated"
	EventCalendarEventDeleted = "event_deleted"

	// Poll events
	EventPollCreated = "poll_created"
	EventPollVoted   = "poll_voted"
	EventPollClosed  = "poll_closed"

	// File events
	EventFileUploaded   = "file_uploaded"
	EventFileDownloaded = "file_downloaded"
	EventFileDeleted    = "file_deleted"
)

// Event category constants
const (
	CategoryUser     = "user"
	CategoryMessage  = "message"
	CategoryTask     = "task"
	CategoryCalendar = "calendar"
	CategoryPoll     = "poll"
	CategoryFile     = "file"
)
