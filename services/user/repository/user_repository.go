package repository

import (
	"errors"
	"fmt"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/shared/database"

	"gorm.io/gorm"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id uint) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByIDs(ids []uint) ([]*models.User, error)
	GetAll(limit, offset int) ([]*models.User, error)
	Update(user *models.User) error
	UpdateFields(id uint, fields map[string]interface{}) error
	Delete(id uint) error
	Count() (int64, error)
	CountWithFilters(departmentID *uint, isActive *bool) (int64, error)
	CountWithFiltersAdvanced(departmentID *uint, isActive *bool, roles []string, excludeRoles []string, searchQuery string) (int64, error)
	CountByRole(role string) (int64, error)
	CountByRoleAndActive(role string, isActive bool) (int64, error)
	CountByTwoFactorEnabled() (int64, error)
	CountByPasskeyEnabled() (int64, error)
	GetWithDepartment(id uint) (*models.User, error)
	GetAllWithDepartments(limit, offset int) ([]*models.User, error)
	GetAllWithDepartmentsFiltered(limit, offset int, departmentID *uint, isActive *bool) ([]*models.User, error)
	GetAllWithDepartmentsFilteredAdvanced(limit, offset int, departmentID *uint, isActive *bool, roles []string, excludeRoles []string, sortBy string, sortOrder string, searchQuery string) ([]*models.User, error)
	GetUsersByDepartment(departmentID uint) ([]*models.User, error)
	SuperAdminExists() (bool, error)
	UpdateTwoFactorStatus(userID uint, enabled bool) error
	UpdatePasskeyStatus(userID uint, enabled bool) error
	ResetAllOnlineStatuses() (int64, error)
	CleanupDisconnectedStatuses(connectedUserIDs []uint) (int64, error)
}

// DepartmentRepository defines the interface for department data operations
type DepartmentRepository interface {
	Create(department *models.Department) error
	GetByID(id uint) (*models.Department, error)
	GetByName(name string) (*models.Department, error)
	GetByNameIncludingDeleted(name string) (*models.Department, error)
	GetAll() ([]*models.Department, error)
	Update(department *models.Department) error
	Delete(id uint) error
	Restore(id uint) error
}

// SubdepartmentRepository defines the interface for subdepartment data operations
type SubdepartmentRepository interface {
	Create(subdepartment *models.Subdepartment) error
	GetByID(id uint) (*models.Subdepartment, error)
	GetByDepartmentID(departmentID uint) ([]*models.Subdepartment, error)
	GetByNameAndDepartmentIncludingDeleted(name string, departmentID uint) (*models.Subdepartment, error)
	GetAll() ([]*models.Subdepartment, error)
	Update(subdepartment *models.Subdepartment) error
	Delete(id uint) error
	Restore(id uint) error
	GetWithDepartment(id uint) (*models.Subdepartment, error)
}

// userRepository implements UserRepository interface
type userRepository struct {
	db *database.DB
}

// departmentRepository implements DepartmentRepository interface
type departmentRepository struct {
	db *database.DB
}

// subdepartmentRepository implements SubdepartmentRepository interface
type subdepartmentRepository struct {
	db *database.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *database.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

// NewDepartmentRepository creates a new department repository
func NewDepartmentRepository(db *database.DB) DepartmentRepository {
	return &departmentRepository{
		db: db,
	}
}

// NewSubdepartmentRepository creates a new subdepartment repository
func NewSubdepartmentRepository(db *database.DB) SubdepartmentRepository {
	return &subdepartmentRepository{
		db: db,
	}
}

// User Repository Methods

// Create creates a new user
func (r *userRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("user with email already exists")
		}
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by ID
func (r *userRepository) GetByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// GetByIDs retrieves multiple users by their IDs
func (r *userRepository) GetByIDs(ids []uint) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Where("id IN ?", ids).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IDs: %w", err)
	}
	return users, nil
}

