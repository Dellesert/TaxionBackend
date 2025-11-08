package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// Department represents a department in the organization
type Department struct {
	models.BaseModel
	Name           string          `gorm:"uniqueIndex;not null;size:100" json:"name" validate:"required,min=2,max=100"`
	HeadID         *uint           `gorm:"index" json:"head_id,omitempty"`
	Users          []User          `gorm:"foreignKey:DepartmentID" json:"users,omitempty"`
	Subdepartments []Subdepartment `gorm:"foreignKey:DepartmentID" json:"subdepartments,omitempty"`
}

// Subdepartment represents a subdivision within a department
type Subdepartment struct {
	models.BaseModel
	Name         string      `gorm:"not null;size:100" json:"name" validate:"required,min=2,max=100"`
	DepartmentID uint        `gorm:"not null;index" json:"department_id" validate:"required"`
	Department   *Department `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	HeadID       *uint       `gorm:"index" json:"head_id,omitempty"`
	Users        []User      `gorm:"foreignKey:SubdepartmentID" json:"users,omitempty"`
}

// TableName returns the table name for Department model
func (Department) TableName() string {
	return "departments"
}

// TableName returns the table name for Subdepartment model
func (Subdepartment) TableName() string {
	return "subdepartments"
}

// User represents a user in the user service
type User struct {
	models.BaseModel
	Email          string            `gorm:"uniqueIndex;not null;size:255" json:"email" validate:"required,email,max=255"`
	Name           string            `gorm:"not null;size:100" json:"name" validate:"required,min=2,max=100"`
	FirstName      string            `gorm:"size:100" json:"first_name,omitempty" validate:"omitempty,max=100"`
	LastName       string            `gorm:"size:100" json:"last_name,omitempty" validate:"omitempty,max=100"`
	MiddleName     string            `gorm:"size:100" json:"middle_name,omitempty" validate:"omitempty,max=100"`
	BirthDate      *time.Time        `gorm:"type:date" json:"birth_date,omitempty"`
	HashedPassword *string           `gorm:"size:255" json:"-"` // Nullable for passkey-only users
	Role           models.Role       `gorm:"not null;default:'employee';size:20" json:"role" validate:"required,oneof=super_admin admin department_head employee"`
	Status         models.UserStatus `gorm:"not null;default:'offline';size:20" json:"status" validate:"oneof=online busy away offline"`
	DepartmentID   *uint             `gorm:"index" json:"department_id,omitempty"`
	Department     *Department       `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	SubdepartmentID *uint            `gorm:"index" json:"subdepartment_id,omitempty"`
	Subdepartment   *Subdepartment   `gorm:"foreignKey:SubdepartmentID" json:"subdepartment,omitempty"`
	Avatar         string            `gorm:"size:500" json:"avatar,omitempty" validate:"omitempty,url,max=500"`
	Phone              string            `gorm:"size:20" json:"phone,omitempty" validate:"omitempty,e164,max=20"`
	Position           string            `gorm:"size:100" json:"position,omitempty" validate:"omitempty,max=100"`
	LastActiveAt       *time.Time        `json:"last_active_at,omitempty"`
	IsActive           bool              `gorm:"not null" json:"is_active"`
	MustChangePassword bool              `gorm:"not null;default:false" json:"must_change_password"`

	// Authentication settings
	TwoFactorEnabled   bool              `gorm:"not null;default:false" json:"two_factor_enabled"`
	PasskeyEnabled     bool              `gorm:"not null;default:false" json:"passkey_enabled"` // True if user has at least one passkey
	PreferredSecondFactor string         `gorm:"size:20;default:'email'" json:"preferred_second_factor"` // "email" | "passkey"
	PasswordChangedAt  *time.Time        `json:"password_changed_at,omitempty"` // Track password age for expiration
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

// IsPasswordExpired checks if the user's password has expired based on system settings
func (u *User) IsPasswordExpired(passwordExpirationDays int) bool {
	// If password expiration is disabled (0), password never expires
	if passwordExpirationDays == 0 {
		return false
	}

	// If PasswordChangedAt is not set, consider it expired for security
	if u.PasswordChangedAt == nil {
		return true
	}

	// Calculate expiration date
	expirationDate := u.PasswordChangedAt.AddDate(0, 0, passwordExpirationDays)
	return time.Now().After(expirationDate)
}

// DaysUntilPasswordExpires returns the number of days until password expires (-1 if already expired, 0 if disabled)
func (u *User) DaysUntilPasswordExpires(passwordExpirationDays int) int {
	if passwordExpirationDays == 0 {
		return 0 // Expiration disabled
	}

	if u.PasswordChangedAt == nil {
		return -1 // Already expired
	}

	expirationDate := u.PasswordChangedAt.AddDate(0, 0, passwordExpirationDays)
	daysRemaining := int(time.Until(expirationDate).Hours() / 24)

	if daysRemaining < 0 {
		return -1 // Already expired
	}

	return daysRemaining
}

// CreateDepartmentRequest represents request for creating a department
type CreateDepartmentRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
}

