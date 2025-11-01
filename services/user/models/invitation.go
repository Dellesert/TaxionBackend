package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// InvitationStatus represents the status of an invitation
type InvitationStatus string

const (
	InvitationStatusPending   InvitationStatus = "pending"
	InvitationStatusAccepted  InvitationStatus = "accepted"
	InvitationStatusExpired   InvitationStatus = "expired"
	InvitationStatusCancelled InvitationStatus = "cancelled"
)

// Invitation represents an invitation to join the system
type Invitation struct {
	models.BaseModel
	Token        string           `gorm:"uniqueIndex;not null;size:128" json:"token"`
	Email        string           `gorm:"index;not null;size:255" json:"email" validate:"required,email,max=255"`
	Name         string           `gorm:"not null;size:100" json:"name" validate:"required,min=2,max=100"`
	Role         models.Role      `gorm:"not null;size:20" json:"role" validate:"required,oneof=super_admin admin department_head employee"`
	DepartmentID *uint            `gorm:"index" json:"department_id,omitempty"`
	Department   *Department      `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Position     string           `gorm:"size:100" json:"position,omitempty" validate:"omitempty,max=100"`
	Phone        string           `gorm:"size:20" json:"phone,omitempty" validate:"omitempty,max=20"`
	Status       InvitationStatus `gorm:"not null;default:'pending';index;size:20" json:"status"`
	ExpiresAt    time.Time        `gorm:"not null;index" json:"expires_at"`
	AcceptedAt   *time.Time       `json:"accepted_at,omitempty"`
	CreatedByID  uint             `gorm:"not null;index" json:"created_by_id"`
	CreatedBy    *User            `gorm:"foreignKey:CreatedByID" json:"created_by,omitempty"`
	UserID       *uint            `gorm:"index" json:"user_id,omitempty"`
	User         *User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for Invitation model
func (Invitation) TableName() string {
	return "invitations"
}

// BeforeCreate hook is called before creating an invitation
func (i *Invitation) BeforeCreate(tx *gorm.DB) error {
	// Set default status if not provided
	if i.Status == "" {
		i.Status = InvitationStatusPending
	}
	return nil
}

// IsValid checks if the invitation is valid and not expired
func (i *Invitation) IsValid() bool {
	return i.Status == InvitationStatusPending && time.Now().Before(i.ExpiresAt)
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// CreateInvitationRequest represents request for creating an invitation
type CreateInvitationRequest struct {
	Email         string      `json:"email" binding:"required,email,max=255" validate:"required,email,max=255"`
	Name          string      `json:"name" binding:"required,min=2,max=100" validate:"required,min=2,max=100"`
	Role          string      `json:"role" binding:"required,oneof=super_admin admin department_head employee" validate:"required,oneof=super_admin admin department_head employee"`
	DepartmentID  *uint       `json:"department_id,omitempty"`
	Position      string      `json:"position,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	Phone         string      `json:"phone,omitempty" binding:"omitempty,max=20" validate:"omitempty,max=20"`
	ExpiresInDays int         `json:"expires_in_days" binding:"required,min=1,max=365" validate:"required,min=1,max=365"` // Configurable expiration
}

// BulkCreateInvitationsRequest represents request for bulk creating invitations
type BulkCreateInvitationsRequest struct {
	Invitations   []CreateInvitationRequest `json:"invitations" binding:"required,min=1,max=100" validate:"required,min=1,dive"`
	ExpiresInDays int                       `json:"expires_in_days" binding:"required,min=1,max=365" validate:"required,min=1,max=365"`
}

// AcceptInvitationRequest represents request for accepting an invitation
type AcceptInvitationRequest struct {
	Password        string `json:"password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6,max=100" validate:"required,min=6,max=100"`
}