// GetAll retrieves all users with pagination
func (r *userRepository) GetAll(limit, offset int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Limit(limit).Offset(offset).Order("created_at DESC").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// Update updates an existing user
func (r *userRepository) Update(user *models.User) error {
	result := r.db.Save(user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("email already exists")
		}
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// UpdateFields updates specific fields of a user (selective update)
// This method is used to update only specified fields without touching other fields
func (r *userRepository) UpdateFields(id uint, fields map[string]interface{}) error {
	result := r.db.Model(&models.User{}).Where("id = ?", id).Updates(fields)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("email already exists")
		}
		return fmt.Errorf("failed to update user fields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// Delete soft deletes a user by ID
func (r *userRepository) Delete(id uint) error {
	result := r.db.Delete(&models.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// Count returns the total number of users
func (r *userRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// GetWithDepartment retrieves a user by ID with department and subdepartment preloaded
func (r *userRepository) GetWithDepartment(id uint) (*models.User, error) {
	var user models.User
	err := r.db.Preload("Department").Preload("Subdepartment").Preload("Subdepartment.Department").First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user with department: %w", err)
	}
	return &user, nil
}

// GetAllWithDepartments retrieves all users with departments and subdepartments preloaded
func (r *userRepository) GetAllWithDepartments(limit, offset int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Preload("Department").Preload("Subdepartment").Preload("Subdepartment.Department").Limit(limit).Offset(offset).Order("created_at DESC").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with departments: %w", err)
	}
	return users, nil
}

// GetAllWithDepartmentsFiltered retrieves users with departments and subdepartments preloaded, filtered by department ID and active status
func (r *userRepository) GetAllWithDepartmentsFiltered(limit, offset int, departmentID *uint, isActive *bool) ([]*models.User, error) {
	var users []*models.User
	query := r.db.Preload("Department").Preload("Subdepartment").Preload("Subdepartment.Department")

	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.Limit(limit).Offset(offset).Order("created_at DESC").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with departments: %w", err)
	}
	return users, nil
}

// GetAllWithDepartmentsFilteredAdvanced retrieves users with advanced filtering options
func (r *userRepository) GetAllWithDepartmentsFilteredAdvanced(limit, offset int, departmentID *uint, isActive *bool, roles []string, excludeRoles []string, sortBy string, sortOrder string, searchQuery string) ([]*models.User, error) {
	var users []*models.User
	query := r.db.Preload("Department").Preload("Subdepartment").Preload("Subdepartment.Department")

	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	if len(roles) > 0 {
		query = query.Where("role IN ?", roles)
	}

	if len(excludeRoles) > 0 {
		query = query.Where("role NOT IN ?", excludeRoles)
	}

	// Apply search filter (case-insensitive)
	// For Unicode (Cyrillic) support, we need to search both lowercase and original case
	// because LOWER() doesn't work with locale 'C' in PostgreSQL
	if searchQuery != "" {
		searchTerm := strings.TrimSpace(searchQuery)
		lowerPattern := "%" + strings.ToLower(searchTerm) + "%"
		upperPattern := "%" + strings.ToUpper(searchTerm) + "%"
		titlePattern := "%" + strings.Title(strings.ToLower(searchTerm)) + "%"

		query = query.Where(
			"name LIKE ? OR name LIKE ? OR name LIKE ? OR "+
				"first_name LIKE ? OR first_name LIKE ? OR first_name LIKE ? OR "+
				"last_name LIKE ? OR last_name LIKE ? OR last_name LIKE ? OR "+
				"email LIKE ? OR email LIKE ? OR email LIKE ? OR "+
				"phone LIKE ? OR phone LIKE ? OR phone LIKE ? OR "+
				"position LIKE ? OR position LIKE ? OR position LIKE ?",
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
		)
	}

	// Build ORDER BY clause
	orderClause := buildOrderClause(sortBy, sortOrder)
	query = query.Order(orderClause)

	err := query.Limit(limit).Offset(offset).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with departments: %w", err)
	}
	return users, nil
}

// buildOrderClause creates an ORDER BY clause based on sortBy and sortOrder
func buildOrderClause(sortBy string, sortOrder string) string {
	// Validate sortOrder
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	// Map sortBy to actual column names
	var column string
	switch sortBy {
	case "name":
		column = "name"
	case "email":
		column = "email"
	case "department":
		column = "department_id"
	case "role":
		column = "role"
	case "created_at":
		column = "created_at"
	default:
		column = "created_at"
	}

	return fmt.Sprintf("%s %s", column, sortOrder)
}

// CountWithFilters returns the total number of users with optional filters
func (r *userRepository) CountWithFilters(departmentID *uint, isActive *bool) (int64, error) {
	var count int64
	query := r.db.Model(&models.User{})

	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	err := query.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// CountWithFiltersAdvanced returns the total number of users with advanced filtering options
func (r *userRepository) CountWithFiltersAdvanced(departmentID *uint, isActive *bool, roles []string, excludeRoles []string, searchQuery string) (int64, error) {
	var count int64
	query := r.db.Model(&models.User{})

	if departmentID != nil {
		query = query.Where("department_id = ?", *departmentID)
	}

	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	if len(roles) > 0 {
		query = query.Where("role IN ?", roles)
	}

	if len(excludeRoles) > 0 {
		query = query.Where("role NOT IN ?", excludeRoles)
	}

	// Apply search filter (case-insensitive)
	// For Unicode (Cyrillic) support, we need to search both lowercase and original case
	// because LOWER() doesn't work with locale 'C' in PostgreSQL
	if searchQuery != "" {
		searchTerm := strings.TrimSpace(searchQuery)
		lowerPattern := "%" + strings.ToLower(searchTerm) + "%"
		upperPattern := "%" + strings.ToUpper(searchTerm) + "%"
		titlePattern := "%" + strings.Title(strings.ToLower(searchTerm)) + "%"

		query = query.Where(
			"name LIKE ? OR name LIKE ? OR name LIKE ? OR "+
				"first_name LIKE ? OR first_name LIKE ? OR first_name LIKE ? OR "+
				"last_name LIKE ? OR last_name LIKE ? OR last_name LIKE ? OR "+
				"email LIKE ? OR email LIKE ? OR email LIKE ? OR "+
				"phone LIKE ? OR phone LIKE ? OR phone LIKE ? OR "+
				"position LIKE ? OR position LIKE ? OR position LIKE ?",
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
			lowerPattern, upperPattern, titlePattern,
		)
	}

	err := query.Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// GetUsersByDepartment retrieves all active users in a specific department
func (r *userRepository) GetUsersByDepartment(departmentID uint) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Where("department_id = ? AND is_active = ?", departmentID, true).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users by department: %w", err)
	}
	return users, nil
}

// CountByRole counts users by their role
func (r *userRepository) CountByRole(role string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("role = ?", role).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role: %w", err)
	}
	return count, nil
}

// CountByRoleAndActive counts users by their role and active status
func (r *userRepository) CountByRoleAndActive(role string, isActive bool) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("role = ? AND is_active = ?", role, isActive).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role and active status: %w", err)
	}
	return count, nil
}

