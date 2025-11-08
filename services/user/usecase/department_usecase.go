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
	GetByName(name string) (*models.DepartmentResponse, error)
	GetDepartmentByNameIncludingDeleted(name string) (*models.Department, error)
	CreateDepartment(req *models.CreateDepartmentRequest) (*models.DepartmentResponse, error)
	UpdateDepartment(id uint, req *models.UpdateDepartmentRequest) (*models.DepartmentResponse, error)
	DeleteDepartment(id uint) error
	RestoreDepartment(id uint) error
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
		response := dept.ToResponse()

		// Count users in this department
		users, err := d.userRepo.GetAllWithDepartments(10000, 0)
		if err == nil {
			count := 0
			for _, user := range users {
				if user.DepartmentID != nil && *user.DepartmentID == dept.ID {
					count++
				}
			}
			response.UserCount = count
		}

		responses[i] = response
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

// GetByName retrieves a department by name
func (d *departmentUsecase) GetByName(name string) (*models.DepartmentResponse, error) {
	department, err := d.departmentRepo.GetByName(name)
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

	// Track if we need to update user roles
	var oldHeadID *uint
	var newHeadID *uint

	if req.HeadID != nil {
		oldHeadID = department.HeadID
		newHeadID = req.HeadID
		department.HeadID = req.HeadID

		// If HeadID is being set or changed, update the new head's role to department_head
		if newHeadID != nil && *newHeadID > 0 {
			// Verify the user exists
			newHead, err := d.userRepo.GetByID(*newHeadID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
					return nil, fmt.Errorf("user with ID %d not found", *newHeadID)
				}
				return nil, fmt.Errorf("failed to get user: %w", err)
			}

			// Update role to department_head if not already admin or super_admin
			if newHead.Role != "admin" && newHead.Role != "super_admin" {
				newHead.Role = "department_head"
				if err := d.userRepo.Update(newHead); err != nil {
					return nil, fmt.Errorf("failed to update new head's role: %w", err)
				}
			}
		}

		// If there was a previous head and they're being replaced, check if they should be demoted
		if oldHeadID != nil && *oldHeadID > 0 && (newHeadID == nil || *newHeadID != *oldHeadID) {
			// Check if the old head is still a head of another department
			allDepartments, err := d.departmentRepo.GetAll()
			if err != nil {
				return nil, fmt.Errorf("failed to get departments: %w", err)
			}

			isStillHead := false
			for _, dept := range allDepartments {
				// Skip the current department (it's being updated)
				if dept.ID == id {
					continue
				}
				if dept.HeadID != nil && *dept.HeadID == *oldHeadID {
					isStillHead = true
					break
				}
			}

			// If they're no longer head of any department, demote to employee
			if !isStillHead {
				oldHead, err := d.userRepo.GetByID(*oldHeadID)
				if err == nil && oldHead.Role == "department_head" {
					// Only demote if they're currently a department_head (not admin/super_admin)
					oldHead.Role = "employee"
					if err := d.userRepo.Update(oldHead); err != nil {
						return nil, fmt.Errorf("failed to update old head's role: %w", err)
					}
				}
			}
		}
	}

	// Save updated department
	if err := d.departmentRepo.Update(department); err != nil {
		return nil, fmt.Errorf("failed to update department: %w", err)
	}

	return department.ToResponse(), nil
}

// DeleteDepartment deletes a department by ID
func (d *departmentUsecase) DeleteDepartment(id uint) error {
	// Check if department exists and get its head
	department, err := d.departmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("department not found")
		}
		return fmt.Errorf("failed to get department: %w", err)
	}

	// Store the head ID before deletion
	oldHeadID := department.HeadID

	// Check if department has users (optional - can be relaxed)
	// For now, we'll allow deletion and set users' department_id to NULL
	// This is handled by the foreign key constraint with ON DELETE SET NULL

	// Delete department
	if err := d.departmentRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete department: %w", err)
	}

	// If there was a head, check if they should be demoted
	if oldHeadID != nil && *oldHeadID > 0 {
		// Check if the old head is still a head of another department
		allDepartments, err := d.departmentRepo.GetAll()
		if err != nil {
			// Don't fail the deletion, just log the error
			return nil
		}

		isStillHead := false
		for _, dept := range allDepartments {
			if dept.HeadID != nil && *dept.HeadID == *oldHeadID {
				isStillHead = true
				break
			}
		}

		// If they're no longer head of any department, demote to employee
		if !isStillHead {
			oldHead, err := d.userRepo.GetByID(*oldHeadID)
			if err == nil && oldHead.Role == "department_head" {
				// Only demote if they're currently a department_head (not admin/super_admin)
				oldHead.Role = "employee"
				_ = d.userRepo.Update(oldHead) // Ignore error to not fail deletion
			}
		}
	}

	return nil
}

