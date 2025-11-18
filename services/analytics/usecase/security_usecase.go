package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/repository"
	"tachyon-messenger/shared/logger"
)

// SecurityUsecase defines the interface for security analytics business logic
type SecurityUsecase interface {
	// Login attempts
	RecordLoginAttempt(attempt *models.LoginAttempt) error
	GetLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error)
	GetFailedLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error)
	GetLoginStats(start, end time.Time) (map[string]interface{}, error)
	GetTopFailedLoginIPs(start, end time.Time, limit int) ([]map[string]interface{}, error)

	// Device tracking
	TrackDevice(userID uint64, userAgent, ipAddress string) (bool, error) // returns isNewDevice
	GetUserKnownDevices(userID uint64) ([]*models.KnownDevice, error)
	RemoveKnownDevice(deviceID uint64) error
	TrustDevice(deviceID uint64) error

	// Session tracking
	TrackSession(userID uint64, sessionID, ipAddress, userAgent string, expiresAt time.Time) error
	GetActiveSessions(userID uint64) ([]*models.SecuritySession, error)
	GetAllActiveSessions() ([]*models.SecuritySession, error)
	DeactivateSession(sessionID string) error
	TerminateSession(sessionID string) error

	// Suspicious activity
	DetectSuspiciousActivity(email, ipAddress string) error
	GetSuspiciousActivities(start, end time.Time, limit int) ([]*models.SuspiciousActivity, error)
	GetUnresolvedSuspiciousActivities(limit int) ([]*models.SuspiciousActivity, error)
	ResolveSuspiciousActivity(activityID, resolvedBy uint64) error

	// Dashboard and stats
	GetSecurityDashboard(start, end time.Time) (map[string]interface{}, error)
}

type securityUsecase struct {
	securityRepo *repository.SecurityRepository
	userClient   UserServiceClient
	log          *logger.Logger
}

// UserServiceClient defines the interface for user service client
type UserServiceClient interface {
	TerminateSession(sessionID string) error
}

// NewSecurityUsecase creates a new security usecase
func NewSecurityUsecase(securityRepo *repository.SecurityRepository, userClient UserServiceClient, log *logger.Logger) SecurityUsecase {
	return &securityUsecase{
		securityRepo: securityRepo,
		userClient:   userClient,
		log:          log,
	}
}

// RecordLoginAttempt records a login attempt
func (u *securityUsecase) RecordLoginAttempt(attempt *models.LoginAttempt) error {
	// Parse device type and browser from user agent
	attempt.DeviceType = detectDeviceType(attempt.UserAgent)
	attempt.Browser = detectBrowser(attempt.UserAgent)

	// Record the attempt
	if err := u.securityRepo.CreateLoginAttempt(attempt); err != nil {
		u.log.WithFields(map[string]interface{}{
			"email":  attempt.Email,
			"ip":     attempt.IPAddress,
			"error":  err.Error(),
		}).Error("Failed to record login attempt")
		return err
	}

	// If failed, check for suspicious activity
	if !attempt.Success {
		go u.DetectSuspiciousActivity(attempt.Email, attempt.IPAddress)
	}

	return nil
}

// GetLoginAttempts retrieves login attempts for a time range
func (u *securityUsecase) GetLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error) {
	return u.securityRepo.GetLoginAttempts(start, end, limit)
}

// GetFailedLoginAttempts retrieves failed login attempts
func (u *securityUsecase) GetFailedLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error) {
	return u.securityRepo.GetFailedLoginAttempts(start, end, limit)
}

// GetLoginStats calculates login statistics
func (u *securityUsecase) GetLoginStats(start, end time.Time) (map[string]interface{}, error) {
	totalLogins, err := u.securityRepo.CountLoginAttempts(start, end, true)
	if err != nil {
		return nil, err
	}

	failedLogins, err := u.securityRepo.CountLoginAttempts(start, end, false)
	if err != nil {
		return nil, err
	}

	totalAttempts := totalLogins + failedLogins
	successRate := 0.0
	if totalAttempts > 0 {
		successRate = float64(totalLogins) / float64(totalAttempts) * 100
	}

	return map[string]interface{}{
		"total_attempts": totalAttempts,
		"successful":     totalLogins,
		"failed":         failedLogins,
		"success_rate":   successRate,
	}, nil
}

