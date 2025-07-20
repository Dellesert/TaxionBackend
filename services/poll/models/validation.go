// File: services/poll/models/validation.go
package models

import (
	"errors"
	"time"
)

// Validation constants
const (
	MaxPollOptions       = 20
	MaxPollTitle         = 255
	MaxPollDescription   = 2000
	MaxOptionText        = 500
	MaxOptionDescription = 1000
	MaxCommentLength     = 1000
	MaxTextResponse      = 2000
	MaxParticipants      = 1000

	MinRatingValue  = 1
	MaxRatingValue  = 10
	MinRankingValue = 1

	// Pagination limits
	DefaultLimit = 20
	MaxLimit     = 100

	// Cache TTL
	PollCacheTTL    = 5 * time.Minute
	ResultsCacheTTL = 1 * time.Minute
)

// Poll validation errors
var (
	ErrPollNotFound           = errors.New("poll not found")
	ErrPollNotActive          = errors.New("poll is not active")
	ErrPollAlreadyVoted       = errors.New("user has already voted on this poll")
	ErrPollNoPermission       = errors.New("no permission to access this poll")
	ErrPollInvalidVote        = errors.New("invalid vote data")
	ErrPollInvalidOptions     = errors.New("invalid poll options")
	ErrPollInvalidTimeRange   = errors.New("invalid time range")
	ErrPollClosed             = errors.New("poll is closed")
	ErrPollDraft              = errors.New("poll is in draft status")
	ErrOptionNotFound         = errors.New("poll option not found")
	ErrInvalidRating          = errors.New("invalid rating value")
	ErrInvalidRanking         = errors.New("invalid ranking value")
	ErrCommentRequired        = errors.New("comment is required for this poll")
	ErrAnonymousNotAllowed    = errors.New("anonymous voting is not allowed")
	ErrMultipleVoteNotAllowed = errors.New("multiple voting is not allowed")
	ErrNotParticipant         = errors.New("user is not a participant of this poll")
	ErrAlreadyParticipant     = errors.New("user is already a participant")
)

// ValidateCreatePollRequest validates poll creation request
func (req *CreatePollRequest) Validate() error {
	// Basic validation
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > MaxPollTitle {
		return errors.New("title is too long")
	}
	if len(req.Description) > MaxPollDescription {
		return errors.New("description is too long")
	}

	// Validate poll type
	if !isValidPollType(req.Type) {
		return errors.New("invalid poll type")
	}

	// Validate visibility
	if req.Visibility != "" && !isValidPollVisibility(req.Visibility) {
		return errors.New("invalid poll visibility")
	}

	// Validate time range
	if req.StartTime != nil && req.EndTime != nil {
		if req.EndTime.Before(*req.StartTime) {
			return ErrPollInvalidTimeRange
		}
	}

	// Validate options
	if len(req.Options) == 0 {
		return errors.New("at least one option is required")
	}
	if len(req.Options) > MaxPollOptions {
		return errors.New("too many options")
	}

	for i, option := range req.Options {
		if err := option.Validate(); err != nil {
			return errors.New("option " + string(rune(i+1)) + ": " + err.Error())
		}
	}

	// Type-specific validations
	switch req.Type {
	case PollTypeOpenText:
		if len(req.Options) > 1 {
			return errors.New("open text polls can have only one option")
		}
	case PollTypeRating, PollTypeRanking:
		if len(req.Options) < 2 {
			return errors.New("rating and ranking polls require at least 2 options")
		}
	}

	// Department validation
	if req.Visibility == PollVisibilityDepartment && req.DepartmentID == nil {
		return errors.New("department_id is required for department visibility")
	}

	return nil
}

// ValidateCreatePollOptionRequest validates poll option creation request
func (req *CreatePollOptionRequest) Validate() error {
	if req.Text == "" {
		return errors.New("option text is required")
	}
	if len(req.Text) > MaxOptionText {
		return errors.New("option text is too long")
	}
	if len(req.Description) > MaxOptionDescription {
		return errors.New("option description is too long")
	}
	if req.Color != "" && len(req.Color) != 7 {
		return errors.New("invalid color format")
	}
	return nil
}

// ValidateVotePollRequest validates poll voting request
func (req *VotePollRequest) Validate(poll *Poll) error {
	switch poll.Type {
	case PollTypeSingleChoice:
		if len(req.OptionIDs) != 1 {
			return errors.New("single choice polls require exactly one option")
		}
	case PollTypeMultipleChoice:
		if len(req.OptionIDs) == 0 {
			return errors.New("multiple choice polls require at least one option")
		}
	case PollTypeOpenText:
		if req.TextValue == "" {
			return errors.New("text value is required for open text polls")
		}
		if len(req.TextValue) > MaxTextResponse {
			return errors.New("text response is too long")
		}
	case PollTypeRating:
		if len(req.RatingValues) == 0 {
			return errors.New("rating values are required for rating polls")
		}
		for _, rating := range req.RatingValues {
			if rating < MinRatingValue || rating > MaxRatingValue {
				return ErrInvalidRating
			}
		}
	case PollTypeRanking:
		if len(req.RankingValues) == 0 {
			return errors.New("ranking values are required for ranking polls")
		}
		// Validate that rankings are unique and within range
		usedRanks := make(map[int]bool)
		for _, rank := range req.RankingValues {
			if rank < MinRankingValue || rank > len(poll.Options) {
				return ErrInvalidRanking
			}
			if usedRanks[rank] {
				return errors.New("duplicate ranking values")
			}
			usedRanks[rank] = true
		}
	}

	// Validate comment requirement
	if poll.RequireComment && req.Comment == "" {
		return ErrCommentRequired
	}
	if len(req.Comment) > MaxCommentLength {
		return errors.New("comment is too long")
	}

	// Validate anonymous voting
	if req.IsAnonymous && !poll.AllowAnonymous {
		return ErrAnonymousNotAllowed
	}

	return nil
}

