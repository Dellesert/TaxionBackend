// File: services/poll/usecase/poll_usecase.go
package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/services/poll/repository"

	"gorm.io/gorm"
)

// PollUsecase defines the interface for poll business logic
type PollUsecase interface {
	// Poll CRUD operations
	CreatePoll(userID uint, req *models.CreatePollRequest) (*models.PollResponse, error)
	GetPoll(userID, pollID uint) (*models.PollResponse, error)
	UpdatePoll(userID, pollID uint, req *models.UpdatePollRequest) (*models.PollResponse, error)
	DeletePoll(userID, pollID uint) error
	GetPolls(userID uint, filter *models.PollFilterRequest) (*models.PollListResponse, error)
	SearchPolls(userID uint, query string, filter *models.PollFilterRequest) (*models.PollListResponse, error)

	// Poll status management
	UpdatePollStatus(userID, pollID uint, status models.PollStatus) error

	// Voting operations
	VotePoll(userID, pollID uint, req *models.VotePollRequest) ([]*models.PollVoteResponse, error)
	GetUserVotes(userID, pollID uint) ([]*models.PollVoteResponse, error)
	GetPollResults(userID, pollID uint) (*models.PollResultsResponse, error)

	// Participant management
	AddParticipants(userID, pollID uint, req *models.AddParticipantsRequest) error
	RemoveParticipant(userID, pollID, participantID uint) error

	// Comment operations
	CreateComment(userID, pollID uint, req *models.CreateCommentRequest) (*models.PollCommentResponse, error)
	GetComments(userID, pollID uint, limit, offset int) ([]*models.PollCommentResponse, int64, error)
	DeleteComment(userID, pollID, commentID uint) error

	// Statistics
	GetPollStats(userID uint) (*models.PollStatsResponse, error)
}

// pollUsecase implements PollUsecase interface
type pollUsecase struct {
	pollRepo        repository.PollRepository
	optionRepo      repository.PollOptionRepository
	voteRepo        repository.PollVoteRepository
	participantRepo repository.PollParticipantRepository
	commentRepo     repository.PollCommentRepository
}

// NewPollUsecase creates a new poll usecase
func NewPollUsecase(
	pollRepo repository.PollRepository,
	optionRepo repository.PollOptionRepository,
	voteRepo repository.PollVoteRepository,
	participantRepo repository.PollParticipantRepository,
	commentRepo repository.PollCommentRepository,
) PollUsecase {
	return &pollUsecase{
		pollRepo:        pollRepo,
		optionRepo:      optionRepo,
		voteRepo:        voteRepo,
		participantRepo: participantRepo,
		commentRepo:     commentRepo,
	}
}

// CreatePoll creates a new poll with options and participants
func (u *pollUsecase) CreatePoll(userID uint, req *models.CreatePollRequest) (*models.PollResponse, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create poll model
	poll := &models.Poll{
		Title:             strings.TrimSpace(req.Title),
		Description:       strings.TrimSpace(req.Description),
		Type:              req.Type,
		CreatedBy:         userID,
		StartTime:         req.StartTime,
		EndTime:           req.EndTime,
		AllowAnonymous:    req.AllowAnonymous,
		AllowMultipleVote: req.AllowMultipleVote,
		RequireComment:    req.RequireComment,
		ShowResults:       req.ShowResults,
		ShowResultsAfter:  req.ShowResultsAfter,
		DepartmentID:      req.DepartmentID,
		Category:          strings.TrimSpace(req.Category),
	}

	// Set visibility (default to public if not provided)
	if req.Visibility != "" {
		poll.Visibility = req.Visibility
	} else {
		poll.Visibility = models.PollVisibilityPublic
	}

	// Set status to draft initially
	poll.Status = models.PollStatusDraft

	// Save poll
	if err := u.pollRepo.Create(poll); err != nil {
		return nil, fmt.Errorf("failed to create poll: %w", err)
	}

	// Create options
	options := make([]*models.PollOption, len(req.Options))
	for i, optionReq := range req.Options {
		option := &models.PollOption{
			PollID:      poll.ID,
			Text:        strings.TrimSpace(optionReq.Text),
			Description: strings.TrimSpace(optionReq.Description),
			Position:    optionReq.Position,
			Color:       optionReq.Color,
			ImageURL:    optionReq.ImageURL,
		}
		options[i] = option
	}

	if err := u.optionRepo.CreateMultiple(options); err != nil {
		return nil, fmt.Errorf("failed to create poll options: %w", err)
	}

	// Add participants for invite-only polls
	if poll.Visibility == models.PollVisibilityInviteOnly && len(req.ParticipantIDs) > 0 {
		participants := make([]*models.PollParticipant, len(req.ParticipantIDs))
		for i, participantID := range req.ParticipantIDs {
			participant := &models.PollParticipant{
				PollID:    poll.ID,
				UserID:    participantID,
				InvitedBy: userID,
				InvitedAt: time.Now(),
			}
			participants[i] = participant
		}

		if err := u.participantRepo.CreateMultiple(participants); err != nil {
			// Log error but don't fail the entire operation
			// In production, you might want to handle this differently
		}
	}

	// Get the created poll with all details
	createdPoll, err := u.pollRepo.GetByIDWithAll(poll.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created poll: %w", err)
	}

	return createdPoll.ToResponse(), nil
}

