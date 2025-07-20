// File: services/poll/repository/poll_repository.go
package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// PollRepository defines the interface for poll data operations
type PollRepository interface {
	Create(poll *models.Poll) error
	GetByID(id uint) (*models.Poll, error)
	GetByIDWithOptions(id uint) (*models.Poll, error)
	GetByIDWithAll(id uint) (*models.Poll, error)
	Update(poll *models.Poll) error
	Delete(id uint) error
	GetPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	SearchPolls(userID uint, query string, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetPollStats(userID uint) (*models.PollStatsResponse, error)
	GetUserPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetParticipatedPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetPollsByStatus(status models.PollStatus, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetExpiredPolls() ([]*models.Poll, error)
	UpdateStatus(id uint, status models.PollStatus) error
	Count() (int64, error)
	CountByCreator(userID uint) (int64, error)
	CountByStatus(status models.PollStatus) (int64, error)
}

// pollRepository implements PollRepository interface
type pollRepository struct {
	db *database.DB
}

// NewPollRepository creates a new poll repository
func NewPollRepository(db *database.DB) PollRepository {
	return &pollRepository{
		db: db,
	}
}

// Create creates a new poll
func (r *pollRepository) Create(poll *models.Poll) error {
	if err := r.db.Create(poll).Error; err != nil {
		return fmt.Errorf("failed to create poll: %w", err)
	}
	return nil
}

// GetByID retrieves a poll by ID
func (r *pollRepository) GetByID(id uint) (*models.Poll, error) {
	var poll models.Poll
	err := r.db.First(&poll, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll: %w", err)
	}
	return &poll, nil
}

// GetByIDWithOptions retrieves a poll by ID with options preloaded
func (r *pollRepository) GetByIDWithOptions(id uint) (*models.Poll, error) {
	var poll models.Poll
	err := r.db.Preload("Options", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	}).First(&poll, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll with options: %w", err)
	}
	return &poll, nil
}

// GetByIDWithAll retrieves a poll by ID with all related data
func (r *pollRepository) GetByIDWithAll(id uint) (*models.Poll, error) {
	var poll models.Poll
	err := r.db.
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Votes").
		Preload("Participants").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Where("parent_id IS NULL").Order("created_at DESC")
		}).
		Preload("Comments.Replies", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		First(&poll, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("poll not found")
		}
		return nil, fmt.Errorf("failed to get poll with all data: %w", err)
	}
	return &poll, nil
}

// Update updates an existing poll
func (r *pollRepository) Update(poll *models.Poll) error {
	result := r.db.Save(poll)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll not found")
	}
	return nil
}

// Delete soft deletes a poll by ID
func (r *pollRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Poll{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete poll: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll not found")
	}
	return nil
}

// GetPolls retrieves polls based on visibility and filters
func (r *pollRepository) GetPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		})

	// Apply visibility filter
	query = r.applyVisibilityFilter(query, userID)

	// Apply other filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count polls: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get polls: %w", err)
	}

	// Load computed fields
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}

// SearchPolls searches polls by title and description
func (r *pollRepository) SearchPolls(userID uint, searchQuery string, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		})

	// Apply visibility filter
	query = r.applyVisibilityFilter(query, userID)

	// Apply search filter
	searchTerm := "%" + strings.ToLower(searchQuery) + "%"
	query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", searchTerm, searchTerm)

	// Apply other filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search polls: %w", err)
	}

	// Load computed fields
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}

