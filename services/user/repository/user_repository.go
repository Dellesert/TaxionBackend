package repository

import (
	"errors"
	"fmt"

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
	Delete(id uint) error
	Count() (int64, error)
	CountWithFilters(departmentID *uint, isActive *bool) (int64, error)
	CountByTwoFactorEnabled() (int64, error)
	CountByPasskeyEnabled() (int64, error)
	GetWithDepartment(id uint) (*models.User, error)
	GetAllWithDepartments(limit, offset int) ([]*models.User, error)
	GetAllWithDepartmentsFiltered(limit, offset int, departmentID *uint, isActive *bool) ([]*models.User, error)
	SuperAdminExists() (bool, error)
	UpdateTwoFactorStatus(userID uint, enabled bool) error
	UpdatePasskeyStatus(userID uint, enabled bool) error
}

// DepartmentRepository defines the interface for department data operations
type DepartmentRepository interface {
	Create(department *models.Department) error
	GetByID(id uint) (*models.Department, error)
	GetByName(name string) (*models.Department, error)
	GetAll() ([]*models.Department, error)
	Update(department *models.Department) error
	Delete(id uint) error
}

// userRepository implements UserRepository interface
type userRepository struct {
	db *database.DB
}

// departmentRepository implements DepartmentRepository interface
type departmentRepository struct {
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

// GetWithDepartment retrieves a user by ID with department preloaded
func (r *userRepository) GetWithDepartment(id uint) (*models.User, error) {
	var user models.User
	err := r.db.Preload("Department").First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user with department: %w", err)
	}
	return &user, nil
}

// GetAllWithDepartments retrieves all users with departments preloaded
func (r *userRepository) GetAllWithDepartments(limit, offset int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Preload("Department").Limit(limit).Offset(offset).Order("created_at DESC").Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with departments: %w", err)
	}
	return users, nil
}

// GetAllWithDepartmentsFiltered retrieves users with departments preloaded, filtered by department ID and active status
func (r *userRepository) GetAllWithDepartmentsFiltered(limit, offset int, departmentID *uint, isActive *bool) ([]*models.User, error) {
	var users []*models.User
	query := r.db.Preload("Department")

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