// GetPoll retrieves a poll by ID with access control
func (u *pollUsecase) GetPoll(userID, pollID uint) (*models.PollResponse, error) {
	poll, err := u.pollRepo.GetByIDWithAll(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Check access rights
	if !u.hasPollAccess(userID, poll) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Load computed statistics
	u.loadPollStatistics(poll, userID)

	return poll.ToResponse(), nil
}

// UpdatePoll updates an existing poll
func (u *pollUsecase) UpdatePoll(userID, pollID uint, req *models.UpdatePollRequest) (*models.PollResponse, error) {
	// Get existing poll
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Check permissions: only creator can update
	if poll.CreatedBy != userID {
		return nil, fmt.Errorf("access denied: only poll creator can update the poll")
	}

	// Validate that poll can be updated (not closed/archived)
	if poll.Status == models.PollStatusClosed || poll.Status == models.PollStatusArchived {
		return nil, fmt.Errorf("cannot update closed or archived poll")
	}

	// Update fields if provided
	if req.Title != nil {
		poll.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		poll.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		// Validate status transition
		if err := u.validateStatusTransition(poll.Status, *req.Status); err != nil {
			return nil, fmt.Errorf("invalid status transition: %w", err)
		}
		poll.Status = *req.Status
	}
	if req.Visibility != nil {
		poll.Visibility = *req.Visibility
	}
	if req.Category != nil {
		poll.Category = strings.TrimSpace(*req.Category)
	}
	if req.StartTime != nil {
		poll.StartTime = req.StartTime
	}
	if req.EndTime != nil {
		poll.EndTime = req.EndTime
	}
	if req.AllowAnonymous != nil {
		poll.AllowAnonymous = *req.AllowAnonymous
	}
	if req.AllowMultipleVote != nil {
		poll.AllowMultipleVote = *req.AllowMultipleVote
	}
	if req.RequireComment != nil {
		poll.RequireComment = *req.RequireComment
	}
	if req.ShowResults != nil {
		poll.ShowResults = *req.ShowResults
	}
	if req.ShowResultsAfter != nil {
		poll.ShowResultsAfter = *req.ShowResultsAfter
	}
	if req.DepartmentID != nil {
		poll.DepartmentID = req.DepartmentID
	}

	// Validate time logic if times are being updated
	if poll.StartTime != nil && poll.EndTime != nil && poll.EndTime.Before(*poll.StartTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	// Save updated poll
	if err := u.pollRepo.Update(poll); err != nil {
		return nil, fmt.Errorf("failed to update poll: %w", err)
	}

	// Get updated poll with all details
	updatedPoll, err := u.pollRepo.GetByIDWithAll(poll.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated poll: %w", err)
	}

	return updatedPoll.ToResponse(), nil
}

// DeletePoll deletes a poll
func (u *pollUsecase) DeletePoll(userID, pollID uint) error {
	// Get existing poll
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	// Check permissions: only creator can delete
	if poll.CreatedBy != userID {
		return fmt.Errorf("access denied: only poll creator can delete the poll")
	}

	// Check if poll has votes (might want to prevent deletion)
	voteCount, err := u.voteRepo.GetVoteCount(pollID)
	if err != nil {
		return fmt.Errorf("failed to check vote count: %w", err)
	}

	if voteCount > 0 {
		return fmt.Errorf("cannot delete poll with existing votes")
	}

	// Delete poll (cascade will handle related records)
	if err := u.pollRepo.Delete(pollID); err != nil {
		return fmt.Errorf("failed to delete poll: %w", err)
	}

	return nil
}

// GetPolls retrieves polls with filtering and pagination
func (u *pollUsecase) GetPolls(userID uint, filter *models.PollFilterRequest) (*models.PollListResponse, error) {
	// Validate filter
	if err := filter.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}

	// Set defaults
	if filter.Limit <= 0 {
		filter.Limit = models.DefaultLimit
	}
	if filter.SortBy == "" {
		filter.SortBy = "created_at"
	}
	if filter.SortOrder == "" {
		filter.SortOrder = "desc"
	}

	// Get polls from repository
	polls, total, err := u.pollRepo.GetPolls(userID, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get polls: %w", err)
	}

	// Convert to response format
	responses := make([]*models.PollResponse, len(polls))
	for i, poll := range polls {
		responses[i] = poll.ToResponse()
	}

	return &models.PollListResponse{
		Polls:   responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		Filters: filter,
	}, nil
}

// SearchPolls searches polls by query with filtering
func (u *pollUsecase) SearchPolls(userID uint, query string, filter *models.PollFilterRequest) (*models.PollListResponse, error) {
	// Validate inputs
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("search query is required")
	}

	if filter == nil {
		filter = &models.PollFilterRequest{}
	}

	if err := filter.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filter: %w", err)
	}

	// Set defaults
	if filter.Limit <= 0 {
		filter.Limit = models.DefaultLimit
	}

	// Search polls
	polls, total, err := u.pollRepo.SearchPolls(userID, query, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search polls: %w", err)
	}

	// Convert to response format
	responses := make([]*models.PollResponse, len(polls))
	for i, poll := range polls {
		responses[i] = poll.ToResponse()
	}

	return &models.PollListResponse{
		Polls:   responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		Filters: filter,
	}, nil
}

