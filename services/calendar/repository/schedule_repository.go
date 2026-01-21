package repository

import (
	"errors"
	"time"

	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// ScheduleRepository defines the interface for schedule data operations
type ScheduleRepository interface {
	// Schedule CRUD
	CreateSchedule(schedule *models.Schedule) error
	GetScheduleByID(id uint) (*models.Schedule, error)
	GetSchedules(filter ScheduleFilter) ([]*models.Schedule, int64, error)
	UpdateSchedule(schedule *models.Schedule) error
	DeleteSchedule(id uint) error
	GetScheduleWithEntries(id uint) (*models.Schedule, error)

	// Schedule Entry CRUD
	CreateScheduleEntry(entry *models.ScheduleEntry) error
	CreateScheduleEntries(entries []*models.ScheduleEntry) error
	GetScheduleEntry(id uint) (*models.ScheduleEntry, error)
	GetScheduleEntries(scheduleID uint, filter EntryFilter) ([]*models.ScheduleEntry, int64, error)
	UpdateScheduleEntry(entry *models.ScheduleEntry) error
	DeleteScheduleEntry(id uint) error
	GetUserScheduleEntries(userID uint, startDate, endDate time.Time) ([]*models.ScheduleEntry, error)
	CheckScheduleConflict(userID uint, date time.Time, startTime, endTime time.Time, excludeEntryID *uint) (bool, error)

	// Schedule Assignment
	AssignUserToSchedule(assignment *models.ScheduleAssignment) error
	RemoveUserFromSchedule(scheduleID, userID uint) error
	GetScheduleAssignments(scheduleID uint) ([]*models.ScheduleAssignment, error)
	IsUserAssignedToSchedule(scheduleID, userID uint) (bool, error)

	// Schedule Template CRUD
	CreateScheduleTemplate(template *models.ScheduleTemplate) error
	GetScheduleTemplate(id uint) (*models.ScheduleTemplate, error)
	GetScheduleTemplates(filter TemplateFilter) ([]*models.ScheduleTemplate, int64, error)
	UpdateScheduleTemplate(template *models.ScheduleTemplate) error
	DeleteScheduleTemplate(id uint) error
	GetTemplateWithEntries(id uint) (*models.ScheduleTemplate, error)

	// Template Entry CRUD
	CreateTemplateEntry(entry *models.ScheduleTemplateEntry) error
	CreateTemplateEntries(entries []*models.ScheduleTemplateEntry) error
	GetTemplateEntries(templateID uint) ([]*models.ScheduleTemplateEntry, error)
	UpdateTemplateEntry(entry *models.ScheduleTemplateEntry) error
	DeleteTemplateEntry(id uint) error

	// Recurring schedule support
	HasEntriesForMonth(scheduleID uint, year int, month time.Month) (bool, error)
	GetRecurringSchedulesForUser(userID uint) ([]*models.Schedule, error)
	DeleteEntriesForMonth(scheduleID uint, year int, month time.Month) error

	// Schedule type compatibility
	AreScheduleTypesCompatible(type1, type2 models.ScheduleType) (bool, error)
	GetConflictingEntries(userID uint, date time.Time, startTime, endTime time.Time, scheduleType models.ScheduleType, excludeEntryID *uint) ([]*models.ScheduleEntry, error)
}

// ScheduleFilter defines filtering parameters for schedules
type ScheduleFilter struct {
	Type         *models.ScheduleType
	IsActive     *bool
	CreatedBy    *uint
	DepartmentID *uint
	StartDate    *time.Time
	EndDate      *time.Time
	Limit        int
	Offset       int
}

// EntryFilter defines filtering parameters for schedule entries
type EntryFilter struct {
	UserID    *uint
	StartDate *time.Time
	EndDate   *time.Time
	ShiftType *models.ShiftType
	Limit     int
	Offset    int
}

// TemplateFilter defines filtering parameters for templates
type TemplateFilter struct {
	Type         *models.ScheduleType
	IsActive     *bool
	CreatedBy    *uint
	DepartmentID *uint
	Limit        int
	Offset       int
}

// scheduleRepository implements ScheduleRepository interface
type scheduleRepository struct {
	db *database.DB
}

// NewScheduleRepository creates a new schedule repository
func NewScheduleRepository(db *database.DB) ScheduleRepository {
	return &scheduleRepository{
		db: db,
	}
}

// CreateSchedule creates a new schedule
func (r *scheduleRepository) CreateSchedule(schedule *models.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil")
	}

	return r.db.Create(schedule).Error
}

