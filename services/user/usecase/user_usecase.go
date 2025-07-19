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
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user model
	user := &models.User{
		Email:          req.Email,
		Name:           req.Name,
		HashedPassword: string(hashedPassword),
		DepartmentID:   req.DepartmentID,
		Position:       req.Position,
		Phone:          req.Phone,
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

	users, err := u.userRepo.GetAll(limit, offset)
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
		user.DepartmentID = req.DepartmentID
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