// UpdatePollStatus updates poll status
func (u *pollUsecase) UpdatePollStatus(userID, pollID uint, status models.PollStatus) error {
	// Get existing poll
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	// Check permissions: only creator can update status
	if poll.CreatedBy != userID {
		return fmt.Errorf("access denied: only poll creator can update status")
	}

	// Validate status transition
	if err := u.validateStatusTransition(poll.Status, status); err != nil {
		return fmt.Errorf("invalid status transition: %w", err)
	}

	// Update status
	if err := u.pollRepo.UpdateStatus(pollID, status); err != nil {
		return fmt.Errorf("failed to update poll status: %w", err)
	}

	return nil
}

// VotePoll handles voting on a poll
func (u *pollUsecase) VotePoll(userID, pollID uint, req *models.VotePollRequest) ([]*models.PollVoteResponse, error) {
	// Get poll with options
	poll, err := u.pollRepo.GetByIDWithOptions(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Check access rights
	if !u.hasPollAccess(userID, poll) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Check if poll is active
	if !poll.IsActive() {
		return nil, fmt.Errorf("poll is not active")
	}

	// Validate vote request
	if err := req.Validate(poll); err != nil {
		return nil, fmt.Errorf("invalid vote: %w", err)
	}

	// Check if user has already voted (if multiple votes not allowed)
	if !poll.AllowMultipleVote {
		hasVoted, err := u.voteRepo.HasUserVoted(userID, pollID)
		if err != nil {
			return nil, fmt.Errorf("failed to check if user voted: %w", err)
		}
		if hasVoted {
			return nil, fmt.Errorf("user has already voted on this poll")
		}
	} else {
		// If multiple votes allowed, delete previous votes first
		if err := u.voteRepo.DeleteByUserAndPoll(userID, pollID); err != nil {
			return nil, fmt.Errorf("failed to delete previous votes: %w", err)
		}
	}

	// Create votes based on poll type
	votes, err := u.createVotes(userID, poll, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create votes: %w", err)
	}

	// Save votes
	if err := u.voteRepo.CreateMultiple(votes); err != nil {
		return nil, fmt.Errorf("failed to save votes: %w", err)
	}

	// Mark participant as voted (for invite-only polls)
	if poll.Visibility == models.PollVisibilityInviteOnly {
		u.participantRepo.MarkAsVoted(userID, pollID) // Ignore error
	}

	// Convert to response format
	responses := make([]*models.PollVoteResponse, len(votes))
	for i, vote := range votes {
		responses[i] = vote.ToResponse()
	}

	return responses, nil
}

// GetUserVotes retrieves user's votes for a poll
func (u *pollUsecase) GetUserVotes(userID, pollID uint) ([]*models.PollVoteResponse, error) {
	// Check if poll exists and user has access
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	if !u.hasPollAccess(userID, poll) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Get user votes
	votes, err := u.voteRepo.GetByUserID(userID, pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user votes: %w", err)
	}

	// Convert to response format
	responses := make([]*models.PollVoteResponse, len(votes))
	for i, vote := range votes {
		responses[i] = vote.ToResponse()
	}

	return responses, nil
}

// GetPollResults retrieves poll results with statistics
func (u *pollUsecase) GetPollResults(userID, pollID uint) (*models.PollResultsResponse, error) {
	// Get poll with all data
	poll, err := u.pollRepo.GetByIDWithAll(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	// Check access rights
	if !u.hasPollAccess(userID, poll) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Check if user can view results
	if !u.canViewResults(userID, poll) {
		return nil, fmt.Errorf("access denied: results not available")
	}

	// Get basic statistics
	totalVotes, err := u.voteRepo.GetVoteCount(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vote count: %w", err)
	}

	totalVoters, err := u.voteRepo.GetVoterCount(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get voter count: %w", err)
	}

	// Load poll statistics
	u.loadPollStatistics(poll, userID)

	// Create response
	results := &models.PollResultsResponse{
		Poll:        poll.ToResponse(),
		TotalVotes:  int(totalVotes),
		TotalVoters: int(totalVoters),
	}

	// Get option-specific data
	optionVoteCounts, err := u.voteRepo.GetOptionVoteCounts(pollID)
	if err != nil {
		return nil, fmt.Errorf("failed to get option vote counts: %w", err)
	}

	results.VotesByOption = make(map[uint]int)
	for optionID, count := range optionVoteCounts {
		results.VotesByOption[optionID] = int(count)
	}

	// Load options with statistics
	options := make([]*models.PollOptionResponse, len(poll.Options))
	for i, option := range poll.Options {
		optionResp := option.ToResponse()
		optionResp.VoteCount = int(optionVoteCounts[option.ID])
		if totalVotes > 0 {
			optionResp.VotePercent = models.CalculateVotePercent(optionResp.VoteCount, int(totalVotes))
		}
		options[i] = optionResp
	}
	results.Options = options

	// Type-specific data
	switch poll.Type {
	case models.PollTypeOpenText:
		textResponses, err := u.voteRepo.GetTextResponses(pollID)
		if err == nil {
			results.TextResponses = textResponses
		}

	case models.PollTypeRating:
		results.RatingStats = make(map[uint]*models.RatingStats)
		for _, option := range poll.Options {
			stats, err := u.voteRepo.GetRatingStats(pollID, option.ID)
			if err == nil {
				results.RatingStats[option.ID] = stats
			}
		}

	case models.PollTypeRanking:
		results.RankingStats = make(map[uint]*models.RankingStats)
		for _, option := range poll.Options {
			stats, err := u.voteRepo.GetRankingStats(pollID, option.ID)
			if err == nil {
				results.RankingStats[option.ID] = stats
			}
		}
	}

	// Include vote details for non-anonymous polls (if allowed)
	if !poll.AllowAnonymous && u.canViewDetailedResults(userID, poll) {
		results.VotesByUser = make(map[uint][]*models.PollVoteResponse)
		// This would require additional repository method to get votes by user
		// Skipping for now to keep complexity manageable
	}

	return results, nil
}

// AddParticipants adds participants to an invite-only poll
func (u *pollUsecase) AddParticipants(userID, pollID uint, req *models.AddParticipantsRequest) error {
	// Get poll
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	// Check permissions: only creator can add participants
	if poll.CreatedBy != userID {
		return fmt.Errorf("access denied: only poll creator can add participants")
	}

	// Check that poll is invite-only
	if poll.Visibility != models.PollVisibilityInviteOnly {
		return fmt.Errorf("can only add participants to invite-only polls")
	}

	// Add participants
	participants := make([]*models.PollParticipant, 0)
	for _, participantID := range req.UserIDs {
		// Check if user is already a participant
		isParticipant, err := u.participantRepo.IsParticipant(participantID, pollID)
		if err != nil {
			continue // Skip on error
		}
		if isParticipant {
			continue // Skip if already participant
		}

		participant := &models.PollParticipant{
			PollID:    pollID,
			UserID:    participantID,
			InvitedBy: userID,
			InvitedAt: time.Now(),
		}
		participants = append(participants, participant)
	}

	if len(participants) > 0 {
		if err := u.participantRepo.CreateMultiple(participants); err != nil {
			return fmt.Errorf("failed to add participants: %w", err)
		}
	}

	return nil
}

// RemoveParticipant removes a participant from a poll
func (u *pollUsecase) RemoveParticipant(userID, pollID, participantID uint) error {
	// Get poll
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("poll not found")
		}
		return fmt.Errorf("failed to get poll: %w", err)
	}

	// Check permissions: creator can remove anyone, participants can remove themselves
	if poll.CreatedBy != userID && participantID != userID {
		return fmt.Errorf("access denied: insufficient permissions")
	}

	// Cannot remove the creator
	if participantID == poll.CreatedBy {
		return fmt.Errorf("cannot remove poll creator")
	}

	// Remove participant
	if err := u.participantRepo.DeleteByUserAndPoll(participantID, pollID); err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	return nil
}