// SuperAdminExists checks if a super admin user exists in the system
func (r *userRepository) SuperAdminExists() (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("role = ?", "super_admin").Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check super admin existence: %w", err)
	}
	return count > 0, nil
}

// Department Repository Methods

// Create creates a new department
func (r *departmentRepository) Create(department *models.Department) error {
	if err := r.db.Create(department).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("department with name already exists")
		}
		return fmt.Errorf("failed to create department: %w", err)
	}
	return nil
}

// GetByID retrieves a department by ID
func (r *departmentRepository) GetByID(id uint) (*models.Department, error) {
	var department models.Department
	err := r.db.First(&department, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}
	return &department, nil
}

// GetByName retrieves a department by name
func (r *departmentRepository) GetByName(name string) (*models.Department, error) {
	var department models.Department
	err := r.db.Where("name = ?", name).First(&department).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department by name: %w", err)
	}
	return &department, nil
}

// GetAll retrieves all departments
func (r *departmentRepository) GetAll() ([]*models.Department, error) {
	var departments []*models.Department
	err := r.db.Order("name ASC").Find(&departments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get departments: %w", err)
	}
	return departments, nil
}

// Update updates an existing department
func (r *departmentRepository) Update(department *models.Department) error {
	result := r.db.Save(department)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("department name already exists")
		}
		return fmt.Errorf("failed to update department: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("department not found")
	}
	return nil
}

// Delete soft deletes a department by ID
func (r *departmentRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Department{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete department: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("department not found")
	}
	return nil
}

// GetByNameIncludingDeleted retrieves a department by name including soft-deleted records
func (r *departmentRepository) GetByNameIncludingDeleted(name string) (*models.Department, error) {
	var department models.Department
	err := r.db.Unscoped().Where("name = ?", name).First(&department).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department by name: %w", err)
	}
	return &department, nil
}

