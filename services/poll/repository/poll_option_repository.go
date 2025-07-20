// File: services/poll/repository/poll_option_repository.go
package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// PollOptionRepository defines the interface for poll option data operations
type PollOptionRepository interface {
	Create(option *models.PollOption) error
	CreateMultiple(options []*models.PollOption) error
	GetByID(id uint) (*models.PollOption, error)
	GetByPollID(pollID uint) ([]*models.PollOption, error)
	Update(option *models.PollOption) error
	Delete(id uint) error
	DeleteByPollID(pollID uint) error
	UpdatePositions(optionPositions map[uint]int) error
	GetWithVoteCount(pollID uint) ([]*models.PollOption, error)
}

// pollOptionRepository implements PollOptionRepository interface
type pollOptionRepository struct {
	db *database.DB
}

// NewPollOptionRepository creates a new poll option repository
func NewPollOptionRepository(db *database.DB) PollOptionRepository {
	return &pollOptionRepository{
		db: db,
	}
}

// Create creates a new poll option
func (r *pollOptionRepository) Create(option *models.PollOption) error {
	if err := r.db.Create(option).Error; err != nil {
		return fmt.Errorf("failed to create poll option: %w", err)
	}
	return nil
}

// CreateMultiple creates multiple poll options
func (r *pollOptionRepository) CreateMultiple(options []*models.PollOption) error {
	if err := r.db.Create(&options).Error; err != nil {
		return fmt.Errorf("failed to create poll options: %w", err)
	}
	return nil
}

// GetByID retrieves a poll option by ID
func (r *pollOptionRepository) GetByID(id uint) (*models.PollOption, error) {
	var option models.PollOption
	err := r.db.First(&option, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll option not found")
		}
		return nil, fmt.Errorf("failed to get poll option: %w", err)
	}
	return &option, nil
}

// GetByPollID retrieves all options for a poll
func (r *pollOptionRepository) GetByPollID(pollID uint) ([]*models.PollOption, error) {
	var options []*models.PollOption
	err := r.db.Where("poll_id = ?", pollID).Order("position ASC").Find(&options).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}
	return options, nil
}

// Update updates an existing poll option
func (r *pollOptionRepository) Update(option *models.PollOption) error {
	result := r.db.Save(option)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll option: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll option not found")
	}
	return nil
}

