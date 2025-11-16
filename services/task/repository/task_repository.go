package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/task/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// TaskRepository defines the interface for task data operations
type TaskRepository interface {
	Create(task *models.Task) error
	CreateAssignee(assignee *models.TaskAssignee) error
	DeleteAllAssignees(taskID uint) error
	RemoveAllAssignees(taskID uint) error // Alias for DeleteAllAssignees
	GetByID(id uint) (*models.Task, error)
	GetByIDWithDetails(id uint) (*models.Task, error) // Get task with all relations loaded
	Update(task *models.Task) error
	Delete(id uint) error
	GetAllTasks(filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTasksByAssignee(assigneeID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTasksByCreator(creatorID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error)
	GetTaskStats(userID uint) (*models.TaskStatsResponse, error)
	Count() (int64, error)
	GetOverdueTasks(userID *uint) ([]*models.Task, error)
	GetTasksWithComments(taskIDs []uint) ([]*models.Task, error)

	// Hierarchy methods
	GetSubtasks(parentTaskID uint) ([]*models.Task, error)
	GetParentTask(taskID uint) (*models.Task, error)
	CountSubtasks(parentTaskID uint) (int64, error)
	CountCompletedSubtasks(parentTaskID uint) (int64, error)
	UpdateProgress(taskID uint, progress int) error

	// Internal statistics
	GetTaskStatsInternal(timeRange *TimeRange) (*models.TaskStatsInternalResponse, error)

	// Analytics methods
	GetTasksByDepartment(departmentID *uint, timeRange *TimeRange) (*DepartmentTaskStats, error)
	GetAllDepartmentsStats(timeRange *TimeRange) ([]*DepartmentTaskStats, error)
	GetTopPerformersByDepartment(departmentID *uint, limit int, timeRange *TimeRange) ([]*EmployeePerformance, error)
	GetTopPerformers(limit int, timeRange *TimeRange) ([]*EmployeePerformance, error)
	GetTaskCompletionTrends(timeRange *TimeRange, interval string) ([]*TrendDataPoint, error)
	GetTasksByPriority(timeRange *TimeRange) (*PriorityDistribution, error)
	GetAverageCompletionTime(departmentID *uint, timeRange *TimeRange) (float64, error)
}

// TimeRange represents a time period for analytics
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// DepartmentTaskStats represents task statistics for a department
type DepartmentTaskStats struct {
	DepartmentID     uint    `json:"department_id"`
	DepartmentName   string  `json:"department_name"`
	TotalTasks       int     `json:"total_tasks"`
	CompletedTasks   int     `json:"completed_tasks"`
	InProgressTasks  int     `json:"in_progress_tasks"`
	OverdueTasks     int     `json:"overdue_tasks"`
	CompletionRate   float64 `json:"completion_rate"`
	AvgCompletionTime float64 `json:"avg_completion_time"` // in hours
	EmployeeCount    int     `json:"employee_count"`
}

// EmployeePerformance represents individual employee task performance
type EmployeePerformance struct {
	UserID             uint    `json:"user_id"`
	UserName           string  `json:"user_name"`
	DepartmentID       *uint   `json:"department_id,omitempty"`
	DepartmentName     string  `json:"department_name,omitempty"`
	TasksCreated       int     `json:"tasks_created"`
	TasksCompleted     int     `json:"tasks_completed"`
	TasksInProgress    int     `json:"tasks_in_progress"`
	TasksOverdue       int     `json:"tasks_overdue"`
	CompletionRate     float64 `json:"completion_rate"`
	AvgCompletionTime  float64 `json:"avg_completion_time"` // in hours
	QualityScore       float64 `json:"quality_score"` // percentage of tasks completed on time
}

// TrendDataPoint represents a single point in time for trend analysis
type TrendDataPoint struct {
	Date      string `json:"date"`
	Created   int    `json:"created"`
	Completed int    `json:"completed"`
	Overdue   int    `json:"overdue"`
}

// PriorityDistribution represents task distribution by priority
type PriorityDistribution struct {
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
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

// CreateAssignee creates a new task assignee relationship
func (r *taskRepository) CreateAssignee(assignee *models.TaskAssignee) error {
	if err := r.db.Create(assignee).Error; err != nil {
		return fmt.Errorf("failed to create task assignee: %w", err)
	}
	return nil
}

// DeleteAllAssignees deletes all assignees for a task
func (r *taskRepository) DeleteAllAssignees(taskID uint) error {
	if err := r.db.Where("task_id = ?", taskID).Delete(&models.TaskAssignee{}).Error; err != nil {
		return fmt.Errorf("failed to delete task assignees: %w", err)
	}
	return nil
}

// RemoveAllAssignees is an alias for DeleteAllAssignees
func (r *taskRepository) RemoveAllAssignees(taskID uint) error {
	return r.DeleteAllAssignees(taskID)
}

// GetByID retrieves a task by ID
func (r *taskRepository) GetByID(id uint) (*models.Task, error) {
	var task models.Task
	err := r.db.Preload("Assignees").Preload("ParentTask").First(&task, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// Debug logging for ParentTaskID
	if task.ParentTaskID != nil {
		fmt.Printf("[GetByID] Task ID: %d has ParentTaskID: %d\n", id, *task.ParentTaskID)
	} else {
		fmt.Printf("[GetByID] Task ID: %d has NO ParentTaskID (nil)\n", id)
	}

	// Load comment count
	var commentCount int64
	r.db.Model(&models.TaskComment{}).Where("task_id = ?", id).Count(&commentCount)
	task.CommentCount = int(commentCount)

	// Load subtask count
	var subtaskCount int64
	r.db.Model(&models.Task{}).Where("parent_task_id = ? AND deleted_at IS NULL", id).Count(&subtaskCount)
	task.SubtaskCount = int(subtaskCount)

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

// GetAllTasks retrieves all tasks with filtering (for admins)
func (r *taskRepository) GetAllTasks(filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	// Query all tasks without user restrictions
	// Only show top-level tasks (exclude subtasks) by default
	query := r.db.Model(&models.Task{}).Where("parent_task_id IS NULL")

	// Apply filters
	query = r.applyFilters(query, filter)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count all tasks: %w", err)
	}

	// Apply pagination and sorting
	query = r.applySortingAndPagination(query, filter)

	var tasks []*models.Task
	if err := query.Preload("Assignees").Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get all tasks: %w", err)
	}

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

	return tasks, total, nil
}

// GetUserTasks retrieves tasks for a user (either assigned to or created by)
func (r *taskRepository) GetUserTasks(userID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	// Query to find tasks where user is either:
	// 1. Top-level tasks where user is creator or assignee
	// 2. Subtasks where user is assignee OR user is creator of parent task
	// 3. Tasks in delegation chain (user delegated it or was original assignee)
	query := r.db.Model(&models.Task{}).Where(
		"((parent_task_id IS NULL AND (created_by = ? OR assigned_to = ? OR id IN (SELECT task_id FROM task_assignees WHERE user_id = ? AND deleted_at IS NULL))) OR "+
			"(parent_task_id IS NOT NULL AND (assigned_to = ? OR id IN (SELECT task_id FROM task_assignees WHERE user_id = ? AND deleted_at IS NULL) OR "+
			"parent_task_id IN (SELECT id FROM tasks WHERE created_by = ? AND deleted_at IS NULL))) OR "+
			"(delegated_from_user_id = ? OR original_assignee_id = ?))",
		userID, userID, userID, userID, userID, userID, userID, userID,
	)

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
	if err := query.Preload("Assignees").Find(&tasks).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get user tasks: %w", err)
	}

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

	return tasks, total, nil
}

// GetTasksByAssignee retrieves tasks assigned to a specific user
func (r *taskRepository) GetTasksByAssignee(assigneeID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	// Only show top-level tasks (exclude subtasks)
	query := r.db.Model(&models.Task{}).Where("assigned_to = ? AND parent_task_id IS NULL", assigneeID)

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

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

	return tasks, total, nil
}

// GetTasksByCreator retrieves tasks created by a specific user
func (r *taskRepository) GetTasksByCreator(creatorID uint, filter *models.TaskFilterRequest) ([]*models.Task, int64, error) {
	// Only show top-level tasks (exclude subtasks)
	query := r.db.Model(&models.Task{}).Where("created_by = ? AND parent_task_id IS NULL", creatorID)

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

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

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

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

	return tasks, nil
}

// GetTasksWithComments retrieves tasks with their comments preloaded
func (r *taskRepository) GetTasksWithComments(taskIDs []uint) ([]*models.Task, error) {
	var tasks []*models.Task
	err := r.db.Preload("Comments").Where("id IN ?", taskIDs).Find(&tasks).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get tasks with comments: %w", err)
	}

	// Load comment counts and subtask counts
	r.loadCommentCounts(tasks)
	r.loadSubtaskCounts(tasks)

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
		// Filter by assignee: check both old field (assigned_to) and new table (task_assignees)
		query = query.Where(
			"assigned_to = ? OR id IN (SELECT task_id FROM task_assignees WHERE user_id = ? AND deleted_at IS NULL)",
			*filter.AssignedTo, *filter.AssignedTo,
		)
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

	// Text search in title and description (case-insensitive)
	if filter.Search != "" {
		// Trim whitespace from search query
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

// loadSubtaskCounts loads subtask counts for a batch of tasks
func (r *taskRepository) loadSubtaskCounts(tasks []*models.Task) {
	if len(tasks) == 0 {
		return
	}

	taskIDs := make([]uint, len(tasks))
	for i, task := range tasks {
		taskIDs[i] = task.ID
	}

	// Get subtask counts for all tasks in one query
	type subtaskCount struct {
		ParentTaskID uint
		Count        int
	}

	var counts []subtaskCount
	r.db.Model(&models.Task{}).
		Select("parent_task_id, COUNT(*) as count").
		Where("parent_task_id IN ?", taskIDs).
		Where("deleted_at IS NULL").
		Group("parent_task_id").
		Scan(&counts)

	// Map counts to tasks
	countMap := make(map[uint]int)
	for _, count := range counts {
		countMap[count.ParentTaskID] = count.Count
	}

	for _, task := range tasks {
		task.SubtaskCount = countMap[task.ID]
	}
}

// Hierarchy methods

// GetByIDWithDetails retrieves a task by ID with all relations loaded
func (r *taskRepository) GetByIDWithDetails(id uint) (*models.Task, error) {
	var task models.Task
	err := r.db.
		Preload("Assignees").
		Preload("Subtasks").
		Preload("ParentTask").
		Preload("Activities", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(50)
		}).
		Preload("Attachments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Preload("Checklists.Items", func(db *gorm.DB) *gorm.DB {
			return db.Order("position ASC")
		}).
		First(&task, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task with details: %w", err)
	}

	// Load comment count
	var commentCount int64
	r.db.Model(&models.TaskComment{}).Where("task_id = ?", id).Count(&commentCount)
	task.CommentCount = int(commentCount)

	// Load subtask count
	var subtaskCount int64
	r.db.Model(&models.Task{}).Where("parent_task_id = ?", id).Count(&subtaskCount)
	task.SubtaskCount = int(subtaskCount)

	// Load attachment count
	var attachmentCount int64
	r.db.Model(&models.TaskAttachment{}).Where("task_id = ?", id).Count(&attachmentCount)
	task.AttachmentCount = int(attachmentCount)

	return &task, nil
}

// GetSubtasks retrieves all subtasks for a parent task
func (r *taskRepository) GetSubtasks(parentTaskID uint) ([]*models.Task, error) {
	var subtasks []*models.Task
	err := r.db.
		Preload("Assignees").
		Where("parent_task_id = ?", parentTaskID).
		Order("created_at ASC").
		Find(&subtasks).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get subtasks: %w", err)
	}

	// Load comment counts and subtask counts (in case subtasks have their own subtasks)
	r.loadCommentCounts(subtasks)
	r.loadSubtaskCounts(subtasks)

	return subtasks, nil
}