// Restore restores a soft-deleted department
func (r *departmentRepository) Restore(id uint) error {
	result := r.db.Model(&models.Department{}).Unscoped().Where("id = ?", id).Update("deleted_at", nil)
	if result.Error != nil {
		return fmt.Errorf("failed to restore department: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("department not found")
	}
	return nil
}

// UpdateTwoFactorStatus updates the two-factor authentication status for a user
func (r *userRepository) UpdateTwoFactorStatus(userID uint, enabled bool) error {
	result := r.db.Model(&models.User{}).Where("id = ?", userID).Update("two_factor_enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("failed to update 2FA status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// UpdatePasskeyStatus updates the passkey enabled status for a user
func (r *userRepository) UpdatePasskeyStatus(userID uint, enabled bool) error {
	result := r.db.Model(&models.User{}).Where("id = ?", userID).Update("passkey_enabled", enabled)
	if result.Error != nil {
		return fmt.Errorf("failed to update passkey status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// CountByTwoFactorEnabled counts users with 2FA enabled
func (r *userRepository) CountByTwoFactorEnabled() (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("two_factor_enabled = ?", true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count users with 2FA: %w", err)
	}
	return count, nil
}

// CountByPasskeyEnabled counts users with passkey enabled
func (r *userRepository) CountByPasskeyEnabled() (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Where("passkey_enabled = ?", true).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count users with passkey: %w", err)
	}
	return count, nil
}

// Subdepartment Repository Methods

// Create creates a new subdepartment
func (r *subdepartmentRepository) Create(subdepartment *models.Subdepartment) error {
	if err := r.db.Create(subdepartment).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("subdepartment with this name already exists in the department")
		}
		return fmt.Errorf("failed to create subdepartment: %w", err)
	}
	return nil
}

// GetByID retrieves a subdepartment by ID
func (r *subdepartmentRepository) GetByID(id uint) (*models.Subdepartment, error) {
	var subdepartment models.Subdepartment
	err := r.db.First(&subdepartment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("subdepartment not found")
		}
		return nil, fmt.Errorf("failed to get subdepartment: %w", err)
	}
	return &subdepartment, nil
}

// GetByDepartmentID retrieves all subdepartments by department ID
func (r *subdepartmentRepository) GetByDepartmentID(departmentID uint) ([]*models.Subdepartment, error) {
	var subdepartments []*models.Subdepartment
	err := r.db.Where("department_id = ?", departmentID).Order("name ASC").Find(&subdepartments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get subdepartments by department ID: %w", err)
	}
	return subdepartments, nil
}

// GetAll retrieves all subdepartments
func (r *subdepartmentRepository) GetAll() ([]*models.Subdepartment, error) {
	var subdepartments []*models.Subdepartment
	err := r.db.Preload("Department").Order("department_id ASC, name ASC").Find(&subdepartments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get subdepartments: %w", err)
	}
	return subdepartments, nil
}

// Update updates an existing subdepartment
func (r *subdepartmentRepository) Update(subdepartment *models.Subdepartment) error {
	result := r.db.Save(subdepartment)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return fmt.Errorf("subdepartment name already exists in the department")
		}
		return fmt.Errorf("failed to update subdepartment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("subdepartment not found")
	}
	return nil
}

// Delete soft deletes a subdepartment by ID
func (r *subdepartmentRepository) Delete(id uint) error {
	result := r.db.Delete(&models.Subdepartment{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete subdepartment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("subdepartment not found")
	}
	return nil
}

// GetByNameAndDepartmentIncludingDeleted retrieves a subdepartment by name and department ID including soft-deleted records
func (r *subdepartmentRepository) GetByNameAndDepartmentIncludingDeleted(name string, departmentID uint) (*models.Subdepartment, error) {
	var subdepartment models.Subdepartment
	err := r.db.Unscoped().Where("name = ? AND department_id = ?", name, departmentID).First(&subdepartment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("subdepartment not found")
		}
		return nil, fmt.Errorf("failed to get subdepartment by name and department: %w", err)
	}
	return &subdepartment, nil
}

// Restore restores a soft-deleted subdepartment
func (r *subdepartmentRepository) Restore(id uint) error {
	result := r.db.Model(&models.Subdepartment{}).Unscoped().Where("id = ?", id).Update("deleted_at", nil)
	if result.Error != nil {
		return fmt.Errorf("failed to restore subdepartment: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("subdepartment not found")
	}
	return nil
}

// GetWithDepartment retrieves a subdepartment by ID with department preloaded
func (r *subdepartmentRepository) GetWithDepartment(id uint) (*models.Subdepartment, error) {
	var subdepartment models.Subdepartment
	err := r.db.Preload("Department").First(&subdepartment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("subdepartment not found")
		}
		return nil, fmt.Errorf("failed to get subdepartment with department: %w", err)
	}
	return &subdepartment, nil
}

// ResetAllOnlineStatuses sets all users with "online" status to "offline"
func (r *userRepository) ResetAllOnlineStatuses() (int64, error) {
	result := r.db.Model(&models.User{}).
		Where("status = ?", "online").
		Updates(map[string]interface{}{
			"status":         "offline",
			"last_active_at": gorm.Expr("NOW()"),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to reset online statuses: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// CleanupDisconnectedStatuses sets users to offline if they are marked as online but not in the connected users list
func (r *userRepository) CleanupDisconnectedStatuses(connectedUserIDs []uint) (int64, error) {
	query := r.db.Model(&models.User{}).Where("status = ?", "online")

	// If there are connected users, exclude them from the update
	if len(connectedUserIDs) > 0 {
		query = query.Where("id NOT IN ?", connectedUserIDs)
	}

	result := query.Updates(map[string]interface{}{
		"status":         "offline",
		"last_active_at": gorm.Expr("NOW()"),
	})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup disconnected statuses: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// ============= User Group Repository =============

// UserGroupRepository defines the interface for user group data operations
type UserGroupRepository interface {
	Create(group *models.UserGroup) error
	GetByID(id uint) (*models.UserGroup, error)
	GetAll() ([]*models.UserGroup, error)
	GetByCreatorID(creatorID uint) ([]*models.UserGroup, error)
	Update(group *models.UserGroup) error
	Delete(id uint) error
	AddMembers(groupID uint, userIDs []uint) error
	RemoveMembers(groupID uint, userIDs []uint) error
	SetMembers(groupID uint, userIDs []uint) error
	GetMembers(groupID uint) ([]*models.User, error)
	GetGroupsForUser(userID uint) ([]*models.UserGroup, error)
	GetAllWithMemberCount() ([]*models.UserGroup, []int64, error)
	CountMembers(groupID uint) (int64, error)
	ReorderGroups(groupIDs []uint) error
}

// userGroupRepository implements UserGroupRepository interface
type userGroupRepository struct {
	db *database.DB
}

// NewUserGroupRepository creates a new user group repository
func NewUserGroupRepository(db *database.DB) UserGroupRepository {
	return &userGroupRepository{db: db}
}

// Create creates a new user group
func (r *userGroupRepository) Create(group *models.UserGroup) error {
	if err := r.db.Create(group).Error; err != nil {
		return fmt.Errorf("failed to create user group: %w", err)
	}
	return nil
}

// GetByID retrieves a user group by ID
func (r *userGroupRepository) GetByID(id uint) (*models.UserGroup, error) {
	var group models.UserGroup
	err := r.db.First(&group, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user group not found")
		}
		return nil, fmt.Errorf("failed to get user group: %w", err)
	}
	return &group, nil
}

// GetAll retrieves all user groups
func (r *userGroupRepository) GetAll() ([]*models.UserGroup, error) {
	var groups []*models.UserGroup
	err := r.db.Order("sort_order ASC, name ASC").Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups: %w", err)
	}
	return groups, nil
}

// GetByCreatorID retrieves all user groups created by a specific user
func (r *userGroupRepository) GetByCreatorID(creatorID uint) ([]*models.UserGroup, error) {
	var groups []*models.UserGroup
	err := r.db.Where("creator_id = ?", creatorID).Order("sort_order ASC, name ASC").Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user groups by creator: %w", err)
	}
	return groups, nil
}

// Update updates an existing user group
func (r *userGroupRepository) Update(group *models.UserGroup) error {
	result := r.db.Save(group)
	if result.Error != nil {
		return fmt.Errorf("failed to update user group: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user group not found")
	}
	return nil
}

// Delete soft deletes a user group and its members
func (r *userGroupRepository) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete members first
		if err := tx.Where("group_id = ?", id).Delete(&models.UserGroupMember{}).Error; err != nil {
			return fmt.Errorf("failed to delete user group members: %w", err)
		}
		// Delete group
		result := tx.Delete(&models.UserGroup{}, id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete user group: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("user group not found")
		}
		return nil
	})
}