// GetDepartmentByNameIncludingDeleted retrieves a department by name including soft-deleted records
func (d *departmentUsecase) GetDepartmentByNameIncludingDeleted(name string) (*models.Department, error) {
	return d.departmentRepo.GetByNameIncludingDeleted(name)
}

// RestoreDepartment restores a soft-deleted department
func (d *departmentUsecase) RestoreDepartment(id uint) error {
	return d.departmentRepo.Restore(id)
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

// SubdepartmentUsecase defines the interface for subdepartment business logic
type SubdepartmentUsecase interface {
	GetAllSubdepartments() ([]*models.SubdepartmentResponse, error)
	GetSubdepartment(id uint) (*models.SubdepartmentResponse, error)
	GetSubdepartmentsByDepartment(departmentID uint) ([]*models.SubdepartmentResponse, error)
	GetSubdepartmentByNameAndDepartmentIncludingDeleted(name string, departmentID uint) (*models.Subdepartment, error)
	CreateSubdepartment(req *models.CreateSubdepartmentRequest) (*models.SubdepartmentResponse, error)
	UpdateSubdepartment(id uint, req *models.UpdateSubdepartmentRequest) (*models.SubdepartmentResponse, error)
	DeleteSubdepartment(id uint) error
	RestoreSubdepartment(id uint) error
}

// subdepartmentUsecase implements SubdepartmentUsecase interface
type subdepartmentUsecase struct {
	subdepartmentRepo repository.SubdepartmentRepository
	departmentRepo    repository.DepartmentRepository
	userRepo          repository.UserRepository
}

// NewSubdepartmentUsecase creates a new subdepartment usecase
func NewSubdepartmentUsecase(subdepartmentRepo repository.SubdepartmentRepository, departmentRepo repository.DepartmentRepository, userRepo repository.UserRepository) SubdepartmentUsecase {
	return &subdepartmentUsecase{
		subdepartmentRepo: subdepartmentRepo,
		departmentRepo:    departmentRepo,
		userRepo:          userRepo,
	}
}

// GetAllSubdepartments retrieves all subdepartments
func (s *subdepartmentUsecase) GetAllSubdepartments() ([]*models.SubdepartmentResponse, error) {
	subdepartments, err := s.subdepartmentRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get subdepartments: %w", err)
	}

	responses := make([]*models.SubdepartmentResponse, len(subdepartments))
	for i, subdept := range subdepartments {
		responses[i] = subdept.ToResponse()
	}

	return responses, nil
}

// GetSubdepartment retrieves a subdepartment by ID
func (s *subdepartmentUsecase) GetSubdepartment(id uint) (*models.SubdepartmentResponse, error) {
	subdepartment, err := s.subdepartmentRepo.GetWithDepartment(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("subdepartment not found")
		}
		return nil, fmt.Errorf("failed to get subdepartment: %w", err)
	}

	return subdepartment.ToResponse(), nil
}

// GetSubdepartmentsByDepartment retrieves all subdepartments for a department
func (s *subdepartmentUsecase) GetSubdepartmentsByDepartment(departmentID uint) ([]*models.SubdepartmentResponse, error) {
	// Verify department exists
	_, err := s.departmentRepo.GetByID(departmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	subdepartments, err := s.subdepartmentRepo.GetByDepartmentID(departmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subdepartments: %w", err)
	}

	responses := make([]*models.SubdepartmentResponse, len(subdepartments))
	for i, subdept := range subdepartments {
		responses[i] = subdept.ToResponse()
	}

	return responses, nil
}

// CreateSubdepartment creates a new subdepartment
func (s *subdepartmentUsecase) CreateSubdepartment(req *models.CreateSubdepartmentRequest) (*models.SubdepartmentResponse, error) {
	// Validate request
	if err := s.validateCreateSubdepartmentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Verify department exists
	_, err := s.departmentRepo.GetByID(req.DepartmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("department not found")
		}
		return nil, fmt.Errorf("failed to get department: %w", err)
	}

	// Verify head user exists if provided
	if req.HeadID != nil && *req.HeadID > 0 {
		_, err := s.userRepo.GetByID(*req.HeadID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("user with ID %d not found", *req.HeadID)
			}
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
	}

	// Create subdepartment
	subdepartment := &models.Subdepartment{
		Name:         strings.TrimSpace(req.Name),
		DepartmentID: req.DepartmentID,
		HeadID:       req.HeadID,
	}

	if err := s.subdepartmentRepo.Create(subdepartment); err != nil {
		return nil, fmt.Errorf("failed to create subdepartment: %w", err)
	}

	// Load department for response
	subdepartment, err = s.subdepartmentRepo.GetWithDepartment(subdepartment.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created subdepartment: %w", err)
	}

	return subdepartment.ToResponse(), nil
}