// Delete deletes a poll option by ID
func (r *pollOptionRepository) Delete(id uint) error {
	result := r.db.Delete(&models.PollOption{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete poll option: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll option not found")
	}
	return nil
}

// DeleteByPollID deletes all options for a poll
func (r *pollOptionRepository) DeleteByPollID(pollID uint) error {
	err := r.db.Where("poll_id = ?", pollID).Delete(&models.PollOption{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete poll options: %w", err)
	}
	return nil
}

// UpdatePositions updates the positions of multiple options
func (r *pollOptionRepository) UpdatePositions(optionPositions map[uint]int) error {
	for optionID, position := range optionPositions {
		err := r.db.Model(&models.PollOption{}).
			Where("id = ?", optionID).
			Update("position", position).Error
		if err != nil {
			return fmt.Errorf("failed to update option position: %w", err)
		}
	}
	return nil
}

// GetWithVoteCount retrieves options with vote counts
func (r *pollOptionRepository) GetWithVoteCount(pollID uint) ([]*models.PollOption, error) {
	var options []*models.PollOption
	err := r.db.Where("poll_id = ?", pollID).
		Order("position ASC").
		Find(&options).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get poll options: %w", err)
	}

	// Load vote counts for each option
	for _, option := range options {
		var voteCount int64
		r.db.Model(&models.PollVote{}).
			Where("option_id = ?", option.ID).
			Count(&voteCount)
		option.VoteCount = int(voteCount)
	}

	return options, nil
}

// File: services/poll/repository/poll_vote_repository.go

// PollVoteRepository defines the interface for poll vote data operations
type PollVoteRepository interface {
	Create(vote *models.PollVote) error
	CreateMultiple(votes []*models.PollVote) error
	GetByID(id uint) (*models.PollVote, error)
	GetByPollID(pollID uint) ([]*models.PollVote, error)
	GetByUserID(userID uint, pollID uint) ([]*models.PollVote, error)
	GetByOptionID(optionID uint) ([]*models.PollVote, error)
	Update(vote *models.PollVote) error
	Delete(id uint) error
	DeleteByUserAndPoll(userID uint, pollID uint) error
	HasUserVoted(userID uint, pollID uint) (bool, error)
	GetVoteCount(pollID uint) (int64, error)
	GetVoterCount(pollID uint) (int64, error)
	GetOptionVoteCounts(pollID uint) (map[uint]int64, error)
	GetRatingStats(pollID uint, optionID uint) (*models.RatingStats, error)
	GetRankingStats(pollID uint, optionID uint) (*models.RankingStats, error)
	GetTextResponses(pollID uint) ([]string, error)
}

// pollVoteRepository implements PollVoteRepository interface
type pollVoteRepository struct {
	db *database.DB
}

// NewPollVoteRepository creates a new poll vote repository
func NewPollVoteRepository(db *database.DB) PollVoteRepository {
	return &pollVoteRepository{
		db: db,
	}
}

// Create creates a new poll vote
func (r *pollVoteRepository) Create(vote *models.PollVote) error {
	if err := r.db.Create(vote).Error; err != nil {
		return fmt.Errorf("failed to create poll vote: %w", err)
	}
	return nil
}

// CreateMultiple creates multiple poll votes
func (r *pollVoteRepository) CreateMultiple(votes []*models.PollVote) error {
	if err := r.db.Create(&votes).Error; err != nil {
		return fmt.Errorf("failed to create poll votes: %w", err)
	}
	return nil
}

// GetByID retrieves a poll vote by ID
func (r *pollVoteRepository) GetByID(id uint) (*models.PollVote, error) {
	var vote models.PollVote
	err := r.db.First(&vote, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll vote not found")
		}
		return nil, fmt.Errorf("failed to get poll vote: %w", err)
	}
	return &vote, nil
}

// GetByPollID retrieves all votes for a poll
func (r *pollVoteRepository) GetByPollID(pollID uint) ([]*models.PollVote, error) {
	var votes []*models.PollVote
	err := r.db.Where("poll_id = ?", pollID).Find(&votes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get poll votes: %w", err)
	}
	return votes, nil
}

// GetByUserID retrieves all votes by a user for a specific poll
func (r *pollVoteRepository) GetByUserID(userID uint, pollID uint) ([]*models.PollVote, error) {
	var votes []*models.PollVote
	err := r.db.Where("user_id = ? AND poll_id = ?", userID, pollID).Find(&votes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user votes: %w", err)
	}
	return votes, nil
}

// GetByOptionID retrieves all votes for a specific option
func (r *pollVoteRepository) GetByOptionID(optionID uint) ([]*models.PollVote, error) {
	var votes []*models.PollVote
	err := r.db.Where("option_id = ?", optionID).Find(&votes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get option votes: %w", err)
	}
	return votes, nil
}

// Update updates an existing poll vote
func (r *pollVoteRepository) Update(vote *models.PollVote) error {
	result := r.db.Save(vote)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll vote: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll vote not found")
	}
	return nil
}

// Delete deletes a poll vote by ID
func (r *pollVoteRepository) Delete(id uint) error {
	result := r.db.Delete(&models.PollVote{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete poll vote: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll vote not found")
	}
	return nil
}

// DeleteByUserAndPoll deletes all votes by a user for a specific poll
func (r *pollVoteRepository) DeleteByUserAndPoll(userID uint, pollID uint) error {
	err := r.db.Where("user_id = ? AND poll_id = ?", userID, pollID).Delete(&models.PollVote{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete user votes: %w", err)
	}
	return nil
}

// HasUserVoted checks if a user has voted in a poll
func (r *pollVoteRepository) HasUserVoted(userID uint, pollID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.PollVote{}).
		Where("user_id = ? AND poll_id = ?", userID, pollID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if user voted: %w", err)
	}
	return count > 0, nil
}

// GetVoteCount returns total number of votes for a poll
func (r *pollVoteRepository) GetVoteCount(pollID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.PollVote{}).
		Where("poll_id = ?", pollID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get vote count: %w", err)
	}
	return count, nil
}

