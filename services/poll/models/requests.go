// File: services/poll/models/requests.go
package models

import (
	"time"
)

// CreatePollRequest represents request for creating a poll
type CreatePollRequest struct {
	Title       string         `json:"title" binding:"required,min=1,max=255" validate:"required,min=1,max=255"`
	Description string         `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Type        PollType       `json:"type" binding:"required,oneof=single_choice multiple_choice ranking rating open_text" validate:"required,oneof=single_choice multiple_choice ranking rating open_text"`
	Visibility  PollVisibility `json:"visibility" binding:"omitempty,oneof=public department invite_only private" validate:"omitempty,oneof=public department invite_only private"`
	Category    string         `json:"category,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`

	// Timing settings
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// Poll settings
	AllowAnonymous    bool `json:"allow_anonymous"`
	AllowMultipleVote bool `json:"allow_multiple_vote"`
	RequireComment    bool `json:"require_comment"`
	ShowResults       bool `json:"show_results"`
	ShowResultsAfter  bool `json:"show_results_after"`

	// Department restriction (if visibility is 'department')
	DepartmentID *uint `json:"department_id,omitempty" validate:"omitempty,min=1"`

	// Poll options
	Options []CreatePollOptionRequest `json:"options" binding:"required,min=1,max=20" validate:"required,min=1,max=20,dive"`

	// Participants (for invite-only polls)
	ParticipantIDs []uint `json:"participant_ids,omitempty" validate:"omitempty,dive,min=1"`
}