// GetTopFailedLoginIPs gets top IPs with failed login attempts
func (u *securityUsecase) GetTopFailedLoginIPs(start, end time.Time, limit int) ([]map[string]interface{}, error) {
	return u.securityRepo.GetTopFailedLoginIPs(start, end, limit)
}

// TrackDevice tracks a device for a user
func (u *securityUsecase) TrackDevice(userID uint64, userAgent, ipAddress string) (bool, error) {
	fingerprint := generateDeviceFingerprint(userID, userAgent)

	// Check if device already exists
	existingDevice, err := u.securityRepo.GetKnownDevice(fingerprint)
	if err == nil && existingDevice != nil {
		// Update last seen
		existingDevice.LastSeen = time.Now()
		existingDevice.LoginCount++
		existingDevice.IPAddress = ipAddress

		// Update trust level based on usage
		existingDevice.TrustLevel = calculateTrustLevel(existingDevice.LoginCount, existingDevice.FirstSeen)
		existingDevice.IsTrusted = existingDevice.TrustLevel >= 70 // Trust threshold

		if err := u.securityRepo.UpdateKnownDevice(existingDevice); err != nil {
			u.log.WithField("error", err.Error()).Warn("Failed to update known device")
		}

		return false, nil // Not a new device
	}

	// Create new device record
	device := &models.KnownDevice{
		UserID:            userID,
		DeviceFingerprint: fingerprint,
		UserAgent:         userAgent,
		DeviceType:        detectDeviceType(userAgent),
		Browser:           detectBrowser(userAgent),
		OS:                detectOS(userAgent),
		IPAddress:         ipAddress,
		FirstSeen:         time.Now(),
		LastSeen:          time.Now(),
		IsTrusted:         false,
		TrustLevel:        0,
		LoginCount:        1,
	}

	if err := u.securityRepo.CreateKnownDevice(device); err != nil {
		u.log.WithField("error", err.Error()).Error("Failed to create known device")
		return true, err
	}

	return true, nil // New device
}

// GetUserKnownDevices retrieves all known devices for a user
func (u *securityUsecase) GetUserKnownDevices(userID uint64) ([]*models.KnownDevice, error) {
	return u.securityRepo.GetUserKnownDevices(userID)
}

// RemoveKnownDevice removes a known device
func (u *securityUsecase) RemoveKnownDevice(deviceID uint64) error {
	return u.securityRepo.DeleteKnownDevice(deviceID)
}

// TrustDevice manually marks a device as trusted
func (u *securityUsecase) TrustDevice(deviceID uint64) error {
	// Get the device
	device, err := u.securityRepo.GetKnownDeviceByID(deviceID)
	if err != nil {
		return err
	}

	// Mark as trusted with 100% trust level
	device.IsTrusted = true
	device.TrustLevel = 100

	return u.securityRepo.UpdateKnownDevice(device)
}

// TrackSession tracks a user session
func (u *securityUsecase) TrackSession(userID uint64, sessionID, ipAddress, userAgent string, expiresAt time.Time) error {
	session := &models.SecuritySession{
		UserID:       userID,
		SessionID:    sessionID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		DeviceType:   detectDeviceType(userAgent),
		Browser:      detectBrowser(userAgent),
		ExpiresAt:    expiresAt,
		LastActivity: time.Now(),
		IsActive:     true,
	}

	return u.securityRepo.CreateSecuritySession(session)
}

// GetActiveSessions retrieves active sessions for a user
func (u *securityUsecase) GetActiveSessions(userID uint64) ([]*models.SecuritySession, error) {
	return u.securityRepo.GetActiveSessions(userID)
}

