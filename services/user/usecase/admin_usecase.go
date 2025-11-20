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

// AdminUsecase defines the interface for admin business logic
type AdminUsecase interface {
	GetUserStats() (*models.UserStatsResponse, error)
	UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest, adminID uint, adminRole sharedmodels.Role) (*models.UserResponse, error)
	UpdateUserStatus(id uint, req *models.AdminUpdateUserStatusRequest, adminID uint) (*models.UserResponse, error)
	ActivateUser(id uint, adminID uint) (*models.UserResponse, error)
	DeactivateUser(id uint, adminID uint, adminRole sharedmodels.Role) (*models.UserResponse, error)
	AssignDepartmentToUser(id uint, departmentID *uint) (*models.UserResponse, error)
	ResetUserPassword(id uint, newPassword string) error
	UpdateUser2FAStatus(id uint, req *models.AdminUpdate2FARequest) (*models.UserResponse, error)
	ImportUsersFromCSV(csvRows []models.CSVUserRow) (*models.ImportUsersResponse, error)
}

// adminUsecase implements AdminUsecase interface
type adminUsecase struct {
	userRepo       repository.UserRepository
	departmentRepo repository.DepartmentRepository
	settingsRepo   repository.SettingsRepository
}

// NewAdminUsecase creates a new admin usecase
func NewAdminUsecase(userRepo repository.UserRepository, departmentRepo repository.DepartmentRepository, settingsRepo repository.SettingsRepository) AdminUsecase {
	return &adminUsecase{
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
		settingsRepo:   settingsRepo,
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
func (a *adminUsecase) UpdateUserRole(id uint, req *models.AdminUpdateUserRoleRequest, adminID uint, adminRole sharedmodels.Role) (*models.UserResponse, error) {
	// Validate request
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	if !isValidRole(string(req.Role)) {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Security check 1: Prevent self-modification
	if id == adminID {
		return nil, fmt.Errorf("cannot modify your own role")
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

	// Security check 2: Prevent removing the last super admin
	if user.Role == sharedmodels.RoleSuperAdmin && req.Role != sharedmodels.RoleSuperAdmin {
		superAdminCount, err := a.userRepo.CountByRole(string(sharedmodels.RoleSuperAdmin))
		if err != nil {
			return nil, fmt.Errorf("failed to check super admin count: %w", err)
		}

		if superAdminCount <= 1 {
			return nil, fmt.Errorf("cannot remove the last super admin from the system")
		}
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
func (a *adminUsecase) UpdateUserStatus(id uint, req *models.AdminUpdateUserStatusRequest, adminID uint) (*models.UserResponse, error) {
	// Validate request
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	if !isValidStatus(req.Status) {
		return nil, fmt.Errorf("invalid status: %s", req.Status)
	}

	// Security check: Prevent self-modification
	if id == adminID {
		return nil, fmt.Errorf("cannot modify your own status")
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
func (a *adminUsecase) ActivateUser(id uint, adminID uint) (*models.UserResponse, error) {
	// Security check: Prevent self-modification
	if id == adminID {
		return nil, fmt.Errorf("cannot modify your own activation status")
	}

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
func (a *adminUsecase) DeactivateUser(id uint, adminID uint, adminRole sharedmodels.Role) (*models.UserResponse, error) {
	// Security check 1: Prevent self-deactivation
	if id == adminID {
		return nil, fmt.Errorf("cannot deactivate your own account")
	}

	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Security check 2: Prevent deactivating the last active super admin
	if user.Role == sharedmodels.RoleSuperAdmin && user.IsActive {
		// Count active super admins
		activeSuperAdminCount, err := a.userRepo.CountByRoleAndActive(string(sharedmodels.RoleSuperAdmin), true)
		if err != nil {
			return nil, fmt.Errorf("failed to check active super admin count: %w", err)
		}

		// If this is the only active super admin, prevent deactivation
		if activeSuperAdminCount <= 1 {
			return nil, fmt.Errorf("cannot deactivate the last active super admin in the system")
		}
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

// AssignDepartmentToUser assigns or removes a department from a user
func (a *adminUsecase) AssignDepartmentToUser(id uint, departmentID *uint) (*models.UserResponse, error) {
	// Get user
	user, err := a.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Validate department exists if departmentID is not nil
	if departmentID != nil && *departmentID > 0 {
		_, err := a.departmentRepo.GetByID(*departmentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("department not found")
			}
			return nil, fmt.Errorf("failed to get department: %w", err)
		}
	}

	// Assign or remove department
	user.DepartmentID = departmentID

	// Save updated user
	if err := a.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to assign department to user: %w", err)
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
	if err := a.validatePasswordStrength(newPassword); err != nil {
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

// validatePasswordStrength validates password strength for admin operations based on system settings
func (a *adminUsecase) validatePasswordStrength(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Get security settings from database
	settings, err := a.settingsRepo.GetOrCreate()
	if err != nil {
		// Fallback to reasonable defaults
		settings = &models.SystemSettings{
			MinPasswordLength:         8,
			RequirePasswordComplexity: true,
		}
	}

	// Check minimum length based on security settings
	if len(password) < settings.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters long", settings.MinPasswordLength)
	}

	// Check maximum length
	if len(password) > 100 {
		return fmt.Errorf("password too long (max 100 characters)")
	}

	// If complexity is required, enforce additional rules
	if settings.RequirePasswordComplexity {
		// Check for at least one letter
		hasLetter := false
		hasNumber := false
		hasSymbol := false

		for _, char := range password {
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
				hasLetter = true
			} else if char >= '0' && char <= '9' {
				hasNumber = true
			} else {
				hasSymbol = true
			}
		}

		if !hasLetter {
			return fmt.Errorf("password must contain at least one letter")
		}
		if !hasNumber && !hasSymbol {
			return fmt.Errorf("password must contain at least one number or symbol")
		}
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
// Users are created as inactive without password - they will activate via invitation
func (a *adminUsecase) validateAndCreateUserFromCSV(row models.CSVUserRow, rowNum int) (*models.UserResponse, error) {
	// Trim all fields
	email := strings.TrimSpace(row.Email)
	name := strings.TrimSpace(row.Name)
	firstName := strings.TrimSpace(row.FirstName)
	lastName := strings.TrimSpace(row.LastName)
	middleName := strings.TrimSpace(row.MiddleName)
	birthDateStr := strings.TrimSpace(row.BirthDate)
	password := strings.TrimSpace(row.Password) // Optional now
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
	if firstName == "" {
		return nil, fmt.Errorf("first_name is required")
	}
	if lastName == "" {
		return nil, fmt.Errorf("last_name is required")
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

	// Password is now optional - validate only if provided
	if password != "" {
		if err := a.validatePasswordStrength(password); err != nil {
			return nil, fmt.Errorf("password validation failed: %w", err)
		}
	}

	// Set default role if not provided
	if role == "" {
		role = string(sharedmodels.RoleEmployee)
	}

	// Validate role
	if !isValidRole(role) {
		return nil, fmt.Errorf("invalid role: %s (must be one of: employee, department_head, admin, super_admin)", role)
	}

	// Parse birth date if provided (format: YYYY-MM-DD)
	var birthDate *time.Time
	if birthDateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", birthDateStr)
		if err != nil {
			return nil, fmt.Errorf("invalid birth_date format (expected YYYY-MM-DD): %s", birthDateStr)
		}
		birthDate = &parsedDate
	}

	// Parse department ID if provided (supports both numeric ID and department name)
	var departmentID *uint
	if departmentIDStr != "" {
		// Try to parse as numeric ID first
		var deptID uint
		_, err := fmt.Sscanf(departmentIDStr, "%d", &deptID)
		if err == nil {
			// It's a number - verify department exists by ID
			_, err = a.departmentRepo.GetByID(deptID)
			if err != nil {
				return nil, fmt.Errorf("department with ID %d not found", deptID)
			}
			departmentID = &deptID
		} else {
			// Not a number - try to find by name
			dept, err := a.departmentRepo.GetByName(departmentIDStr)
			if err != nil {
				return nil, fmt.Errorf("department '%s' not found (use numeric ID or exact department name)", departmentIDStr)
			}
			departmentID = &dept.ID
		}
	}

	// Hash password only if provided, otherwise leave it nil
	var hashedPassword *string
	var passwordChangedAt *time.Time
	if password != "" {
		hashed, err := hashPasswordAdmin(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		hashedPassword = &hashed
		now := time.Now()
		passwordChangedAt = &now
	}

	// Create user as inactive - they will activate via invitation
	user := &models.User{
		Email:             email,
		Name:              name,
		FirstName:         firstName,
		LastName:          lastName,
		MiddleName:        middleName,
		BirthDate:         birthDate,
		HashedPassword:    hashedPassword,    // nil if no password provided
		PasswordChangedAt: passwordChangedAt, // nil if no password provided
		Role:              sharedmodels.Role(role),
		Status:            sharedmodels.StatusOffline,
		DepartmentID:      departmentID,
		Phone:             phone,
		Position:          position,
		IsActive:          false, // Inactive until they accept invitation
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