// GetScheduleByID retrieves a schedule by ID
func (r *scheduleRepository) GetScheduleByID(id uint) (*models.Schedule, error) {
	var schedule models.Schedule
	err := r.db.
		Preload("Creator").
		Preload("Template").
		Preload("Template.Entries").
		Preload("Template.Entries.User").
		First(&schedule, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("schedule not found")
		}
		return nil, err
	}
	return &schedule, nil
}

// GetSchedules retrieves schedules with filtering
func (r *scheduleRepository) GetSchedules(filter ScheduleFilter) ([]*models.Schedule, int64, error) {
	var schedules []*models.Schedule
	var total int64

	query := r.db.Model(&models.Schedule{}).Preload("Creator")

	// Apply filters
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}
	if filter.DepartmentID != nil {
		query = query.Where("department_id = ?", *filter.DepartmentID)
	}
	if filter.StartDate != nil {
		query = query.Where("end_date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("start_date <= ?", *filter.EndDate)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by start date descending
	query = query.Order("start_date DESC")

	if err := query.Find(&schedules).Error; err != nil {
		return nil, 0, err
	}

	return schedules, total, nil
}

// UpdateSchedule updates an existing schedule
func (r *scheduleRepository) UpdateSchedule(schedule *models.Schedule) error {
	if schedule == nil {
		return errors.New("schedule cannot be nil")
	}

	return r.db.Save(schedule).Error
}

// DeleteSchedule deletes a schedule
func (r *scheduleRepository) DeleteSchedule(id uint) error {
	result := r.db.Delete(&models.Schedule{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("schedule not found")
	}
	return nil
}

// GetScheduleWithEntries retrieves a schedule with all its entries
func (r *scheduleRepository) GetScheduleWithEntries(id uint) (*models.Schedule, error) {
	var schedule models.Schedule
	err := r.db.
		Preload("Creator").
		Preload("Entries").
		Preload("Entries.User").
		Preload("Assignments").
		Preload("Assignments.User").
		Preload("Template").
		Preload("Template.Entries").
		Preload("Template.Entries.User").
		First(&schedule, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("schedule not found")
		}
		return nil, err
	}
	return &schedule, nil
}

// CreateScheduleEntry creates a new schedule entry
func (r *scheduleRepository) CreateScheduleEntry(entry *models.ScheduleEntry) error {
	if entry == nil {
		return errors.New("schedule entry cannot be nil")
	}

	return r.db.Create(entry).Error
}

// CreateScheduleEntries creates multiple schedule entries in batch
func (r *scheduleRepository) CreateScheduleEntries(entries []*models.ScheduleEntry) error {
	if len(entries) == 0 {
		return nil
	}

	return r.db.Create(&entries).Error
}

// GetScheduleEntry retrieves a schedule entry by ID
func (r *scheduleRepository) GetScheduleEntry(id uint) (*models.ScheduleEntry, error) {
	var entry models.ScheduleEntry
	err := r.db.
		Preload("User").
		Preload("Schedule").
		Preload("Creator").
		First(&entry, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("schedule entry not found")
		}
		return nil, err
	}
	return &entry, nil
}

