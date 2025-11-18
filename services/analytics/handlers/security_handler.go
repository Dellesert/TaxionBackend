package handlers

import (
	"net/http"
	"strconv"
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/usecase"

	"github.com/gin-gonic/gin"
)

// SecurityHandler handles security analytics HTTP requests
type SecurityHandler struct {
	securityUsecase usecase.SecurityUsecase
}

// NewSecurityHandler creates a new security handler
func NewSecurityHandler(securityUsecase usecase.SecurityUsecase) *SecurityHandler {
	return &SecurityHandler{securityUsecase: securityUsecase}
}

// RecordLoginAttempt records a login attempt (for internal use by auth service)
// POST /api/v1/analytics/security/login-attempt
func (h *SecurityHandler) RecordLoginAttempt(c *gin.Context) {
	var req struct {
		Email        string  `json:"email" binding:"required"`
		UserID       *uint64 `json:"user_id"`
		IPAddress    string  `json:"ip_address" binding:"required"`
		UserAgent    string  `json:"user_agent"`
		Success      bool    `json:"success"`
		FailReason   string  `json:"fail_reason"`
		AuthMode     string  `json:"auth_mode"`
		IsSuperAdmin bool    `json:"is_super_admin"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	attempt := &models.LoginAttempt{
		Email:        req.Email,
		UserID:       req.UserID,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		Success:      req.Success,
		FailReason:   req.FailReason,
		AuthMode:     req.AuthMode,
		IsSuperAdmin: req.IsSuperAdmin,
		Timestamp:    time.Now(),
	}

	if err := h.securityUsecase.RecordLoginAttempt(attempt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to record login attempt",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Login attempt recorded successfully",
	})
}

// TrackSession tracks a user session (for internal use by auth service)
// POST /api/v1/analytics/security/track-session
func (h *SecurityHandler) TrackSession(c *gin.Context) {
	var req struct {
		UserID    uint64    `json:"user_id" binding:"required"`
		SessionID string    `json:"session_id" binding:"required"`
		IPAddress string    `json:"ip_address" binding:"required"`
		UserAgent string    `json:"user_agent"`
		ExpiresAt time.Time `json:"expires_at" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if err := h.securityUsecase.TrackSession(req.UserID, req.SessionID, req.IPAddress, req.UserAgent, req.ExpiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to track session",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Session tracked successfully",
	})
}

// DeactivateSession deactivates a user session (for internal use by auth service)
// POST /api/v1/analytics/security/sessions/:session_id/deactivate
func (h *SecurityHandler) DeactivateSession(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Session ID is required",
		})
		return
	}

	if err := h.securityUsecase.DeactivateSession(sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to deactivate session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Session deactivated successfully",
	})
}

// GetDashboard returns the main security dashboard with all key metrics
// GET /api/v1/analytics/security/dashboard?period=7d
func (h *SecurityHandler) GetDashboard(c *gin.Context) {
	// Parse period parameter (default: last 7 days)
	period := c.DefaultQuery("period", "7d")
	start, end := parsePeriod(period)

	dashboard, err := h.securityUsecase.GetSecurityDashboard(start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch security dashboard",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": dashboard,
	})
}

// GetLoginAttempts returns login attempts history
// GET /api/v1/analytics/security/login-attempts?start=2024-01-01&end=2024-12-31&limit=100
func (h *SecurityHandler) GetLoginAttempts(c *gin.Context) {
	start, end, limit := parseTimeRange(c)

	attempts, err := h.securityUsecase.GetLoginAttempts(start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch login attempts",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  attempts,
		"count": len(attempts),
		"period": gin.H{
			"start": start,
			"end":   end,
		},
	})
}

// GetFailedLogins returns failed login attempts
// GET /api/v1/analytics/security/failed-logins?period=24h&limit=50
func (h *SecurityHandler) GetFailedLogins(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")
	start, end := parsePeriod(period)

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	attempts, err := h.securityUsecase.GetFailedLoginAttempts(start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch failed login attempts",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  attempts,
		"count": len(attempts),
		"period": gin.H{
			"start": start,
			"end":   end,
		},
	})
}

// GetLoginStats returns login statistics
// GET /api/v1/analytics/security/login-stats?period=30d
func (h *SecurityHandler) GetLoginStats(c *gin.Context) {
	period := c.DefaultQuery("period", "30d")
	start, end := parsePeriod(period)

	stats, err := h.securityUsecase.GetLoginStats(start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch login stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": stats,
		"period": gin.H{
			"start": start,
			"end":   end,
		},
	})
}

