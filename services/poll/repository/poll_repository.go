// File: services/poll/repository/poll_repository.go
package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	sharedModels "tachyon-messenger/shared/models"

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
	GetPolls(userID uint, filter *models.PollFilterRequest, userRole sharedModels.Role) ([]*models.Poll, int64, error)
	SearchPolls(userID uint, query string, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetPollStats(userID uint) (*models.PollStatsResponse, error)
	GetUserPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetParticipatedPolls(userID uint, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetPollsByStatus(status models.PollStatus, filter *models.PollFilterRequest) ([]*models.Poll, int64, error)
	GetExpiredPolls() ([]*models.Poll, error)
	GetExpiringPolls(expiryTime time.Time) ([]*models.Poll, error)
	UpdateStatus(id uint, status models.PollStatus) error
	BatchUpdateStatus(ids []uint, status models.PollStatus) (int64, error)
	Count() (int64, error)
	CountByCreator(userID uint) (int64, error)
	CountByStatus(status models.PollStatus) (int64, error)

	// Sync methods
	GetDeletedPollIDsSince(since time.Time) ([]uint, error)
	RecordDeletion(pollID uint, deletedBy *uint) error

	// Dashboard methods
	GetPendingPollsForUser(userID uint, limit int) ([]*models.Poll, int64, error)
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
	}).Preload("Creator").First(&poll, id).Error
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
		Preload("Creator").
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
// Polls are sorted with user_has_voted=false first (unvoted polls)
func (r *pollRepository) GetPolls(userID uint, filter *models.PollFilterRequest, userRole sharedModels.Role) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Creator")

	// Apply visibility filter with draft filtering
	query = r.applyVisibilityFilterWithDrafts(query, userID, userRole)

	// Apply other filters
	query = r.applyFilters(query, filter)

	// Apply updated_since filter for incremental sync
	if filter != nil && filter.UpdatedSince != nil {
		query = query.Where("polls.updated_at > ?", *filter.UpdatedSince)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count polls: %w", err)
	}

	// Apply sorting and pagination with user voting priority
	query = r.applySortingAndPaginationWithVotePriority(query, filter, userID)

	// Debug: Log SQL with Session DryRun
	stmt := query.Session(&gorm.Session{DryRun: true}).Find(&[]models.Poll{}).Statement
	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"sql":     stmt.SQL.String(),
		"vars":    stmt.Vars,
	}).Info("GetPolls SQL query")

	var polls []*models.Poll
	if err := query.Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get polls: %w", err)
	}

	// Debug logging
	logger.WithFields(map[string]interface{}{
		"user_id":     userID,
		"user_role":   userRole,
		"total_polls": total,
		"found_polls": len(polls),
	}).Info("GetPolls result")

	// Load computed fields
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}

// SearchPolls searches polls by title and description
// Polls are sorted with user_has_voted=false first (unvoted polls)
func (r *pollRepository) SearchPolls(userID uint, searchQuery string, filter *models.PollFilterRequest) ([]*models.Poll, int64, error) {
	query := r.db.Model(&models.Poll{}).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Creator")

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

	// Apply sorting and pagination with user voting priority
	query = r.applySortingAndPaginationWithVotePriority(query, filter, userID)

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
		}).
		Preload("Creator")

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
		}).
		Preload("Creator")

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
		}).
		Preload("Creator")

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
// Uses indexed query and limits results for performance
func (r *pollRepository) GetExpiredPolls() ([]*models.Poll, error) {
	var polls []*models.Poll
	now := time.Now()

	// Optimized query:
	// 1. Uses index on (status, end_time)
	// 2. Only selects ID (minimal data)
	// 3. Limits to 100 polls per batch to avoid heavy load
	err := r.db.
		Select("id").
		Where("status = ? AND end_time IS NOT NULL AND end_time < ?",
			models.PollStatusActive, now).
		Limit(100).
		Find(&polls).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get expired polls: %w", err)
	}

	return polls, nil
}

