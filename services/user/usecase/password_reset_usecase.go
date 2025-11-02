package usecase

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/email"
	"tachyon-messenger/shared/logger"

	"golang.org/x/crypto/bcrypt"
)

// PasswordResetUsecase defines the interface for password reset business logic
type PasswordResetUsecase interface {
	InitiatePasswordReset(userID uint, adminID *uint) (*models.PasswordResetResponse, error)
	RequestPasswordResetByEmail(email string) error
	ValidateResetToken(token string) (*models.PublicPasswordResetResponse, error)
	ResetPassword(token string, req *models.ResetPasswordRequest) error
	ExpireOldResets() (int64, error)
}

// passwordResetUsecase implements PasswordResetUsecase interface
type passwordResetUsecase struct {
	passwordResetRepo repository.PasswordResetRepository
	userRepo          repository.UserRepository
	emailService      *email.EmailService
	authUsecase       AuthUsecase
}

// NewPasswordResetUsecase creates a new password reset usecase
func NewPasswordResetUsecase(
	passwordResetRepo repository.PasswordResetRepository,
	userRepo repository.UserRepository,
	emailService *email.EmailService,
	authUsecase AuthUsecase,
) PasswordResetUsecase {
	return &passwordResetUsecase{
		passwordResetRepo: passwordResetRepo,
		userRepo:          userRepo,
		emailService:      emailService,
		authUsecase:       authUsecase,
	}
}

// InitiatePasswordReset initiates a password reset for a user
func (u *passwordResetUsecase) InitiatePasswordReset(userID uint, adminID *uint) (*models.PasswordResetResponse, error) {
	// Get user
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("cannot reset password for inactive user")
	}

	// Cancel any existing pending reset tokens for this user
	if err := u.passwordResetRepo.CancelPendingResetsByUserID(userID); err != nil {
		logger.WithFields(map[string]interface{}{
			"user_id": userID,
			"error":   err.Error(),
		}).Warn("Failed to cancel pending resets, continuing anyway")
	}

	// Generate secure token
	token, err := generateSecureResetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate reset token: %w", err)
	}

	// Calculate expiration time (24 hours)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Create password reset
	passwordReset := &models.PasswordReset{
		Token:       token,
		UserID:      userID,
		Email:       user.Email,
		Status:      models.PasswordResetStatusPending,
		ExpiresAt:   expiresAt,
		CreatedByID: adminID,
	}

	// Save password reset
	if err := u.passwordResetRepo.Create(passwordReset); err != nil {
		return nil, fmt.Errorf("failed to create password reset: %w", err)
	}

	// Get password reset with relations for response
	passwordResetWithRelations, err := u.passwordResetRepo.GetWithRelations(passwordReset.ID)
	if err != nil {
		// Fallback to password reset without relations
		passwordResetWithRelations = passwordReset
	}

	// Generate reset link
	resetLink := generateResetLink(token)

	// Send password reset email
	if err := u.emailService.SendPasswordResetEmail(user.Email, resetLink, user.Name); err != nil {
		logger.WithFields(map[string]interface{}{
			"password_reset_id": passwordReset.ID,
			"user_id":           userID,
			"email":             user.Email,
			"error":             err.Error(),
		}).Error("Failed to send password reset email")
		// Don't fail the operation, just log it
	}

	response := passwordResetWithRelations.ToResponse()
	response.ResetLink = resetLink

	logger.WithFields(map[string]interface{}{
		"password_reset_id": passwordReset.ID,
		"user_id":           userID,
		"admin_id":          adminID,
		"expires_at":        expiresAt,
	}).Info("Password reset initiated")

	return response, nil
}