// GetTopFailedIPs returns top IPs with failed login attempts
// GET /api/v1/analytics/security/top-failed-ips?period=7d&limit=10
func (h *SecurityHandler) GetTopFailedIPs(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	start, end := parsePeriod(period)

	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	ips, err := h.securityUsecase.GetTopFailedLoginIPs(start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch top failed IPs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  ips,
		"count": len(ips),
	})
}

// GetSuspiciousActivities returns suspicious activities
// GET /api/v1/analytics/security/suspicious-activities?period=7d&limit=20
func (h *SecurityHandler) GetSuspiciousActivities(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	start, end := parsePeriod(period)

	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	activities, err := h.securityUsecase.GetSuspiciousActivities(start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch suspicious activities",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  activities,
		"count": len(activities),
	})
}

// GetUnresolvedSuspiciousActivities returns unresolved suspicious activities
// GET /api/v1/analytics/security/suspicious-activities/unresolved?limit=50
func (h *SecurityHandler) GetUnresolvedSuspiciousActivities(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	activities, err := h.securityUsecase.GetUnresolvedSuspiciousActivities(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch unresolved suspicious activities",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  activities,
		"count": len(activities),
	})
}

// ResolveSuspiciousActivity marks a suspicious activity as resolved
// POST /api/v1/analytics/security/suspicious-activities/:id/resolve
func (h *SecurityHandler) ResolveSuspiciousActivity(c *gin.Context) {
	// Get activity ID from URL
	activityID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid activity ID",
		})
		return
	}

	// Get user ID from context (who is resolving)
	resolvedBy, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	userID, ok := resolvedBy.(uint64)
	if !ok {
		// Try uint conversion
		if uintVal, ok := resolvedBy.(uint); ok {
			userID = uint64(uintVal)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid user ID format",
			})
			return
		}
	}

	if err := h.securityUsecase.ResolveSuspiciousActivity(activityID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to resolve suspicious activity",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Suspicious activity resolved successfully",
	})
}

// GetActiveSessions returns all active sessions
// GET /api/v1/analytics/security/active-sessions
func (h *SecurityHandler) GetActiveSessions(c *gin.Context) {
	sessions, err := h.securityUsecase.GetAllActiveSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch active sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  sessions,
		"count": len(sessions),
	})
}

// GetUserActiveSessions returns active sessions for a specific user
// GET /api/v1/analytics/security/users/:user_id/sessions
func (h *SecurityHandler) GetUserActiveSessions(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	sessions, err := h.securityUsecase.GetActiveSessions(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch user sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  sessions,
		"count": len(sessions),
	})
}

// GetUserKnownDevices returns known devices for a specific user
// GET /api/v1/analytics/security/users/:user_id/devices
func (h *SecurityHandler) GetUserKnownDevices(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	devices, err := h.securityUsecase.GetUserKnownDevices(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch user devices",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  devices,
		"count": len(devices),
	})
}

// RemoveKnownDevice removes a known device
// DELETE /api/v1/analytics/security/devices/:device_id
func (h *SecurityHandler) RemoveKnownDevice(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid device ID",
		})
		return
	}

	if err := h.securityUsecase.RemoveKnownDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to remove device",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Device removed successfully",
	})
}

// Helper functions

func parsePeriod(period string) (time.Time, time.Time) {
	end := time.Now()
	var start time.Time

	switch period {
	case "1h":
		start = end.Add(-1 * time.Hour)
	case "6h":
		start = end.Add(-6 * time.Hour)
	case "12h":
		start = end.Add(-12 * time.Hour)
	case "24h", "1d":
		start = end.Add(-24 * time.Hour)
	case "7d":
		start = end.Add(-7 * 24 * time.Hour)
	case "30d":
		start = end.Add(-30 * 24 * time.Hour)
	case "90d":
		start = end.Add(-90 * 24 * time.Hour)
	default:
		// Default to last 7 days
		start = end.Add(-7 * 24 * time.Hour)
	}

	return start, end
}

func parseTimeRange(c *gin.Context) (time.Time, time.Time, int) {
	// Try to parse start and end from query params
	startStr := c.Query("start")
	endStr := c.Query("end")
	limitStr := c.DefaultQuery("limit", "100")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			start = time.Now().Add(-7 * 24 * time.Hour)
		}
	} else {
		// Use period if start/end not provided
		period := c.DefaultQuery("period", "7d")
		start, end = parsePeriod(period)
		limit, _ := strconv.Atoi(limitStr)
		return start, end, limit
	}

	if endStr != "" {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			end = time.Now()
		}
	} else {
		end = time.Now()
	}

	limit, _ := strconv.Atoi(limitStr)
	return start, end, limit
}
