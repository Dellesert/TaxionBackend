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
	UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest, adminRole sharedmodels.Role) (*models.UserResponse, error)
	UpdateUserStatus(id uint, req *models.AdminUpdateUserStatusRequest) (*models.UserResponse, error)
	ActivateUser(id uint) (*models.UserResponse, error)
	DeactivateUser(id uint) (*models.UserResponse, error)
	ResetUserPassword(id uint, newPassword string) error
	UpdateUser2FAStatus(id uint, req *models.AdminUpdate2FARequest) (*models.UserResponse, error)
	ImportUsersFromCSV(csvRows []models.CSVUserRow) (*models.ImportUsersResponse, error)
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
func (a *adminUsecase) UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest, adminRole sharedmodels.Role) (*models.UserResponse, error) {
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

	// Check permissions: Only super_admin can assign/remove admin or super_admin roles
	isTargetingAdminRole := req.Role == sharedmodels.RoleAdmin || req.Role == sharedmodels.RoleSuperAdmin
	isCurrentAdminRole := user.Role == sharedmodels.RoleAdmin || user.Role == sharedmodels.RoleSuperAdmin

	if (isTargetingAdminRole || isCurrentAdminRole) && adminRole != sharedmodels.RoleSuperAdmin {
		return nil, fmt.Errorf("only super admin can assign or remove admin roles")
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
	user.HashedPassword = &hashedPassword

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

// UpdateUser2FAStatus enables or disables 2FA for a specific user
func (a *adminUsecase) UpdateUser2FAStatus(id uint, req *models.AdminUpdate2FARequest) (*models.UserResponse, error) {
	// Get user to verify exists
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update 2FA status
	if err := a.userRepo.UpdateTwoFactorStatus(id, req.TwoFactorEnabled); err != nil {
		return nil, fmt.Errorf("failed to update 2FA status: %w", err)
	}

	// Update local user object
	user.TwoFactorEnabled = req.TwoFactorEnabled

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// ImportUsersFromCSV imports users from CSV rows with validation
func (a *adminUsecase) ImportUsersFromCSV(csvRows []models.CSVUserRow) (*models.ImportUsersResponse, error) {
	response := &models.ImportUsersResponse{
		TotalRows:    len(csvRows),
		SuccessCount: 0,
		ErrorCount:   0,
		SuccessUsers: []*models.UserResponse{},
		Errors:       []models.ImportError{},
	}

	for i, row := range csvRows {
		rowNum := i + 2 // +2 because row 1 is header, and array is 0-indexed

		// Validate and create user
		user, err := a.validateAndCreateUserFromCSV(row, rowNum)
		if err != nil {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.ImportError{
				Row:     rowNum,
				Email:   row.Email,
				Message: err.Error(),
			})
			continue
		}

		response.SuccessCount++
		response.SuccessUsers = append(response.SuccessUsers, user)
	}

	return response, nil
}

// validateAndCreateUserFromCSV validates a CSV row and creates a user
func (a *adminUsecase) validateAndCreateUserFromCSV(row models.CSVUserRow, rowNum int) (*models.UserResponse, error) {
	// Trim all fields
	email := strings.TrimSpace(row.Email)
	name := strings.TrimSpace(row.Name)
	password := strings.TrimSpace(row.Password)
	role := strings.TrimSpace(row.Role)
	phone := strings.TrimSpace(row.Phone)
	position := strings.TrimSpace(row.Position)
	departmentIDStr := strings.TrimSpace(row.DepartmentID)

	// Validate required fields
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Validate email format
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return nil, fmt.Errorf("invalid email format")
	}

	// Check if user already exists
	existingUser, err := a.userRepo.GetByEmail(email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", email)
	}

	// Validate password strength
	if err := validatePasswordStrength(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	// Set default role if not provided
	if role == "" {
		role = string(sharedmodels.RoleEmployee)
	}

	// Validate role
	if !isValidRole(role) {
		return nil, fmt.Errorf("invalid role: %s (must be one of: employee, department_head, admin, super_admin)", role)
	}

	// Parse department ID if provided
	var departmentID *uint
	if departmentIDStr != "" {
		var deptID uint
		_, err := fmt.Sscanf(departmentIDStr, "%d", &deptID)
		if err != nil {
			return nil, fmt.Errorf("invalid department_id: must be a number")
		}

		// Verify department exists
		_, err = a.departmentRepo.GetByID(deptID)
		if err != nil {
			return nil, fmt.Errorf("department with ID %d not found", deptID)
		}

		departmentID = &deptID
	}

	// Hash password
	hashedPassword, err := hashPasswordAdmin(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		Email:          email,
		Name:           name,
		HashedPassword: &hashedPassword,
		Role:           sharedmodels.Role(role),
		Status:         sharedmodels.StatusOffline,
		DepartmentID:   departmentID,
		Phone:          phone,
		Position:       position,
		IsActive:       true,
	}

	// Save user
	if err := a.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}
