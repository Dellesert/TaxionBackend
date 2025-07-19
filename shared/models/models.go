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
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleManager    Role = "manager"
	RoleEmployee   Role = "employee"
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
	User   User      `json:"user"`
	Tokens TokenPair `json:"tokens"`
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
