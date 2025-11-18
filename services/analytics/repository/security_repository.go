package repository

import (
	"time"

	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/shared/database"
)

// SecurityRepository handles security analytics data access
type SecurityRepository struct {
	db *database.DB
}

// NewSecurityRepository creates a new security repository
func NewSecurityRepository(db *database.DB) *SecurityRepository {
	return &SecurityRepository{db: db}
}

// --- Login Attempts ---

// CreateLoginAttempt records a login attempt
func (r *SecurityRepository) CreateLoginAttempt(attempt *models.LoginAttempt) error {
	return r.db.DB.Create(attempt).Error
}

// GetLoginAttempts retrieves login attempts for a time range
func (r *SecurityRepository) GetLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error) {
	var attempts []*models.LoginAttempt
	query := r.db.DB.Where("timestamp >= ? AND timestamp <= ?", start, end).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&attempts).Error
	return attempts, err
}

// GetLoginAttemptsByEmail retrieves login attempts for a specific email
func (r *SecurityRepository) GetLoginAttemptsByEmail(email string, start, end time.Time) ([]*models.LoginAttempt, error) {
	var attempts []*models.LoginAttempt
	err := r.db.DB.Where("email = ? AND timestamp >= ? AND timestamp <= ?", email, start, end).
		Order("timestamp DESC").
		Find(&attempts).Error
	return attempts, err
}

// GetLoginAttemptsByIP retrieves login attempts from a specific IP
func (r *SecurityRepository) GetLoginAttemptsByIP(ipAddress string, start, end time.Time) ([]*models.LoginAttempt, error) {
	var attempts []*models.LoginAttempt
	err := r.db.DB.Where("ip_address = ? AND timestamp >= ? AND timestamp <= ?", ipAddress, start, end).
		Order("timestamp DESC").
		Find(&attempts).Error
	return attempts, err
}

// GetFailedLoginAttempts retrieves only failed login attempts
func (r *SecurityRepository) GetFailedLoginAttempts(start, end time.Time, limit int) ([]*models.LoginAttempt, error) {
	var attempts []*models.LoginAttempt
	query := r.db.DB.Where("success = ? AND timestamp >= ? AND timestamp <= ?", false, start, end).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&attempts).Error
	return attempts, err
}

// CountLoginAttempts counts login attempts for a time range
func (r *SecurityRepository) CountLoginAttempts(start, end time.Time, success bool) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.LoginAttempt{}).
		Where("success = ? AND timestamp >= ? AND timestamp <= ?", success, start, end).
		Count(&count).Error
	return count, err
}

// CountFailedLoginsByEmail counts failed login attempts for a specific email
func (r *SecurityRepository) CountFailedLoginsByEmail(email string, since time.Time) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.LoginAttempt{}).
		Where("email = ? AND success = ? AND timestamp >= ?", email, false, since).
		Count(&count).Error
	return count, err
}

// CountFailedLoginsByIP counts failed login attempts from a specific IP
func (r *SecurityRepository) CountFailedLoginsByIP(ipAddress string, since time.Time) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.LoginAttempt{}).
		Where("ip_address = ? AND success = ? AND timestamp >= ?", ipAddress, false, since).
		Count(&count).Error
	return count, err
}

// GetTopFailedLoginIPs gets top IPs with most failed login attempts
func (r *SecurityRepository) GetTopFailedLoginIPs(start, end time.Time, limit int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := r.db.DB.Model(&models.LoginAttempt{}).
		Select("ip_address, COUNT(*) as count").
		Where("success = ? AND timestamp >= ? AND timestamp <= ?", false, start, end).
		Group("ip_address").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	return results, err
}

// --- Known Devices ---

// CreateKnownDevice creates a new known device record
func (r *SecurityRepository) CreateKnownDevice(device *models.KnownDevice) error {
	return r.db.DB.Create(device).Error
}

// GetKnownDevice retrieves a known device by fingerprint
func (r *SecurityRepository) GetKnownDevice(deviceFingerprint string) (*models.KnownDevice, error) {
	var device models.KnownDevice
	err := r.db.DB.Where("device_fingerprint = ?", deviceFingerprint).First(&device).Error
	return &device, err
}

// GetKnownDeviceByID retrieves a known device by ID
func (r *SecurityRepository) GetKnownDeviceByID(id uint64) (*models.KnownDevice, error) {
	var device models.KnownDevice
	err := r.db.DB.First(&device, id).Error
	return &device, err
}

// GetUserKnownDevices retrieves all known devices for a user
func (r *SecurityRepository) GetUserKnownDevices(userID uint64) ([]*models.KnownDevice, error) {
	var devices []*models.KnownDevice
	err := r.db.DB.Where("user_id = ?", userID).
		Order("last_seen DESC").
		Find(&devices).Error
	return devices, err
}

// UpdateKnownDevice updates a known device
func (r *SecurityRepository) UpdateKnownDevice(device *models.KnownDevice) error {
	return r.db.DB.Save(device).Error
}