// GetExpiringPolls returns active polls expiring before the given time
func (r *pollRepository) GetExpiringPolls(expiryTime time.Time) ([]*models.Poll, error) {
	var polls []*models.Poll
	now := time.Now()

	// Get active polls that:
	// 1. Have an end_time
	// 2. End time is between now and expiryTime
	// 3. Are currently active
	err := r.db.
		Where("status = ? AND end_time IS NOT NULL AND end_time > ? AND end_time <= ?",
			models.PollStatusActive, now, expiryTime).
		Limit(100).
		Find(&polls).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get expiring polls: %w", err)
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

// BatchUpdateStatus updates status for multiple polls in a single query
// Returns the number of updated rows
func (r *pollRepository) BatchUpdateStatus(ids []uint, status models.PollStatus) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	result := r.db.Model(&models.Poll{}).
		Where("id IN ?", ids).
		Update("status", status)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to batch update poll status: %w", result.Error)
	}

	return result.RowsAffected, nil
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
	// Get user's department ID
	var result struct {
		DepartmentID *uint
	}

	if err := r.db.Table("users").Select("department_id").Where("id = ?", userID).First(&result).Error; err == nil && result.DepartmentID != nil {
		// User has a department - show:
		// 1. All public polls (regardless of department_id)
		// 2. Polls created by this user
		// 3. Invite-only polls where user is a participant
		// 4. Department polls for user's department
		return query.Where(
			"visibility = ? OR created_by = ? OR (visibility = ? AND id IN (SELECT poll_id FROM poll_participants WHERE user_id = ?)) OR (visibility = ? AND department_id = ?)",
			models.PollVisibilityPublic, userID, models.PollVisibilityInviteOnly, userID, models.PollVisibilityDepartment, *result.DepartmentID,
		)
	}

	// User has no department - show:
	// 1. All public polls
	// 2. Polls created by this user
	// 3. Invite-only polls where user is a participant
	return query.Where(
		"visibility = ? OR created_by = ? OR (visibility = ? AND id IN (SELECT poll_id FROM poll_participants WHERE user_id = ?))",
		models.PollVisibilityPublic, userID, models.PollVisibilityInviteOnly, userID,
	)
}

// applyVisibilityFilterWithDrafts applies visibility filtering with draft poll filtering
// System administrators (admin, super_admin) can see ALL polls without restrictions
// Other users see polls based on visibility rules and draft restrictions
func (r *pollRepository) applyVisibilityFilterWithDrafts(query *gorm.DB, userID uint, userRole sharedModels.Role) *gorm.DB {
	isSuperAdmin := userRole == sharedModels.RoleSuperAdmin

	// Only super administrators can see ALL polls from all departments and all users
	if isSuperAdmin {
		// No filters applied for super admins - they see everything
		return query
	}

	// For non-super-admins (including regular admins), apply regular visibility filter
	query = r.applyVisibilityFilter(query, userID)

	// For non-super-admins, exclude draft polls they didn't create
	query = query.Where("status != ? OR created_by = ?", models.PollStatusDraft, userID)

	return query
}

// applyFilters applies filters to the query
func (r *pollRepository) applyFilters(query *gorm.DB, filter *models.PollFilterRequest) *gorm.DB {
	if filter == nil {
		return query
	}

	if len(filter.Status) > 0 {
		query = query.Where("status IN ?", filter.Status)
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

	// Text search in title and description (case-insensitive)
	if filter.Search != "" {
		searchTerm := strings.TrimSpace(filter.Search)

		// For Unicode (Cyrillic) support, we need to search both lowercase and original case
		// because LOWER() doesn't work with locale 'C' in PostgreSQL
		lowerPattern := "%" + strings.ToLower(searchTerm) + "%"
		upperPattern := "%" + strings.ToUpper(searchTerm) + "%"
		titlePattern := "%" + strings.Title(strings.ToLower(searchTerm)) + "%"

		query = query.Where(
			"title LIKE ? OR title LIKE ? OR title LIKE ? OR description LIKE ? OR description LIKE ? OR description LIKE ?",
			lowerPattern, upperPattern, titlePattern, lowerPattern, upperPattern, titlePattern,
		)
	}

	return query
}

// applySortingAndPagination applies sorting and pagination to the query
func (r *pollRepository) applySortingAndPagination(query *gorm.DB, filter *models.PollFilterRequest) *gorm.DB {
	if filter == nil {
		// Add secondary sort by id for deterministic ordering
		return query.Order("created_at DESC").Order("id DESC").Limit(models.DefaultLimit)
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

	// Primary sort
	query = query.Order(sortBy + " " + sortOrder)

	// Always add secondary sort by id to ensure deterministic ordering for pagination
	// This prevents duplicates when primary sort field has same values
	query = query.Order("id " + sortOrder)

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

	// Debug logging
	logger.WithFields(map[string]interface{}{
		"limit":  limit,
		"offset": offset,
	}).Info("Applying pagination")

	return query.Limit(limit).Offset(offset)
}

// applySortingAndPaginationWithVotePriority applies sorting with priority for unvoted polls
// Unvoted polls appear first, then voted polls
func (r *pollRepository) applySortingAndPaginationWithVotePriority(query *gorm.DB, filter *models.PollFilterRequest, userID uint) *gorm.DB {
	if filter == nil {
		filter = &models.PollFilterRequest{
			Limit:     models.DefaultLimit,
			Offset:    0,
			SortBy:    "created_at",
			SortOrder: "desc",
		}
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

	// Primary sort: unvoted polls first (0), then voted polls (1)
	// Use raw SQL for the EXISTS subquery in ORDER BY
	// Note: We need to format the SQL directly since gorm.Expr with parameters doesn't always work in ORDER BY
	votePrioritySQL := fmt.Sprintf(
		"CASE WHEN EXISTS (SELECT 1 FROM poll_votes WHERE poll_votes.poll_id = polls.id AND poll_votes.user_id = %d) THEN 1 ELSE 0 END",
		userID,
	)
	query = query.Order(votePrioritySQL)

	// Secondary sort: by user's chosen field
	query = query.Order(sortBy + " " + sortOrder)

	// Tertiary sort: by id for deterministic ordering
	query = query.Order("id " + sortOrder)

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

	// Debug logging
	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"limit":   limit,
		"offset":  offset,
		"sort_by": sortBy,
		"order":   sortOrder,
	}).Info("Applying pagination with vote priority")

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
		Select("poll_id, COUNT(DISTINCT user_id) as count").
		Where("poll_id IN ? AND user_id IS NOT NULL", pollIDs).
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

	// Load department names for polls with department visibility
	type departmentInfo struct {
		PollID         uint
		DepartmentName string
	}

	var departmentInfos []departmentInfo
	r.db.Table("polls").
		Select("polls.id as poll_id, departments.name as department_name").
		Joins("LEFT JOIN departments ON departments.id = polls.department_id").
		Where("polls.id IN ? AND polls.department_id IS NOT NULL", pollIDs).
		Scan(&departmentInfos)

	departmentMap := make(map[uint]string)
	for _, di := range departmentInfos {
		departmentMap[di.PollID] = di.DepartmentName
	}

	logger.WithFields(map[string]interface{}{
		"department_map": departmentMap,
	}).Info("Loaded department names")

	// Apply computed fields to polls
	pollDebugInfo := make([]map[string]interface{}, 0, len(polls))
	for _, poll := range polls {
		poll.TotalVotes = int(voteCountMap[poll.ID])
		poll.TotalVoters = int(voterCountMap[poll.ID])
		poll.UserHasVoted = userVoteMap[poll.ID]
		poll.DepartmentName = departmentMap[poll.ID]

		// Debug info for each poll
		pollDebugInfo = append(pollDebugInfo, map[string]interface{}{
			"poll_id":        poll.ID,
			"title":          poll.Title,
			"user_has_voted": poll.UserHasVoted,
			"total_votes":    poll.TotalVotes,
			"visibility":     poll.Visibility,
		})

		if poll.DepartmentID != nil {
			logger.WithFields(map[string]interface{}{
				"poll_id":         poll.ID,
				"department_id":   *poll.DepartmentID,
				"department_name": poll.DepartmentName,
			}).Info("Poll with department")
		}

		// Calculate participation rate (simplified)
		if poll.Visibility == models.PollVisibilityInviteOnly {
			var participantCount int64
			r.db.Model(&models.PollParticipant{}).Where("poll_id = ?", poll.ID).Count(&participantCount)
			if participantCount > 0 {
				poll.ParticipantRate = models.CalculateParticipantRate(poll.TotalVoters, int(participantCount))
			}
		}
	}

	// Debug: Log poll order and voting status
	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"polls":   pollDebugInfo,
	}).Info("Poll statistics loaded")
}