// UpdateDepartmentRequest represents request for updating a department
type UpdateDepartmentRequest struct {
	HeadID *uint   `json:"head_id,omitempty"`
	Name *string `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
}

// CreateSubdepartmentRequest represents request for creating a subdepartment
type CreateSubdepartmentRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
	DepartmentID uint   `json:"department_id" binding:"required" validate:"required"`
	HeadID       *uint  `json:"head_id,omitempty"`
}

// UpdateSubdepartmentRequest represents request for updating a subdepartment
type UpdateSubdepartmentRequest struct {
	Name   *string `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
	HeadID *uint   `json:"head_id,omitempty"`
}

// CreateUserRequest represents request for creating a user
type CreateUserRequest struct {
	Email           string     `json:"email" binding:"required,email,max=255" validate:"required,email,max=255"`
	Name            string     `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
	FirstName       string     `json:"first_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	LastName        string     `json:"last_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	MiddleName      string     `json:"middle_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	BirthDate       *time.Time `json:"birth_date,omitempty"`
	Password        string     `json:"password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
	Role            string     `json:"role,omitempty" binding:"omitempty,oneof=super_admin admin department_head employee" validate:"omitempty,oneof=super_admin admin department_head employee"`
	DepartmentID    *uint      `json:"department_id,omitempty"`
	SubdepartmentID *uint      `json:"subdepartment_id,omitempty"`
	Phone           string     `json:"phone,omitempty" binding:"omitempty,e164,max=20" validate:"omitempty,e164,max=20"`
	Position        string     `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
}

// UpdateUserRequest represents request for updating a user
type UpdateUserRequest struct {
	Name            *string            `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
	FirstName       *string            `json:"first_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	LastName        *string            `json:"last_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	MiddleName      *string            `json:"middle_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	BirthDate       *time.Time         `json:"birth_date,omitempty"`
	Status          *models.UserStatus `json:"status,omitempty" binding:"omitempty,oneof=online busy away offline" validate:"omitempty,oneof=online busy away offline"`
	Avatar          *string            `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	Phone           *string            `json:"phone,omitempty" binding:"omitempty,e164,max=20" validate:"omitempty,e164,max=20"`
	Position        *string            `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	DepartmentID    *uint              `json:"department_id,omitempty"`
	SubdepartmentID *uint              `json:"subdepartment_id,omitempty"`
	IsActive        *bool              `json:"is_active,omitempty"`
}

// DepartmentResponse represents department response
type DepartmentResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	HeadID    *uint     `json:"head_id,omitempty"`
	UserCount int       `json:"user_count,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubdepartmentResponse represents subdepartment response
type SubdepartmentResponse struct {
	ID           uint                `json:"id"`
	Name         string              `json:"name"`
	DepartmentID uint                `json:"department_id"`
	Department   *DepartmentResponse `json:"department,omitempty"`
	HeadID       *uint               `json:"head_id,omitempty"`
	UserCount    int                 `json:"user_count,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// UserResponse represents user response (without sensitive data)