// GetPollStats retrieves poll statistics for a user
func (r *pollRepository) GetPollStats(userID uint) (*models.PollStatsResponse, error) {
	stats := &models.PollStatsResponse{}

	// Total polls accessible to user
	var totalCount int64
	query := r.db.Model(&models.Poll{})
	query = r.applyVisibilityFilter(query, userID)
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total polls: %w", err)
	}
	stats.TotalPolls = int(totalCount)

	// Polls by status
	statusCounts := []struct {
		Status models.PollStatus
		Count  *int
	}{
		{models.PollStatusActive, &stats.ActivePolls},
		{models.PollStatusDraft, &stats.DraftPolls},
		{models.PollStatusClosed, &stats.ClosedPolls},
	}

	for _, sc := range statusCounts {
		var count int64
		query = r.db.Model(&models.Poll{}).Where("status = ?", sc.Status)
		query = r.applyVisibilityFilter(query, userID)
		if err := query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count polls by status %s: %w", sc.Status, err)
		}
		*sc.Count = int(count)
	}

	// My polls (created by user)
	var myCount int64
	if err := r.db.Model(&models.Poll{}).Where("created_by = ?", userID).Count(&myCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count my polls: %w", err)
	}
	stats.MyPolls = int(myCount)

	// Participated polls
	var participatedCount int64
	err := r.db.Model(&models.Poll{}).
		Joins("JOIN poll_votes ON polls.id = poll_votes.poll_id").
		Where("poll_votes.user_id = ?", userID).
		Distinct("polls.id").
		Count(&participatedCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count participated polls: %w", err)
	}
	stats.ParticipatedIn = int(participatedCount)

	// Polls by type
	stats.PollsByType = make(map[models.PollType]int)
	var typeStats []struct {
		Type  models.PollType
		Count int64
	}
	query = r.db.Model(&models.Poll{}).Select("type, COUNT(*) as count").Group("type")
	query = r.applyVisibilityFilter(query, userID)
	if err := query.Scan(&typeStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get polls by type: %w", err)
	}
	for _, ts := range typeStats {
		stats.PollsByType[ts.Type] = int(ts.Count)
	}

	// Polls by category
	stats.PollsByCategory = make(map[string]int)
	var categoryStats []struct {
		Category string
		Count    int64
	}
	query = r.db.Model(&models.Poll{}).
		Select("COALESCE(category, 'Uncategorized') as category, COUNT(*) as count").
		Group("category")
	query = r.applyVisibilityFilter(query, userID)
	if err := query.Scan(&categoryStats).Error; err != nil {
		return nil, fmt.Errorf("failed to get polls by category: %w", err)
	}
	for _, cs := range categoryStats {
		stats.PollsByCategory[cs.Category] = int(cs.Count)
	}

	// Recent activity (simplified)
	stats.RecentActivity = []*models.PollActivityResponse{}

	return stats, nil
}

// GetUserPolls retrieves polls created by a specific user
func (r *pollRepository) GetUserPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Where("created_by = ?", userID).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		})

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count user polls: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get user polls: %w", err)
	}

	// Load computed fields
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}

// GetParticipatedPolls retrieves polls where user has voted
func (r *pollRepository) GetParticipatedPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Joins("JOIN poll_votes ON polls.id = poll_votes.poll_id").
		Where("poll_votes.user_id = ?", userID).
		Group("polls.id").
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		})

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count participated polls: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get participated polls: %w", err)
	}

	// Load computed fields
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}

// GetPollsByStatus retrieves polls by status
func (r *pollRepository) GetPollsByStatus(status models.PollStatus, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Where("status = ?", status).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		})

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count polls by status: %w", err)
	}

	// Apply sorting and pagination
	query = r.applySortingAndPagination(query, filter)

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get polls by status: %w", err)
	}

	return polls, total, nil
}

// GetExpiredPolls retrieves polls that should be closed (end_time has passed)
func (r *pollRepository) GetExpiredPolls() ([]*models.Poll, error) {
	var polls []*models.Poll
	now := time.Now()

	err := r.db.Where("status = ? AND end_time IS NOT NULL AND end_time < ?",
		models.PollStatusActive, now).Find(&polls).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get expired polls: %w", err)
	}

	return polls, nil
}

// UpdateStatus updates poll status
func (r *pollRepository) UpdateStatus(id uint, status models.PollStatus) error {
	result := r.db.Model(&models.Poll{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("failed to update poll status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("poll not found")
	}
	return nil
}

// Count returns the total number of polls
func (r *pollRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Poll{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count polls: %w", err)
	}
	return count, nil
}

// CountByCreator returns the number of polls created by a user
func (r *pollRepository) CountByCreator(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Poll{}).Where("created_by = ?", userID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count polls by creator: %w", err)
	}
	return count, nil
}

// CountByStatus returns the number of polls with a specific status
func (r *pollRepository) CountByStatus(status models.PollStatus) (int64, error) {
	var count int64
	err := r.db.Model(&models.Poll{}).Where("status = ?", status).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count polls by status: %w", err)
	}
	return count, nil
}

// Helper methods

