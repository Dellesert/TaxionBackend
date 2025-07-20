package repository

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// TaskRepository defines the interface for task data operations
type TaskRepository interface {
	Create(task *models.Task) error
	GetByID(id uint) (*models.Task, error)
	Update(task *models.Task) error
	Delete(id uint) error
	GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTasksByAssignee(assigneeID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTasksByCreator(creatorID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTaskStats(userID uint) (*models.TaskStatsResponse, error)
	Count() (int64, error)
	GetOverdueTasks(userID *uint) ([]*models.Task, error)
	GetTasksWithComments(taskIDs []uint) ([]*models.Task, error)
}

// TaskCommentRepository defines the interface for task comment data operations
type TaskCommentRepository interface {
	Create(comment *models.TaskComment) error
	GetByID(id uint) (*models.TaskComment, error)
	GetByTaskID(taskID uint) ([]*models.TaskComment, error)
	Update(comment *models.TaskComment) error
	Delete(id uint) error
	GetCommentsWithReplies(taskID uint) ([]*models.TaskComment, error)
}

// taskRepository implements TaskRepository interface
type taskRepository struct {
	db *database.DB
}

// taskCommentRepository implements TaskCommentRepository interface
type taskCommentRepository struct {
	db *database.DB
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *database.DB) TaskRepository {
	return &taskRepository{
		db: db,
	}
}

// Task Repository Methods

// Create creates a new task
func (r *taskRepository) Create(task *models.Task) error {
	if err := r.db.Create(task).Error; err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

// GetByID retrieves a task by ID
func (r *taskRepository) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	err := r.db.First(&task, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Load comment count
	var commentCount int64
	r.db.Model(&models.TaskComment{}).Where("task_id = ?", id).Count(&commentCount)
	task.CommentCount = int(commentCount)

	return &task, nil
}

// Update updates an existing task
func (r *taskRepository) Update(task *models.Task) error {
	result := r.db.Save(task)
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// Delete soft deletes a task by ID
func (r *taskRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Task{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}

// GetUserTasks retrieves tasks for a user (either assigned to or created by)
func (r *taskRepository) GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	query := r.db.Model(&models.Task{}).Where("assigned_to = ? OR created_by = ?", userID, userID)

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count user tasks: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var tasks []*models.Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get user tasks: %w", err)
	}

	// Load comment counts
	r.loadCommentCounts(tasks)

	return tasks, total, nil
}

// GetTasksByAssignee retrieves tasks assigned to a specific user
func (r *taskRepository) GetTasksByAssignee(assigneeID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	query := r.db.Model(&models.Task{}).Where("assigned_to = ?", assigneeID)

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count assignee tasks: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var tasks []*models.Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get assignee tasks: %w", err)
	}

	// Load comment counts
	r.loadCommentCounts(tasks)

	return tasks, total, nil
}

// GetTasksByCreator retrieves tasks created by a specific user
func (r *taskRepository) GetTasksByCreator(creatorID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	query := r.db.Model(&models.Task{}).Where("created_by = ?", creatorID)

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count creator tasks: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var tasks []*models.Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get creator tasks: %w", err)
	}

	// Load comment counts
	r.loadCommentCounts(tasks)

	return tasks, total, nil
}

// GetTaskStats retrieves task statistics for a user
func (r *taskRepository) GetTaskStats(userID uint) (*models.TaskStatsResponse, error) {
	stats := &models.TaskStatsResponse{}

	// Base query for user's tasks (assigned to or created by)
	baseQuery := "assigned_to = ? OR created_by = ?"

	// Total tasks
	var totalCount int64
	if err := r.db.Model(&models.Task{}).Where(baseQuery, userID, userID).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total tasks: %w", err)
	}
	stats.TotalTasks = int(totalCount)

	// Tasks by status
	statusCounts := []struct {
		Status models.TaskStatus
		Count  *int
	}{
		{models.TaskStatusNew, &stats.NewTasks},
		{models.TaskStatusInProgress, &stats.InProgressTasks},
		{models.TaskStatusReview, &stats.ReviewTasks},
		{models.TaskStatusDone, &stats.DoneTasks},
		{models.TaskStatusCancelled, &stats.CancelledTasks},
	}

	for _, sc := range statusCounts {
		var count int64
		query := r.db.Model(&models.Task{}).Where(baseQuery+" AND status = ?", userID, userID, sc.Status)
		if err := query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count tasks by status %s: %w", sc.Status, err)
		}
		*sc.Count = int(count)
	}

	// Overdue tasks (due date in the past and not done/cancelled)
	var overdueCount int64
	overdueQuery := r.db.Model(&models.Task{}).Where(
		baseQuery+" AND due_date < ? AND status NOT IN (?)",
		userID, userID, time.Now(), []models.TaskStatus{models.TaskStatusDone, models.TaskStatusCancelled},
	)
	if err := overdueQuery.Count(&overdueCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count overdue tasks: %w", err)
	}
	stats.OverdueTasks = int(overdueCount)

	// Tasks assigned to me
	var assignedCount int64
	if err := r.db.Model(&models.Task{}).Where("assigned_to = ?", userID).Count(&assignedCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count assigned tasks: %w", err)
	}
	stats.TasksAssignedToMe = int(assignedCount)

	// Tasks created by me
	var createdCount int64
	if err := r.db.Model(&models.Task{}).Where("created_by = ?", userID).Count(&createdCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count created tasks: %w", err)
	}
	stats.TasksCreatedByMe = int(createdCount)

	return stats, nil
}