// InvitationResponse represents invitation response
type InvitationResponse struct {
	ID            uint                `json:"id"`
	Token         string              `json:"token"`
	Email         string              `json:"email"`
	Name          string              `json:"name"`
	Role          models.Role         `json:"role"`
	DepartmentID  *uint               `json:"department_id,omitempty"`
	Department    *DepartmentResponse `json:"department,omitempty"`
	Position      string              `json:"position,omitempty"`
	Phone         string              `json:"phone,omitempty"`
	Status        InvitationStatus    `json:"status"`
	ExpiresAt     time.Time           `json:"expires_at"`
	AcceptedAt    *time.Time          `json:"accepted_at,omitempty"`
	CreatedByID   uint                `json:"created_by_id"`
	CreatedBy     *UserResponse       `json:"created_by,omitempty"`
	UserID        *uint               `json:"user_id,omitempty"`
	User          *UserResponse       `json:"user,omitempty"`
	InviteLink    string              `json:"invite_link,omitempty"` // Only included when creating
	IsValid       bool                `json:"is_valid"`
	IsExpired     bool                `json:"is_expired"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

// ToResponse converts Invitation to InvitationResponse
func (i *Invitation) ToResponse() *InvitationResponse {
	response := &InvitationResponse{
		ID:           i.ID,
		Token:        i.Token,
		Email:        i.Email,
		Name:         i.Name,
		Role:         i.Role,
		DepartmentID: i.DepartmentID,
		Position:     i.Position,
		Phone:        i.Phone,
		Status:       i.Status,
		ExpiresAt:    i.ExpiresAt,
		AcceptedAt:   i.AcceptedAt,
		CreatedByID:  i.CreatedByID,
		UserID:       i.UserID,
		IsValid:      i.IsValid(),
		IsExpired:    i.IsExpired(),
		CreatedAt:    i.CreatedAt,
		UpdatedAt:    i.UpdatedAt,
	}

	// Include department if loaded
	if i.Department != nil {
		response.Department = i.Department.ToResponse()
	}

	// Include created by user if loaded
	if i.CreatedBy != nil {
		response.CreatedBy = i.CreatedBy.ToResponse()
	}

	// Include created user if loaded
	if i.User != nil {
		response.User = i.User.ToResponse()
	}

	return response
}

// PublicInvitationResponse represents public invitation response (for validation endpoint)
// This doesn't include sensitive information like token or creator details
type PublicInvitationResponse struct {
	Email        string              `json:"email"`
	Name         string              `json:"name"`
	Role         models.Role         `json:"role"`
	DepartmentID *uint               `json:"department_id,omitempty"`
	Department   *DepartmentResponse `json:"department,omitempty"`
	Position     string              `json:"position,omitempty"`
	ExpiresAt    time.Time           `json:"expires_at"`
	IsValid      bool                `json:"is_valid"`
}

// ToPublicResponse converts Invitation to PublicInvitationResponse
func (i *Invitation) ToPublicResponse() *PublicInvitationResponse {
	response := &PublicInvitationResponse{
		Email:        i.Email,
		Name:         i.Name,
		Role:         i.Role,
		DepartmentID: i.DepartmentID,
		Position:     i.Position,
		ExpiresAt:    i.ExpiresAt,
		IsValid:      i.IsValid(),
	}

	// Include department if loaded
	if i.Department != nil {
		response.Department = i.Department.ToResponse()
	}

	return response
}

// InvitationListResponse represents paginated list of invitations
type InvitationListResponse struct {
	Invitations []*InvitationResponse `json:"invitations"`
	Total       int64                 `json:"total"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
	TotalPages  int                   `json:"total_pages"`
}

// InvitationStatsResponse represents invitation statistics
type InvitationStatsResponse struct {
	TotalInvitations     int `json:"total_invitations"`
	PendingInvitations   int `json:"pending_invitations"`
	AcceptedInvitations  int `json:"accepted_invitations"`
	ExpiredInvitations   int `json:"expired_invitations"`
	CancelledInvitations int `json:"cancelled_invitations"`
}

// CSVInvitationRow represents a single invitation row from CSV import
type CSVInvitationRow struct {
	Email        string `csv:"email"`
	Name         string `csv:"name"`
	Role         string `csv:"role"`
	DepartmentID string `csv:"department_id"`
	Position     string `csv:"position"`
	Phone        string `csv:"phone"`
}

// ImportInvitationsResponse represents response from CSV import
type ImportInvitationsResponse struct {
	TotalRows          int                    `json:"total_rows"`
	SuccessCount       int                    `json:"success_count"`
	ErrorCount         int                    `json:"error_count"`
	SuccessInvitations []*InvitationResponse  `json:"success_invitations"`
	Errors             []ImportError          `json:"errors"`
}