// DeleteKnownDevice deletes a known device
func (r *SecurityRepository) DeleteKnownDevice(id uint64) error {
	return r.db.DB.Delete(&models.KnownDevice{}, id).Error
}

// --- Security Sessions ---

// CreateSecuritySession creates a new security session
func (r *SecurityRepository) CreateSecuritySession(session *models.SecuritySession) error {
	return r.db.DB.Create(session).Error
}

// GetSecuritySession retrieves a security session by session ID
func (r *SecurityRepository) GetSecuritySession(sessionID string) (*models.SecuritySession, error) {
	var session models.SecuritySession
	err := r.db.DB.Where("session_id = ?", sessionID).First(&session).Error
	return &session, err
}

// GetActiveSessions retrieves all active sessions for a user
func (r *SecurityRepository) GetActiveSessions(userID uint64) ([]*models.SecuritySession, error) {
	var sessions []*models.SecuritySession
	err := r.db.DB.Where("user_id = ? AND is_active = ? AND expires_at > ?", userID, true, time.Now()).
		Order("last_activity DESC").
		Find(&sessions).Error
	return sessions, err
}

// GetAllActiveSessions retrieves all active sessions system-wide
func (r *SecurityRepository) GetAllActiveSessions() ([]*models.SecuritySession, error) {
	var sessions []*models.SecuritySession
	err := r.db.DB.Where("is_active = ? AND expires_at > ?", true, time.Now()).
		Order("last_activity DESC").
		Find(&sessions).Error
	return sessions, err
}

// UpdateSecuritySession updates a security session
func (r *SecurityRepository) UpdateSecuritySession(session *models.SecuritySession) error {
	return r.db.DB.Save(session).Error
}

// DeactivateSession marks a session as inactive
func (r *SecurityRepository) DeactivateSession(sessionID string) error {
	return r.db.DB.Model(&models.SecuritySession{}).
		Where("session_id = ?", sessionID).
		Update("is_active", false).Error
}

// CountActiveSessions counts active sessions for a user
func (r *SecurityRepository) CountActiveSessions(userID uint64) (int64, error) {
	var count int64
	err := r.db.DB.Model(&models.SecuritySession{}).
		Where("user_id = ? AND is_active = ? AND expires_at > ?", userID, true, time.Now()).
		Count(&count).Error
	return count, err
}

// CleanupExpiredSessions removes expired sessions
func (r *SecurityRepository) CleanupExpiredSessions() (int64, error) {
	result := r.db.DB.Where("expires_at < ?", time.Now()).
		Delete(&models.SecuritySession{})
	return result.RowsAffected, result.Error
}

// --- Suspicious Activities ---

// CreateSuspiciousActivity records a suspicious activity
func (r *SecurityRepository) CreateSuspiciousActivity(activity *models.SuspiciousActivity) error {
	return r.db.DB.Create(activity).Error
}

// GetSuspiciousActivities retrieves suspicious activities for a time range
func (r *SecurityRepository) GetSuspiciousActivities(start, end time.Time, limit int) ([]*models.SuspiciousActivity, error) {
	var activities []*models.SuspiciousActivity
	query := r.db.DB.Where("timestamp >= ? AND timestamp <= ?", start, end).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&activities).Error
	return activities, err
}

// GetUnresolvedSuspiciousActivities retrieves unresolved suspicious activities
func (r *SecurityRepository) GetUnresolvedSuspiciousActivities(limit int) ([]*models.SuspiciousActivity, error) {
	var activities []*models.SuspiciousActivity
	query := r.db.DB.Where("is_resolved = ?", false).
		Order("timestamp DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&activities).Error
	return activities, err
}

// GetSuspiciousActivitiesByUser retrieves suspicious activities for a specific user
func (r *SecurityRepository) GetSuspiciousActivitiesByUser(userID uint64, start, end time.Time) ([]*models.SuspiciousActivity, error) {
	var activities []*models.SuspiciousActivity
	err := r.db.DB.Where("user_id = ? AND timestamp >= ? AND timestamp <= ?", userID, start, end).
		Order("timestamp DESC").
		Find(&activities).Error
	return activities, err
}

// ResolveSuspiciousActivity marks a suspicious activity as resolved
func (r *SecurityRepository) ResolveSuspiciousActivity(id uint64, resolvedBy uint64) error {
	now := time.Now()
	return r.db.DB.Model(&models.SuspiciousActivity{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_resolved": true,
			"resolved_at": now,
			"resolved_by": resolvedBy,
		}).Error
}

// CountSuspiciousActivities counts suspicious activities by severity
func (r *SecurityRepository) CountSuspiciousActivities(start, end time.Time, severity string) (int64, error) {
	var count int64
	query := r.db.DB.Model(&models.SuspiciousActivity{}).
		Where("timestamp >= ? AND timestamp <= ?", start, end)

	if severity != "" {
		query = query.Where("severity = ?", severity)
	}

	err := query.Count(&count).Error
	return count, err
}
