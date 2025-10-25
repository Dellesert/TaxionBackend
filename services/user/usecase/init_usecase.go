package usecase

import (
	"fmt"
	"os"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/logger"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
)

// InitUsecase handles initialization tasks
type InitUsecase interface {
	InitializeSuperAdmin() error
}

type initUsecase struct {
	userRepo repository.UserRepository
}

// NewInitUsecase creates a new init usecase
func NewInitUsecase(userRepo repository.UserRepository) InitUsecase {
	return &initUsecase{
		userRepo: userRepo,
	}
}

// InitializeSuperAdmin creates super admin if not exists
func (u *initUsecase) InitializeSuperAdmin() error {
	// Check if super admin already exists
	exists, err := u.userRepo.SuperAdminExists()
	if err != nil {
		return fmt.Errorf("failed to check super admin existence: %w", err)
	}

	if exists {
		logger.Info("Super admin already exists, skipping initialization")
		return nil
	}

	// Get super admin credentials from environment
	email := os.Getenv("SUPER_ADMIN_EMAIL")
	password := os.Getenv("SUPER_ADMIN_PASSWORD")
	name := os.Getenv("SUPER_ADMIN_NAME")

	// Set defaults if not provided
	if email == "" {
		email = "superadmin@taxion.local"
		logger.Warn("SUPER_ADMIN_EMAIL not set, using default: superadmin@taxion.local")
	}
	if password == "" {
		password = "ChangeMe123!@#SuperSecure"
		logger.Warn("SUPER_ADMIN_PASSWORD not set, using default (PLEASE CHANGE IMMEDIATELY!)")
	}
	if name == "" {
		name = "Super Administrator"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create super admin user
	superAdmin := &models.User{
		Email:              email,
		Name:               name,
		HashedPassword:     string(hashedPassword),
		Role:               sharedmodels.RoleSuperAdmin,
		Status:             sharedmodels.StatusOffline,
		IsActive:           true,
		MustChangePassword: true, // Force password change on first login
	}

	if err := u.userRepo.Create(superAdmin); err != nil {
		return fmt.Errorf("failed to create super admin: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"email": email,
		"name":  name,
	}).Info("Super admin created successfully - PLEASE CHANGE PASSWORD ON FIRST LOGIN")

	return nil
}