// GetVoterCount returns number of unique voters for a poll
func (r *pollVoteRepository) GetVoterCount(pollID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.PollVote{}).
		Where("poll_id = ?", pollID).
		Distinct("COALESCE(user_id, id)").
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get voter count: %w", err)
	}
	return count, nil
}

// GetOptionVoteCounts returns vote counts for each option in a poll
func (r *pollVoteRepository) GetOptionVoteCounts(pollID uint) (map[uint]int64, error) {
	type optionCount struct {
		OptionID uint
		Count    int64
	}

	var counts []optionCount
	err := r.db.Model(&models.PollVote{}).
		Select("option_id, COUNT(*) as count").
		Where("poll_id = ? AND option_id IS NOT NULL", pollID).
		Group("option_id").
		Scan(&counts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get option vote counts: %w", err)
	}

	result := make(map[uint]int64)
	for _, count := range counts {
		result[count.OptionID] = count.Count
	}

	return result, nil
}

// GetRatingStats calculates rating statistics for an option
func (r *pollVoteRepository) GetRatingStats(pollID uint, optionID uint) (*models.RatingStats, error) {
	var ratings []int
	err := r.db.Model(&models.PollVote{}).
		Select("rating_value").
		Where("poll_id = ? AND option_id = ? AND rating_value IS NOT NULL", pollID, optionID).
		Pluck("rating_value", &ratings).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get rating values: %w", err)
	}

	if len(ratings) == 0 {
		return &models.RatingStats{
			OptionID:     optionID,
			TotalRatings: 0,
		}, nil
	}

	stats := &models.RatingStats{
		OptionID:     optionID,
		TotalRatings: len(ratings),
		Distribution: make(map[int]int),
	}

	// Calculate min, max, sum
	sum := 0
	stats.Min = ratings[0]
	stats.Max = ratings[0]

	for _, rating := range ratings {
		sum += rating
		if rating < stats.Min {
			stats.Min = rating
		}
		if rating > stats.Max {
			stats.Max = rating
		}
		stats.Distribution[rating]++
	}

	stats.Average = float64(sum) / float64(len(ratings))

	return stats, nil
}

// GetRankingStats calculates ranking statistics for an option
func (r *pollVoteRepository) GetRankingStats(pollID uint, optionID uint) (*models.RankingStats, error) {
	var rankings []int
	err := r.db.Model(&models.PollVote{}).
		Select("ranking_value").
		Where("poll_id = ? AND option_id = ? AND ranking_value IS NOT NULL", pollID, optionID).
		Pluck("ranking_value", &rankings).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get ranking values: %w", err)
	}

	if len(rankings) == 0 {
		return &models.RankingStats{
			OptionID:      optionID,
			TotalRankings: 0,
		}, nil
	}

	stats := &models.RankingStats{
		OptionID:         optionID,
		TotalRankings:    len(rankings),
		RankDistribution: make(map[int]int),
	}

	// Calculate best, worst, sum
	sum := 0
	stats.BestRank = rankings[0]
	stats.WorstRank = rankings[0]

	for _, ranking := range rankings {
		sum += ranking
		if ranking < stats.BestRank {
			stats.BestRank = ranking
		}
		if ranking > stats.WorstRank {
			stats.WorstRank = ranking
		}
		stats.RankDistribution[ranking]++
	}

	stats.AverageRank = float64(sum) / float64(len(rankings))

	return stats, nil
}

// GetTextResponses returns all text responses for a poll
func (r *pollVoteRepository) GetTextResponses(pollID uint) ([]string, error) {
	var responses []string
	err := r.db.Model(&models.PollVote{}).
		Select("text_value").
		Where("poll_id = ? AND text_value != ''", pollID).
		Pluck("text_value", &responses).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get text responses: %w", err)
	}
	return responses, nil
}

// File: services/poll/repository/poll_participant_repository.go