// AddMembers adds users to a group
func (r *userGroupRepository) AddMembers(groupID uint, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, userID := range userIDs {
			// Check if record exists (including soft-deleted) to avoid unique constraint violation
			var existing models.UserGroupMember
			err := tx.Unscoped().Where("group_id = ? AND user_id = ?", groupID, userID).First(&existing).Error
			if err == nil {
				// Record exists — restore if soft-deleted
				if existing.DeletedAt.Valid {
					if err := tx.Unscoped().Model(&existing).Updates(map[string]interface{}{"deleted_at": nil}).Error; err != nil {
						return fmt.Errorf("failed to restore member %d in group: %w", userID, err)
					}
				}
				continue
			}
			// No record exists, create new one
			member := models.UserGroupMember{
				GroupID: groupID,
				UserID:  userID,
			}
			if err := tx.Create(&member).Error; err != nil {
				return fmt.Errorf("failed to add member %d to group: %w", userID, err)
			}
		}
		return nil
	})
}

// RemoveMembers removes users from a group
func (r *userGroupRepository) RemoveMembers(groupID uint, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}
	// Hard delete to avoid soft-deleted records conflicting with unique index on re-add
	result := r.db.Unscoped().Where("group_id = ? AND user_id IN ?", groupID, userIDs).Delete(&models.UserGroupMember{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove members from group: %w", result.Error)
	}
	return nil
}

