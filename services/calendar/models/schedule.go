package models

import (
	"time"

	sharedmodels "tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// ScheduleType represents the type of schedule
type ScheduleType string

const (
	ScheduleTypeWork         ScheduleType = "work"          // Рабочий график (повторяющийся)
	ScheduleTypePaidServices ScheduleType = "paid_services" // Платные услуги (ежемесячный)
	ScheduleTypeOnDuty       ScheduleType = "on_duty"       // Дежурства (ежемесячный)
	ScheduleTypeVK           ScheduleType = "vk"            // ВК (ежемесячный)
	ScheduleTypeTrips        ScheduleType = "trips"         // Выезды (ежемесячный)
)

// ScheduleVisibility represents who can view the schedule
type ScheduleVisibility string

const (
	VisibilityCreatorOnly  ScheduleVisibility = "creator_only" // Только создатель
	VisibilityManagement   ScheduleVisibility = "management"   // Руководство
	VisibilityParticipants ScheduleVisibility = "participants" // Участники
)

// ScheduleMode represents whether schedule is recurring or monthly
type ScheduleMode string

const (
	ScheduleModeRecurring ScheduleMode = "recurring" // Повторяющийся (автоматически генерируется из шаблона)
	ScheduleModeMonthly   ScheduleMode = "monthly"   // Ежемесячный (загружается вручную каждый месяц)
)

// Schedule represents a work schedule
type Schedule struct {
	sharedmodels.BaseModel
	Title         string             `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Description   string             `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=2000"`
	Type          ScheduleType       `gorm:"not null;default:'work';size:30" json:"type" validate:"required,oneof=work paid_services on_duty vk trips"`
	Visibility    ScheduleVisibility `gorm:"not null;default:'management';size:30" json:"visibility" validate:"required,oneof=creator_only management participants"`
	CreatedBy     uint               `gorm:"not null;index" json:"created_by" validate:"required,min=1"`
	StartDate     time.Time          `gorm:"not null;index" json:"start_date" validate:"required"`
	EndDate       time.Time          `gorm:"not null;index" json:"end_date" validate:"required"`
	IsForAllUsers bool               `gorm:"not null;default:false" json:"is_for_all_users"`
	DepartmentID  *uint              `gorm:"index" json:"department_id,omitempty" validate:"omitempty,min=1"`
	Color         string             `gorm:"size:7;default:'#4CAF50'" json:"color" validate:"omitempty,len=7"`
	IsActive      bool               `gorm:"not null;default:true;index" json:"is_active"`
	Mode          ScheduleMode       `gorm:"not null;default:'monthly';size:20" json:"mode" validate:"omitempty,oneof=recurring monthly"`
	TemplateID    *uint              `gorm:"index" json:"template_id,omitempty" validate:"omitempty,min=1"`
	ImportedFrom  *string            `gorm:"size:500" json:"imported_from,omitempty"`

	// Default shift times
	MorningStart string `gorm:"size:5;default:'10:00'" json:"morning_start" validate:"omitempty,len=5"` // "10:00"
	MorningEnd   string `gorm:"size:5;default:'14:00'" json:"morning_end" validate:"omitempty,len=5"`   // "14:00"
	EveningStart string `gorm:"size:5;default:'14:00'" json:"evening_start" validate:"omitempty,len=5"` // "14:00"
	EveningEnd   string `gorm:"size:5;default:'18:00'" json:"evening_end" validate:"omitempty,len=5"`   // "18:00"

	// Associations
	Creator     *sharedmodels.User   `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Entries     []ScheduleEntry      `gorm:"foreignKey:ScheduleID;constraint:OnDelete:CASCADE" json:"entries,omitempty"`
	Assignments []ScheduleAssignment `gorm:"foreignKey:ScheduleID;constraint:OnDelete:CASCADE" json:"assignments,omitempty"`
	Template    *ScheduleTemplate    `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
}

// TableName returns the table name for Schedule model
func (Schedule) TableName() string {
	return "schedules"
}

// BeforeCreate hook is called before creating a schedule
func (s *Schedule) BeforeCreate(tx *gorm.DB) error {
	// Set default values
	if s.Type == "" {
		s.Type = ScheduleTypeWork
	}
	if s.Visibility == "" {
		s.Visibility = VisibilityManagement
	}
	if s.Color == "" {
		s.Color = "#4CAF50"
	}
	if s.MorningStart == "" {
		s.MorningStart = "10:00"
	}
	if s.MorningEnd == "" {
		s.MorningEnd = "14:00"
	}
	if s.EveningStart == "" {
		s.EveningStart = "14:00"
	}
	if s.EveningEnd == "" {
		s.EveningEnd = "18:00"
	}
	if s.Mode == "" {
		// Set default mode based on schedule type
		switch s.Type {
		case ScheduleTypeWork:
			s.Mode = ScheduleModeRecurring
		default:
			// Платные услуги, ВК, дежурства, выезды - ежемесячные
			s.Mode = ScheduleModeMonthly
		}
	}

	// Validate date logic
	if s.EndDate.Before(s.StartDate) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// BeforeUpdate hook is called before updating a schedule
func (s *Schedule) BeforeUpdate(tx *gorm.DB) error {
	// Validate date logic
	if s.EndDate.Before(s.StartDate) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// ShiftType represents the type of shift
type ShiftType string

const (
	ShiftMorning ShiftType = "morning"  // Утро (У)
	ShiftEvening ShiftType = "evening"  // Вечер (В)
	ShiftFullDay ShiftType = "full_day" // Весь день (У/В)
	ShiftCustom  ShiftType = "custom"   // Кастомное время
)

// ScheduleEntry represents an entry in a schedule
type ScheduleEntry struct {
	sharedmodels.BaseModel
	ScheduleID  uint      `gorm:"not null;index" json:"schedule_id" validate:"required"`
	UserID      uint      `gorm:"not null;index" json:"user_id" validate:"required"`
	Date        time.Time `gorm:"not null;index;type:date" json:"date" validate:"required"`
	ShiftType   ShiftType `gorm:"not null;default:'morning';size:20" json:"shift_type" validate:"required,oneof=morning evening full_day custom"`
	StartTime   time.Time `gorm:"not null" json:"start_time" validate:"required"`
	EndTime     time.Time `gorm:"not null" json:"end_time" validate:"required"`
	Title       string    `gorm:"size:255" json:"title,omitempty" validate:"omitempty,max=255"`
	Description string    `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=1000"`
	Location    string    `gorm:"size:500" json:"location,omitempty" validate:"omitempty,max=500"`
	EventID     *uint     `gorm:"index" json:"event_id,omitempty"`
	CreatedBy   uint      `gorm:"not null;index" json:"created_by" validate:"required,min=1"`

	// Associations
	Schedule *Schedule          `gorm:"foreignKey:ScheduleID" json:"schedule,omitempty"`
	User     *sharedmodels.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Event    *Event             `gorm:"foreignKey:EventID" json:"event,omitempty"`
	Creator  *sharedmodels.User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

// TableName returns the table name for ScheduleEntry model
func (ScheduleEntry) TableName() string {
	return "schedule_entries"
}

// BeforeCreate hook is called before creating a schedule entry
func (se *ScheduleEntry) BeforeCreate(tx *gorm.DB) error {
	// Set default shift type
	if se.ShiftType == "" {
		se.ShiftType = ShiftMorning
	}

	// Validate time logic
	if se.EndTime.Before(se.StartTime) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// BeforeUpdate hook is called before updating a schedule entry
func (se *ScheduleEntry) BeforeUpdate(tx *gorm.DB) error {
	// Validate time logic
	if se.EndTime.Before(se.StartTime) {
		return gorm.ErrInvalidValue
	}
	return nil
}

// ScheduleTemplate represents a reusable schedule template
type ScheduleTemplate struct {
	sharedmodels.BaseModel
	Title        string       `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Description  string       `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=2000"`
	Type         ScheduleType `gorm:"not null;default:'work';size:30" json:"type" validate:"required,oneof=work paid_services on_duty vk trips"`
	CreatedBy    uint         `gorm:"not null;index" json:"created_by" validate:"required,min=1"`
	DepartmentID *uint        `gorm:"index" json:"department_id,omitempty" validate:"omitempty,min=1"`
	Color        string       `gorm:"size:7;default:'#4CAF50'" json:"color" validate:"omitempty,len=7"`
	IsActive     bool         `gorm:"not null;default:true;index" json:"is_active"`

	// Associations
	Creator *sharedmodels.User      `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
	Entries []ScheduleTemplateEntry `gorm:"foreignKey:TemplateID;constraint:OnDelete:CASCADE" json:"entries,omitempty"`
}

// TableName returns the table name for ScheduleTemplate model
func (ScheduleTemplate) TableName() string {
	return "schedule_templates"
}

// BeforeCreate hook is called before creating a schedule template
func (st *ScheduleTemplate) BeforeCreate(tx *gorm.DB) error {
	if st.Type == "" {
		st.Type = ScheduleTypeWork
	}
	if st.Color == "" {
		st.Color = "#4CAF50"
	}
	return nil
}

// ScheduleTemplateEntry represents an entry in a schedule template
type ScheduleTemplateEntry struct {
	sharedmodels.BaseModel
	TemplateID uint   `gorm:"not null;index" json:"template_id" validate:"required"`
	UserID     *uint  `gorm:"index" json:"user_id,omitempty"`                              // nil = apply to all assigned users
	DayOfWeek  int    `gorm:"not null" json:"day_of_week" validate:"required,min=0,max=6"` // 0-6 (Sunday-Saturday)
	StartTime  string `gorm:"not null;size:5" json:"start_time" validate:"required,len=5"` // "09:00"
	EndTime    string `gorm:"not null;size:5" json:"end_time" validate:"required,len=5"`   // "18:00"
	Title      string `gorm:"size:255" json:"title,omitempty" validate:"omitempty,max=255"`
	Location   string `gorm:"size:500" json:"location,omitempty" validate:"omitempty,max=500"`

	// Associations
	Template *ScheduleTemplate  `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	User     *sharedmodels.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName returns the table name for ScheduleTemplateEntry model
func (ScheduleTemplateEntry) TableName() string {
	return "schedule_template_entries"
}

// ScheduleAssignment represents a user assignment to a schedule
type ScheduleAssignment struct {
	sharedmodels.BaseModel
	ScheduleID uint      `gorm:"not null;index" json:"schedule_id" validate:"required"`
	UserID     uint      `gorm:"not null;index" json:"user_id" validate:"required"`
	AssignedBy uint      `gorm:"not null;index" json:"assigned_by" validate:"required,min=1"`
	AssignedAt time.Time `gorm:"not null" json:"assigned_at"`

	// Associations
	Schedule *Schedule          `gorm:"foreignKey:ScheduleID" json:"schedule,omitempty"`
	User     *sharedmodels.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Assigner *sharedmodels.User `gorm:"foreignKey:AssignedBy" json:"assigner,omitempty"`
}

// TableName returns the table name for ScheduleAssignment model
func (ScheduleAssignment) TableName() string {
	return "schedule_assignments"
}

// BeforeCreate hook is called before creating a schedule assignment
func (sa *ScheduleAssignment) BeforeCreate(tx *gorm.DB) error {
	if sa.AssignedAt.IsZero() {
		sa.AssignedAt = time.Now()
	}
	return nil
}

// Request/Response Models

// CreateScheduleRequest represents request for creating a schedule
type CreateScheduleRequest struct {
	Title         string             `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description   string             `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type          ScheduleType       `json:"type" binding:"required,oneof=work paid_services on_duty vk trips" validate:"required,oneof=work paid_services on_duty vk trips"`
	Visibility    ScheduleVisibility `json:"visibility" binding:"omitempty,oneof=creator_only management participants" validate:"omitempty,oneof=creator_only management participants"`
	StartDate     time.Time          `json:"start_date" binding:"required" validate:"required"`
	EndDate       time.Time          `json:"end_date" binding:"required" validate:"required"`
	IsForAllUsers bool               `json:"is_for_all_users"`
	DepartmentID  *uint              `json:"department_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	Color         string             `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	Mode          *ScheduleMode      `json:"mode,omitempty" binding:"omitempty,oneof=recurring monthly" validate:"omitempty,oneof=recurring monthly"`
	TemplateID    *uint              `json:"template_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`

	// Default shift times
	MorningStart string `json:"morning_start,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	MorningEnd   string `json:"morning_end,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	EveningStart string `json:"evening_start,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	EveningEnd   string `json:"evening_end,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
}

// UpdateScheduleRequest represents request for updating a schedule
type UpdateScheduleRequest struct {
	Title         *string             `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description   *string             `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type          *ScheduleType       `json:"type,omitempty" binding:"omitempty,oneof=work paid_services on_duty vk trips" validate:"omitempty,oneof=work paid_services on_duty vk trips"`
	Visibility    *ScheduleVisibility `json:"visibility,omitempty" binding:"omitempty,oneof=creator_only management participants" validate:"omitempty,oneof=creator_only management participants"`
	StartDate     *time.Time          `json:"start_date,omitempty"`
	EndDate       *time.Time          `json:"end_date,omitempty"`
	IsForAllUsers *bool               `json:"is_for_all_users,omitempty"`
	DepartmentID  *uint               `json:"department_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	Color         *string             `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	IsActive      *bool               `json:"is_active,omitempty"`
	Mode          *ScheduleMode       `json:"mode,omitempty" binding:"omitempty,oneof=recurring monthly" validate:"omitempty,oneof=recurring monthly"`
	MorningStart  *string             `json:"morning_start,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	MorningEnd    *string             `json:"morning_end,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	EveningStart  *string             `json:"evening_start,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
	EveningEnd    *string             `json:"evening_end,omitempty" binding:"omitempty,len=5" validate:"omitempty,len=5"`
}

// CreateScheduleEntryRequest represents request for creating a schedule entry
type CreateScheduleEntryRequest struct {
	UserID      uint      `json:"user_id" binding:"required,min=1" validate:"required,min=1"`
	Date        time.Time `json:"date" binding:"required" validate:"required"`
	ShiftType   ShiftType `json:"shift_type" binding:"required,oneof=morning evening full_day custom" validate:"required,oneof=morning evening full_day custom"`
	StartTime   *string   `json:"start_time,omitempty"` // Required if ShiftType is custom
	EndTime     *string   `json:"end_time,omitempty"`   // Required if ShiftType is custom
	Title       string    `json:"title,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Description string    `json:"description,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
	Location    string    `json:"location,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// BatchCreateScheduleEntriesRequest represents batch creation request
type BatchCreateScheduleEntriesRequest struct {
	Entries []CreateScheduleEntryRequest `json:"entries" binding:"required,min=1,dive" validate:"required,min=1,dive"`
}

// UpdateScheduleEntryRequest represents request for updating a schedule entry
type UpdateScheduleEntryRequest struct {
	UserID      *uint      `json:"user_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	Date        *time.Time `json:"date,omitempty"`
	ShiftType   *ShiftType `json:"shift_type,omitempty" binding:"omitempty,oneof=morning evening full_day custom" validate:"omitempty,oneof=morning evening full_day custom"`
	StartTime   *string    `json:"start_time,omitempty"`
	EndTime     *string    `json:"end_time,omitempty"`
	Title       *string    `json:"title,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Description *string    `json:"description,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
	Location    *string    `json:"location,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// ScheduleResponse represents a schedule in API responses
type ScheduleResponse struct {
	ID            uint                      `json:"id"`
	Title         string                    `json:"title"`
	Description   string                    `json:"description,omitempty"`
	Type          ScheduleType              `json:"type"`
	Visibility    ScheduleVisibility        `json:"visibility"`
	CreatedBy     uint                      `json:"created_by"`
	Creator       *sharedmodels.User        `json:"creator,omitempty"`
	StartDate     time.Time                 `json:"start_date"`
	EndDate       time.Time                 `json:"end_date"`
	IsForAllUsers bool                      `json:"is_for_all_users"`
	DepartmentID  *uint                     `json:"department_id,omitempty"`
	Color         string                    `json:"color"`
	IsActive      bool                      `json:"is_active"`
	Mode          ScheduleMode              `json:"mode"`
	TemplateID    *uint                     `json:"template_id,omitempty"`
	Template      *ScheduleTemplateResponse `json:"template,omitempty"`
	ImportedFrom  *string                   `json:"imported_from,omitempty"`
	MorningStart  string                    `json:"morning_start"`
	MorningEnd    string                    `json:"morning_end"`
	EveningStart  string                    `json:"evening_start"`
	EveningEnd    string                    `json:"evening_end"`
	EntryCount    int                       `json:"entry_count,omitempty"`
	CreatedAt     time.Time                 `json:"created_at"`
	UpdatedAt     time.Time                 `json:"updated_at"`
}

// ToResponse converts Schedule model to ScheduleResponse
func (s *Schedule) ToResponse() *ScheduleResponse {
	resp := &ScheduleResponse{
		ID:            s.ID,
		Title:         s.Title,
		Description:   s.Description,
		Type:          s.Type,
		Visibility:    s.Visibility,
		CreatedBy:     s.CreatedBy,
		Creator:       s.Creator,
		StartDate:     s.StartDate,
		EndDate:       s.EndDate,
		IsForAllUsers: s.IsForAllUsers,
		DepartmentID:  s.DepartmentID,
		Color:         s.Color,
		IsActive:      s.IsActive,
		Mode:          s.Mode,
		TemplateID:    s.TemplateID,
		ImportedFrom:  s.ImportedFrom,
		MorningStart:  s.MorningStart,
		MorningEnd:    s.MorningEnd,
		EveningStart:  s.EveningStart,
		EveningEnd:    s.EveningEnd,
		EntryCount:    len(s.Entries),
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}

	// Include template for recurring schedules
	if s.Template != nil {
		resp.Template = s.Template.ToResponse()
	}

	return resp
}

// ScheduleEntryResponse represents a schedule entry in API responses
type ScheduleEntryResponse struct {
	ID          uint               `json:"id"`
	ScheduleID  uint               `json:"schedule_id"`
	UserID      uint               `json:"user_id"`
	User        *sharedmodels.User `json:"user,omitempty"`
	Date        time.Time          `json:"date"`
	ShiftType   ShiftType          `json:"shift_type"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Location    string             `json:"location,omitempty"`
	EventID     *uint              `json:"event_id,omitempty"`
	CreatedBy   uint               `json:"created_by"`
	Creator     *sharedmodels.User `json:"creator,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// ToResponse converts ScheduleEntry model to ScheduleEntryResponse
func (se *ScheduleEntry) ToResponse() *ScheduleEntryResponse {
	return &ScheduleEntryResponse{
		ID:          se.ID,
		ScheduleID:  se.ScheduleID,
		UserID:      se.UserID,
		User:        se.User,
		Date:        se.Date,
		ShiftType:   se.ShiftType,
		StartTime:   se.StartTime,
		EndTime:     se.EndTime,
		Title:       se.Title,
		Description: se.Description,
		Location:    se.Location,
		EventID:     se.EventID,
		CreatedBy:   se.CreatedBy,
		Creator:     se.Creator,
		CreatedAt:   se.CreatedAt,
		UpdatedAt:   se.UpdatedAt,
	}
}

// ScheduleListResponse represents a paginated list of schedules
type ScheduleListResponse struct {
	Schedules []*ScheduleResponse `json:"schedules"`
	Total     int64               `json:"total"`
	Limit     int                 `json:"limit"`
	Offset    int                 `json:"offset"`
}

// ScheduleEntryListResponse represents a paginated list of schedule entries
type ScheduleEntryListResponse struct {
	Entries []*ScheduleEntryResponse `json:"entries"`
	Total   int64                    `json:"total"`
	Limit   int                      `json:"limit"`
	Offset  int                      `json:"offset"`
}

// Template Request/Response Models

// CreateScheduleTemplateRequest represents request for creating a template
type CreateScheduleTemplateRequest struct {
	Title        string       `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description  string       `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type         ScheduleType `json:"type" binding:"required,oneof=work paid_services on_duty vk trips" validate:"required,oneof=work paid_services on_duty vk trips"`
	DepartmentID *uint        `json:"department_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	Color        string       `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
}

// UpdateScheduleTemplateRequest represents request for updating a template
type UpdateScheduleTemplateRequest struct {
	Title        *string       `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description  *string       `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type         *ScheduleType `json:"type,omitempty" binding:"omitempty,oneof=work paid_services on_duty vk trips" validate:"omitempty,oneof=work paid_services on_duty vk trips"`
	DepartmentID *uint         `json:"department_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	Color        *string       `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	IsActive     *bool         `json:"is_active,omitempty"`
}

// CreateTemplateEntryRequest represents request for creating a template entry
type CreateTemplateEntryRequest struct {
	UserID    *uint  `json:"user_id,omitempty" binding:"omitempty,min=1" validate:"omitempty,min=1"`
	DayOfWeek int    `json:"day_of_week" binding:"required,min=0,max=6" validate:"required,min=0,max=6"`
	StartTime string `json:"start_time" binding:"required,len=5" validate:"required,len=5"`
	EndTime   string `json:"end_time" binding:"required,len=5" validate:"required,len=5"`
	Title     string `json:"title,omitempty" binding:"omitempty,max=255" validate:"omitempty,max=255"`
	Location  string `json:"location,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// ApplyTemplateRequest represents request for applying a template to a period
type ApplyTemplateRequest struct {
	StartDate time.Time `json:"start_date" binding:"required" validate:"required"`
	EndDate   time.Time `json:"end_date" binding:"required" validate:"required"`
	UserIDs   []uint    `json:"user_ids,omitempty" binding:"omitempty,dive,min=1" validate:"omitempty,dive,min=1"`
}

// ScheduleTemplateResponse represents a template in API responses
type ScheduleTemplateResponse struct {
	ID           uint                            `json:"id"`
	Title        string                          `json:"title"`
	Description  string                          `json:"description,omitempty"`
	Type         ScheduleType                    `json:"type"`
	CreatedBy    uint                            `json:"created_by"`
	Creator      *sharedmodels.User              `json:"creator,omitempty"`
	DepartmentID *uint                           `json:"department_id,omitempty"`
	Color        string                          `json:"color"`
	IsActive     bool                            `json:"is_active"`
	Entries      []*ScheduleTemplateEntryResponse `json:"entries,omitempty"`
	CreatedAt    time.Time                       `json:"created_at"`
	UpdatedAt    time.Time                       `json:"updated_at"`
}

// ToResponse converts ScheduleTemplate model to ScheduleTemplateResponse
func (st *ScheduleTemplate) ToResponse() *ScheduleTemplateResponse {
	resp := &ScheduleTemplateResponse{
		ID:           st.ID,
		Title:        st.Title,
		Description:  st.Description,
		Type:         st.Type,
		CreatedBy:    st.CreatedBy,
		Creator:      st.Creator,
		DepartmentID: st.DepartmentID,
		Color:        st.Color,
		IsActive:     st.IsActive,
		CreatedAt:    st.CreatedAt,
		UpdatedAt:    st.UpdatedAt,
	}

	// Include entries if loaded
	if len(st.Entries) > 0 {
		resp.Entries = make([]*ScheduleTemplateEntryResponse, len(st.Entries))
		for i, entry := range st.Entries {
			resp.Entries[i] = entry.ToResponse()
		}
	}

	return resp
}

// ScheduleTemplateEntryResponse represents a template entry in API responses
type ScheduleTemplateEntryResponse struct {
	ID         uint               `json:"id"`
	TemplateID uint               `json:"template_id"`
	UserID     *uint              `json:"user_id,omitempty"`
	User       *sharedmodels.User `json:"user,omitempty"`
	DayOfWeek  int                `json:"day_of_week"`
	StartTime  string             `json:"start_time"`
	EndTime    string             `json:"end_time"`
	Title      string             `json:"title,omitempty"`
	Location   string             `json:"location,omitempty"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

// ToResponse converts ScheduleTemplateEntry model to ScheduleTemplateEntryResponse
func (ste *ScheduleTemplateEntry) ToResponse() *ScheduleTemplateEntryResponse {
	return &ScheduleTemplateEntryResponse{
		ID:         ste.ID,
		TemplateID: ste.TemplateID,
		UserID:     ste.UserID,
		User:       ste.User,
		DayOfWeek:  ste.DayOfWeek,
		StartTime:  ste.StartTime,
		EndTime:    ste.EndTime,
		Title:      ste.Title,
		Location:   ste.Location,
		CreatedAt:  ste.CreatedAt,
		UpdatedAt:  ste.UpdatedAt,
	}
}

// ScheduleTemplateListResponse represents a paginated list of templates
type ScheduleTemplateListResponse struct {
	Templates []*ScheduleTemplateResponse `json:"templates"`
	Total     int64                       `json:"total"`
	Limit     int                         `json:"limit"`
	Offset    int                         `json:"offset"`
}

// Import Models

// ImportScheduleRequest represents request for importing schedule from file
type ImportScheduleRequest struct {
	FileID      string       `json:"file_id" binding:"required" validate:"required"`
	Title       string       `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description string       `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type        ScheduleType `json:"type" binding:"required" validate:"required"`
	StartDate   time.Time    `json:"start_date" binding:"required" validate:"required"`
	EndDate     time.Time    `json:"end_date" binding:"required" validate:"required"`
	Color       string       `json:"color,omitempty"` // Schedule color (hex format)
	Preview     bool         `json:"preview"`         // If true, returns preview without creating
}

// ImportPreviewResponse represents preview of imported schedule
type ImportPreviewResponse struct {
	Schedule     *ScheduleResponse        `json:"schedule"`
	Entries      []*ScheduleEntryResponse `json:"entries"`
	EntriesCount int                      `json:"entries_count"`
	Users        []*ImportedUser          `json:"users"`
	Warnings     []string                 `json:"warnings,omitempty"`
}

// ImportedUser represents a user detected during import
type ImportedUser struct {
	Name        string  `json:"name"`                  // Name from document
	UserID      *uint   `json:"user_id,omitempty"`     // Matched user ID
	MatchScore  float64 `json:"match_score,omitempty"` // Fuzzy match score
	IsUnmatched bool    `json:"is_unmatched"`          // No match found
}

// ImportScheduleResponse represents successful import result
type ImportScheduleResponse struct {
	Schedule     *ScheduleResponse `json:"schedule"`
	EntriesCount int               `json:"entries_count"`
	ImportedFrom string            `json:"imported_from"`
	Warnings     []string          `json:"warnings,omitempty"`
}
