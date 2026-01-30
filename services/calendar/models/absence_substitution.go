package models

import (
	"time"

	sharedmodels "tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// AbsenceSubstitution represents a substitute for an absent user
type AbsenceSubstitution struct {
	sharedmodels.BaseModel
	AbsenceID    uint      `gorm:"not null;index" json:"absence_id" validate:"required,min=1"`
	SubstituteID uint      `gorm:"not null;index" json:"substitute_id" validate:"required,min=1"`
	StartDate    time.Time `gorm:"not null;type:date;index" json:"start_date" validate:"required"`
	EndDate      time.Time `gorm:"not null;type:date;index" json:"end_date" validate:"required"`
	Note         string    `gorm:"size:500" json:"note,omitempty" validate:"omitempty,max=500"`
	CreatedBy    uint      `gorm:"not null;index" json:"created_by" validate:"required,min=1"`

	// Associations
	Absence    *Absence           `gorm:"foreignKey:AbsenceID" json:"absence,omitempty"`
	Substitute *sharedmodels.User `gorm:"foreignKey:SubstituteID" json:"substitute,omitempty"`
	Creator    *sharedmodels.User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// TableName returns the table name for AbsenceSubstitution model
func (AbsenceSubstitution) TableName() string {
	return "absence_substitutions"
}

// BeforeCreate hook is called before creating a substitution
func (s *AbsenceSubstitution) BeforeCreate(tx *gorm.DB) error {
	// Validate date logic
	if s.EndDate.Before(s.StartDate) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// BeforeUpdate hook is called before updating a substitution
func (s *AbsenceSubstitution) BeforeUpdate(tx *gorm.DB) error {
	// Validate date logic
	if s.EndDate.Before(s.StartDate) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// Request/Response Models

// CreateSubstitutionRequest represents request for creating a substitution
type CreateSubstitutionRequest struct {
	SubstituteID uint      `json:"substitute_id" binding:"required,min=1" validate:"required,min=1"`
	StartDate    time.Time `json:"start_date" binding:"required" validate:"required"`
	EndDate      time.Time `json:"end_date" binding:"required" validate:"required"`
	Note         string    `json:"note,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// UpdateSubstitutionRequest represents request for updating a substitution
type UpdateSubstitutionRequest struct {
	SubstituteID *uint      `json:"substitute_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Note         *string    `json:"note,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// SubstitutionResponse represents a substitution in API responses
type SubstitutionResponse struct {
	ID           uint               `json:"id"`
	AbsenceID    uint               `json:"absence_id"`
	SubstituteID uint               `json:"substitute_id"`
	Substitute   *sharedmodels.User `json:"substitute,omitempty"`
	StartDate    time.Time          `json:"start_date"`
	EndDate      time.Time          `json:"end_date"`
	Note         string             `json:"note,omitempty"`
	CreatedBy    uint               `json:"created_by"`
	Creator      *sharedmodels.User `json:"creator,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// ToResponse converts AbsenceSubstitution model to SubstitutionResponse
func (s *AbsenceSubstitution) ToResponse() *SubstitutionResponse {
	return &SubstitutionResponse{
		ID:           s.ID,
		AbsenceID:    s.AbsenceID,
		SubstituteID: s.SubstituteID,
		Substitute:   s.Substitute,
		StartDate:    s.StartDate,
		EndDate:      s.EndDate,
		Note:         s.Note,
		CreatedBy:    s.CreatedBy,
		Creator:      s.Creator,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

// SubstitutionListResponse represents a list of substitutions
type SubstitutionListResponse struct {
	Substitutions []*SubstitutionResponse `json:"substitutions"`
	Total         int64                   `json:"total"`
}