// GetAllActiveSessions retrieves all active sessions
func (u *securityUsecase) GetAllActiveSessions() ([]*models.SecuritySession, error) {
	return u.securityRepo.GetAllActiveSessions()
}

// DeactivateSession deactivates a session
func (u *securityUsecase) DeactivateSession(sessionID string) error {
	return u.securityRepo.DeactivateSession(sessionID)
}

// DetectSuspiciousActivity detects and records suspicious activity
func (u *securityUsecase) DetectSuspiciousActivity(email, ipAddress string) error {
	now := time.Now()
	since := now.Add(-10 * time.Minute) // Check last 10 minutes

	// Check for multiple failed logins from same email
	failedCount, err := u.securityRepo.CountFailedLoginsByEmail(email, since)
	if err != nil {
		u.log.WithField("error", err.Error()).Warn("Failed to count failed logins by email")
		return err
	}

	if failedCount >= 5 {
		metadataMap := map[string]interface{}{
			"failed_count": failedCount,
			"time_window":  "10m",
		}
		metadataJSON, _ := json.Marshal(metadataMap)

		activity := &models.SuspiciousActivity{
			Email:        email,
			IPAddress:    ipAddress,
			ActivityType: models.ActivityMultipleFailedLogins,
			Severity:     models.SeverityHigh,
			Description:  fmt.Sprintf("Multiple failed login attempts (%d) detected in the last 10 minutes", failedCount),
			Metadata:     datatypes.JSON(metadataJSON),
			Timestamp:    now,
		}

		if err := u.securityRepo.CreateSuspiciousActivity(activity); err != nil {
			u.log.WithField("error", err.Error()).Error("Failed to create suspicious activity record")
			return err
		}
	}

	// Check for brute force from same IP
	ipFailedCount, err := u.securityRepo.CountFailedLoginsByIP(ipAddress, since)
	if err != nil {
		u.log.WithField("error", err.Error()).Warn("Failed to count failed logins by IP")
		return err
	}

	if ipFailedCount >= 10 {
		metadataMap := map[string]interface{}{
			"failed_count": ipFailedCount,
			"time_window":  "10m",
		}
		metadataJSON, _ := json.Marshal(metadataMap)

		activity := &models.SuspiciousActivity{
			Email:        email,
			IPAddress:    ipAddress,
			ActivityType: models.ActivityBruteForce,
			Severity:     models.SeverityCritical,
			Description:  fmt.Sprintf("Potential brute force attack detected from IP %s (%d failed attempts)", ipAddress, ipFailedCount),
			Metadata:     datatypes.JSON(metadataJSON),
			Timestamp:    now,
		}

		if err := u.securityRepo.CreateSuspiciousActivity(activity); err != nil {
			u.log.WithField("error", err.Error()).Error("Failed to create suspicious activity record")
			return err
		}
	}

	return nil
}

// GetSuspiciousActivities retrieves suspicious activities
func (u *securityUsecase) GetSuspiciousActivities(start, end time.Time, limit int) ([]*models.SuspiciousActivity, error) {
	return u.securityRepo.GetSuspiciousActivities(start, end, limit)
}

// GetUnresolvedSuspiciousActivities retrieves unresolved suspicious activities
func (u *securityUsecase) GetUnresolvedSuspiciousActivities(limit int) ([]*models.SuspiciousActivity, error) {
	return u.securityRepo.GetUnresolvedSuspiciousActivities(limit)
}

// ResolveSuspiciousActivity marks a suspicious activity as resolved
func (u *securityUsecase) ResolveSuspiciousActivity(activityID, resolvedBy uint64) error {
	return u.securityRepo.ResolveSuspiciousActivity(activityID, resolvedBy)
}

