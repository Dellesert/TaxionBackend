package usecase

import (
	"errors"
	"fmt"

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
	GetUsersByIDs(ids []uint, currentUserRole string) ([]*models.UserResponse, error)
	UpdateUser(id uint, req *models.UpdateUserRequest) (*models.UserResponse, error)
	DeleteUser(id uint) error
}

// userUsecase implements UserUsecase interface
type userUsecase struct {
	userRepo repository.UserRepository
}

// NewUserUsecase creates a new user usecase
func NewUserUsecase(userRepo repository.UserRepository) UserUsecase {
	return &userUsecase{
		userRepo: userRepo,
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

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user model
	hashedPwd := string(hashedPassword)
	user := &models.User{
		Email:           req.Email,
		Name:            req.Name,
		FirstName:       req.FirstName,
		LastName:        req.LastName,
		MiddleName:      req.MiddleName,
		BirthDate:       req.BirthDate,
		HashedPassword:  &hashedPwd,
		DepartmentID:    req.DepartmentID,
		SubdepartmentID: req.SubdepartmentID,
		Position:        req.Position,
		Phone:           req.Phone,
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

// UpdateUser updates an existing user
func (u *userUsecase) UpdateUser(id uint, req *models.UpdateUserRequest) (*models.UserResponse, error) {
	// Get existing user
	user, err := u.userRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Update fields if provided
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		user.LastName = *req.LastName
	}
	if req.MiddleName != nil {
		user.MiddleName = *req.MiddleName
	}
	if req.BirthDate != nil {
		user.BirthDate = req.BirthDate
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Position != nil {
		user.Position = *req.Position
	}
	if req.DepartmentID != nil {
		// If DepartmentID is 0, set to nil to remove from department
		if *req.DepartmentID == 0 {
			user.DepartmentID = nil
		} else {
			user.DepartmentID = req.DepartmentID
		}
	}
	if req.SubdepartmentID != nil {
		// If SubdepartmentID is 0, set to nil to remove from subdepartment
		if *req.SubdepartmentID == 0 {
			user.SubdepartmentID = nil
		} else {
			user.SubdepartmentID = req.SubdepartmentID
		}
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	// Save updated user
	if err := u.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user.ToResponse(), nil
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