// CreateComment creates a comment on a poll
func (u *pollUsecase) CreateComment(userID, pollID uint, req *models.CreateCommentRequest) (*models.PollCommentResponse, error) {
	// Validate request
	if req.Content == "" {
		return nil, fmt.Errorf("comment content is required")
	}
	if len(req.Content) > models.MaxCommentLength {
		return nil, fmt.Errorf("comment is too long")
	}

	// Get poll to check access
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}

	if !u.hasPollAccess(userID, poll) {
		return nil, fmt.Errorf("access denied: insufficient permissions")
	}

	// Validate parent comment if provided
	if req.ParentID != nil {
		parentComment, err := u.commentRepo.GetByID(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent comment not found")
		}
		if parentComment.PollID != pollID {
			return nil, fmt.Errorf("parent comment does not belong to this poll")
		}
	}

	// Create comment
	comment := &models.PollComment{
		PollID:   pollID,
		UserID:   userID,
		Content:  strings.TrimSpace(req.Content),
		ParentID: req.ParentID,
	}

	if err := u.commentRepo.Create(comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment.ToResponse(), nil
}

// GetComments retrieves comments for a poll
func (u *pollUsecase) GetComments(userID, pollID uint, limit, offset int) ([]*models.PollCommentResponse, int64, error) {
	// Check poll access
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, 0, fmt.Errorf("poll not found")
		}
		return nil, 0, fmt.Errorf("failed to get poll: %w", err)
	}

	if !u.hasPollAccess(userID, poll) {
		return nil, 0, fmt.Errorf("access denied: insufficient permissions")
	}

	// Set defaults
	if limit <= 0 {
		limit = models.DefaultLimit
	}
	if limit > models.MaxLimit {
		limit = models.MaxLimit
	}
	if offset < 0 {
		offset = 0
	}

	// Get comments with replies
	comments, total, err := u.commentRepo.GetWithReplies(pollID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get comments: %w", err)
	}

	// Convert to response format
	responses := make([]*models.PollCommentResponse, len(comments))
	for i, comment := range comments {
		responses[i] = comment.ToResponse()
	}

	return responses, total, nil
}