// Count returns the total number of tasks
func (r *taskRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count tasks: %w", err)
	}
	return count, nil
}

// GetOverdueTasks retrieves tasks that are overdue
func (r *taskRepository) GetOverdueTasks(userID *uint) ([]*models.Task, error) {
	query := r.db.Model(&models.Task{}).Where(
		"due_date < ? AND status NOT IN (?)",
		time.Now(), []models.TaskStatus{models.TaskStatusDone, models.TaskStatusCancelled},
	)

	// If userID is provided, filter by user's tasks
	if userID != nil {
		query = query.Where("assigned_to = ? OR created_by = ?", *userID, *userID)
	}

	var tasks []*models.Task
	if err := query.Order("due_date ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get overdue tasks: %w", err)
	}

	// Load comment counts
	r.loadCommentCounts(tasks)

	return tasks, nil
}

// GetTasksWithComments retrieves tasks with their comments preloaded
func (r *taskRepository) GetTasksWithComments(taskIDs []uint) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.Preload("Comments").Where("id IN ?", taskIDs).Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks with comments: %w", err)
	}

	// Load comment counts
	r.loadCommentCounts(tasks)

	return tasks, nil
}

// Helper methods

// applyFilters applies filtering conditions to the query
func (r *taskRepository) applyFilters(query *gorm.DB, filter *models.TaskFilterRequest) *gorm.DB {
	if filter == nil {
		return query
	}

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}

	if filter.Priority != nil {
		query = query.Where("priority = ?", *filter.Priority)
	}

	if filter.AssignedTo != nil {
		query = query.Where("assigned_to = ?", *filter.AssignedTo)
	}

	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}

	if filter.DueBefore != nil {
		query = query.Where("due_date < ?", *filter.DueBefore)
	}

	if filter.DueAfter != nil {
		query = query.Where("due_date > ?", *filter.DueAfter)
	}

	return query
}

// applySortingAndPagination applies sorting and pagination to the query
func (r *taskRepository) applySortingAndPagination(query *gorm.DB, filter *models.TaskFilterRequest) *gorm.DB {
	if filter == nil {
		return query.Order("created_at DESC").Limit(20)
	}

	// Apply sorting
	sortBy := "created_at"
	sortOrder := "DESC"

	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}

	if filter.SortOrder != "" {
		sortOrder = filter.SortOrder
	}

	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	limit := 20
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	return query.Limit(limit).Offset(offset)
}

// loadCommentCounts loads comment counts for tasks
func (r *taskRepository) loadCommentCounts(tasks []*models.Task) {
	if len(tasks) == 0 {
		return
	}

	taskIDs := make([]uint, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	// Get comment counts for all tasks in one query
	type commentCount struct {
		TaskID uint
		Count  int
	}

	var counts []commentCount
	r.db.Model(&models.TaskComment{}).
		Select("task_id, COUNT(*) as count").
		Where("task_id IN ?", taskIDs).
		Group("task_id").
		Scan(&counts)

	// Map counts to tasks
	countMap := make(map[uint]int)
	for _, count := range counts {
		countMap[count.TaskID] = count.Count
	}

	for _, task := range tasks {
		task.CommentCount = countMap[task.ID]
	}
}
