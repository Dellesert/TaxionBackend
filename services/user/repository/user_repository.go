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
	GetAll(limit, offset int) ([]*models.User, error)
	Update(user *models.User) error
	Delete(id uint) error
	Count() (int64, error)
	GetWithDepartment(id uint) (*models.User, error)
	GetAllWithDepartments(limit, offset int) ([]*models.User, error)
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