// PollParticipantRepository defines the interface for poll participant data operations
type PollParticipantRepository interface {
	Create(participant *models.PollParticipant) error
	CreateMultiple(participants []*models.PollParticipant) error
	GetByID(id uint) (*models.PollParticipant, error)
	GetByPollID(pollID uint) ([]*models.PollParticipant, error)
	GetByUserID(userID uint) ([]*models.PollParticipant, error)
	Update(participant *models.PollParticipant) error
	Delete(id uint) error
	DeleteByUserAndPoll(userID uint, pollID uint) error
	IsParticipant(userID uint, pollID uint) (bool, error)
	MarkAsVoted(userID uint, pollID uint) error
	MarkAsNotified(userID uint, pollID uint) error
	GetParticipantCount(pollID uint) (int64, error)
}

// pollParticipantRepository implements PollParticipantRepository interface
type pollParticipantRepository struct {
	db *database.DB
}

// NewPollParticipantRepository creates a new poll participant repository
func NewPollParticipantRepository(db *database.DB) PollParticipantRepository {
	return &pollParticipantRepository{
		db: db,
	}
}

// Create creates a new poll participant
func (r *pollParticipantRepository) Create(participant *models.PollParticipant) error {
	if err := r.db.Create(participant).Error; err != nil {
		return fmt.Errorf("failed to create poll participant: %w", err)
	}
	return nil
}

// CreateMultiple creates multiple poll participants
func (r *pollParticipantRepository) CreateMultiple(participants []*models.PollParticipant) error {
	if err := r.db.Create(&participants).Error; err != nil {
		return fmt.Errorf("failed to create poll participants: %w", err)
	}
	return nil
}

// GetByID retrieves a poll participant by ID
func (r *pollParticipantRepository) GetByID(id uint) (*models.PollParticipant, error) {
	var participant models.PollParticipant
	err := r.db.First(&participant, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll participant not found")
		}
		return nil, fmt.Errorf("failed to get poll participant: %w", err)
	}
	return &participant, nil
}

// GetByPollID retrieves all participants for a poll
func (r *pollParticipantRepository) GetByPollID(pollID uint) ([]*models.PollParticipant, error) {
	var participants []*models.PollParticipant
	err := r.db.Where("poll_id = ?", pollID).Find(&participants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get poll participants: %w", err)
	}
	return participants, nil
}

// GetByUserID retrieves all polls where user is a participant
func (r *pollParticipantRepository) GetByUserID(userID uint) ([]*models.PollParticipant, error) {
	var participants []*models.PollParticipant
	err := r.db.Where("user_id = ?", userID).Find(&participants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user participations: %w", err)
	}
	return participants, nil
}

// Update updates an existing poll participant
func (r *pollParticipantRepository) Update(participant *models.PollParticipant) error {
	result := r.db.Save(participant)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll participant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll participant not found")
	}
	return nil
}

// Delete deletes a poll participant by ID
func (r *pollParticipantRepository) Delete(id uint) error {
	result := r.db.Delete(&models.PollParticipant{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete poll participant: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll participant not found")
	}
	return nil
}

// DeleteByUserAndPoll deletes a participant by user and poll
func (r *pollParticipantRepository) DeleteByUserAndPoll(userID uint, pollID uint) error {
	err := r.db.Where("user_id = ? AND poll_id = ?", userID, pollID).Delete(&models.PollParticipant{}).Error
	if err != nil {
		return fmt.Errorf("failed to delete poll participant: %w", err)
	}
	return nil
}

// IsParticipant checks if a user is a participant of a poll
func (r *pollParticipantRepository) IsParticipant(userID uint, pollID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.PollParticipant{}).
		Where("user_id = ? AND poll_id = ?", userID, pollID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if user is participant: %w", err)
	}
	return count > 0, nil
}

// MarkAsVoted marks a participant as having voted
func (r *pollParticipantRepository) MarkAsVoted(userID uint, pollID uint) error {
	now := time.Now()
	result := r.db.Model(&models.PollParticipant{}).
		Where("user_id = ? AND poll_id = ?", userID, pollID).
		Update("voted_at", &now)
	if result.Error != nil {
		return fmt.Errorf("failed to mark participant as voted: %w", result.Error)
	}
	return nil
}

