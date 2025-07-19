package usecase

import (
	"errors"
	"fmt"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminUsecase defines the interface for admin business logic
type AdminUsecase interface {
	GetUserStats() (*models.UserStatsResponse, error)
	UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest) (*models.UserResponse, error)
	UpdateUserStatus(id uint, req *models.AdminUpdateUserStatusRequest) (*models.UserResponse, error)
	ActivateUser(id uint) (*models.UserResponse, error)
	DeactivateUser(id uint) (*models.UserResponse, error)
	ResetUserPassword(id uint, newPassword string) error
}

// adminUsecase implements AdminUsecase interface
type adminUsecase struct {
	userRepo       repository.UserRepository
	departmentRepo repository.DepartmentRepository
}

// NewAdminUsecase creates a new admin usecase
func NewAdminUsecase(userRepo repository.UserRepository, departmentRepo repository.DepartmentRepository) AdminUsecase {
	return &adminUsecase{
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
	}
}

// GetUserStats retrieves user statistics
func (a *adminUsecase) GetUserStats() (*models.UserStatsResponse, error) {
	// Get total count
	total, err := a.userRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	// For more detailed stats, we would need additional methods in the repository
	// For now, return basic stats
	stats := &models.UserStatsResponse{
		TotalUsers:    int(total),
		ActiveUsers:   0, // TODO: Implement active user count
		InactiveUsers: 0, // TODO: Implement inactive user count
		OnlineUsers:   0, // TODO: Implement online user count
	}

	return stats, nil
}

// UpdateUserRole updates a user's role (admin only)
func (a *adminUsecase) UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest) (*models.UserResponse, error) {
	// Validate request
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	if !isValidRole(string(req.Role)) {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update role
	user.Role = req.Role

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user role: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// UpdateUserStatus updates a user's status (admin only)
func (a *adminUsecase) UpdateUserStatus(id uint, req *models.AdminUpdateUserStatusRequest) (*models.UserResponse, error) {
	// Validate request
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	if !isValidStatus(req.Status) {
		return nil, fmt.Errorf("invalid status: %s", req.Status)
	}

	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update status
	user.Status = req.Status

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user status: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// ActivateUser activates a user account
func (a *adminUsecase) ActivateUser(id uint) (*models.UserResponse, error) {
	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Activate user
	user.IsActive = true

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to activate user: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// DeactivateUser deactivates a user account
func (a *adminUsecase) DeactivateUser(id uint) (*models.UserResponse, error) {
	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Deactivate user
	user.IsActive = false
	user.Status = sharedmodels.StatusOffline // Set status to offline when deactivating

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to deactivate user: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// ResetUserPassword resets a user's password (admin only)
func (a *adminUsecase) ResetUserPassword(id uint, newPassword string) error {
	// Validate password
	if err := validatePasswordStrength(newPassword); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Hash new password
	hashedPassword, err := hashPasswordAdmin(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.HashedPassword = hashedPassword

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// validatePasswordStrength validates password strength for admin operations
func validatePasswordStrength(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Check minimum length
	if len(password) < 6 {
		return fmt.Errorf("password must be at least 6 characters long")
	}

	// Check maximum length
	if len(password) > 100 {
		return fmt.Errorf("password too long (max 100 characters)")
	}

	return nil
}

// hashPasswordAdmin hashes a password using bcrypt (admin specific to avoid conflicts)
func hashPasswordAdmin(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
