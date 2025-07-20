// File: services/poll/models/poll.go
package models

import (
	"time"

	"tachyon-messenger/shared/models"

	"gorm.io/gorm"
)

// PollType represents the type of poll
type PollType string

const (
	PollTypeSingleChoice   PollType = "single_choice"   // Один вариант ответа
	PollTypeMultipleChoice PollType = "multiple_choice" // Несколько вариантов ответа
	PollTypeRanking        PollType = "ranking"         // Ранжирование вариантов
	PollTypeRating         PollType = "rating"          // Оценка по шкале
	PollTypeOpenText       PollType = "open_text"       // Свободный текст
)

// PollStatus represents the status of a poll
type PollStatus string

const (
	PollStatusDraft     PollStatus = "draft"     // Черновик
	PollStatusActive    PollStatus = "active"    // Активный
	PollStatusClosed    PollStatus = "closed"    // Закрыт
	PollStatusArchived  PollStatus = "archived"  // Архивирован
	PollStatusCancelled PollStatus = "cancelled" // Отменен
)

// PollVisibility represents who can see and participate in the poll
type PollVisibility string

const (
	PollVisibilityPublic     PollVisibility = "public"      // Все пользователи
	PollVisibilityDepartment PollVisibility = "department"  // Только департамент
	PollVisibilityInviteOnly PollVisibility = "invite_only" // Только приглашенные
	PollVisibilityPrivate    PollVisibility = "private"     // Только создатель
)

// Poll represents a poll/survey in the system
type Poll struct {
	models.BaseModel
	Title       string         `gorm:"not null;size:255" json:"title" validate:"required,min=1,max=255"`
	Description string         `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=2000"`
	Type        PollType       `gorm:"not null;size:20" json:"type" validate:"required,oneof=single_choice multiple_choice ranking rating open_text"`
	Status      PollStatus     `gorm:"not null;default:'draft';size:20" json:"status" validate:"required,oneof=draft active closed archived cancelled"`
	Visibility  PollVisibility `gorm:"not null;default:'public';size:20" json:"visibility" validate:"required,oneof=public department invite_only private"`
	CreatedBy   uint           `gorm:"not null;index" json:"created_by" validate:"required,min=1"`

	// Timing settings
	StartTime *time.Time `gorm:"index" json:"start_time,omitempty"`
	EndTime   *time.Time `gorm:"index" json:"end_time,omitempty"`

	// Poll settings
	AllowAnonymous    bool `gorm:"not null;default:false" json:"allow_anonymous"`
	AllowMultipleVote bool `gorm:"not null;default:false" json:"allow_multiple_vote"`
	RequireComment    bool `gorm:"not null;default:false" json:"require_comment"`
	ShowResults       bool `gorm:"not null;default:true" json:"show_results"`
	ShowResultsAfter  bool `gorm:"not null;default:false" json:"show_results_after"` // Показывать результаты только после голосования

	// Department restriction (if visibility is 'department')
	DepartmentID *uint `gorm:"index" json:"department_id,omitempty" validate:"omitempty,min=1"`

	// Category for organization
	Category string `gorm:"size:100" json:"category,omitempty" validate:"omitempty,max=100"`

	// Associations
	Options      []PollOption      `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"options,omitempty"`
	Votes        []PollVote        `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"votes,omitempty"`
	Participants []PollParticipant `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"participants,omitempty"`
	Comments     []PollComment     `gorm:"foreignKey:PollID;constraint:OnDelete:CASCADE" json:"comments,omitempty"`

	// Computed fields (not stored in DB)
	TotalVotes      int     `gorm:"-" json:"total_votes,omitempty"`
	TotalVoters     int     `gorm:"-" json:"total_voters,omitempty"`
	UserHasVoted    bool    `gorm:"-" json:"user_has_voted,omitempty"`
	ParticipantRate float64 `gorm:"-" json:"participant_rate,omitempty"` // Процент участия
}

// TableName returns the table name for Poll model
func (Poll) TableName() string {
	return "polls"
}

