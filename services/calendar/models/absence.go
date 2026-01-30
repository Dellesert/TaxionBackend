package models

import (
	"time"

	sharedmodels "tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// AbsenceType represents the type of absence
type AbsenceType string

const (
	AbsenceTypeVacation     AbsenceType = "vacation"      // Отпуск
	AbsenceTypeSickLeave    AbsenceType = "sick_leave"    // Больничный
	AbsenceTypeDayOff       AbsenceType = "day_off"       // Отгул
	AbsenceTypeBusinessTrip AbsenceType = "business_trip" // Командировка
	AbsenceTypeStudyLeave   AbsenceType = "study_leave"   // Учебный
)

// Absence represents a user absence record
type Absence struct {
	sharedmodels.BaseModel
	UserID    uint        `gorm:"not null;index" json:"user_id" validate:"required,min=1"`
	Type      AbsenceType `gorm:"not null;size:20;index" json:"type" validate:"required,oneof=vacation sick_leave day_off business_trip study_leave"`
	StartDate time.Time   `gorm:"not null;type:date;index" json:"start_date" validate:"required"`
	EndDate   time.Time   `gorm:"not null;type:date;index" json:"end_date" validate:"required"`
	Reason    string      `gorm:"size:500" json:"reason,omitempty" validate:"omitempty,max=500"`
	CreatedBy uint        `gorm:"not null;index" json:"created_by" validate:"required,min=1"`

	// Associations
	User    *sharedmodels.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Creator *sharedmodels.User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// TableName returns the table name for Absence model
func (Absence) TableName() string {
	return "absences"
}

// BeforeCreate hook is called before creating an absence
func (a *Absence) BeforeCreate(tx *gorm.DB) error {
	// Validate date logic
	if a.EndDate.Before(a.StartDate) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// BeforeUpdate hook is called before updating an absence
func (a *Absence) BeforeUpdate(tx *gorm.DB) error {
	// Validate date logic
	if a.EndDate.Before(a.StartDate) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// Request/Response Models

// CreateAbsenceRequest represents request for creating an absence
type CreateAbsenceRequest struct {
	UserID    uint        `json:"user_id" binding:"required,min=1" validate:"required,min=1"`
	Type      AbsenceType `json:"type" binding:"required,oneof=vacation sick_leave day_off business_trip study_leave" validate:"required,oneof=vacation sick_leave day_off business_trip study_leave"`
	StartDate time.Time   `json:"start_date" binding:"required" validate:"required"`
	EndDate   time.Time   `json:"end_date" binding:"required" validate:"required"`
	Reason    string      `json:"reason,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// UpdateAbsenceRequest represents request for updating an absence
type UpdateAbsenceRequest struct {
	Type      *AbsenceType `json:"type,omitempty" binding:"omitempty,oneof=vacation sick_leave day_off business_trip study_leave" validate:"omitempty,oneof=vacation sick_leave day_off business_trip study_leave"`
	StartDate *time.Time   `json:"start_date,omitempty"`
	EndDate   *time.Time   `json:"end_date,omitempty"`
	Reason    *string      `json:"reason,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// AbsenceResponse represents an absence in API responses
type AbsenceResponse struct {
	ID            uint                    `json:"id"`
	UserID        uint                    `json:"user_id"`
	User          *sharedmodels.User      `json:"user,omitempty"`
	Type          AbsenceType             `json:"type"`
	StartDate     time.Time               `json:"start_date"`
	EndDate       time.Time               `json:"end_date"`
	Reason        string                  `json:"reason,omitempty"`
	CreatedBy     uint                    `json:"created_by"`
	Creator       *sharedmodels.User      `json:"creator,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	Substitutions []*SubstitutionResponse `json:"substitutions,omitempty"`
}

// ToResponse converts Absence model to AbsenceResponse
func (a *Absence) ToResponse() *AbsenceResponse {
	return &AbsenceResponse{
		ID:        a.ID,
		UserID:    a.UserID,
		User:      a.User,
		Type:      a.Type,
		StartDate: a.StartDate,
		EndDate:   a.EndDate,
		Reason:    a.Reason,
		CreatedBy: a.CreatedBy,
		Creator:   a.Creator,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

// AbsenceListResponse represents a paginated list of absences
type AbsenceListResponse struct {
	Absences []*AbsenceResponse `json:"absences"`
	Total    int64              `json:"total"`
	Limit    int                `json:"limit"`
	Offset   int                `json:"offset"`
}