// CreatePollOptionRequest represents request for creating a poll option
type CreatePollOptionRequest struct {
	Text        string `json:"text" binding:"required,min=1,max=500" validate:"required,min=1,max=500"`
	Description string `json:"description,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
	Position    int    `json:"position"`
	Color       string `json:"color,omitempty" binding:"omitempty,len=7" validate:"omitempty,len=7"`
	ImageURL    string `json:"image_url,omitempty" binding:"omitempty,url,max=500" validate:"omitempty,url,max=500"`
}

// UpdatePollRequest represents request for updating a poll
type UpdatePollRequest struct {
	Title             *string         `json:"title,omitempty" binding:"omitempty,min=1,max=255" validate:"omitempty,min=1,max=255"`
	Description       *string         `json:"description,omitempty" binding:"omitempty,max=2000" validate:"omitempty,max=2000"`
	Status            *PollStatus     `json:"status,omitempty" binding:"omitempty,oneof=draft active closed archived cancelled" validate:"omitempty,oneof=draft active closed archived cancelled"`
	Visibility        *PollVisibility `json:"visibility,omitempty" binding:"omitempty,oneof=public department invite_only private" validate:"omitempty,oneof=public department invite_only private"`
	Category          *string         `json:"category,omitempty" binding:"omitempty,max=100" validate:"omitempty,max=100"`
	StartTime         *time.Time      `json:"start_time,omitempty"`
	EndTime           *time.Time      `json:"end_time,omitempty"`
	AllowAnonymous    *bool           `json:"allow_anonymous,omitempty"`
	AllowMultipleVote *bool           `json:"allow_multiple_vote,omitempty"`
	RequireComment    *bool           `json:"require_comment,omitempty"`
	ShowResults       *bool           `json:"show_results,omitempty"`
	ShowResultsAfter  *bool           `json:"show_results_after,omitempty"`
	DepartmentID      *uint           `json:"department_id,omitempty" validate:"omitempty,min=1"`
}

// VotePollRequest represents request for voting on a poll
type VotePollRequest struct {
	OptionIDs     []uint       `json:"option_ids,omitempty" validate:"omitempty,dive,min=1"` // Для single/multiple choice
	TextValue     string       `json:"text_value,omitempty" validate:"omitempty,max=2000"`   // Для open_text
	RatingValues  map[uint]int `json:"rating_values,omitempty"`                              // Для rating: option_id -> rating
	RankingValues map[uint]int `json:"ranking_values,omitempty"`                             // Для ranking: option_id -> rank
	Comment       string       `json:"comment,omitempty" binding:"omitempty,max=1000" validate:"omitempty,max=1000"`
	IsAnonymous   bool         `json:"is_anonymous"`
}

// AddParticipantsRequest represents request for adding participants to a poll
type AddParticipantsRequest struct {
	UserIDs []uint `json:"user_ids" binding:"required,min=1" validate:"required,min=1,dive,min=1"`
	Message string `json:"message,omitempty" binding:"omitempty,max=500" validate:"omitempty,max=500"`
}

// CreateCommentRequest represents request for creating a comment
type CreateCommentRequest struct {
	Content  string `json:"content" binding:"required,min=1,max=1000" validate:"required,min=1,max=1000"`
	ParentID *uint  `json:"parent_id,omitempty" validate:"omitempty,min=1"`
}

// PollFilterRequest represents request for filtering polls
type PollFilterRequest struct {
	Status       PollStatus     `json:"status,omitempty" validate:"omitempty,oneof=draft active closed archived cancelled"`
	Type         PollType       `json:"type,omitempty" validate:"omitempty,oneof=single_choice multiple_choice ranking rating open_text"`
	Visibility   PollVisibility `json:"visibility,omitempty" validate:"omitempty,oneof=public department invite_only private"`
	Category     string         `json:"category,omitempty" validate:"omitempty,max=100"`
	CreatedBy    *uint          `json:"created_by,omitempty" validate:"omitempty,min=1"`
	DepartmentID *uint          `json:"department_id,omitempty" validate:"omitempty,min=1"`

	// Date filters
	StartDateFrom *time.Time `json:"start_date_from,omitempty"`
	StartDateTo   *time.Time `json:"start_date_to,omitempty"`
	EndDateFrom   *time.Time `json:"end_date_from,omitempty"`
	EndDateTo     *time.Time `json:"end_date_to,omitempty"`

	// Participation filters
	UserHasVoted  *bool `json:"user_has_voted,omitempty"`
	UserIsInvited *bool `json:"user_is_invited,omitempty"`

	// Pagination
	Limit  int `json:"limit" validate:"omitempty,min=1,max=100"`
	Offset int `json:"offset" validate:"omitempty,min=0"`

	// Sorting
	SortBy    string `json:"sort_by,omitempty" validate:"omitempty,oneof=created_at updated_at title start_time end_time total_votes"`
	SortOrder string `json:"sort_order,omitempty" validate:"omitempty,oneof=asc desc"`
}

// File: services/poll/models/responses.go

// PollResponse represents a poll in API responses
type PollResponse struct {
	ID          uint           `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Type        PollType       `json:"type"`
	Status      PollStatus     `json:"status"`
	Visibility  PollVisibility `json:"visibility"`
	Category    string         `json:"category,omitempty"`
	CreatedBy   uint           `json:"created_by"`

	// Timing
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`

	// Settings
	AllowAnonymous    bool `json:"allow_anonymous"`
	AllowMultipleVote bool `json:"allow_multiple_vote"`
	RequireComment    bool `json:"require_comment"`
	ShowResults       bool `json:"show_results"`
	ShowResultsAfter  bool `json:"show_results_after"`

	// Department
	DepartmentID *uint `json:"department_id,omitempty"`

	// Statistics
	TotalVotes      int     `json:"total_votes"`
	TotalVoters     int     `json:"total_voters"`
	UserHasVoted    bool    `json:"user_has_voted"`
	ParticipantRate float64 `json:"participant_rate"`

	// Associations
	Options      []*PollOptionResponse      `json:"options,omitempty"`
	Participants []*PollParticipantResponse `json:"participants,omitempty"`
	Comments     []*PollCommentResponse     `json:"comments,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PollOptionResponse represents a poll option in API responses
type PollOptionResponse struct {
	ID          uint      `json:"id"`
	PollID      uint      `json:"poll_id"`
	Text        string    `json:"text"`
	Description string    `json:"description,omitempty"`
	Position    int       `json:"position"`
	Color       string    `json:"color,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	VoteCount   int       `json:"vote_count"`
	VotePercent float64   `json:"vote_percent"`
	RatingAvg   float64   `json:"rating_avg,omitempty"`
	RankingAvg  float64   `json:"ranking_avg,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PollVoteResponse represents a vote in API responses
type PollVoteResponse struct {
	ID           uint      `json:"id"`
	PollID       uint      `json:"poll_id"`
	OptionID     *uint     `json:"option_id,omitempty"`
	UserID       *uint     `json:"user_id,omitempty"`
	IsAnonymous  bool      `json:"is_anonymous"`
	TextValue    string    `json:"text_value,omitempty"`
	RatingValue  *int      `json:"rating_value,omitempty"`
	RankingValue *int      `json:"ranking_value,omitempty"`
	Comment      string    `json:"comment,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PollParticipantResponse represents a participant in API responses
type PollParticipantResponse struct {
	ID         uint       `json:"id"`
	PollID     uint       `json:"poll_id"`
	UserID     uint       `json:"user_id"`
	InvitedBy  uint       `json:"invited_by"`
	InvitedAt  time.Time  `json:"invited_at"`
	VotedAt    *time.Time `json:"voted_at,omitempty"`
	NotifiedAt *time.Time `json:"notified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// PollCommentResponse represents a comment in API responses
type PollCommentResponse struct {
	ID        uint                   `json:"id"`
	PollID    uint                   `json:"poll_id"`
	UserID    uint                   `json:"user_id"`
	Content   string                 `json:"content"`
	ParentID  *uint                  `json:"parent_id,omitempty"`
	Replies   []*PollCommentResponse `json:"replies,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// PollListResponse represents a list of polls with pagination
type PollListResponse struct {
	Polls   []*PollResponse    `json:"polls"`
	Total   int64              `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Filters *PollFilterRequest `json:"filters,omitempty"`
}

// PollStatsResponse represents poll statistics
type PollStatsResponse struct {
	TotalPolls      int                     `json:"total_polls"`
	ActivePolls     int                     `json:"active_polls"`
	DraftPolls      int                     `json:"draft_polls"`
	ClosedPolls     int                     `json:"closed_polls"`
	MyPolls         int                     `json:"my_polls"`
	ParticipatedIn  int                     `json:"participated_in"`
	PollsByType     map[PollType]int        `json:"polls_by_type"`
	PollsByCategory map[string]int          `json:"polls_by_category"`
	RecentActivity  []*PollActivityResponse `json:"recent_activity"`
}

// PollActivityResponse represents recent poll activity
type PollActivityResponse struct {
	PollID       uint      `json:"poll_id"`
	PollTitle    string    `json:"poll_title"`
	ActivityType string    `json:"activity_type"` // "created", "voted", "commented", "closed"
	UserID       uint      `json:"user_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// PollResultsResponse represents detailed poll results
type PollResultsResponse struct {
	Poll          *PollResponse                `json:"poll"`
	Options       []*PollOptionResponse        `json:"options"`
	TotalVotes    int                          `json:"total_votes"`
	TotalVoters   int                          `json:"total_voters"`
	VotesByOption map[uint]int                 `json:"votes_by_option"`
	VotesByUser   map[uint][]*PollVoteResponse `json:"votes_by_user,omitempty"`  // Только для не анонимных
	TextResponses []string                     `json:"text_responses,omitempty"` // Для open_text polls
	RatingStats   map[uint]*RatingStats        `json:"rating_stats,omitempty"`   // Статистика по рейтингам
	RankingStats  map[uint]*RankingStats       `json:"ranking_stats,omitempty"`  // Статистика по рейтингам
}

// RatingStats represents rating statistics for an option
type RatingStats struct {
	OptionID     uint        `json:"option_id"`
	Average      float64     `json:"average"`
	Min          int         `json:"min"`
	Max          int         `json:"max"`
	TotalRatings int         `json:"total_ratings"`
	Distribution map[int]int `json:"distribution"` // rating -> count
}

// RankingStats represents ranking statistics for an option
type RankingStats struct {
	OptionID         uint        `json:"option_id"`
	AverageRank      float64     `json:"average_rank"`
	BestRank         int         `json:"best_rank"`
	WorstRank        int         `json:"worst_rank"`
	TotalRankings    int         `json:"total_rankings"`
	RankDistribution map[int]int `json:"rank_distribution"` // rank -> count
}

// ToResponse converts Poll model to PollResponse
func (p *Poll) ToResponse() *PollResponse {
	response := &PollResponse{
		ID:                p.ID,
		Title:             p.Title,
		Description:       p.Description,
		Type:              p.Type,
		Status:            p.Status,
		Visibility:        p.Visibility,
		Category:          p.Category,
		CreatedBy:         p.CreatedBy,
		StartTime:         p.StartTime,
		EndTime:           p.EndTime,
		AllowAnonymous:    p.AllowAnonymous,
		AllowMultipleVote: p.AllowMultipleVote,
		RequireComment:    p.RequireComment,
		ShowResults:       p.ShowResults,
		ShowResultsAfter:  p.ShowResultsAfter,
		DepartmentID:      p.DepartmentID,
		TotalVotes:        p.TotalVotes,
		TotalVoters:       p.TotalVoters,
		UserHasVoted:      p.UserHasVoted,
		ParticipantRate:   p.ParticipantRate,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}

	// Convert options if they exist
	if len(p.Options) > 0 {
		response.Options = make([]*PollOptionResponse, len(p.Options))
		for i, option := range p.Options {
			response.Options[i] = option.ToResponse()
		}
	}

	// Convert participants if they exist
	if len(p.Participants) > 0 {
		response.Participants = make([]*PollParticipantResponse, len(p.Participants))
		for i, participant := range p.Participants {
			response.Participants[i] = participant.ToResponse()
		}
	}

	// Convert comments if they exist
	if len(p.Comments) > 0 {
		response.Comments = make([]*PollCommentResponse, len(p.Comments))
		for i, comment := range p.Comments {
			response.Comments[i] = comment.ToResponse()
		}
	}

	return response
}

// ToResponse converts PollOption model to PollOptionResponse
func (po *PollOption) ToResponse() *PollOptionResponse {
	return &PollOptionResponse{
		ID:          po.ID,
		PollID:      po.PollID,
		Text:        po.Text,
		Description: po.Description,
		Position:    po.Position,
		Color:       po.Color,
		ImageURL:    po.ImageURL,
		VoteCount:   po.VoteCount,
		VotePercent: po.VotePercent,
		RatingAvg:   po.RatingAvg,
		RankingAvg:  po.RankingAvg,
		CreatedAt:   po.CreatedAt,
		UpdatedAt:   po.UpdatedAt,
	}
}

// ToResponse converts PollVote model to PollVoteResponse
func (pv *PollVote) ToResponse() *PollVoteResponse {
	return &PollVoteResponse{
		ID:           pv.ID,
		PollID:       pv.PollID,
		OptionID:     pv.OptionID,
		UserID:       pv.UserID,
		IsAnonymous:  pv.IsAnonymous,
		TextValue:    pv.TextValue,
		RatingValue:  pv.RatingValue,
		RankingValue: pv.RankingValue,
		Comment:      pv.Comment,
		CreatedAt:    pv.CreatedAt,
		UpdatedAt:    pv.UpdatedAt,
	}
}

// ToResponse converts PollParticipant model to PollParticipantResponse
func (pp *PollParticipant) ToResponse() *PollParticipantResponse {
	return &PollParticipantResponse{
		ID:         pp.ID,
		PollID:     pp.PollID,
		UserID:     pp.UserID,
		InvitedBy:  pp.InvitedBy,
		InvitedAt:  pp.InvitedAt,
		VotedAt:    pp.VotedAt,
		NotifiedAt: pp.NotifiedAt,
		CreatedAt:  pp.CreatedAt,
		UpdatedAt:  pp.UpdatedAt,
	}
}

// ToResponse converts PollComment model to PollCommentResponse
func (pc *PollComment) ToResponse() *PollCommentResponse {
	response := &PollCommentResponse{
		ID:        pc.ID,
		PollID:    pc.PollID,
		UserID:    pc.UserID,
		Content:   pc.Content,
		ParentID:  pc.ParentID,
		CreatedAt: pc.CreatedAt,
		UpdatedAt: pc.UpdatedAt,
	}

	// Convert replies if they exist
	if len(pc.Replies) > 0 {
		response.Replies = make([]*PollCommentResponse, len(pc.Replies))
		for i, reply := range pc.Replies {
			response.Replies[i] = reply.ToResponse()
		}
	}

	return response
}
