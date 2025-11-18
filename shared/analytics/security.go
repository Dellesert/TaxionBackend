package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LoginAttemptRequest represents a login attempt to be sent to analytics
type LoginAttemptRequest struct {
	Email        string `json:"email"`
	UserID       *uint64 `json:"user_id,omitempty"`
	IPAddress    string `json:"ip_address"`
	UserAgent    string `json:"user_agent"`
	Success      bool   `json:"success"`
	FailReason   string `json:"fail_reason,omitempty"`
	AuthMode     string `json:"auth_mode,omitempty"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

// SendLoginAttempt sends a login attempt record to the analytics service
func (c *Client) SendLoginAttempt(attempt LoginAttemptRequest) error {
	jsonData, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("failed to marshal login attempt: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/analytics/security/login-attempt", c.analyticsURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.WithFields(map[string]interface{}{
			"email":   attempt.Email,
			"success": attempt.Success,
			"error":   err.Error(),
		}).Warn("Failed to send login attempt to analytics")
		return nil // Don't fail the login on analytics error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.log.WithFields(map[string]interface{}{
			"email":       attempt.Email,
			"success":     attempt.Success,
			"status_code": resp.StatusCode,
		}).Warn("Analytics service returned non-OK status for login attempt")
	}

	return nil
}

// SendLoginAttemptAsync sends a login attempt asynchronously
func (c *Client) SendLoginAttemptAsync(attempt LoginAttemptRequest) {
	go func() {
		if err := c.SendLoginAttempt(attempt); err != nil {
			c.log.WithFields(map[string]interface{}{
				"email":  attempt.Email,
				"error":  err.Error(),
			}).Debug("Failed to send login attempt (async)")
		}
	}()
}

// TrackDevice sends device tracking information to analytics
func (c *Client) TrackDevice(userID uint64, userAgent, ipAddress string) {
	metadata := map[string]interface{}{
		"user_agent":  userAgent,
		"ip_address":  ipAddress,
		"tracked_at":  time.Now(),
	}

	c.SendEvent(EventNewDeviceLogin, CategorySecurity, userID, metadata)
}

// SessionTrackingRequest represents a session to be tracked
type SessionTrackingRequest struct {
	UserID    uint64    `json:"user_id"`
	SessionID string    `json:"session_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
}

// TrackSession sends session tracking information to analytics (creates actual session record)
func (c *Client) TrackSession(userID uint64, sessionID, ipAddress, userAgent string, expiresAt time.Time) error {
	req := SessionTrackingRequest{
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: expiresAt,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal session tracking request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/analytics/security/track-session", c.analyticsURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.log.WithFields(map[string]interface{}{
			"user_id":    userID,
			"session_id": sessionID,
			"error":      err.Error(),
		}).Warn("Failed to track session in analytics")
		return nil // Don't fail on analytics error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.log.WithFields(map[string]interface{}{
			"user_id":     userID,
			"session_id":  sessionID,
			"status_code": resp.StatusCode,
		}).Warn("Analytics service returned non-OK status for session tracking")
	}

	return nil
}

// TrackSessionAsync tracks a session asynchronously
func (c *Client) TrackSessionAsync(userID uint64, sessionID, ipAddress, userAgent string, expiresAt time.Time) {
	go func() {
		if err := c.TrackSession(userID, sessionID, ipAddress, userAgent, expiresAt); err != nil {
			c.log.WithFields(map[string]interface{}{
				"user_id":    userID,
				"session_id": sessionID,
				"error":      err.Error(),
			}).Debug("Failed to track session (async)")
		}
	}()
}

// DeactivateSession deactivates a session in analytics
func (c *Client) DeactivateSession(sessionID string) error {
	url := fmt.Sprintf("%s/api/v1/analytics/security/sessions/%s/deactivate", c.analyticsURL, sessionID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.WithFields(map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		}).Warn("Failed to deactivate session in analytics")
		return nil // Don't fail on analytics error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		c.log.WithFields(map[string]interface{}{
			"session_id":  sessionID,
			"status_code": resp.StatusCode,
		}).Warn("Analytics service returned non-OK status for session deactivation")
	}

	return nil
}

// DeactivateSessionAsync deactivates a session asynchronously
func (c *Client) DeactivateSessionAsync(sessionID string) {
	go func() {
		if err := c.DeactivateSession(sessionID); err != nil {
			c.log.WithFields(map[string]interface{}{
				"session_id": sessionID,
				"error":      err.Error(),
			}).Debug("Failed to deactivate session (async)")
		}
	}()
}

// DetermineFailReason determines the fail reason from an error message
func DetermineFailReason(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	switch {
	case contains(errMsg, "invalid email or password"):
		return "invalid_credentials"
	case contains(errMsg, "deactivated"):
		return "account_deactivated"
	case contains(errMsg, "2FA is required"):
		return "2fa_required"
	case contains(errMsg, "password login is disabled"):
		return "passkey_only"
	case contains(errMsg, "super admin access is restricted"):
		return "super_admin_web_only"
	case contains(errMsg, "password expired"):
		return "password_expired"
	case contains(errMsg, "account is locked"):
		return "account_locked"
	default:
		return "unknown"
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
