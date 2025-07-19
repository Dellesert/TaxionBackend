package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ProfileUsecase defines the interface for profile business logic
type ProfileUsecase interface {
	GetProfile(id uint) (*models.UserResponse, error)
	UpdateProfile(id uint, req *models.UpdateProfileRequest) (*models.UserResponse, error)
	ChangePassword(id uint, req *models.ChangePasswordRequest) error
	UpdateStatus(id uint, status sharedmodels.UserStatus) (*models.UserResponse, error)
}

// profileUsecase implements ProfileUsecase interface
type profileUsecase struct {
	userRepo       repository.UserRepository
	departmentRepo repository.DepartmentRepository
}

// NewProfileUsecase creates a new profile usecase
func NewProfileUsecase(userRepo repository.UserRepository, departmentRepo repository.DepartmentRepository) ProfileUsecase {
	return &profileUsecase{
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
	}
}

// GetProfile retrieves a user profile by ID
func (p *profileUsecase) GetProfile(id uint) (*models.UserResponse, error) {
	user, err := p.userRepo.GetWithDepartment(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("profile is deactivated")
	}

	return user.ToResponse(), nil
}

// UpdateProfile updates a user's profile
func (p *profileUsecase) UpdateProfile(id uint, req *models.UpdateProfileRequest) (*models.UserResponse, error) {
	// Validate request
	if err := p.validateUpdateProfileRequest(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing user
	user, err := p.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("profile is deactivated")
	}

	// Update fields if provided
	if req.Name != nil {
		user.Name = strings.TrimSpace(*req.Name)
	}
	if req.Avatar != nil {
		user.Avatar = strings.TrimSpace(*req.Avatar)
	}
	if req.Phone != nil {
		user.Phone = strings.TrimSpace(*req.Phone)
	}
	if req.Position != nil {
		user.Position = strings.TrimSpace(*req.Position)
	}
	if req.DepartmentID != nil {
		// Validate department exists
		if *req.DepartmentID > 0 {
			_, err := p.departmentRepo.GetByID(*req.DepartmentID)
			if err != nil {
				return nil, fmt.Errorf("department not found")
			}
		}
		user.DepartmentID = req.DepartmentID
	}

	// Save updated user
	if err := p.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Get user with department for response
	userWithDept, err := p.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// ChangePassword changes a user's password
func (p *profileUsecase) ChangePassword(id uint, req *models.ChangePasswordRequest) error {
	// Validate request
	if err := p.validateChangePasswordRequest(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get user
	user, err := p.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("profile not found")
		}
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return fmt.Errorf("profile is deactivated")
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(req.CurrentPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	user.HashedPassword = string(hashedPassword)
	if err := p.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateStatus updates a user's status
func (p *profileUsecase) UpdateStatus(id uint, status sharedmodels.UserStatus) (*models.UserResponse, error) {
	// Validate status
	if !isValidStatus(status) {
		return nil, fmt.Errorf("invalid status: %s", status)
	}

	// Get user
	user, err := p.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("profile is deactivated")
	}

	// Update status
	user.Status = status

	// Update last active time if going online
	if status == sharedmodels.StatusOnline {
		now := time.Now()
		user.LastActiveAt = &now
	}

	// Save updated user
	if err := p.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	// Get user with department for response
	userWithDept, err := p.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// validateUpdateProfileRequest validates profile update request
func (p *profileUsecase) validateUpdateProfileRequest(req *models.UpdateProfileRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	// Validate name if provided
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if len(name) < 2 {
			return fmt.Errorf("name must be at least 2 characters long")
		}
		if len(name) > 100 {
			return fmt.Errorf("name must be less than 100 characters")
		}
	}

	// Validate phone if provided
	if req.Phone != nil {
		phone := strings.TrimSpace(*req.Phone)
		if phone != "" && len(phone) > 20 {
			return fmt.Errorf("phone number must be less than 20 characters")
		}
	}

	// Validate position if provided
	if req.Position != nil {
		position := strings.TrimSpace(*req.Position)
		if position != "" && len(position) > 100 {
			return fmt.Errorf("position must be less than 100 characters")
		}
	}

	// Validate avatar URL if provided
	if req.Avatar != nil {
		avatar := strings.TrimSpace(*req.Avatar)
		if avatar != "" && len(avatar) > 500 {
			return fmt.Errorf("avatar URL must be less than 500 characters")
		}
	}

	return nil
}

// validateChangePasswordRequest validates password change request
func (p *profileUsecase) validateChangePasswordRequest(req *models.ChangePasswordRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}

	if req.CurrentPassword == "" {
		return fmt.Errorf("current password is required")
	}

	if req.NewPassword == "" {
		return fmt.Errorf("new password is required")
	}

	// Validate new password strength
	if len(req.NewPassword) < 6 {
		return fmt.Errorf("new password must be at least 6 characters long")
	}

	if len(req.NewPassword) > 100 {
		return fmt.Errorf("new password must be less than 100 characters")
	}

	if req.CurrentPassword == req.NewPassword {
		return fmt.Errorf("new password must be different from current password")
	}

	return nil
}