// GetParentTask retrieves the parent task of a subtask
func (r *taskRepository) GetParentTask(taskID uint) (*models.Task, error) {
	var task models.Task
	err := r.db.First(&task, taskID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	// If task has no parent, return nil
	if task.ParentTaskID == nil {
		return nil, nil
	}

	// Get parent task
	var parentTask models.Task
	err = r.db.
		Preload("Assignees").
		First(&parentTask, *task.ParentTaskID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("parent task not found")
		}
		return nil, fmt.Errorf("failed to get parent task: %w", err)
	}

	return &parentTask, nil
}

// CountSubtasks returns the total number of subtasks for a parent task
func (r *taskRepository) CountSubtasks(parentTaskID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).Where("parent_task_id = ?", parentTaskID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count subtasks: %w", err)
	}
	return count, nil
}

// CountCompletedSubtasks returns the number of completed subtasks for a parent task
func (r *taskRepository) CountCompletedSubtasks(parentTaskID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Task{}).
		Where("parent_task_id = ? AND status IN (?)", parentTaskID, []models.TaskStatus{
			models.TaskStatusDone,
			models.TaskStatusCancelled,
		}).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count completed subtasks: %w", err)
	}
	return count, nil
}

// UpdateProgress updates the progress percentage of a task
func (r *taskRepository) UpdateProgress(taskID uint, progress int) error {
	// Ensure progress is between 0 and 100
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	result := r.db.Model(&models.Task{}).
		Where("id = ?", taskID).
		Update("progress_percentage", progress)

	if result.Error != nil {
		return fmt.Errorf("failed to update task progress: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// GetTaskStatsInternal returns internal task statistics for analytics with optional time range filter
func (r *taskRepository) GetTaskStatsInternal(timeRange *TimeRange) (*models.TaskStatsInternalResponse, error) {
	var stats models.TaskStatsInternalResponse

	var totalTasks, newTasks, inProgressTasks, reviewTasks, completedTasks, cancelledTasks, overdueTasks int64

	// Base query
	baseQuery := r.db.Model(&models.Task{})
	if timeRange != nil {
		baseQuery = baseQuery.Where("created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}

	// Count total tasks
	if err := baseQuery.Session(&gorm.Session{}).Count(&totalTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count total tasks: %w", err)
	}

	// Count tasks by status
	if err := baseQuery.Session(&gorm.Session{}).Where("status = ?", "new").Count(&newTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count new tasks: %w", err)
	}

	if err := baseQuery.Session(&gorm.Session{}).Where("status = ?", "in_progress").Count(&inProgressTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count in progress tasks: %w", err)
	}

	if err := baseQuery.Session(&gorm.Session{}).Where("status = ?", "review").Count(&reviewTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count review tasks: %w", err)
	}

	if err := baseQuery.Session(&gorm.Session{}).Where("status = ?", "done").Count(&completedTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count completed tasks: %w", err)
	}

	if err := baseQuery.Session(&gorm.Session{}).Where("status = ?", "cancelled").Count(&cancelledTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count cancelled tasks: %w", err)
	}

	// Count overdue tasks (not done/cancelled and due date < now)
	overdueQuery := baseQuery.Session(&gorm.Session{}).
		Where("status NOT IN ?", []string{"done", "cancelled"}).
		Where("due_date < ?", time.Now())
	if err := overdueQuery.Count(&overdueTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count overdue tasks: %w", err)
	}

	// Convert int64 to int
	stats.TotalTasks = int(totalTasks)
	stats.NewTasks = int(newTasks)
	stats.InProgressTasks = int(inProgressTasks)
	stats.ReviewTasks = int(reviewTasks)
	stats.CompletedTasks = int(completedTasks)
	stats.CancelledTasks = int(cancelledTasks)
	stats.OverdueTasks = int(overdueTasks)

	return &stats, nil
}

// Analytics methods implementation

// GetTasksByDepartment returns task statistics for a specific department
func (r *taskRepository) GetTasksByDepartment(departmentID *uint, timeRange *TimeRange) (*DepartmentTaskStats, error) {
	stats := &DepartmentTaskStats{}

	if departmentID != nil {
		stats.DepartmentID = *departmentID
	}

	// Base query with JOIN to users table to get department from user
	// Use COALESCE to fallback: assignee department -> creator department -> direct department assignment
	baseQuery := r.db.Table("tasks").
		Joins("LEFT JOIN public.users AS assignee ON tasks.assigned_to_user_id = assignee.id").
		Joins("LEFT JOIN public.users AS creator ON tasks.created_by_user_id = creator.id").
		Where("tasks.deleted_at IS NULL")

	if departmentID != nil {
		baseQuery = baseQuery.Where("COALESCE(assignee.department_id, creator.department_id, tasks.assigned_to_department_id) = ?", *departmentID)
	}

	// Count total tasks created in the period
	totalQuery := baseQuery.Session(&gorm.Session{})
	if timeRange != nil {
		totalQuery = totalQuery.Where("tasks.created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}
	var totalTasks int64
	if err := totalQuery.Count(&totalTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count total tasks: %w", err)
	}
	stats.TotalTasks = int(totalTasks)

	// Count completed tasks created in the period
	completedQuery := baseQuery.Session(&gorm.Session{}).Where("tasks.status = ?", models.TaskStatusDone)
	if timeRange != nil {
		completedQuery = completedQuery.Where("tasks.created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}
	var completedTasks int64
	if err := completedQuery.Count(&completedTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count completed tasks: %w", err)
	}
	stats.CompletedTasks = int(completedTasks)

	// Count in progress tasks created in the period
	inProgressQuery := baseQuery.Session(&gorm.Session{}).Where("tasks.status = ?", models.TaskStatusInProgress)
	if timeRange != nil {
		inProgressQuery = inProgressQuery.Where("tasks.created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}
	var inProgressTasks int64
	if err := inProgressQuery.Count(&inProgressTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count in progress tasks: %w", err)
	}
	stats.InProgressTasks = int(inProgressTasks)

	// Count overdue tasks created in the period
	overdueQuery := baseQuery.Session(&gorm.Session{}).
		Where("tasks.status NOT IN ?", []string{"done", "cancelled"}).
		Where("tasks.due_date < ?", time.Now())
	if timeRange != nil {
		overdueQuery = overdueQuery.Where("tasks.created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}
	var overdueTasks int64
	if err := overdueQuery.Count(&overdueTasks).Error; err != nil {
		return nil, fmt.Errorf("failed to count overdue tasks: %w", err)
	}
	stats.OverdueTasks = int(overdueTasks)

	// Calculate completion rate based on completed tasks in period vs total tasks
	if stats.TotalTasks > 0 {
		stats.CompletionRate = (float64(stats.CompletedTasks) / float64(stats.TotalTasks)) * 100.0
	}

	// Calculate average completion time for tasks completed in the time range
	avgTime, _ := r.GetAverageCompletionTime(departmentID, timeRange)
	stats.AvgCompletionTime = avgTime

	return stats, nil
}

// GetAllDepartmentsStats returns task statistics for all departments
func (r *taskRepository) GetAllDepartmentsStats(timeRange *TimeRange) ([]*DepartmentTaskStats, error) {
	// Get distinct department IDs with names from users table
	// This handles tasks assigned to users (most common case)
	type DeptResult struct {
		DepartmentID   uint
		DepartmentName string
		Count          int
	}

	var deptResults []DeptResult
	query := r.db.Table("tasks").
		Select("COALESCE(assignee.department_id, creator.department_id, tasks.assigned_to_department_id) as department_id, departments.name as department_name, COUNT(*) as count").
		Joins("LEFT JOIN public.users AS assignee ON tasks.assigned_to_user_id = assignee.id").
		Joins("LEFT JOIN public.users AS creator ON tasks.created_by_user_id = creator.id").
		Joins("LEFT JOIN public.departments ON COALESCE(assignee.department_id, creator.department_id, tasks.assigned_to_department_id) = departments.id").
		Where("tasks.deleted_at IS NULL").
		Where("COALESCE(assignee.department_id, creator.department_id, tasks.assigned_to_department_id) IS NOT NULL")

	if err := query.Group("COALESCE(assignee.department_id, creator.department_id, tasks.assigned_to_department_id), departments.name").Scan(&deptResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get departments: %w", err)
	}

	// Get stats for each department
	var allStats []*DepartmentTaskStats
	for _, dept := range deptResults {
		stats, err := r.GetTasksByDepartment(&dept.DepartmentID, timeRange)
		if err != nil {
			continue
		}
		stats.DepartmentName = dept.DepartmentName
		allStats = append(allStats, stats)
	}

	return allStats, nil
}

// GetTopPerformersByDepartment returns top performing employees in a department
func (r *taskRepository) GetTopPerformersByDepartment(departmentID *uint, limit int, timeRange *TimeRange) ([]*EmployeePerformance, error) {
	type PerformanceData struct {
		UserID             uint
		UserName           string
		DepartmentID       *uint
		DepartmentName     string
		TasksCompleted     int
		TasksCreated       int
		TasksInProgress    int
		TasksOverdue       int
		AvgCompletionHours float64
	}

	query := r.db.Table("tasks").
		Select(`
			tasks.assigned_to_user_id as user_id,
			users.name as user_name,
			users.department_id as department_id,
			departments.name as department_name,
			COUNT(CASE WHEN tasks.status = 'done' THEN 1 END) as tasks_completed,
			COUNT(CASE WHEN tasks.created_by_user_id = tasks.assigned_to_user_id THEN 1 END) as tasks_created,
			COUNT(CASE WHEN tasks.status = 'in_progress' THEN 1 END) as tasks_in_progress,
			COUNT(CASE WHEN tasks.status NOT IN ('done', 'cancelled') AND tasks.due_date < NOW() THEN 1 END) as tasks_overdue,
			AVG(CASE WHEN tasks.status = 'done' AND tasks.completed_at IS NOT NULL
				THEN EXTRACT(EPOCH FROM (tasks.completed_at - tasks.created_at)) / 3600
				ELSE NULL END) as avg_completion_hours
		`).
		Joins("LEFT JOIN public.users ON tasks.assigned_to_user_id = users.id").
		Joins("LEFT JOIN public.departments ON users.department_id = departments.id").
		Where("tasks.assigned_to_user_id IS NOT NULL")

	if departmentID != nil {
		query = query.Where("users.department_id = ?", *departmentID)
	}

	if timeRange != nil {
		query = query.Where("tasks.created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}

	var performanceData []PerformanceData
	if err := query.Group("tasks.assigned_to_user_id, users.name, users.department_id, departments.name").
		Order("tasks_completed DESC").
		Limit(limit).
		Scan(&performanceData).Error; err != nil {
		return nil, fmt.Errorf("failed to get top performers: %w", err)
	}

	// Convert to EmployeePerformance
	var performers []*EmployeePerformance
	for _, data := range performanceData {
		totalTasks := data.TasksCompleted + data.TasksInProgress + data.TasksOverdue
		completionRate := 0.0
		if totalTasks > 0 {
			completionRate = (float64(data.TasksCompleted) / float64(totalTasks)) * 100.0
		}

		// Calculate quality score (tasks completed on time)
		qualityScore := 100.0
		if data.TasksCompleted > 0 {
			qualityScore = ((float64(data.TasksCompleted) - float64(data.TasksOverdue)) / float64(data.TasksCompleted)) * 100.0
			if qualityScore < 0 {
				qualityScore = 0
			}
		}

		performers = append(performers, &EmployeePerformance{
			UserID:            data.UserID,
			UserName:          data.UserName,
			DepartmentID:      data.DepartmentID,
			DepartmentName:    data.DepartmentName,
			TasksCreated:      data.TasksCreated,
			TasksCompleted:    data.TasksCompleted,
			TasksInProgress:   data.TasksInProgress,
			TasksOverdue:      data.TasksOverdue,
			CompletionRate:    completionRate,
			AvgCompletionTime: data.AvgCompletionHours,
			QualityScore:      qualityScore,
		})
	}

	return performers, nil
}

// GetTopPerformers returns top performing employees across all departments
func (r *taskRepository) GetTopPerformers(limit int, timeRange *TimeRange) ([]*EmployeePerformance, error) {
	return r.GetTopPerformersByDepartment(nil, limit, timeRange)
}

// GetTaskCompletionTrends returns task completion trends over time
func (r *taskRepository) GetTaskCompletionTrends(timeRange *TimeRange, interval string) ([]*TrendDataPoint, error) {
	if timeRange == nil {
		// Default to last 30 days
		end := time.Now()
		start := end.AddDate(0, 0, -30)
		timeRange = &TimeRange{Start: start, End: end}
	}

	// Determine date truncation based on interval
	dateTrunc := "day"
	switch interval {
	case "hour":
		dateTrunc = "hour"
	case "week":
		dateTrunc = "week"
	case "month":
		dateTrunc = "month"
	default:
		dateTrunc = "day"
	}

	type TrendData struct {
		Date      time.Time
		Created   int
		Completed int
		Overdue   int
	}

	var trends []TrendData
	query := r.db.Model(&models.Task{}).
		Select(fmt.Sprintf(`
			DATE_TRUNC('%s', created_at) as date,
			COUNT(*) as created,
			COUNT(CASE WHEN status = 'done' THEN 1 END) as completed,
			COUNT(CASE WHEN status NOT IN ('done', 'cancelled') AND due_date < NOW() THEN 1 END) as overdue
		`, dateTrunc)).
		Where("created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End).
		Group(fmt.Sprintf("DATE_TRUNC('%s', created_at)", dateTrunc)).
		Order("date ASC")

	if err := query.Scan(&trends).Error; err != nil {
		return nil, fmt.Errorf("failed to get task trends: %w", err)
	}

	// Convert to TrendDataPoint with appropriate date formatting
	var dataPoints []*TrendDataPoint
	for _, trend := range trends {
		var dateStr string
		switch dateTrunc {
		case "hour":
			dateStr = trend.Date.Format("2006-01-02 15:04")
		case "week":
			// For weekly grouping, show the first day of the week
			dateStr = trend.Date.Format("2006-01-02")
		case "month":
			// For monthly grouping, show year-month
			dateStr = trend.Date.Format("2006-01")
		default: // day
			dateStr = trend.Date.Format("2006-01-02")
		}

		dataPoints = append(dataPoints, &TrendDataPoint{
			Date:      dateStr,
			Created:   trend.Created,
			Completed: trend.Completed,
			Overdue:   trend.Overdue,
		})
	}

	return dataPoints, nil
}

// GetTasksByPriority returns task distribution by priority
func (r *taskRepository) GetTasksByPriority(timeRange *TimeRange) (*PriorityDistribution, error) {
	dist := &PriorityDistribution{}

	query := r.db.Model(&models.Task{})
	if timeRange != nil {
		query = query.Where("created_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}

	// Count by priority
	priorities := []struct {
		Priority models.TaskPriority
		Count    *int
	}{
		{models.TaskPriorityLow, &dist.Low},
		{models.TaskPriorityMedium, &dist.Medium},
		{models.TaskPriorityHigh, &dist.High},
		{models.TaskPriorityCritical, &dist.Critical},
	}

	for _, p := range priorities {
		var count int64
		// Clone the query to avoid accumulating WHERE conditions
		priorityQuery := query.Session(&gorm.Session{})
		if err := priorityQuery.Where("priority = ?", p.Priority).Count(&count).Error; err != nil {
			return nil, fmt.Errorf("failed to count tasks by priority %s: %w", p.Priority, err)
		}
		*p.Count = int(count)
	}

	return dist, nil
}

// GetAverageCompletionTime returns average task completion time in hours
func (r *taskRepository) GetAverageCompletionTime(departmentID *uint, timeRange *TimeRange) (float64, error) {
	type AvgResult struct {
		AvgHours float64
	}

	query := r.db.Model(&models.Task{}).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - created_at)) / 3600) as avg_hours").
		Where("status = ? AND completed_at IS NOT NULL", models.TaskStatusDone)

	if departmentID != nil {
		query = query.Where("assigned_to_department_id = ?", *departmentID)
	}

	if timeRange != nil {
		query = query.Where("completed_at BETWEEN ? AND ?", timeRange.Start, timeRange.End)
	}

	var result AvgResult
	if err := query.Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("failed to calculate average completion time: %w", err)
	}

	return result.AvgHours, nil
}