// GetScheduleEntries retrieves entries for a schedule with filtering
func (r *scheduleRepository) GetScheduleEntries(scheduleID uint, filter EntryFilter) ([]*models.ScheduleEntry, int64, error) {
	var entries []*models.ScheduleEntry
	var total int64

	query := r.db.Model(&models.ScheduleEntry{}).
		Preload("User").
		Where("schedule_id = ?", scheduleID)

	// Apply filters
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.StartDate != nil {
		query = query.Where("date >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("date <= ?", *filter.EndDate)
	}
	if filter.ShiftType != nil {
		query = query.Where("shift_type = ?", *filter.ShiftType)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by date and start time
	query = query.Order("date ASC, start_time ASC")

	if err := query.Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// UpdateScheduleEntry updates an existing schedule entry
func (r *scheduleRepository) UpdateScheduleEntry(entry *models.ScheduleEntry) error {
	if entry == nil {
		return errors.New("schedule entry cannot be nil")
	}

	return r.db.Save(entry).Error
}

// DeleteScheduleEntry deletes a schedule entry
func (r *scheduleRepository) DeleteScheduleEntry(id uint) error {
	result := r.db.Delete(&models.ScheduleEntry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("schedule entry not found")
	}
	return nil
}

// GetUserScheduleEntries retrieves all schedule entries for a user in a date range
func (r *scheduleRepository) GetUserScheduleEntries(userID uint, startDate, endDate time.Time) ([]*models.ScheduleEntry, error) {
	var entries []*models.ScheduleEntry

	err := r.db.
		Preload("Schedule").
		Where("user_id = ? AND date >= ? AND date <= ?", userID, startDate, endDate).
		Order("date ASC, start_time ASC").
		Find(&entries).Error

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// CheckScheduleConflict checks if there is a time conflict for a user
func (r *scheduleRepository) CheckScheduleConflict(userID uint, date time.Time, startTime, endTime time.Time, excludeEntryID *uint) (bool, error) {
	query := r.db.Model(&models.ScheduleEntry{}).
		Where("user_id = ? AND date = ?", userID, date).
		Where("((start_time < ? AND end_time > ?) OR (start_time < ? AND end_time > ?) OR (start_time >= ? AND end_time <= ?))",
			endTime, startTime, endTime, endTime, startTime, endTime)

	if excludeEntryID != nil {
		query = query.Where("id != ?", *excludeEntryID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

// AssignUserToSchedule assigns a user to a schedule
func (r *scheduleRepository) AssignUserToSchedule(assignment *models.ScheduleAssignment) error {
	if assignment == nil {
		return errors.New("assignment cannot be nil")
	}

	return r.db.Create(assignment).Error
}

// RemoveUserFromSchedule removes a user from a schedule
func (r *scheduleRepository) RemoveUserFromSchedule(scheduleID, userID uint) error {
	result := r.db.Where("schedule_id = ? AND user_id = ?", scheduleID, userID).
		Delete(&models.ScheduleAssignment{})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("assignment not found")
	}
	return nil
}

// GetScheduleAssignments retrieves all assignments for a schedule
func (r *scheduleRepository) GetScheduleAssignments(scheduleID uint) ([]*models.ScheduleAssignment, error) {
	var assignments []*models.ScheduleAssignment

	err := r.db.
		Preload("User").
		Preload("Assigner").
		Where("schedule_id = ?", scheduleID).
		Find(&assignments).Error

	if err != nil {
		return nil, err
	}

	return assignments, nil
}

// IsUserAssignedToSchedule checks if a user is assigned to a schedule
func (r *scheduleRepository) IsUserAssignedToSchedule(scheduleID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ScheduleAssignment{}).
		Where("schedule_id = ? AND user_id = ?", scheduleID, userID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// CreateScheduleTemplate creates a new schedule template
func (r *scheduleRepository) CreateScheduleTemplate(template *models.ScheduleTemplate) error {
	if template == nil {
		return errors.New("template cannot be nil")
	}

	return r.db.Create(template).Error
}

// GetScheduleTemplate retrieves a template by ID
func (r *scheduleRepository) GetScheduleTemplate(id uint) (*models.ScheduleTemplate, error) {
	var template models.ScheduleTemplate
	err := r.db.Preload("Creator").First(&template, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, err
	}
	return &template, nil
}

// GetScheduleTemplates retrieves templates with filtering
func (r *scheduleRepository) GetScheduleTemplates(filter TemplateFilter) ([]*models.ScheduleTemplate, int64, error) {
	var templates []*models.ScheduleTemplate
	var total int64

	query := r.db.Model(&models.ScheduleTemplate{}).Preload("Creator")

	// Apply filters
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}
	if filter.IsActive != nil {
		query = query.Where("is_active = ?", *filter.IsActive)
	}
	if filter.CreatedBy != nil {
		query = query.Where("created_by = ?", *filter.CreatedBy)
	}
	if filter.DepartmentID != nil {
		query = query.Where("department_id = ?", *filter.DepartmentID)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by created_at descending
	query = query.Order("created_at DESC")

	if err := query.Find(&templates).Error; err != nil {
		return nil, 0, err
	}

	return templates, total, nil
}

// UpdateScheduleTemplate updates an existing template
func (r *scheduleRepository) UpdateScheduleTemplate(template *models.ScheduleTemplate) error {
	if template == nil {
		return errors.New("template cannot be nil")
	}

	return r.db.Save(template).Error
}

// DeleteScheduleTemplate deletes a template
func (r *scheduleRepository) DeleteScheduleTemplate(id uint) error {
	result := r.db.Delete(&models.ScheduleTemplate{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("template not found")
	}
	return nil
}

// GetTemplateWithEntries retrieves a template with all its entries
func (r *scheduleRepository) GetTemplateWithEntries(id uint) (*models.ScheduleTemplate, error) {
	var template models.ScheduleTemplate
	err := r.db.
		Preload("Creator").
		Preload("Entries").
		Preload("Entries.User").
		First(&template, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, err
	}
	return &template, nil
}

// CreateTemplateEntry creates a new template entry
func (r *scheduleRepository) CreateTemplateEntry(entry *models.ScheduleTemplateEntry) error {
	if entry == nil {
		return errors.New("template entry cannot be nil")
	}

	return r.db.Create(entry).Error
}

// CreateTemplateEntries creates multiple template entries in batch
func (r *scheduleRepository) CreateTemplateEntries(entries []*models.ScheduleTemplateEntry) error {
	if len(entries) == 0 {
		return nil
	}

	return r.db.Create(&entries).Error
}

// GetTemplateEntries retrieves all entries for a template
func (r *scheduleRepository) GetTemplateEntries(templateID uint) ([]*models.ScheduleTemplateEntry, error) {
	var entries []*models.ScheduleTemplateEntry

	err := r.db.
		Preload("User").
		Where("template_id = ?", templateID).
		Order("day_of_week ASC, start_time ASC").
		Find(&entries).Error

	if err != nil {
		return nil, err
	}

	return entries, nil
}

// UpdateTemplateEntry updates an existing template entry
func (r *scheduleRepository) UpdateTemplateEntry(entry *models.ScheduleTemplateEntry) error {
	if entry == nil {
		return errors.New("template entry cannot be nil")
	}

	return r.db.Save(entry).Error
}

// DeleteTemplateEntry deletes a template entry
func (r *scheduleRepository) DeleteTemplateEntry(id uint) error {
	result := r.db.Delete(&models.ScheduleTemplateEntry{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("template entry not found")
	}
	return nil
}

// HasEntriesForMonth checks if a schedule has entries for a specific month
func (r *scheduleRepository) HasEntriesForMonth(scheduleID uint, year int, month time.Month) (bool, error) {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	var count int64
	err := r.db.Model(&models.ScheduleEntry{}).
		Where("schedule_id = ? AND date >= ? AND date <= ?", scheduleID, startOfMonth, endOfMonth).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRecurringSchedulesForUser retrieves all recurring schedules where user is assigned
func (r *scheduleRepository) GetRecurringSchedulesForUser(userID uint) ([]*models.Schedule, error) {
	var schedules []*models.Schedule

	err := r.db.
		Preload("Template").
		Preload("Template.Entries").
		Joins("JOIN schedule_assignments ON schedule_assignments.schedule_id = schedules.id").
		Where("schedule_assignments.user_id = ? AND schedules.mode = ? AND schedules.is_active = ?",
			userID, models.ScheduleModeRecurring, true).
		Find(&schedules).Error

	if err != nil {
		return nil, err
	}

	return schedules, nil
}

// DeleteEntriesForMonth deletes all entries for a schedule in a specific month
func (r *scheduleRepository) DeleteEntriesForMonth(scheduleID uint, year int, month time.Month) error {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	endOfMonth := startOfMonth.AddDate(0, 1, -1)

	return r.db.Where("schedule_id = ? AND date >= ? AND date <= ?", scheduleID, startOfMonth, endOfMonth).
		Delete(&models.ScheduleEntry{}).Error
}

// AreScheduleTypesCompatible checks if two schedule types are compatible
func (r *scheduleRepository) AreScheduleTypesCompatible(type1, type2 models.ScheduleType) (bool, error) {
	// Same type is always compatible with itself for the same schedule
	// but not compatible for different schedules of the same type
	if type1 == type2 {
		return false, nil
	}

	var count int64
	err := r.db.Model(&models.ScheduleTypeCompatibility{}).
		Where("schedule_type = ? AND compatible_with = ?", type1, type2).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetConflictingEntries returns schedule entries that conflict by time and are not compatible by type
func (r *scheduleRepository) GetConflictingEntries(userID uint, date time.Time, startTime, endTime time.Time, scheduleType models.ScheduleType, excludeEntryID *uint) ([]*models.ScheduleEntry, error) {
	// Get compatible types for the given schedule type
	var compatibleTypes []models.ScheduleType
	err := r.db.Model(&models.ScheduleTypeCompatibility{}).
		Where("schedule_type = ?", scheduleType).
		Pluck("compatible_with", &compatibleTypes).Error
	if err != nil {
		return nil, err
	}

	// Build query for conflicting entries
	query := r.db.Model(&models.ScheduleEntry{}).
		Preload("Schedule").
		Joins("JOIN schedules ON schedules.id = schedule_entries.schedule_id").
		Where("schedule_entries.user_id = ? AND schedule_entries.date = ?", userID, date).
		Where("((schedule_entries.start_time < ? AND schedule_entries.end_time > ?) OR (schedule_entries.start_time < ? AND schedule_entries.end_time > ?) OR (schedule_entries.start_time >= ? AND schedule_entries.end_time <= ?))",
			endTime, startTime, endTime, endTime, startTime, endTime)

	// Exclude compatible schedule types
	if len(compatibleTypes) > 0 {
		query = query.Where("schedules.type NOT IN ?", compatibleTypes)
	}

	// Exclude entry being updated
	if excludeEntryID != nil {
		query = query.Where("schedule_entries.id != ?", *excludeEntryID)
	}

	var entries []*models.ScheduleEntry
	if err := query.Find(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}
