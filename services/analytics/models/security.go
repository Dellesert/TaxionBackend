package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// LoginAttempt represents a login attempt (successful or failed)
type LoginAttempt struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Email      string    `gorm:"type:varchar(255);index:idx_login_email" json:"email"`
	UserID     *uint64   `gorm:"index:idx_login_user" json:"user_id,omitempty"`
	IPAddress  string    `gorm:"type:varchar(45);index:idx_login_ip" json:"ip_address"`
	UserAgent  string    `gorm:"type:text" json:"user_agent"`
	Success    bool      `gorm:"index:idx_login_success" json:"success"`
	FailReason string    `gorm:"type:varchar(100)" json:"fail_reason,omitempty"`
	AuthMode   string    `gorm:"type:varchar(20)" json:"auth_mode,omitempty"` // jwt, session, passkey
	IsSuperAdmin bool    `gorm:"default:false" json:"is_super_admin"`
	DeviceType string    `gorm:"type:varchar(50)" json:"device_type,omitempty"` // mobile, desktop, tablet
	Browser    string    `gorm:"type:varchar(50)" json:"browser,omitempty"`
	Timestamp  time.Time `gorm:"not null;index:idx_login_timestamp" json:"timestamp"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
}

// TableName specifies the table name for LoginAttempt
func (LoginAttempt) TableName() string {
	return "login_attempts"
}

// BeforeCreate hook
func (l *LoginAttempt) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if l.Timestamp.IsZero() {
		l.Timestamp = now
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = now
	}
	return nil
}

// KnownDevice represents a known device for a user
type KnownDevice struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID            uint64    `gorm:"not null;index:idx_device_user" json:"user_id"`
	DeviceFingerprint string    `gorm:"type:varchar(255);uniqueIndex:idx_device_fingerprint" json:"device_fingerprint"`
	UserAgent         string    `gorm:"type:text" json:"user_agent"`
	DeviceType        string    `gorm:"type:varchar(50)" json:"device_type,omitempty"` // mobile, desktop, tablet
	Browser           string    `gorm:"type:varchar(50)" json:"browser,omitempty"`
	OS                string    `gorm:"type:varchar(50)" json:"os,omitempty"`
	IPAddress         string    `gorm:"type:varchar(45)" json:"ip_address"`
	FirstSeen         time.Time `gorm:"not null" json:"first_seen"`
	LastSeen          time.Time `gorm:"not null" json:"last_seen"`
	IsTrusted         bool      `gorm:"default:false" json:"is_trusted"`
	TrustLevel        int       `gorm:"default:0" json:"trust_level"` // 0-100
	LoginCount        int       `gorm:"default:0" json:"login_count"`
	CreatedAt         time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt         time.Time `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for KnownDevice
func (KnownDevice) TableName() string {
	return "known_devices"
}

// BeforeCreate hook
func (k *KnownDevice) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if k.FirstSeen.IsZero() {
		k.FirstSeen = now
	}
	if k.LastSeen.IsZero() {
		k.LastSeen = now
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = now
	}
	if k.UpdatedAt.IsZero() {
		k.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate hook
func (k *KnownDevice) BeforeUpdate(tx *gorm.DB) error {
	k.UpdatedAt = time.Now()
	return nil
}

// SecuritySession represents an active security session with detailed tracking
type SecuritySession struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       uint64    `gorm:"not null;index:idx_session_user" json:"user_id"`
	SessionID    string    `gorm:"type:varchar(255);uniqueIndex:idx_session_id" json:"session_id"`
	IPAddress    string    `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent    string    `gorm:"type:text" json:"user_agent"`
	DeviceType   string    `gorm:"type:varchar(50)" json:"device_type,omitempty"`
	Browser      string    `gorm:"type:varchar(50)" json:"browser,omitempty"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
	ExpiresAt    time.Time `gorm:"not null;index:idx_session_expires" json:"expires_at"`
	LastActivity time.Time `gorm:"not null" json:"last_activity"`
	IsActive     bool      `gorm:"index:idx_session_active;default:true" json:"is_active"`
}

// TableName specifies the table name for SecuritySession
func (SecuritySession) TableName() string {
	return "security_sessions"
}

// BeforeCreate hook
func (s *SecuritySession) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.LastActivity.IsZero() {
		s.LastActivity = now
	}
	return nil
}

// SuspiciousActivity represents detected suspicious activity
type SuspiciousActivity struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      *uint64        `gorm:"index:idx_suspicious_user" json:"user_id,omitempty"`
	Email       string         `gorm:"type:varchar(255);index:idx_suspicious_email" json:"email,omitempty"`
	IPAddress   string         `gorm:"type:varchar(45);index:idx_suspicious_ip" json:"ip_address"`
	ActivityType string        `gorm:"type:varchar(100)" json:"activity_type"` // multiple_failed_logins, new_device, unusual_location, etc.
	Severity    string         `gorm:"type:varchar(20)" json:"severity"` // low, medium, high, critical
	Description string         `gorm:"type:text" json:"description"`
	Metadata    datatypes.JSON `gorm:"type:jsonb" json:"metadata,omitempty"`
	IsResolved  bool           `gorm:"default:false;index:idx_suspicious_resolved" json:"is_resolved"`
	ResolvedAt  *time.Time     `json:"resolved_at,omitempty"`
	ResolvedBy  *uint64        `json:"resolved_by,omitempty"`
	Timestamp   time.Time      `gorm:"not null;index:idx_suspicious_timestamp" json:"timestamp"`
	CreatedAt   time.Time      `gorm:"not null" json:"created_at"`
}

// TableName specifies the table name for SuspiciousActivity
func (SuspiciousActivity) TableName() string {
	return "suspicious_activities"
}

// BeforeCreate hook
func (s *SuspiciousActivity) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if s.Timestamp.IsZero() {
		s.Timestamp = now
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	return nil
}

// Fail reason constants
const (
	FailReasonInvalidCredentials   = "invalid_credentials"
	FailReasonAccountDeactivated   = "account_deactivated"
	FailReason2FARequired          = "2fa_required"
	FailReason2FAFailed            = "2fa_failed"
	FailReasonPasskeyOnly          = "passkey_only"
	FailReasonSuperAdminWebOnly    = "super_admin_web_only"
	FailReasonPasswordExpired      = "password_expired"
	FailReasonAccountLocked        = "account_locked"
	FailReasonUnknown              = "unknown"
)

// Suspicious activity types
const (
	ActivityMultipleFailedLogins = "multiple_failed_logins"
	ActivityNewDevice            = "new_device"
	ActivityUnusualLocation      = "unusual_location"
	ActivityMultipleSessions     = "multiple_sessions"
	ActivityBruteForce           = "brute_force"
	ActivityPasswordSpray        = "password_spray"
	ActivityCredentialStuffing   = "credential_stuffing"
)

// Severity levels
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)
