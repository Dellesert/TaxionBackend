package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// Department represents a department in the organization
type Department struct {
	models.BaseModel
	Name  string `gorm:"uniqueIndex;not null;size:100" json:"name" validate:"required,min=2,max=100"`
	Users []User `gorm:"foreignKey:DepartmentID" json:"users,omitempty"`
}

// TableName returns the table name for Department model
func (Department) TableName() string {
	return "departments"
}

// User represents a user in the user service
type User struct {
	models.BaseModel
	Email          string            `gorm:"uniqueIndex;not null;size:255" json:"email" validate:"required,email,max=255"`
	Name           string            `gorm:"not null;size:100" json:"name" validate:"required,min=2,max=100"`
	HashedPassword string            `gorm:"not null;size:255" json:"-" validate:"required"`
	Role           models.Role       `gorm:"not null;default:'employee';size:20" json:"role" validate:"required,oneof=super_admin admin manager employee"`
	Status         models.UserStatus `gorm:"not null;default:'offline';size:20" json:"status" validate:"oneof=online busy away offline"`
	DepartmentID   *uint             `gorm:"index" json:"department_id,omitempty"`
	Department     *Department       `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Avatar         string            `gorm:"size:500" json:"avatar,omitempty" validate:"omitempty,url,max=500"`
	Phone          string            `gorm:"size:20" json:"phone,omitempty" validate:"omitempty,e164,max=20"`
	Position       string            `gorm:"size:100" json:"position,omitempty" validate:"omitempty,max=100"`
	LastActiveAt   *time.Time        `json:"last_active_at,omitempty"`
	IsActive       bool              `gorm:"not null;default:true" json:"is_active"`
}

// TableName returns the table name for User model
func (User) TableName() string {
	return "users"
}

// BeforeCreate hook is called before creating a user
func (u *User) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if u.Role == "" {
		u.Role = models.RoleEmployee
	}
	if u.Status == "" {
		u.Status = models.StatusOffline
	}
	return nil
}

// BeforeUpdate hook is called before updating a user
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	// Update last active time when status changes to online
	if u.Status == models.StatusOnline {
		now := time.Now()
		u.LastActiveAt = &now
	}
	return nil
}

// CreateDepartmentRequest represents request for creating a department
type CreateDepartmentRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
}

// UpdateDepartmentRequest represents request for updating a department
type UpdateDepartmentRequest struct {
	Name *string `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
}

// CreateUserRequest represents request for creating a user
type CreateUserRequest struct {
	Email        string `json:"email" binding:"required,email,max=255" validate:"required,email,max=255"`
	Name         string `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
	Password     string `json:"password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
	Role         string `json:"role,omitempty" binding:"omitempty,oneof=super_admin admin manager employee" validate:"omitempty,oneof=super_admin admin manager employee"`
	DepartmentID *uint  `json:"department_id,omitempty" validate:"omitempty,min=1"`
	Phone        string `json:"phone,omitempty" binding:"omitempty,e164,max=20" validate:"omitempty,e164,max=20"`
	Position     string `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
}

// UpdateUserRequest represents request for updating a user
type UpdateUserRequest struct {
	Name         *string            `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
	Status       *models.UserStatus `json:"status,omitempty" binding:"omitempty,oneof=online busy away offline" validate:"omitempty,oneof=online busy away offline"`
	Avatar       *string            `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	Phone        *string            `json:"phone,omitempty" binding:"omitempty,e164,max=20" validate:"omitempty,e164,max=20"`
	Position     *string            `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	DepartmentID *uint              `json:"department_id,omitempty" validate:"omitempty,min=1"`
	IsActive     *bool              `json:"is_active,omitempty"`
}

// DepartmentResponse represents department response
type DepartmentResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserResponse represents user response (without sensitive data)
type UserResponse struct {
	ID           uint                `json:"id"`
	Email        string              `json:"email"`
	Name         string              `json:"name"`
	Role         models.Role         `json:"role"`
	Status       models.UserStatus   `json:"status"`
	DepartmentID *uint               `json:"department_id,omitempty"`
	Department   *DepartmentResponse `json:"department,omitempty"`
	Avatar       string              `json:"avatar,omitempty"`
	Phone        string              `json:"phone,omitempty"`
	Position     string              `json:"position,omitempty"`
	LastActiveAt *time.Time          `json:"last_active_at,omitempty"`
	IsActive     bool                `json:"is_active"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	response := &UserResponse{
		ID:           u.ID,
		Email:        u.Email,
		Name:         u.Name,
		Role:         u.Role,
		Status:       u.Status,
		DepartmentID: u.DepartmentID,
		Avatar:       u.Avatar,
		Phone:        u.Phone,
		Position:     u.Position,
		LastActiveAt: u.LastActiveAt,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}

	// Include department if loaded
	if u.Department != nil {
		response.Department = &DepartmentResponse{
			ID:        u.Department.ID,
			Name:      u.Department.Name,
			CreatedAt: u.Department.CreatedAt,
			UpdatedAt: u.Department.UpdatedAt,
		}
	}

	return response
}

// ToResponse converts Department to DepartmentResponse
func (d *Department) ToResponse() *DepartmentResponse {
	return &DepartmentResponse{
		ID:        d.ID,
		Name:      d.Name,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// Profile related request structures

// UpdateProfileRequest represents profile update request payload
type UpdateProfileRequest struct {
	Name         *string `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
	Avatar       *string `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	Phone        *string `json:"phone,omitempty" binding:"omitempty,max=20" validate:"omitempty,max=20"`
	Position     *string `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	DepartmentID *uint   `json:"department_id,omitempty" validate:"omitempty,min=0"`
}

// ChangePasswordRequest represents password change request payload
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required" validate:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
}

// UpdateStatusRequest represents status update request payload
type UpdateStatusRequest struct {
	Status models.UserStatus `json:"status" binding:"required,oneof=online busy away offline" validate:"required,oneof=online busy away offline"`
}

// DepartmentWithUsersResponse represents department with users response
type DepartmentWithUsersResponse struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	Users     []*UserResponse `json:"users"`
	UserCount int             `json:"user_count"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// UserStatsResponse represents user statistics response
type UserStatsResponse struct {
	TotalUsers    int `json:"total_users"`
	ActiveUsers   int `json:"active_users"`
	InactiveUsers int `json:"inactive_users"`
	OnlineUsers   int `json:"online_users"`
}

// AdminUpdateUserRoleRequest represents admin request to update user role
type AdminUpdateUserRoleRequest struct {
	Role models.Role `json:"role" binding:"required,oneof=super_admin admin manager employee" validate:"required,oneof=super_admin admin manager employee"`
}

// AdminUpdateUserStatusRequest represents admin request to update user status
type AdminUpdateUserStatusRequest struct {
	Status models.UserStatus `json:"status" binding:"required,oneof=online busy away offline" validate:"required,oneof=online busy away offline"`
}
