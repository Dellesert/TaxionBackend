package usecase

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/email"
	"tachyon-messenger/shared/logger"

	"golang.org/x/crypto/bcrypt"
)

// TwoFAUsecase defines interface for 2FA business logic
type TwoFAUsecase interface {
	SendCode(email, password, ipAddress, userAgent string) error
	VerifyCode(email, code string) (*models.User, error)
}

// twoFAUsecase implements TwoFAUsecase interface
type twoFAUsecase struct {
	userRepo   repository.UserRepository
	twoFARepo  repository.TwoFARepository
	emailSvc   *email.EmailService
	authUsecase AuthUsecase
}

// NewTwoFAUsecase creates a new 2FA usecase
func NewTwoFAUsecase(
	userRepo repository.UserRepository,
	twoFARepo repository.TwoFARepository,
	emailSvc *email.EmailService,
	authUsecase AuthUsecase,
) TwoFAUsecase {
	return &twoFAUsecase{
		userRepo:    userRepo,
		twoFARepo:   twoFARepo,
		emailSvc:    emailSvc,
		authUsecase: authUsecase,
	}
}

// SendCode validates credentials and sends a 2FA code via email
func (u *twoFAUsecase) SendCode(email, password, ipAddress, userAgent string) error {
	// Validate input
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get user by email
	user, err := u.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal whether user exists for security
		return fmt.Errorf("invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return fmt.Errorf("user account is deactivated")
	}

	// Check if 2FA is enabled for this user
	if !user.TwoFactorEnabled {
		return fmt.Errorf("two factor authentication is not enabled for this account")
	}

	// Verify password using auth usecase
	if err := u.authUsecase.ValidatePassword(password); err != nil {
		return fmt.Errorf("invalid password format")
	}

	// Verify password hash (we need to expose this in auth usecase or recreate the logic)
	// For now, we'll use a simple bcrypt check
	hashedPwd := ""
	if user.HashedPassword != nil {
		hashedPwd = *user.HashedPassword
	}
	if err := verifyPasswordHash(hashedPwd, password); err != nil {
		return fmt.Errorf("invalid email or password")
	}

	// Generate 6-digit code
	code, err := generateSecureCode(6)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to generate 2FA code")
		return fmt.Errorf("failed to generate verification code")
	}

	// Delete any existing codes for this user (cleanup)
	if err := u.twoFARepo.DeleteByUserID(user.ID); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to delete old 2FA codes")
		// Don't fail, just log
	}

	// Create 2FA code record
	twoFACode := &models.TwoFactorCode{
		UserID:    user.ID,
		Code:      code,
		Email:     email,
		ExpiresAt: time.Now().Add(5 * time.Minute), // Code expires in 5 minutes
		Verified:  false,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	if err := u.twoFARepo.Create(twoFACode); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to save 2FA code")
		return fmt.Errorf("failed to create verification code")
	}

	// Send email with code
	if err := u.emailSvc.Send2FACode(email, code, user.Name); err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"email": email,
			"code":  code, // DEVELOPMENT ONLY: Log code for testing
		}).Warn("Failed to send 2FA code email - using code from logs for development")
		// Don't fail, just log the code for development
		// In production, you would want to return the error
		// return fmt.Errorf("failed to send verification code via email")
	} else {
		logger.WithFields(map[string]interface{}{
			"user_id": user.ID,
			"email":   email,
		}).Info("2FA code sent successfully")
	}

	// DEVELOPMENT ONLY: Log code for easy testing
	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   email,
		"code":    code,
	}).Warn("2FA CODE FOR DEVELOPMENT: Use this code to login")

	return nil
}

// VerifyCode verifies the 2FA code and returns the user if valid
func (u *twoFAUsecase) VerifyCode(email, code string) (*models.User, error) {
	// Validate input
	if email == "" {
		return nil, fmt.Errorf("email is required")
	}
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	if len(code) != 6 {
		return nil, fmt.Errorf("code must be 6 digits")
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get valid 2FA code
	twoFACode, err := u.twoFARepo.GetValidCode(email, code)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"email": email,
		}).Warn("Invalid or expired 2FA code")
		return nil, fmt.Errorf("invalid or expired verification code")
	}

	// Get user
	user, err := u.userRepo.GetByID(twoFACode.UserID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to get user for 2FA verification")
		return nil, fmt.Errorf("user not found")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	// Mark code as verified
	if err := u.twoFARepo.MarkAsVerified(twoFACode.ID); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to mark 2FA code as verified")
		// Don't fail, code is still valid
	}

	// Get user with department for complete response
	userWithDept, err := u.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		// Fallback to user without department
		userWithDept = user
	}

	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   email,
	}).Info("2FA code verified successfully")

	return userWithDept, nil
}

// generateSecureCode generates a cryptographically secure random numeric code
func generateSecureCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}

	return string(code), nil
}

// verifyPasswordHash verifies a password against its hash using bcrypt
func verifyPasswordHash(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