// MarkAsNotified marks a participant as having been notified
func (r *pollParticipantRepository) MarkAsNotified(userID uint, pollID uint) error {
	now := time.Now()
	result := r.db.Model(&models.PollParticipant{}).
		Where("user_id = ? AND poll_id = ?", userID, pollID).
		Update("notified_at", &now)
	if result.Error != nil {
		return fmt.Errorf("failed to mark participant as notified: %w", result.Error)
	}
	return nil
}

// GetParticipantCount returns the number of participants for a poll
func (r *pollParticipantRepository) GetParticipantCount(pollID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.PollParticipant{}).
		Where("poll_id = ?", pollID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get participant count: %w", err)
	}
	return count, nil
}

// File: services/poll/repository/poll_comment_repository.go

// PollCommentRepository defines the interface for poll comment data operations
type PollCommentRepository interface {
	Create(comment *models.PollComment) error
	GetByID(id uint) (*models.PollComment, error)
	GetByPollID(pollID uint, limit, offset int) ([]*models.PollComment, int64, error)
	GetWithReplies(pollID uint, limit, offset int) ([]*models.PollComment, int64, error)
	Update(comment *models.PollComment) error
	Delete(id uint) error
	CountByPollID(pollID uint) (int64, error)
	GetByUserID(userID uint, limit, offset int) ([]*models.PollComment, error)
}

// pollCommentRepository implements PollCommentRepository interface
type pollCommentRepository struct {
	db *database.DB
}

// NewPollCommentRepository creates a new poll comment repository
func NewPollCommentRepository(db *database.DB) PollCommentRepository {
	return &pollCommentRepository{
		db: db,
	}
}

// Create creates a new poll comment
func (r *pollCommentRepository) Create(comment *models.PollComment) error {
	if err := r.db.Create(comment).Error; err != nil {
		return fmt.Errorf("failed to create poll comment: %w", err)
	}
	return nil
}

// GetByID retrieves a poll comment by ID
func (r *pollCommentRepository) GetByID(id uint) (*models.PollComment, error) {
	var comment models.PollComment
	err := r.db.First(&comment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll comment not found")
		}
		return nil, fmt.Errorf("failed to get poll comment: %w", err)
	}
	return &comment, nil
}

// GetByPollID retrieves all top-level comments for a poll with pagination
func (r *pollCommentRepository) GetByPollID(pollID uint, limit, offset int) ([]*models.PollComment, int64, error) {
	query := r.db.Model(&models.PollComment{}).
		Where("poll_id = ? AND parent_id IS NULL", pollID)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count poll comments: %w", err)
	}

	var comments []*models.PollComment
	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&comments).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get poll comments: %w", err)
	}

	return comments, total, nil
}

// GetWithReplies retrieves comments with their replies
func (r *pollCommentRepository) GetWithReplies(pollID uint, limit, offset int) ([]*models.PollComment, int64, error) {
	query := r.db.Model(&models.PollComment{}).
		Where("poll_id = ? AND parent_id IS NULL", pollID)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count poll comments: %w", err)
	}

	var comments []*models.PollComment
	err := r.db.Preload("Replies", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC")
	}).Where("poll_id = ? AND parent_id IS NULL", pollID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&comments).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get poll comments with replies: %w", err)
	}

	return comments, total, nil
}

// Update updates an existing poll comment
func (r *pollCommentRepository) Update(comment *models.PollComment) error {
	result := r.db.Save(comment)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll comment not found")
	}
	return nil
}

// Delete deletes a poll comment by ID
func (r *pollCommentRepository) Delete(id uint) error {
	result := r.db.Delete(&models.PollComment{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete poll comment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll comment not found")
	}
	return nil
}

// CountByPollID returns the total number of comments for a poll
func (r *pollCommentRepository) CountByPollID(pollID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.PollComment{}).
		Where("poll_id = ?", pollID).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count poll comments: %w", err)
	}
	return count, nil
}

// GetByUserID retrieves comments by a specific user
func (r *pollCommentRepository) GetByUserID(userID uint, limit, offset int) ([]*models.PollComment, error) {
	var comments []*models.PollComment
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&comments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user comments: %w", err)
	}
	return comments, nil
}
