package usecase

import (
	"errors"
	"fmt"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserUsecase defines the interface for user business logic
type UserUsecase interface {
	CreateUser(req *models.CreateUserRequest) (*models.UserResponse, error)
	GetUser(id uint) (*models.UserResponse, error)
	GetUsers(limit, offset int) ([]*models.UserResponse, int64, error)
	GetUsersWithFilters(limit, offset int, departmentID *uint, isActive *bool, role *string, currentUserRole string) ([]*models.UserResponse, int64, error)
	GetUsersWithFiltersAdvanced(limit, offset int, departmentID *uint, isActive *bool, roles []string, excludeRoles []string, currentUserRole string, currentUserDeptID *uint, sortBy string, sortOrder string, searchQuery string) ([]*models.UserResponse, int64, error)
	GetUsersByIDs(ids []uint, currentUserRole string) ([]*models.UserResponse, error)
	GetUsersByDepartment(departmentID uint) ([]*models.UserResponse, error)
	UpdateUser(id uint, req *models.UpdateUserRequest) (*models.UserResponse, error)
	DeleteUser(id uint) error
	ResetAllOnlineStatuses() (int64, error)
	CleanupDisconnectedStatuses(connectedUserIDs []uint) (int64, error)
	GetUsersWithBirthdays() ([]*models.BirthdayUserInfo, error)
}

// userUsecase implements UserUsecase interface
type userUsecase struct {
	userRepo     repository.UserRepository
	settingsRepo repository.SettingsRepository
}

// NewUserUsecase creates a new user usecase
func NewUserUsecase(userRepo repository.UserRepository, settingsRepo repository.SettingsRepository) UserUsecase {
	return &userUsecase{
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
	}
}

// CreateUser creates a new user
func (u *userUsecase) CreateUser(req *models.CreateUserRequest) (*models.UserResponse, error) {
	// Check if user already exists
	existingUser, err := u.userRepo.GetByEmail(req.Email)
	if err != nil {
		// Only return error if it's NOT a "not found" error
		// GetByEmail returns "user not found" error when user doesn't exist, which is expected here
		if !errors.Is(err, gorm.ErrRecordNotFound) && err.Error() != "user not found" {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}
		// User not found is expected - continue with creation
	} else if existingUser != nil {
		// User found - cannot create duplicate
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Validate password based on security settings
	if err := u.validatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("invalid password: %w", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user model
	hashedPwd := string(hashedPassword)
	now := time.Now()
	user := &models.User{
		Email:             req.Email,
		Name:              req.Name,
		FirstName:         req.FirstName,
		LastName:          req.LastName,
		MiddleName:        req.MiddleName,
		BirthDate:         req.BirthDate,
		HashedPassword:    &hashedPwd,
		PasswordChangedAt: &now,
		DepartmentID:      req.DepartmentID,
		SubdepartmentID:   req.SubdepartmentID,
		Position:          req.Position,
		Phone:             req.Phone,
		Color:             req.Color,
	}

	// Set role if provided, otherwise use default
	if req.Role != "" {
		user.Role = sharedmodels.Role(req.Role)
	}

	// Save user
	if err := u.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user.ToResponse(), nil
}

// GetUser retrieves a user by ID
func (u *userUsecase) GetUser(id uint) (*models.UserResponse, error) {
	user, err := u.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user.ToResponse(), nil
}

// GetUsers retrieves all users with pagination
func (u *userUsecase) GetUsers(limit, offset int) ([]*models.UserResponse, int64, error) {
	// Set default pagination values
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	users, err := u.userRepo.GetAllWithDepartments(limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	// Get total count
	total, err := u.userRepo.Count()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Convert to response format
	responses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}

// GetUsersWithFilters retrieves users with pagination and optional filters
func (u *userUsecase) GetUsersWithFilters(limit, offset int, departmentID *uint, isActive *bool, role *string, currentUserRole string) ([]*models.UserResponse, int64, error) {
	// Set default pagination values
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	users, err := u.userRepo.GetAllWithDepartmentsFiltered(limit, offset, departmentID, isActive)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	// Filter super_admin users - only super_admin can see other super_admins
	if currentUserRole != string(sharedmodels.RoleSuperAdmin) {
		filteredUsers := make([]*models.User, 0)
		for _, user := range users {
			if user.Role != sharedmodels.RoleSuperAdmin {
				filteredUsers = append(filteredUsers, user)
			}
		}
		users = filteredUsers
	}

	// Filter by role if specified (done in-memory for now)
	if role != nil && *role != "" {
		filteredUsers := make([]*models.User, 0)
		for _, user := range users {
			if string(user.Role) == *role {
				filteredUsers = append(filteredUsers, user)
			}
		}
		users = filteredUsers
	}

	// Get total count with filters
	total, err := u.userRepo.CountWithFilters(departmentID, isActive)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Adjust total based on filtering
	if currentUserRole != string(sharedmodels.RoleSuperAdmin) || (role != nil && *role != "") {
		total = int64(len(users))
	}

	// Convert to response format
	responses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}

// GetUsersWithFiltersAdvanced retrieves users with advanced filtering options
func (u *userUsecase) GetUsersWithFiltersAdvanced(limit, offset int, departmentID *uint, isActive *bool, roles []string, excludeRoles []string, currentUserRole string, currentUserDeptID *uint, sortBy string, sortOrder string, searchQuery string) ([]*models.UserResponse, int64, error) {
	// Set default pagination values
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Non-super admins cannot see super admins
	if currentUserRole != string(sharedmodels.RoleSuperAdmin) {
		excludeRoles = append(excludeRoles, string(sharedmodels.RoleSuperAdmin))
	}

	users, err := u.userRepo.GetAllWithDepartmentsFilteredAdvanced(limit, offset, departmentID, isActive, roles, excludeRoles, sortBy, sortOrder, searchQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	// Get total count with filters
	total, err := u.userRepo.CountWithFiltersAdvanced(departmentID, isActive, roles, excludeRoles, searchQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Convert to response format
	responses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, total, nil
}

// UpdateUser updates an existing user's profile information
// Note: Status can be updated via internal endpoints for presence tracking (online/offline)
// IsActive and Role are managed through dedicated secure admin endpoints
func (u *userUsecase) UpdateUser(id uint, req *models.UpdateUserRequest) (*models.UserResponse, error) {
	// Check if user exists first
	existingUser, err := u.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Build a map of fields to update (selective update to avoid changing protected fields)
	updates := make(map[string]interface{})

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.MiddleName != nil {
		updates["middle_name"] = *req.MiddleName
	}
	if req.BirthDate != nil {
		updates["birth_date"] = req.BirthDate
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}
	if req.AvatarThumbnail != nil {
		updates["avatar_thumbnail"] = *req.AvatarThumbnail
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Position != nil {
		updates["position"] = *req.Position
	}
	if req.Color != nil {
		updates["color"] = *req.Color
	}
	if req.DepartmentID != nil {
		if *req.DepartmentID == 0 {
			updates["department_id"] = nil
		} else {
			updates["department_id"] = *req.DepartmentID
		}
	}
	if req.SubdepartmentID != nil {
		if *req.SubdepartmentID == 0 {
			updates["subdepartment_id"] = nil
		} else {
			updates["subdepartment_id"] = *req.SubdepartmentID
		}
	}
	// Allow status updates for presence tracking (internal use only)
	if req.Status != nil {
		updates["status"] = *req.Status
		// Always update last_active_at when status changes
		now := time.Now()
		updates["last_active_at"] = &now
	}

	// If no fields to update, return current user
	if len(updates) == 0 {
		return existingUser.ToResponse(), nil
	}

	// Perform selective update - this ensures Status, IsActive, and Role are NEVER touched
	// Even if someone somehow passes them in the request, they won't be updated
	if err := u.userRepo.UpdateFields(id, updates); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Get updated user with department for response
	updatedUser, err := u.userRepo.GetWithDepartment(id)
	if err != nil {
		// Fallback to basic user data
		user, _ := u.userRepo.GetByID(id)
		return user.ToResponse(), nil
	}

	return updatedUser.ToResponse(), nil
}

// DeleteUser deletes a user by ID
func (u *userUsecase) DeleteUser(id uint) error {
	// Check if user exists
	_, err := u.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Delete user
	if err := u.userRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// GetUsersByIDs retrieves multiple users by their IDs
func (u *userUsecase) GetUsersByIDs(ids []uint, currentUserRole string) ([]*models.UserResponse, error) {
	if len(ids) == 0 {
		return []*models.UserResponse{}, nil
	}

	users, err := u.userRepo.GetByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IDs: %w", err)
	}

	// Filter super_admin users - only super_admin can see other super_admins
	if currentUserRole != string(sharedmodels.RoleSuperAdmin) {
		filteredUsers := make([]*models.User, 0)
		for _, user := range users {
			if user.Role != sharedmodels.RoleSuperAdmin {
				filteredUsers = append(filteredUsers, user)
			}
		}
		users = filteredUsers
	}

	// Convert to response format
	responses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, nil
}

// GetUsersByDepartment retrieves all active users in a specific department
func (u *userUsecase) GetUsersByDepartment(departmentID uint) ([]*models.UserResponse, error) {
	users, err := u.userRepo.GetUsersByDepartment(departmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by department: %w", err)
	}

	// Convert to response format
	responses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		responses[i] = user.ToResponse()
	}

	return responses, nil
}

// validatePassword validates password strength based on system settings
func (u *userUsecase) validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Get security settings from database
	settings, err := u.settingsRepo.GetOrCreate()
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

// ResetAllOnlineStatuses resets all users with "online" status to "offline"
// This is useful for cleaning up stuck statuses after service restarts
func (u *userUsecase) ResetAllOnlineStatuses() (int64, error) {
	count, err := u.userRepo.ResetAllOnlineStatuses()
	if err != nil {
		return 0, fmt.Errorf("failed to reset online statuses: %w", err)
	}
	return count, nil
}

// CleanupDisconnectedStatuses sets users to offline if they are marked as online but not in the connected users list
func (u *userUsecase) CleanupDisconnectedStatuses(connectedUserIDs []uint) (int64, error) {
	count, err := u.userRepo.CleanupDisconnectedStatuses(connectedUserIDs)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup disconnected statuses: %w", err)
	}
	return count, nil
}

// GetUsersWithBirthdays retrieves all active users who have a birth_date set
func (u *userUsecase) GetUsersWithBirthdays() ([]*models.BirthdayUserInfo, error) {
	users, err := u.userRepo.GetUsersWithBirthdays()
	if err != nil {
		return nil, fmt.Errorf("failed to get users with birthdays: %w", err)
	}

	result := make([]*models.BirthdayUserInfo, 0, len(users))
	for _, user := range users {
		if user.BirthDate == nil {
			continue
		}
		info := &models.BirthdayUserInfo{
			ID:           user.ID,
			Name:         user.Name,
			FirstName:    user.FirstName,
			LastName:     user.LastName,
			Avatar:       user.Avatar,
			BirthDate:    user.BirthDate.Format("2006-01-02"),
			DepartmentID: user.DepartmentID,
		}
		result = append(result, info)
	}

	return result, nil
}