// RequestPasswordResetByEmail initiates a password reset by email (self-service)
func (u *passwordResetUsecase) RequestPasswordResetByEmail(email string) error {
	// Get user by email
	user, err := u.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal if user exists - just silently return
		logger.WithFields(map[string]interface{}{
			"email": email,
		}).Info("Password reset requested for non-existent email")
		return nil
	}

	// Check if user is active
	if !user.IsActive {
		// Don't reveal user status - just silently return
		logger.WithFields(map[string]interface{}{
			"email":   email,
			"user_id": user.ID,
		}).Info("Password reset requested for inactive user")
		return nil
	}

	// Initiate password reset (with nil adminID for self-service)
	_, err = u.InitiatePasswordReset(user.ID, nil)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"email":   email,
			"user_id": user.ID,
			"error":   err.Error(),
		}).Error("Failed to initiate self-service password reset")
		// Don't return error to prevent revealing internal issues
		return nil
	}

	logger.WithFields(map[string]interface{}{
		"email":   email,
		"user_id": user.ID,
	}).Info("Self-service password reset initiated")

	return nil
}

// ValidateResetToken validates a password reset token
func (u *passwordResetUsecase) ValidateResetToken(token string) (*models.PublicPasswordResetResponse, error) {
	// Get password reset by token
	passwordReset, err := u.passwordResetRepo.GetByToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid reset token")
	}

	// Check if token is valid
	if !passwordReset.IsValid() {
		// Update status if expired
		if passwordReset.IsExpired() && passwordReset.Status == models.PasswordResetStatusPending {
			passwordReset.Status = models.PasswordResetStatusExpired
			_ = u.passwordResetRepo.Update(passwordReset)
		}
		return nil, fmt.Errorf("reset token is expired or already used")
	}

	return &models.PublicPasswordResetResponse{
		Valid:     true,
		Email:     passwordReset.Email,
		ExpiresAt: passwordReset.ExpiresAt,
	}, nil
}

// ResetPassword resets a user's password using a reset token
func (u *passwordResetUsecase) ResetPassword(token string, req *models.ResetPasswordRequest) error {
	// Validate passwords match
	if req.Password != req.ConfirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	// Validate password strength
	if err := u.authUsecase.ValidatePassword(req.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	// Get password reset by token
	passwordReset, err := u.passwordResetRepo.GetByToken(token)
	if err != nil {
		return fmt.Errorf("invalid reset token")
	}

	// Check if token is valid
	if !passwordReset.IsValid() {
		// Update status if expired
		if passwordReset.IsExpired() && passwordReset.Status == models.PasswordResetStatusPending {
			passwordReset.Status = models.PasswordResetStatusExpired
			_ = u.passwordResetRepo.Update(passwordReset)
		}
		return fmt.Errorf("reset token is expired or already used")
	}

	// Get user
	user, err := u.userRepo.GetByID(passwordReset.UserID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Hash new password
	hashedPassword, err := hashPasswordReset(req.Password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user password
	user.HashedPassword = &hashedPassword
	now := time.Now()
	user.PasswordChangedAt = &now

	if err := u.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark reset token as used
	passwordReset.Status = models.PasswordResetStatusUsed
	passwordReset.UsedAt = &now

	if err := u.passwordResetRepo.Update(passwordReset); err != nil {
		logger.WithFields(map[string]interface{}{
			"password_reset_id": passwordReset.ID,
			"error":             err.Error(),
		}).Error("Failed to mark reset token as used")
	}

	logger.WithFields(map[string]interface{}{
		"password_reset_id": passwordReset.ID,
		"user_id":           user.ID,
		"email":             user.Email,
	}).Info("Password reset successfully")

	return nil
}

// ExpireOldResets marks old password reset tokens as expired
func (u *passwordResetUsecase) ExpireOldResets() (int64, error) {
	count, err := u.passwordResetRepo.ExpireOldResets()
	if err != nil {
		return 0, fmt.Errorf("failed to expire old resets: %w", err)
	}

	if count > 0 {
		logger.WithFields(map[string]interface{}{
			"count": count,
		}).Info("Expired old password reset tokens")
	}

	return count, nil
}

// generateSecureResetToken generates a cryptographically secure reset token
func generateSecureResetToken() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Encode to base64 URL-safe string
	token := base64.URLEncoding.EncodeToString(bytes)

	// Remove padding
	token = strings.TrimRight(token, "=")

	return token, nil
}

// generateResetLink generates a password reset link
func generateResetLink(token string) string {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:8093"
	}

	return fmt.Sprintf("%s/reset-password/%s", frontendURL, token)
}

// hashPasswordReset hashes a password using bcrypt
func hashPasswordReset(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