// SetMembers replaces all members of a group with the given user IDs
func (r *userGroupRepository) SetMembers(groupID uint, userIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Hard delete all existing members (Unscoped to avoid soft-delete + unique index conflict)
		if err := tx.Unscoped().Where("group_id = ?", groupID).Delete(&models.UserGroupMember{}).Error; err != nil {
			return fmt.Errorf("failed to clear group members: %w", err)
		}
		// Add new members
		for _, userID := range userIDs {
			member := models.UserGroupMember{
				GroupID: groupID,
				UserID:  userID,
			}
			if err := tx.Create(&member).Error; err != nil {
				return fmt.Errorf("failed to add member %d to group: %w", userID, err)
			}
		}
		return nil
	})
}

// GetMembers retrieves all users in a group with department info
func (r *userGroupRepository) GetMembers(groupID uint) ([]*models.User, error) {
	var users []*models.User
	err := r.db.
		Joins("JOIN user_group_members ON user_group_members.user_id = users.id").
		Where("user_group_members.group_id = ? AND user_group_members.deleted_at IS NULL", groupID).
		Preload("Department").
		Preload("Subdepartment").
		Order("users.name ASC").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}
	return users, nil
}

// GetGroupsForUser retrieves all groups a user belongs to
func (r *userGroupRepository) GetGroupsForUser(userID uint) ([]*models.UserGroup, error) {
	var groups []*models.UserGroup
	err := r.db.
		Joins("JOIN user_group_members ON user_group_members.group_id = user_groups.id").
		Where("user_group_members.user_id = ? AND user_group_members.deleted_at IS NULL", userID).
		Order("user_groups.sort_order ASC, user_groups.name ASC").
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get groups for user: %w", err)
	}
	return groups, nil
}

// GetAllWithMemberCount retrieves all groups with their member counts
func (r *userGroupRepository) GetAllWithMemberCount() ([]*models.UserGroup, []int64, error) {
	var groups []*models.UserGroup
	err := r.db.Order("sort_order ASC, name ASC").Find(&groups).Error
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user groups: %w", err)
	}

	counts := make([]int64, len(groups))
	for i, group := range groups {
		var count int64
		err := r.db.Model(&models.UserGroupMember{}).Where("group_id = ?", group.ID).Count(&count).Error
		if err != nil {
			return nil, nil, fmt.Errorf("failed to count members for group %d: %w", group.ID, err)
		}
		counts[i] = count
	}

	return groups, counts, nil
}

// CountMembers counts the number of members in a group
func (r *userGroupRepository) CountMembers(groupID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserGroupMember{}).Where("group_id = ?", groupID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count group members: %w", err)
	}
	return count, nil
}

// ReorderGroups updates the sort_order of groups based on the order of IDs provided
func (r *userGroupRepository) ReorderGroups(groupIDs []uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i, id := range groupIDs {
			if err := tx.Model(&models.UserGroup{}).Where("id = ?", id).Update("sort_order", i).Error; err != nil {
				return fmt.Errorf("failed to update sort_order for group %d: %w", id, err)
			}
		}
		return nil
	})
}
