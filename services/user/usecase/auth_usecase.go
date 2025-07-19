package usecase

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthUsecase defines the interface for authentication business logic
type AuthUsecase interface {
	Register(req *models.CreateUserRequest) (*models.UserResponse, error)
	Login(email, password string) (*sharedmodels.LoginResponse, error)
	ValidateEmail(email string) error
	ValidatePassword(password string) error
}

// authUsecase implements AuthUsecase interface
type authUsecase struct {
	userRepo       repository.UserRepository
	departmentRepo repository.DepartmentRepository
	jwtConfig      *middleware.JWTConfig
}

// NewAuthUsecase creates a new auth usecase
func NewAuthUsecase(userRepo repository.UserRepository, departmentRepo repository.DepartmentRepository, jwtConfig *middleware.JWTConfig) AuthUsecase {
	return &authUsecase{
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
		jwtConfig:      jwtConfig,
	}
}

// Register handles user registration
func (a *authUsecase) Register(req *models.CreateUserRequest) (*models.UserResponse, error) {
	// Validate email format
	if err := a.ValidateEmail(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Validate password strength
	if err := a.ValidatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("invalid password: %w", err)
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	existingUser, err := a.userRepo.GetByEmail(req.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		// If it's not a "not found" error, it's a real database error
		if !strings.Contains(err.Error(), "user not found") {
			return nil, fmt.Errorf("failed to check existing user: %w", err)
		}
	}
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Validate department if provided
	if req.DepartmentID != nil {
		_, err := a.departmentRepo.GetByID(*req.DepartmentID)
		if err != nil {
			return nil, fmt.Errorf("invalid department: %w", err)
		}
	}

	// Hash password
	hashedPassword, err := a.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user model
	user := &models.User{
		Email:          req.Email,
		Name:           strings.TrimSpace(req.Name),
		HashedPassword: hashedPassword,
		DepartmentID:   req.DepartmentID,
		Position:       strings.TrimSpace(req.Position),
		Phone:          strings.TrimSpace(req.Phone),
	}

	// Set role if provided, otherwise use default (employee)
	if req.Role != "" {
		if !isValidRole(req.Role) {
			return nil, fmt.Errorf("invalid role: %s", req.Role)
		}
		user.Role = sharedmodels.Role(req.Role)
	} else {
		user.Role = sharedmodels.RoleEmployee
	}

	// Save user
	if err := a.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Get user with department for response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// If we can't get with department, just return the user without it
		return user.ToResponse(), nil
	}

	return userWithDept.ToResponse(), nil
}

// Login handles user authentication
func (a *authUsecase) Login(email, password string) (*sharedmodels.LoginResponse, error) {
	// Validate input
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get user by email
	user, err := a.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal whether user exists or not for security
		return nil, fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	// Verify password
	if err := a.verifyPassword(user.HashedPassword, password); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Update user status to online and last active time
	if err := a.updateUserLoginStatus(user); err != nil {
		// Log error but don't fail login
		// In production, you might want to log this error properly
	}

	// Get user with department for complete response
	userWithDept, err := a.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		userWithDept = user
	}

	// Generate JWT tokens
	tokens, err := middleware.GenerateTokens(user.ID, user.Email, user.Role, a.jwtConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Convert user to shared model format for response
	responseUser := convertUserToSharedModel(userWithDept)

	// Create login response
	response := &sharedmodels.LoginResponse{
		User:   *responseUser,
		Tokens: *tokens,
	}

	return response, nil
}

// updateUserLoginStatus updates user status and last active time on login
func (a *authUsecase) updateUserLoginStatus(user *models.User) error {
	// Update status to online
	user.Status = sharedmodels.StatusOnline

	// BeforeUpdate hook will automatically set LastActiveAt
	return a.userRepo.Update(user)
}

// convertUserToSharedModel converts service user model to shared user model
func convertUserToSharedModel(user *models.User) *sharedmodels.User {
	sharedUser := &sharedmodels.User{
		BaseModel:    user.BaseModel,
		Email:        user.Email,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		Avatar:       user.Avatar,
		Phone:        user.Phone,
		Position:     user.Position,
		LastActiveAt: user.LastActiveAt,
		IsActive:     user.IsActive,
	}

	// Set department as string if available
	if user.Department != nil {
		sharedUser.Department = user.Department.Name
	}

	return sharedUser
}

// ValidateEmail validates email format
func (a *authUsecase) ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Trim whitespace
	email = strings.TrimSpace(email)

	// Check length
	if len(email) > 255 {
		return fmt.Errorf("email too long (max 255 characters)")
	}

	// Simple email regex validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	// Additional checks
	if strings.Count(email, "@") != 1 {
		return fmt.Errorf("invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// ValidatePassword validates password strength
func (a *authUsecase) ValidatePassword(password string) error {
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

	// Check for at least one letter
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)
	if !hasLetter {
		return fmt.Errorf("password must contain at least one letter")
	}

	// Check for at least one number or symbol (optional but recommended)
	hasNumberOrSymbol := regexp.MustCompile(`[0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)
	if len(password) >= 8 && !hasNumberOrSymbol {
		return fmt.Errorf("password should contain at least one number or symbol for better security")
	}

	// Check for common weak passwords
	weakPasswords := []string{
		"password", "123456", "qwerty", "abc123", "password123",
		"admin", "letmein", "welcome", "monkey", "dragon",
	}

	lowerPassword := strings.ToLower(password)
	for _, weak := range weakPasswords {
		if lowerPassword == weak {
			return fmt.Errorf("password is too common, please choose a stronger password")
		}
	}

	return nil
}

// hashPassword hashes a password using bcrypt (private method for auth usecase)
func (a *authUsecase) hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// verifyPassword verifies a password against its hash (private method for auth usecase)
func (a *authUsecase) verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