// GetSecurityDashboard retrieves comprehensive security dashboard data
func (u *securityUsecase) GetSecurityDashboard(start, end time.Time) (map[string]interface{}, error) {
	// Get login stats
	loginStats, err := u.GetLoginStats(start, end)
	if err != nil {
		return nil, err
	}

	// Get top failed login IPs
	topFailedIPs, err := u.GetTopFailedLoginIPs(start, end, 10)
	if err != nil {
		return nil, err
	}

	// Get recent suspicious activities
	suspiciousActivities, err := u.GetSuspiciousActivities(start, end, 20)
	if err != nil {
		return nil, err
	}

	// Count unresolved suspicious activities
	unresolvedActivities, err := u.GetUnresolvedSuspiciousActivities(0)
	if err != nil {
		return nil, err
	}

	// Get all active sessions count
	activeSessions, err := u.GetAllActiveSessions()
	if err != nil {
		return nil, err
	}

	// Get failed login attempts for timeline
	failedAttempts, err := u.GetFailedLoginAttempts(start, end, 100)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"login_stats":               loginStats,
		"top_failed_ips":           topFailedIPs,
		"suspicious_activities":     suspiciousActivities,
		"unresolved_activities_count": len(unresolvedActivities),
		"active_sessions_count":    len(activeSessions),
		"recent_failed_attempts":   failedAttempts,
		"period": map[string]interface{}{
			"start": start,
			"end":   end,
		},
	}, nil
}

// Helper functions

func generateDeviceFingerprint(userID uint64, userAgent string) string {
	data := fmt.Sprintf("%d:%s", userID, userAgent)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func detectDeviceType(userAgent string) string {
	ua := strings.ToLower(userAgent)

	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		return "mobile"
	}
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "tablet"
	}
	return "desktop"
}

func detectBrowser(userAgent string) string {
	ua := strings.ToLower(userAgent)

	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		return "Chrome"
	}
	if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		return "Safari"
	}
	if strings.Contains(ua, "firefox") {
		return "Firefox"
	}
	if strings.Contains(ua, "edg") {
		return "Edge"
	}
	if strings.Contains(ua, "opera") || strings.Contains(ua, "opr") {
		return "Opera"
	}
	return "Unknown"
}

func detectOS(userAgent string) string {
	ua := strings.ToLower(userAgent)

	if strings.Contains(ua, "windows") {
		return "Windows"
	}
	if strings.Contains(ua, "mac os") || strings.Contains(ua, "macos") {
		return "macOS"
	}
	if strings.Contains(ua, "linux") {
		return "Linux"
	}
	if strings.Contains(ua, "android") {
		return "Android"
	}
	if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ios") {
		return "iOS"
	}
	return "Unknown"
}

// calculateTrustLevel calculates trust level based on login count and device age
// Trust level is 0-100:
// - New devices start at 0
// - Each successful login increases trust
// - Older devices get bonus trust
// - 70+ is considered trusted
func calculateTrustLevel(loginCount int, firstSeen time.Time) int {
	// Base trust from login count (up to 60 points)
	loginTrust := loginCount * 10
	if loginTrust > 60 {
		loginTrust = 60
	}

	// Age bonus (up to 40 points)
	daysSinceFirst := int(time.Since(firstSeen).Hours() / 24)
	ageTrust := 0

	if daysSinceFirst >= 30 {
		ageTrust = 40 // 30+ days = max age trust
	} else if daysSinceFirst >= 14 {
		ageTrust = 30 // 14-29 days
	} else if daysSinceFirst >= 7 {
		ageTrust = 20 // 7-13 days
	} else if daysSinceFirst >= 3 {
		ageTrust = 10 // 3-6 days
	}

	total := loginTrust + ageTrust
	if total > 100 {
		total = 100
	}

	return total
}

// TerminateSession terminates a session by calling user service
func (u *securityUsecase) TerminateSession(sessionID string) error {
	// Call user service to terminate the session
	if err := u.userClient.TerminateSession(sessionID); err != nil {
		u.log.WithFields(map[string]interface{}{
			"session_id": sessionID,
			"error":      err.Error(),
		}).Error("Failed to terminate session via user service")
		return err
	}

	// Session will be deactivated in analytics by the user service
	u.log.WithField("session_id", sessionID).Info("Session terminated successfully")
	return nil
}