type UserResponse struct {
	ID              uint                   `json:"id"`
	Email           string                 `json:"email"`
	Name            string                 `json:"name"`
	FirstName       string                 `json:"first_name,omitempty"`
	LastName        string                 `json:"last_name,omitempty"`
	MiddleName      string                 `json:"middle_name,omitempty"`
	BirthDate       *string                `json:"birth_date,omitempty"`
	Role            models.Role            `json:"role"`
	Status          models.UserStatus      `json:"status"`
	DepartmentID    *uint                  `json:"department_id,omitempty"`
	Department      *DepartmentResponse    `json:"department,omitempty"`
	SubdepartmentID *uint                  `json:"subdepartment_id,omitempty"`
	Subdepartment   *SubdepartmentResponse `json:"subdepartment,omitempty"`
	Avatar          string                 `json:"avatar,omitempty"`
	Phone              string              `json:"phone,omitempty"`
	Position           string              `json:"position,omitempty"`
	LastActiveAt       *time.Time          `json:"last_active_at,omitempty"`
	IsActive           bool                `json:"is_active"`
	MustChangePassword bool                `json:"must_change_password"`
	TwoFactorEnabled   bool                `json:"two_factor_enabled"`
	PasskeyEnabled     bool                `json:"passkey_enabled"`
	PreferredSecondFactor string           `json:"preferred_second_factor,omitempty"`
	PasswordChangedAt  *time.Time          `json:"password_changed_at,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

// ToResponse converts User to UserResponse
func (u *User) ToResponse() *UserResponse {
	// Convert BirthDate from *time.Time to *string (YYYY-MM-DD format)
	var birthDateStr *string
	if u.BirthDate != nil {
		formatted := u.BirthDate.Format("2006-01-02")
		birthDateStr = &formatted
	}

	response := &UserResponse{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		MiddleName:      u.MiddleName,
		BirthDate:       birthDateStr,
		Role:            u.Role,
		Status:          u.Status,
		DepartmentID:    u.DepartmentID,
		SubdepartmentID: u.SubdepartmentID,
		Avatar:             u.Avatar,
		Phone:              u.Phone,
		Position:           u.Position,
		LastActiveAt:       u.LastActiveAt,
		IsActive:           u.IsActive,
		MustChangePassword: u.MustChangePassword,
		TwoFactorEnabled:   u.TwoFactorEnabled,
		PasskeyEnabled:     u.PasskeyEnabled,
		PreferredSecondFactor: u.PreferredSecondFactor,
		PasswordChangedAt:  u.PasswordChangedAt,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
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

	// Include subdepartment if loaded
	if u.Subdepartment != nil {
		response.Subdepartment = &SubdepartmentResponse{
			ID:           u.Subdepartment.ID,
			Name:         u.Subdepartment.Name,
			DepartmentID: u.Subdepartment.DepartmentID,
			HeadID:       u.Subdepartment.HeadID,
			CreatedAt:    u.Subdepartment.CreatedAt,
			UpdatedAt:    u.Subdepartment.UpdatedAt,
		}
	}

	return response
}

// ToResponse converts Department to DepartmentResponse
func (d *Department) ToResponse() *DepartmentResponse {
	return &DepartmentResponse{
		ID:        d.ID,
		Name:      d.Name,
		HeadID:    d.HeadID,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// ToResponse converts Subdepartment to SubdepartmentResponse
func (s *Subdepartment) ToResponse() *SubdepartmentResponse {
	response := &SubdepartmentResponse{
		ID:           s.ID,
		Name:         s.Name,
		DepartmentID: s.DepartmentID,
		HeadID:       s.HeadID,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}

	// Include department if loaded
	if s.Department != nil {
		response.Department = &DepartmentResponse{
			ID:        s.Department.ID,
			Name:      s.Department.Name,
			HeadID:    s.Department.HeadID,
			CreatedAt: s.Department.CreatedAt,
			UpdatedAt: s.Department.UpdatedAt,
		}
	}

	return response
}

// Profile related request structures

// UpdateProfileRequest represents profile update request payload
type UpdateProfileRequest struct {
	Name            *string `json:"name,omitempty" binding:"omitempty,min=2,max=100" validate:"omitempty,min=2,max=100"`
	FirstName       *string `json:"first_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	LastName        *string `json:"last_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	MiddleName      *string `json:"middle_name,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	BirthDate       *string `json:"birth_date,omitempty" binding:"omitempty" validate:"omitempty"`
	Avatar          *string `json:"avatar,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
	Phone           *string `json:"phone,omitempty" binding:"omitempty,max=20" validate:"omitempty,max=20"`
	Position        *string `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	DepartmentID    *uint   `json:"department_id,omitempty" validate:"omitempty,min=0"`
	SubdepartmentID *uint   `json:"subdepartment_id,omitempty" validate:"omitempty,min=0"`
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
	Role models.Role `json:"role" binding:"required,oneof=super_admin admin department_head employee" validate:"required,oneof=super_admin admin department_head employee"`
}

// AdminUpdateUserStatusRequest represents admin request to update user status
type AdminUpdateUserStatusRequest struct {
	Status models.UserStatus `json:"status" binding:"required,oneof=online busy away offline" validate:"required,oneof=online busy away offline"`
}

// AdminUpdate2FARequest represents admin request to enable/disable 2FA for a user
type AdminUpdate2FARequest struct {
	TwoFactorEnabled bool `json:"two_factor_enabled"`
}

// CSVUserRow represents a single user row from CSV import
type CSVUserRow struct {
	Email        string `csv:"email"`
	Name         string `csv:"name"`
	FirstName    string `csv:"first_name"`
	LastName     string `csv:"last_name"`
	MiddleName   string `csv:"middle_name"`
	BirthDate    string `csv:"birth_date"`
	Password     string `csv:"password"`
	Role         string `csv:"role"`
	DepartmentID string `csv:"department_id"`
	Phone        string `csv:"phone"`
	Position     string `csv:"position"`
}

// ImportUsersResponse represents response from CSV import
type ImportUsersResponse struct {
	TotalRows      int                    `json:"total_rows"`
	SuccessCount   int                    `json:"success_count"`
	ErrorCount     int                    `json:"error_count"`
	SuccessUsers   []*UserResponse        `json:"success_users"`
	Errors         []ImportError          `json:"errors"`
}

// ImportError represents an error that occurred during import
type ImportError struct {
	Row     int    `json:"row"`
	Email   string `json:"email,omitempty"`
	Message string `json:"message"`
}
