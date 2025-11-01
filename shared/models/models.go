package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Role represents user role in the system
type Role string

const (
	RoleSuperAdmin     Role = "super_admin"
	RoleAdmin          Role = "admin"
	RoleDepartmentHead Role = "department_head"
	RoleEmployee       Role = "employee"
)

// UserStatus represents user online status
type UserStatus string

const (
	StatusOnline  UserStatus = "online"
	StatusBusy    UserStatus = "busy"
	StatusAway    UserStatus = "away"
	StatusOffline UserStatus = "offline"
)

// User represents a user in the system
type User struct {
	BaseModel
	Email          string     `gorm:"uniqueIndex;not null" json:"email"`
	Name           string     `gorm:"not null" json:"name"`
	HashedPassword string     `gorm:"not null" json:"-"`
	Role           Role       `gorm:"not null;default:'employee'" json:"role"`
	Status         UserStatus `gorm:"not null;default:'offline'" json:"status"`
	Avatar         string     `json:"avatar,omitempty"`
	Phone          string     `json:"phone,omitempty"`
	Department     string     `json:"department,omitempty"`
	Position       string     `json:"position,omitempty"`
	LastActiveAt   *time.Time `json:"last_active_at,omitempty"`
	IsActive       bool       `gorm:"not null;default:true" json:"is_active"`
}

// JWT Related Structures

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Claims represents JWT token claims
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

// LoginRequest represents login request payload
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

// LoginResponse represents login response payload
type LoginResponse struct {
	User               User             `json:"user"`
	Tokens             TokenPair        `json:"tokens,omitempty"`             // Only for JWT mode
	Session            *SessionResponse `json:"session,omitempty"`            // Only for session mode
	MustChangePassword bool             `json:"must_change_password,omitempty"`
	AuthMode           AuthMode         `json:"auth_mode"` // Indicates which auth mode was used
}

// RegisterRequest represents registration request payload
type RegisterRequest struct {
	Email      string `json:"email" validate:"required,email"`
	Name       string `json:"name" validate:"required,min=2"`
	Password   string `json:"password" validate:"required,min=6"`
	Department string `json:"department,omitempty"`
	Position   string `json:"position,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

// RefreshTokenRequest represents refresh token request payload
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Session Related Structures

// AuthMode represents authentication mode
// NOTE: Only session-based authentication is used (JWT support deprecated)
type AuthMode string

const (
	AuthModeJWT     AuthMode = "jwt"     // Deprecated: Use session instead
	AuthModeSession AuthMode = "session" // Session-based authentication (default)
)

// Session represents user session in stateful authentication
type Session struct {
	SessionID    string    `json:"session_id" redis:"session_id"`
	UserID       uint      `json:"user_id" redis:"user_id"`
	Email        string    `json:"email" redis:"email"`
	Role         Role      `json:"role" redis:"role"`
	IPAddress    string    `json:"ip_address" redis:"ip_address"`
	UserAgent    string    `json:"user_agent" redis:"user_agent"`
	CreatedAt    time.Time `json:"created_at" redis:"created_at"`
	ExpiresAt    time.Time `json:"expires_at" redis:"expires_at"`
	LastActiveAt time.Time `json:"last_active_at" redis:"last_active_at"`
}

// SessionResponse represents session data in API responses
type SessionResponse struct {
	SessionID string `json:"session_id"`
	ExpiresAt int64  `json:"expires_at"` // Unix timestamp
}