// DeleteComment deletes a comment
func (u *pollUsecase) DeleteComment(userID, pollID, commentID uint) error {
	// Get comment
	comment, err := u.commentRepo.GetByID(commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("comment not found")
		}
		return fmt.Errorf("failed to get comment: %w", err)
	}

	// Verify comment belongs to the poll
	if comment.PollID != pollID {
		return fmt.Errorf("comment does not belong to this poll")
	}

	// Check permissions: user can delete their own comments, poll creator can delete any
	poll, err := u.pollRepo.GetByID(pollID)
	if err != nil {
		return fmt.Errorf("failed to get poll: %w", err)
	}

	if comment.UserID != userID && poll.CreatedBy != userID {
		return fmt.Errorf("access denied: insufficient permissions")
	}

	// Delete comment
	if err := u.commentRepo.Delete(commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

// GetPollStats retrieves poll statistics for a user
func (u *pollUsecase) GetPollStats(userID uint) (*models.PollStatsResponse, error) {
	stats, err := u.pollRepo.GetPollStats(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get poll stats: %w", err)
	}

	return stats, nil
}

// Helper methods

// hasPollAccess checks if user has access to view a poll
func (u *pollUsecase) hasPollAccess(userID uint, poll *models.Poll) bool {
	// User is creator
	if poll.CreatedBy == userID {
		return true
	}

	// Public polls
	if poll.Visibility == models.PollVisibilityPublic {
		return true
	}

	// Department polls (simplified - in real app, check user's department)
	if poll.Visibility == models.PollVisibilityDepartment {
		// TODO: Implement department check
		return true
	}

	// Invite-only polls
	if poll.Visibility == models.PollVisibilityInviteOnly {
		isParticipant, err := u.participantRepo.IsParticipant(userID, poll.ID)
		return err == nil && isParticipant
	}

	// Private polls
	return poll.Visibility == models.PollVisibilityPrivate && poll.CreatedBy == userID
}

// canViewResults checks if user can view poll results
func (u *pollUsecase) canViewResults(userID uint, poll *models.Poll) bool {
	// Creator can always view results
	if poll.CreatedBy == userID {
		return true
	}

	// If show results is disabled, only creator can view
	if !poll.ShowResults {
		return false
	}

	// If show results after voting is enabled, check if user has voted
	if poll.ShowResultsAfter {
		hasVoted, err := u.voteRepo.HasUserVoted(userID, poll.ID)
		return err == nil && hasVoted
	}

	// General access check
	return u.hasPollAccess(userID, poll)
}

// canViewDetailedResults checks if user can view detailed voting results
func (u *pollUsecase) canViewDetailedResults(userID uint, poll *models.Poll) bool {
	// Only creator can view detailed results for now
	return poll.CreatedBy == userID
}

// validateStatusTransition validates if status transition is allowed
func (u *pollUsecase) validateStatusTransition(from, to models.PollStatus) error {
	// Define allowed transitions
	allowedTransitions := map[models.PollStatus][]models.PollStatus{
		models.PollStatusDraft: {
			models.PollStatusActive,
			models.PollStatusCancelled,
		},
		models.PollStatusActive: {
			models.PollStatusClosed,
			models.PollStatusCancelled,
		},
		models.PollStatusClosed: {
			models.PollStatusArchived,
			models.PollStatusActive, // Allow reopening
		},
		models.PollStatusArchived: {
			// No transitions allowed from archived
		},
		models.PollStatusCancelled: {
			// No transitions allowed from cancelled
		},
	}

	if allowed, exists := allowedTransitions[from]; exists {
		for _, allowedStatus := range allowed {
			if to == allowedStatus {
				return nil
			}
		}
	}

	return fmt.Errorf("transition from %s to %s is not allowed", from, to)
}

// createVotes creates vote records based on poll type and request
func (u *pollUsecase) createVotes(userID uint, poll *models.Poll, req *models.VotePollRequest) ([]*models.PollVote, error) {
	var votes []*models.PollVote

	// Determine if vote should be anonymous
	var voteUserID *uint
	if !req.IsAnonymous {
		voteUserID = &userID
	}

	switch poll.Type {
	case models.PollTypeSingleChoice, models.PollTypeMultipleChoice:
		for _, optionID := range req.OptionIDs {
			// Validate option belongs to poll
			if !u.isValidOption(poll, optionID) {
				return nil, fmt.Errorf("invalid option ID: %d", optionID)
			}

			vote := &models.PollVote{
				PollID:      poll.ID,
				OptionID:    &optionID,
				UserID:      voteUserID,
				IsAnonymous: req.IsAnonymous,
				Comment:     req.Comment,
			}
			votes = append(votes, vote)
		}

	case models.PollTypeOpenText:
		vote := &models.PollVote{
			PollID:      poll.ID,
			UserID:      voteUserID,
			IsAnonymous: req.IsAnonymous,
			TextValue:   req.TextValue,
			Comment:     req.Comment,
		}
		votes = append(votes, vote)

	case models.PollTypeRating:
		for optionID, rating := range req.RatingValues {
			// Validate option belongs to poll
			if !u.isValidOption(poll, optionID) {
				return nil, fmt.Errorf("invalid option ID: %d", optionID)
			}

			vote := &models.PollVote{
				PollID:      poll.ID,
				OptionID:    &optionID,
				UserID:      voteUserID,
				IsAnonymous: req.IsAnonymous,
				RatingValue: &rating,
				Comment:     req.Comment,
			}
			votes = append(votes, vote)
		}

	case models.PollTypeRanking:
		for optionID, ranking := range req.RankingValues {
			// Validate option belongs to poll
			if !u.isValidOption(poll, optionID) {
				return nil, fmt.Errorf("invalid option ID: %d", optionID)
			}

			vote := &models.PollVote{
				PollID:       poll.ID,
				OptionID:     &optionID,
				UserID:       voteUserID,
				IsAnonymous:  req.IsAnonymous,
				RankingValue: &ranking,
				Comment:      req.Comment,
			}
			votes = append(votes, vote)
		}
	}

	return votes, nil
}

// isValidOption checks if an option ID belongs to the poll
func (u *pollUsecase) isValidOption(poll *models.Poll, optionID uint) bool {
	for _, option := range poll.Options {
		if option.ID == optionID {
			return true
		}
	}
	return false
}

// loadPollStatistics loads computed statistics for a poll
func (u *pollUsecase) loadPollStatistics(poll *models.Poll, userID uint) {
	// Load vote count
	if voteCount, err := u.voteRepo.GetVoteCount(poll.ID); err == nil {
		poll.TotalVotes = int(voteCount)
	}

	// Load voter count
	if voterCount, err := u.voteRepo.GetVoterCount(poll.ID); err == nil {
		poll.TotalVoters = int(voterCount)
	}

	// Check if user has voted
	if hasVoted, err := u.voteRepo.HasUserVoted(userID, poll.ID); err == nil {
		poll.UserHasVoted = hasVoted
	}

	// Calculate participation rate for invite-only polls
	if poll.Visibility == models.PollVisibilityInviteOnly {
		if participantCount, err := u.participantRepo.GetParticipantCount(poll.ID); err == nil {
			poll.ParticipantRate = models.CalculateParticipantRate(poll.TotalVoters, int(participantCount))
		}
	}

	// Load option statistics
	if optionVoteCounts, err := u.voteRepo.GetOptionVoteCounts(poll.ID); err == nil {
		for _, option := range poll.Options {
			if count, exists := optionVoteCounts[option.ID]; exists {
				option.VoteCount = int(count)
				if poll.TotalVotes > 0 {
					option.VotePercent = models.CalculateVotePercent(option.VoteCount, poll.TotalVotes)
				}
			}
		}
	}
}