// BeforeCreate hook is called before creating a poll
func (p *Poll) BeforeCreate(tx *gorm.DB) error {
	// Set default values if not provided
	if p.Status == "" {
		p.Status = PollStatusDraft
	}
	if p.Visibility == "" {
		p.Visibility = PollVisibilityPublic
	}
	if p.Type == "" {
		p.Type = PollTypeSingleChoice
	}

	// Validate time logic
	if p.StartTime != nil && p.EndTime != nil && p.EndTime.Before(*p.StartTime) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// BeforeUpdate hook is called before updating a poll
func (p *Poll) BeforeUpdate(tx *gorm.DB) error {
	// Validate time logic
	if p.StartTime != nil && p.EndTime != nil && p.EndTime.Before(*p.StartTime) {
		return gorm.ErrInvalidValue
	}

	return nil
}

// IsActive checks if poll is currently active
func (p *Poll) IsActive() bool {
	if p.Status != PollStatusActive {
		return false
	}

	now := time.Now()

	// Check start time
	if p.StartTime != nil && now.Before(*p.StartTime) {
		return false
	}

	// Check end time
	if p.EndTime != nil && now.After(*p.EndTime) {
		return false
	}

	return true
}

// PollOption represents an option in a poll
type PollOption struct {
	models.BaseModel
	PollID      uint   `gorm:"not null;index" json:"poll_id" validate:"required"`
	Text        string `gorm:"not null;size:500" json:"text" validate:"required,min=1,max=500"`
	Description string `gorm:"type:text" json:"description,omitempty" validate:"omitempty,max=1000"`
	Position    int    `gorm:"not null;default:0" json:"position"` // Порядок отображения
	Color       string `gorm:"size:7" json:"color,omitempty" validate:"omitempty,len=7"`
	ImageURL    string `gorm:"size:500" json:"image_url,omitempty" validate:"omitempty,url,max=500"`

	// Associations
	Poll  *Poll      `gorm:"foreignKey:PollID" json:"poll,omitempty"`
	Votes []PollVote `gorm:"foreignKey:OptionID" json:"votes,omitempty"`

	// Computed fields (not stored in DB)
	VoteCount   int     `gorm:"-" json:"vote_count,omitempty"`
	VotePercent float64 `gorm:"-" json:"vote_percent,omitempty"`
	RatingAvg   float64 `gorm:"-" json:"rating_avg,omitempty"`  // Средняя оценка для rating polls
	RankingAvg  float64 `gorm:"-" json:"ranking_avg,omitempty"` // Средний ранг для ranking polls
}

// TableName returns the table name for PollOption model
func (PollOption) TableName() string {
	return "poll_options"
}

// PollVote represents a vote on a poll
type PollVote struct {
	models.BaseModel
	PollID      uint  `gorm:"not null;index" json:"poll_id" validate:"required"`
	OptionID    *uint `gorm:"index" json:"option_id,omitempty"` // Null для open_text polls
	UserID      *uint `gorm:"index" json:"user_id,omitempty"`   // Null для анонимных голосов
	IsAnonymous bool  `gorm:"not null;default:false" json:"is_anonymous"`

	// Different vote types
	TextValue    string `gorm:"type:text" json:"text_value,omitempty"` // Для open_text polls
	RatingValue  *int   `gorm:"" json:"rating_value,omitempty"`        // Для rating polls (1-5, 1-10, etc.)
	RankingValue *int   `gorm:"" json:"ranking_value,omitempty"`       // Для ranking polls (позиция в рейтинге)
	Comment      string `gorm:"type:text" json:"comment,omitempty"`    // Дополнительный комментарий

	// Associations
	Poll   *Poll       `gorm:"foreignKey:PollID" json:"poll,omitempty"`
	Option *PollOption `gorm:"foreignKey:OptionID" json:"option,omitempty"`
}

// TableName returns the table name for PollVote model
func (PollVote) TableName() string {
	return "poll_votes"
}

// BeforeCreate hook is called before creating a poll vote
func (pv *PollVote) BeforeCreate(tx *gorm.DB) error {
	// Validate that anonymous vote doesn't have user_id
	if pv.IsAnonymous && pv.UserID != nil {
		return gorm.ErrInvalidValue
	}

	// Validate that non-anonymous vote has user_id
	if !pv.IsAnonymous && pv.UserID == nil {
		return gorm.ErrInvalidValue
	}

	return nil
}

// PollParticipant represents invited participants for invite-only polls
type PollParticipant struct {
	models.BaseModel
	PollID     uint       `gorm:"not null;index" json:"poll_id" validate:"required"`
	UserID     uint       `gorm:"not null;index" json:"user_id" validate:"required"`
	InvitedBy  uint       `gorm:"not null;index" json:"invited_by" validate:"required"`
	InvitedAt  time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"invited_at"`
	VotedAt    *time.Time `json:"voted_at,omitempty"`
	NotifiedAt *time.Time `json:"notified_at,omitempty"`

	// Associations
	Poll *Poll `gorm:"foreignKey:PollID" json:"poll,omitempty"`
}

// TableName returns the table name for PollParticipant model
func (PollParticipant) TableName() string {
	return "poll_participants"
}

// PollComment represents a comment on a poll
type PollComment struct {
	models.BaseModel
	PollID   uint   `gorm:"not null;index" json:"poll_id" validate:"required"`
	UserID   uint   `gorm:"not null;index" json:"user_id" validate:"required"`
	Content  string `gorm:"not null;type:text" json:"content" validate:"required,min=1,max=1000"`
	ParentID *uint  `gorm:"index" json:"parent_id,omitempty" validate:"omitempty,min=1"`

	// Associations
	Poll    *Poll         `gorm:"foreignKey:PollID" json:"poll,omitempty"`
	Parent  *PollComment  `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Replies []PollComment `gorm:"foreignKey:ParentID;constraint:OnDelete:CASCADE" json:"replies,omitempty"`
}

// TableName returns the table name for PollComment model
func (PollComment) TableName() string {
	return "poll_comments"
}

// BeforeCreate hook is called before creating a poll comment
func (pc *PollComment) BeforeCreate(tx *gorm.DB) error {
	// Validate that parent comment belongs to the same poll if ParentID is set
	if pc.ParentID != nil {
		var parentComment PollComment
		if err := tx.Where("id = ? AND poll_id = ?", *pc.ParentID, pc.PollID).First(&parentComment).Error; err != nil {
			return gorm.ErrInvalidValue
		}
	}
	return nil
}