// applyVisibilityFilter applies visibility filtering based on user access
func (r *pollRepository) applyVisibilityFilter(query *gorm.DB, userID uint) *gorm.DB {
	return query.Where(
		"visibility = ? OR created_by = ? OR (visibility = ? AND id IN (SELECT poll_id FROM poll_participants WHERE user_id = ?))",
		models.PollVisibilityPublic, userID, models.PollVisibilityInviteOnly, userID,
	)
}

// applyFilters applies filters to the query
func (r *pollRepository) applyFilters(query *gorm.DB, filter *models.PollFilterRequest) *gorm.DB {
	if filter == nil {
		return query
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}

	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}

	if filter.Visibility != "" {
		query = query.Where("visibility = ?", filter.Visibility)
	}

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}

	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}

	if filter.DepartmentID != nil {
		query = query.Where("department_id = ?", *filter.DepartmentID)
	}

	if filter.StartDateFrom != nil {
		query = query.Where("start_time >= ?", *filter.StartDateFrom)
	}

	if filter.StartDateTo != nil {
		query = query.Where("start_time <= ?", *filter.StartDateTo)
	}

	if filter.EndDateFrom != nil {
		query = query.Where("end_time >= ?", *filter.EndDateFrom)
	}

	if filter.EndDateTo != nil {
		query = query.Where("end_time <= ?", *filter.EndDateTo)
	}

	return query
}

// applySortingAndPagination applies sorting and pagination to the query
func (r *pollRepository) applySortingAndPagination(query *gorm.DB, filter *models.PollFilterRequest) *gorm.DB {
	if filter == nil {
		return query.Order("created_at DESC").Limit(models.DefaultLimit)
	}

	// Apply sorting
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	query = query.Order(sortBy + " " + sortOrder)

	// Apply pagination
	limit := filter.Limit
	if limit <= 0 {
		limit = models.DefaultLimit
	}
	if limit > models.MaxLimit {
		limit = models.MaxLimit
	}

	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	return query.Limit(limit).Offset(offset)
}

// loadPollStatistics loads computed statistics for polls
func (r *pollRepository) loadPollStatistics(polls []*models.Poll, userID uint) {
	if len(polls) == 0 {
		return
	}

	pollIDs := make([]uint, len(polls))
	for i, poll := range polls {
		pollIDs[i] = poll.ID
	}

	// Load vote counts
	type voteCount struct {
		PollID uint
		Count  int64
	}

	var voteCounts []voteCount
	r.db.Model(&models.PollVote{}).
		Select("poll_id, COUNT(*) as count").
		Where("poll_id IN ?", pollIDs).
		Group("poll_id").
		Scan(&voteCounts)

	// Load voter counts
	type voterCount struct {
		PollID uint
		Count  int64
	}

	var voterCounts []voterCount
	r.db.Model(&models.PollVote{}).
		Select("poll_id, COUNT(DISTINCT COALESCE(user_id, id)) as count").
		Where("poll_id IN ?", pollIDs).
		Group("poll_id").
		Scan(&voterCounts)

	// Check if user has voted
	type userVote struct {
		PollID uint
	}

	var userVotes []userVote
	r.db.Model(&models.PollVote{}).
		Select("DISTINCT poll_id").
		Where("poll_id IN ? AND user_id = ?", pollIDs, userID).
		Scan(&userVotes)

	// Create maps for quick lookup
	voteCountMap := make(map[uint]int64)
	for _, vc := range voteCounts {
		voteCountMap[vc.PollID] = vc.Count
	}

	voterCountMap := make(map[uint]int64)
	for _, vc := range voterCounts {
		voterCountMap[vc.PollID] = vc.Count
	}

	userVoteMap := make(map[uint]bool)
	for _, uv := range userVotes {
		userVoteMap[uv.PollID] = true
	}

	// Apply computed fields to polls
	for _, poll := range polls {
		poll.TotalVotes = int(voteCountMap[poll.ID])
		poll.TotalVoters = int(voterCountMap[poll.ID])
		poll.UserHasVoted = userVoteMap[poll.ID]

		// Calculate participation rate (simplified)
		if poll.Visibility == models.PollVisibilityInviteOnly {
			var participantCount int64
			r.db.Model(&models.PollParticipant{}).Where("poll_id = ?", poll.ID).Count(&participantCount)
			if participantCount > 0 {
				poll.ParticipantRate = models.CalculateParticipantRate(poll.TotalVoters, int(participantCount))
			}
		}
	}
}