// UpdateSubdepartment updates an existing subdepartment
func (s *subdepartmentUsecase) UpdateSubdepartment(id uint, req *models.UpdateSubdepartmentRequest) (*models.SubdepartmentResponse, error) {
	// Validate request
	if err := s.validateUpdateSubdepartmentRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing subdepartment
	subdepartment, err := s.subdepartmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("subdepartment not found")
		}
		return nil, fmt.Errorf("failed to get subdepartment: %w", err)
	}

	// Update fields if provided
	if req.Name != nil {
		subdepartment.Name = strings.TrimSpace(*req.Name)
	}

	if req.HeadID != nil {
		// Verify the user exists if HeadID is being set
		if *req.HeadID > 0 {
			_, err := s.userRepo.GetByID(*req.HeadID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
					return nil, fmt.Errorf("user with ID %d not found", *req.HeadID)
				}
				return nil, fmt.Errorf("failed to get user: %w", err)
			}
		}
		subdepartment.HeadID = req.HeadID
	}

	// Save updated subdepartment
	if err := s.subdepartmentRepo.Update(subdepartment); err != nil {
		return nil, fmt.Errorf("failed to update subdepartment: %w", err)
	}

	// Load with department for response
	subdepartment, err = s.subdepartmentRepo.GetWithDepartment(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated subdepartment: %w", err)
	}

	return subdepartment.ToResponse(), nil
}

// DeleteSubdepartment deletes a subdepartment by ID
func (s *subdepartmentUsecase) DeleteSubdepartment(id uint) error {
	// Check if subdepartment exists
	_, err := s.subdepartmentRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("subdepartment not found")
		}
		return fmt.Errorf("failed to get subdepartment: %w", err)
	}

	// Delete subdepartment (users' subdepartment_id will be set to NULL by foreign key constraint)
	if err := s.subdepartmentRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete subdepartment: %w", err)
	}

	return nil
}

// GetSubdepartmentByNameAndDepartmentIncludingDeleted retrieves a subdepartment by name and department ID including soft-deleted records
func (s *subdepartmentUsecase) GetSubdepartmentByNameAndDepartmentIncludingDeleted(name string, departmentID uint) (*models.Subdepartment, error) {
	return s.subdepartmentRepo.GetByNameAndDepartmentIncludingDeleted(name, departmentID)
}

// RestoreSubdepartment restores a soft-deleted subdepartment
func (s *subdepartmentUsecase) RestoreSubdepartment(id uint) error {
	return s.subdepartmentRepo.Restore(id)
}

// validateCreateSubdepartmentRequest validates subdepartment creation request
func (s *subdepartmentUsecase) validateCreateSubdepartmentRequest(req *models.CreateSubdepartmentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return fmt.Errorf("subdepartment name is required")
	}
	if len(name) < 2 {
		return fmt.Errorf("subdepartment name must be at least 2 characters long")
	}
	if len(name) > 100 {
		return fmt.Errorf("subdepartment name must be less than 100 characters")
	}

	// Validate department ID
	if req.DepartmentID == 0 {
		return fmt.Errorf("department ID is required")
	}

	return nil
}

// validateUpdateSubdepartmentRequest validates subdepartment update request
func (s *subdepartmentUsecase) validateUpdateSubdepartmentRequest(req *models.UpdateSubdepartmentRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate name if provided
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return fmt.Errorf("subdepartment name cannot be empty")
		}
		if len(name) < 2 {
			return fmt.Errorf("subdepartment name must be at least 2 characters long")
		}
		if len(name) > 100 {
			return fmt.Errorf("subdepartment name must be less than 100 characters")
		}
	}

	return nil
}
