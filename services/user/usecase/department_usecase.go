package usecase

import (
	"errors"
	"fmt"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"

	"gorm.io/gorm"
)

// DepartmentUsecase defines the interface for department business logic
type DepartmentUsecase interface {
	GetAllDepartments() ([]*models.DepartmentResponse, error)
	GetDepartment(id uint) (*models.DepartmentResponse, error)
	CreateDepartment(req *models.CreateDepartmentRequest) (*models.DepartmentResponse, error)
	UpdateDepartment(id uint, req *models.UpdateDepartmentRequest) (*models.DepartmentResponse, error)
	DeleteDepartment(id uint) error
	GetDepartmentWithUsers(id uint) (*models.DepartmentWithUsersResponse, error)
}

// departmentUsecase implements DepartmentUsecase interface
type departmentUsecase struct {
	departmentRepo repository.DepartmentRepository
	userRepo       repository.UserRepository
}

// NewDepartmentUsecase creates a new department usecase
func NewDepartmentUsecase(departmentRepo repository.DepartmentRepository, userRepo repository.UserRepository) DepartmentUsecase {
	return &departmentUsecase{
		departmentRepo: departmentRepo,
		userRepo:       userRepo,
	}
}

// GetAllDepartments retrieves all departments
func (d *departmentUsecase) GetAllDepartments() ([]*models.DepartmentResponse, error) {
	departments, err := d.departmentRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get departments: %w", err)
	}

	responses := make([]*models.DepartmentResponse, len(departments))
	for i, dept := range departments {
		responses[i] = dept.ToResponse()
	}

	return responses, nil
}

// GetDepartment retrieves a department by ID
func (d *departmentUsecase) GetDepartment(id uint) (*models.DepartmentResponse, error) {
	department, err := d.departmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	return department.ToResponse(), nil
}

// CreateDepartment creates a new department
func (d *departmentUsecase) CreateDepartment(req *models.CreateDepartmentRequest) (*models.DepartmentResponse, error) {
	// Validate request
	if err := d.validateCreateDepartmentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if department with same name already exists
	existingDept, err := d.departmentRepo.GetByName(req.Name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("failed to check existing department: %w", err)
	}
	if existingDept != nil {
		return nil, fmt.Errorf("department with name '%s' already exists", req.Name)
	}

	// Create department
	department := &models.Department{
		Name: strings.TrimSpace(req.Name),
	}

	if err := d.departmentRepo.Create(department); err != nil {
		return nil, fmt.Errorf("failed to create department: %w", err)
	}

	return department.ToResponse(), nil
}

// UpdateDepartment updates an existing department
func (d *departmentUsecase) UpdateDepartment(id uint, req *models.UpdateDepartmentRequest) (*models.DepartmentResponse, error) {
	// Validate request
	if err := d.validateUpdateDepartmentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing department
	department, err := d.departmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	// Update fields if provided
	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)

		// Check if new name conflicts with existing department
		if newName != department.Name {
			existingDept, err := d.departmentRepo.GetByName(newName)
			if err != nil && !strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("failed to check existing department: %w", err)
			}
			if existingDept != nil {
				return nil, fmt.Errorf("department with name '%s' already exists", newName)
			}
		}

		department.Name = newName
	}

	// Save updated department
	if err := d.departmentRepo.Update(department); err != nil {
		return nil, fmt.Errorf("failed to update department: %w", err)
	}

	return department.ToResponse(), nil
}

// DeleteDepartment deletes a department by ID
func (d *departmentUsecase) DeleteDepartment(id uint) error {
	// Check if department exists
	_, err := d.departmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("department not found")
		}
		return fmt.Errorf("failed to get department: %w", err)
	}

	// Check if department has users (optional - can be relaxed)
	// For now, we'll allow deletion and set users' department_id to NULL
	// This is handled by the foreign key constraint with ON DELETE SET NULL

	// Delete department
	if err := d.departmentRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete department: %w", err)
	}

	return nil
}

// GetDepartmentWithUsers retrieves a department with its users
func (d *departmentUsecase) GetDepartmentWithUsers(id uint) (*models.DepartmentWithUsersResponse, error) {
	// Get department
	department, err := d.departmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	// Get users in this department
	users, err := d.userRepo.GetAllWithDepartments(100, 0) // Get all users
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	// Filter users by department
	var departmentUsers []*models.UserResponse
	for _, user := range users {
		if user.DepartmentID != nil && *user.DepartmentID == id {
			departmentUsers = append(departmentUsers, user.ToResponse())
		}
	}

	response := &models.DepartmentWithUsersResponse{
		ID:        department.ID,
		Name:      department.Name,
		CreatedAt: department.CreatedAt,
		UpdatedAt: department.UpdatedAt,
		Users:     departmentUsers,
		UserCount: len(departmentUsers),
	}

	return response, nil
}

// validateCreateDepartmentRequest validates department creation request
func (d *departmentUsecase) validateCreateDepartmentRequest(req *models.CreateDepartmentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("department name is required")
	}
	if len(name) < 2 {
		return fmt.Errorf("department name must be at least 2 characters long")
	}
	if len(name) > 100 {
		return fmt.Errorf("department name must be less than 100 characters")
	}

	return nil
}

// validateUpdateDepartmentRequest validates department update request
func (d *departmentUsecase) validateUpdateDepartmentRequest(req *models.UpdateDepartmentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate name if provided
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return fmt.Errorf("department name cannot be empty")
		}
		if len(name) < 2 {
			return fmt.Errorf("department name must be at least 2 characters long")
		}
		if len(name) > 100 {
			return fmt.Errorf("department name must be less than 100 characters")
		}
	}

	return nil
}