// GetDeletedPollIDsSince returns IDs of deleted polls since the given timestamp
func (r *pollRepository) GetDeletedPollIDsSince(since time.Time) ([]uint, error) {
	var records []sharedModels.DeletedRecord
	err := r.db.
		Where("entity_type = ? AND deleted_at > ?", database.EntityTypePoll, since).
		Select("entity_id").
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	ids := make([]uint, len(records))
	for i, record := range records {
		ids[i] = record.EntityID
	}
	return ids, nil
}

// RecordDeletion records a deleted poll for sync tracking
func (r *pollRepository) RecordDeletion(pollID uint, deletedBy *uint) error {
	record := sharedModels.DeletedRecord{
		EntityType: database.EntityTypePoll,
		EntityID:   pollID,
		DeletedAt:  time.Now(),
		DeletedBy:  deletedBy,
	}
	return r.db.Create(&record).Error
}

// GetPendingPollsForUser retrieves active polls where user hasn't voted yet (excluding polls created by the user)
// Query: WHERE status = 'active' AND created_by != current_user AND NOT EXISTS (vote where user_id = current_user) ORDER BY created_at DESC LIMIT n
func (r *pollRepository) GetPendingPollsForUser(userID uint, limit int) ([]*models.Poll, int64, error) {
	var polls []*models.Poll
	var total int64

	// Base query: active polls where user hasn't voted and user is not the creator
	baseCondition := "status = ? AND created_by != ? AND NOT EXISTS (SELECT 1 FROM poll_votes WHERE poll_votes.poll_id = polls.id AND poll_votes.user_id = ?)"

	// Count total
	countQuery := r.db.Model(&models.Poll{}).
		Where(baseCondition, models.PollStatusActive, userID, userID)
	countQuery = r.applyVisibilityFilter(countQuery, userID)

	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count pending polls: %w", err)
	}

	// Get polls
	query := r.db.
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		Preload("Creator").
		Where(baseCondition, models.PollStatusActive, userID, userID)
	query = r.applyVisibilityFilter(query, userID)

	if err := query.Order("created_at DESC").Limit(limit).Find(&polls).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get pending polls: %w", err)
	}

	// Load statistics
	r.loadPollStatistics(polls, userID)

	return polls, total, nil
}