// ValidatePollFilterRequest validates poll filter request
func (req *PollFilterRequest) Validate() error {
	// Validate status
	if req.Status != "" && !isValidPollStatus(req.Status) {
		return errors.New("invalid poll status")
	}

	// Validate type
	if req.Type != "" && !isValidPollType(req.Type) {
		return errors.New("invalid poll type")
	}

	// Validate visibility
	if req.Visibility != "" && !isValidPollVisibility(req.Visibility) {
		return errors.New("invalid poll visibility")
	}

	// Validate pagination
	if req.Limit < 0 || req.Limit > MaxLimit {
		return errors.New("invalid limit")
	}
	if req.Offset < 0 {
		return errors.New("invalid offset")
	}

	// Validate sorting
	if req.SortBy != "" && !isValidSortField(req.SortBy) {
		return errors.New("invalid sort field")
	}
	if req.SortOrder != "" && req.SortOrder != "asc" && req.SortOrder != "desc" {
		return errors.New("invalid sort order")
	}

	// Validate date ranges
	if req.StartDateFrom != nil && req.StartDateTo != nil {
		if req.StartDateTo.Before(*req.StartDateFrom) {
			return errors.New("invalid start date range")
		}
	}
	if req.EndDateFrom != nil && req.EndDateTo != nil {
		if req.EndDateTo.Before(*req.EndDateFrom) {
			return errors.New("invalid end date range")
		}
	}

	return nil
}

// Helper validation functions
func isValidPollType(pollType PollType) bool {
	validTypes := []PollType{
		PollTypeSingleChoice,
		PollTypeMultipleChoice,
		PollTypeRanking,
		PollTypeRating,
		PollTypeOpenText,
	}
	for _, valid := range validTypes {
		if pollType == valid {
			return true
		}
	}
	return false
}

func isValidPollStatus(status PollStatus) bool {
	validStatuses := []PollStatus{
		PollStatusDraft,
		PollStatusActive,
		PollStatusClosed,
		PollStatusArchived,
		PollStatusCancelled,
	}
	for _, valid := range validStatuses {
		if status == valid {
			return true
		}
	}
	return false
}

func isValidPollVisibility(visibility PollVisibility) bool {
	validVisibilities := []PollVisibility{
		PollVisibilityPublic,
		PollVisibilityDepartment,
		PollVisibilityInviteOnly,
		PollVisibilityPrivate,
	}
	for _, valid := range validVisibilities {
		if visibility == valid {
			return true
		}
	}
	return false
}

func isValidSortField(field string) bool {
	validFields := []string{
		"created_at",
		"updated_at",
		"title",
		"start_time",
		"end_time",
		"total_votes",
	}
	for _, valid := range validFields {
		if field == valid {
			return true
		}
	}
	return false
}

// File: services/poll/models/utils.go

// CalculateParticipantRate calculates participation rate
func CalculateParticipantRate(totalVoters, totalInvited int) float64 {
	if totalInvited == 0 {
		return 0.0
	}
	return float64(totalVoters) / float64(totalInvited) * 100.0
}

// CalculateVotePercent calculates vote percentage for an option
func CalculateVotePercent(optionVotes, totalVotes int) float64 {
	if totalVotes == 0 {
		return 0.0
	}
	return float64(optionVotes) / float64(totalVotes) * 100.0
}

// CalculateRatingAverage calculates average rating
func CalculateRatingAverage(ratings []int) float64 {
	if len(ratings) == 0 {
		return 0.0
	}

	sum := 0
	for _, rating := range ratings {
		sum += rating
	}

	return float64(sum) / float64(len(ratings))
}

// CalculateRankingAverage calculates average ranking position
func CalculateRankingAverage(rankings []int) float64 {
	if len(rankings) == 0 {
		return 0.0
	}

	sum := 0
	for _, ranking := range rankings {
		sum += ranking
	}

	return float64(sum) / float64(len(rankings))
}

// PollPermissions contains permission flags for a poll
type PollPermissions struct {
	CanView        bool `json:"can_view"`
	CanVote        bool `json:"can_vote"`
	CanEdit        bool `json:"can_edit"`
	CanDelete      bool `json:"can_delete"`
	CanAddOptions  bool `json:"can_add_options"`
	CanInvite      bool `json:"can_invite"`
	CanComment     bool `json:"can_comment"`
	CanViewResults bool `json:"can_view_results"`
}

// GetDefaultPollPermissions returns default permissions for different user roles
func GetDefaultPollPermissions(userID, creatorID uint, userRole string, poll *Poll) *PollPermissions {
	permissions := &PollPermissions{
		CanView:        true,
		CanVote:        poll.IsActive(),
		CanEdit:        false,
		CanDelete:      false,
		CanAddOptions:  false,
		CanInvite:      false,
		CanComment:     true,
		CanViewResults: poll.ShowResults,
	}

	// Creator permissions
	if userID == creatorID {
		permissions.CanEdit = true
		permissions.CanDelete = true
		permissions.CanAddOptions = poll.Status == PollStatusDraft
		permissions.CanInvite = poll.Visibility == PollVisibilityInviteOnly
		permissions.CanViewResults = true
	}

	// Admin permissions
	if userRole == "admin" || userRole == "super_admin" {
		permissions.CanEdit = true
		permissions.CanDelete = true
		permissions.CanViewResults = true
	}

	// Show results after voting logic
	if poll.ShowResultsAfter && !poll.ShowResults {
		// Check if user has voted (this would need to be set externally)
		// permissions.CanViewResults = userHasVoted
	}

	return permissions
}
