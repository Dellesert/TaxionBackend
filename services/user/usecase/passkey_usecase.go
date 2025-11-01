package usecase

import (
	"fmt"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/logger"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// PasskeyUsecase defines interface for passkey business logic
type PasskeyUsecase interface {
	// Registration flow
	BeginRegistration(userID uint, passkeyName string) (*protocol.CredentialCreation, error)
	FinishRegistration(userID uint, response *protocol.ParsedCredentialCreationData) error

	// Authentication flow
	BeginAuthentication(email string) (*protocol.CredentialAssertion, error)
	FinishAuthentication(email string, response *protocol.ParsedCredentialAssertionData) (*models.User, error)
	FinishAuthenticationByCredential(response *protocol.ParsedCredentialAssertionData) (*models.User, error)

	// Management
	ListUserPasskeys(userID uint) ([]*models.PasskeyCredential, error)
	DeletePasskey(userID, passkeyID uint) error
	UpdatePasskeyName(userID, passkeyID uint, name string) error
}

// passkeyUsecase implements PasskeyUsecase interface
type passkeyUsecase struct {
	userRepo     repository.UserRepository
	passkeyRepo  repository.PasskeyRepository
	settingsRepo repository.SettingsRepository
	webAuthn     *WebAuthnService
	sessionStore map[string]interface{} // In production, use Redis or similar
}

// NewPasskeyUsecase creates a new passkey usecase
func NewPasskeyUsecase(
	userRepo repository.UserRepository,
	passkeyRepo repository.PasskeyRepository,
	settingsRepo repository.SettingsRepository,
	webAuthn *WebAuthnService,
) PasskeyUsecase {
	return &passkeyUsecase{
		userRepo:     userRepo,
		passkeyRepo:  passkeyRepo,
		settingsRepo: settingsRepo,
		webAuthn:     webAuthn,
		sessionStore: make(map[string]interface{}),
	}
}

// BeginRegistration starts the passkey registration process
func (u *passkeyUsecase) BeginRegistration(userID uint, passkeyName string) (*protocol.CredentialCreation, error) {
	// Get user
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to get user for passkey registration")
		return nil, fmt.Errorf("user not found")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	// Get system settings to check passkey limits
	settings, err := u.settingsRepo.GetOrCreate()
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to get system settings")
		// Continue with defaults
	}

	// Check if multiple passkeys are allowed
	if settings != nil && !settings.AllowMultiplePasskeys {
		count, err := u.passkeyRepo.CountByUserID(userID)
		if err == nil && count > 0 {
			return nil, fmt.Errorf("only one passkey is allowed per user")
		}
	}

	// Check max passkeys limit
	if settings != nil && settings.MaxPasskeysPerUser > 0 {
		count, err := u.passkeyRepo.CountByUserID(userID)
		if err == nil && count >= int64(settings.MaxPasskeysPerUser) {
			return nil, fmt.Errorf("maximum number of passkeys (%d) reached", settings.MaxPasskeysPerUser)
		}
	}

	// Get existing passkeys
	existingPasskeys, err := u.passkeyRepo.GetByUserID(userID)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to get existing passkeys")
		existingPasskeys = []*models.PasskeyCredential{}
	}

	// Create WebAuthnUser
	webAuthnUser := NewWebAuthnUser(user, existingPasskeys)

	// Begin registration
	options, sessionData, err := u.webAuthn.webAuthn.BeginRegistration(webAuthnUser)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to begin passkey registration")
		return nil, fmt.Errorf("failed to begin passkey registration: %w", err)
	}

	// Store session data (in production, use Redis with expiration)
	sessionKey := fmt.Sprintf("passkey_reg_%d", userID)
	u.sessionStore[sessionKey] = sessionData

	logger.WithFields(map[string]interface{}{
		"user_id": userID,
		"email":   user.Email,
	}).Info("Passkey registration started")

	return options, nil
}

// FinishRegistration completes the passkey registration process
func (u *passkeyUsecase) FinishRegistration(userID uint, response *protocol.ParsedCredentialCreationData) error {
	// Get user
	user, err := u.userRepo.GetByID(userID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to get user for passkey registration")
		return fmt.Errorf("user not found")
	}

	// Get existing passkeys
	existingPasskeys, err := u.passkeyRepo.GetByUserID(userID)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to get existing passkeys")
		existingPasskeys = []*models.PasskeyCredential{}
	}

	// Create WebAuthnUser
	webAuthnUser := NewWebAuthnUser(user, existingPasskeys)

	// Get session data
	sessionKey := fmt.Sprintf("passkey_reg_%d", userID)
	sessionData, ok := u.sessionStore[sessionKey]
	if !ok {
		return fmt.Errorf("registration session not found or expired")
	}

	webAuthnSessionData, ok := sessionData.(*webauthn.SessionData)
	if !ok {
		return fmt.Errorf("invalid session data")
	}

	// Verify and create credential
	credential, err := u.webAuthn.webAuthn.CreateCredential(webAuthnUser, *webAuthnSessionData, response)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to create passkey credential")
		return fmt.Errorf("failed to verify passkey: %w", err)
	}

	// Save to database
	passkeyCredential := &models.PasskeyCredential{
		UserID:          userID,
		CredentialID:    credential.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Name:            "", // Will be set from handler
		Transports:      formatTransports(credential.Transport),
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	if err := u.passkeyRepo.Create(passkeyCredential); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to save passkey credential")
		return fmt.Errorf("failed to save passkey credential: %w", err)
	}

	// Update user's passkey enabled status
	if err := u.userRepo.UpdatePasskeyStatus(userID, true); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to update user passkey status")
	}

	// Clean up session
	delete(u.sessionStore, sessionKey)

	logger.WithFields(map[string]interface{}{
		"user_id":    userID,
		"passkey_id": passkeyCredential.ID,
	}).Info("Passkey registered successfully")

	return nil
}

// BeginAuthentication starts the passkey authentication process
func (u *passkeyUsecase) BeginAuthentication(email string) (*protocol.CredentialAssertion, error) {
	// Check security settings to see if passkey login is allowed
	settings, err := u.settingsRepo.GetOrCreate()
	if err == nil && settings != nil {
		// Check if passkey authentication is allowed
		if settings.AuthMode == models.AuthModePassword {
			return nil, fmt.Errorf("passkey login is disabled. Please use password authentication")
		}
	}

	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get user
	user, err := u.userRepo.GetByEmail(email)
	if err != nil {
		// Don't reveal whether user exists
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is deactivated")
	}

	// Get user's passkeys
	passkeys, err := u.passkeyRepo.GetByUserID(user.ID)
	if err != nil || len(passkeys) == 0 {
		return nil, fmt.Errorf("no passkeys found for this user")
	}

	// Create WebAuthnUser
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Begin authentication
	options, sessionData, err := u.webAuthn.webAuthn.BeginLogin(webAuthnUser)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to begin passkey authentication")
		return nil, fmt.Errorf("failed to begin passkey authentication: %w", err)
	}

	// Store session data (in production, use Redis with expiration)
	sessionKey := fmt.Sprintf("passkey_auth_%s", email)
	u.sessionStore[sessionKey] = sessionData

	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   email,
	}).Info("Passkey authentication started")

	return options, nil
}

// FinishAuthentication completes the passkey authentication process
func (u *passkeyUsecase) FinishAuthentication(email string, response *protocol.ParsedCredentialAssertionData) (*models.User, error) {
	// Normalize email
	email = strings.ToLower(strings.TrimSpace(email))

	// Get user
	user, err := u.userRepo.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Get user's passkeys
	passkeys, err := u.passkeyRepo.GetByUserID(user.ID)
	if err != nil || len(passkeys) == 0 {
		return nil, fmt.Errorf("no passkeys found")
	}

	// Create WebAuthnUser
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Get session data
	sessionKey := fmt.Sprintf("passkey_auth_%s", email)
	sessionData, ok := u.sessionStore[sessionKey]
	if !ok {
		return nil, fmt.Errorf("authentication session not found or expired")
	}

	webAuthnSessionData, ok := sessionData.(*webauthn.SessionData)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	// Verify the assertion
	credential, err := u.webAuthn.webAuthn.ValidateLogin(webAuthnUser, *webAuthnSessionData, response)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to validate passkey authentication")
		return nil, fmt.Errorf("invalid passkey: %w", err)
	}

	// Update the credential's sign count and last used time
	passkeyCredential, err := u.passkeyRepo.GetByCredentialID(credential.ID)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to get passkey credential for update")
	} else {
		now := time.Now()
		passkeyCredential.SignCount = credential.Authenticator.SignCount
		passkeyCredential.LastUsedAt = &now
		if err := u.passkeyRepo.Update(passkeyCredential); err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to update passkey credential")
		}
	}

	// Clean up session
	delete(u.sessionStore, sessionKey)

	// Get user with department for complete response
	userWithDept, err := u.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		userWithDept = user
	}

	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   email,
	}).Info("Passkey authentication successful")

	return userWithDept, nil
}

// FinishAuthenticationByCredential completes the passkey authentication process by credential ID
// This method looks up the user by the credential ID instead of requiring email
func (u *passkeyUsecase) FinishAuthenticationByCredential(response *protocol.ParsedCredentialAssertionData) (*models.User, error) {
	// Check security settings to see if passkey login is allowed
	settings, err := u.settingsRepo.GetOrCreate()
	if err == nil && settings != nil {
		// Check if passkey authentication is allowed
		if settings.AuthMode == models.AuthModePassword {
			return nil, fmt.Errorf("passkey login is disabled. Please use password authentication")
		}
	}

	// Get the credential ID from the response
	credentialID := response.RawID

	// Find the passkey by credential ID
	passkeyCredential, err := u.passkeyRepo.GetByCredentialID(credentialID)
	if err != nil {
		return nil, fmt.Errorf("credential not found")
	}

	// Get the user
	user, err := u.userRepo.GetByID(passkeyCredential.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// Get all user's passkeys
	passkeys, err := u.passkeyRepo.GetByUserID(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user passkeys")
	}

	// Create WebAuthnUser
	webAuthnUser := NewWebAuthnUser(user, passkeys)

	// Get session data using email
	sessionKey := fmt.Sprintf("passkey_auth_%s", user.Email)
	sessionData, ok := u.sessionStore[sessionKey]
	if !ok {
		return nil, fmt.Errorf("authentication session not found or expired")
	}

	webAuthnSessionData, ok := sessionData.(*webauthn.SessionData)
	if !ok {
		return nil, fmt.Errorf("invalid session data")
	}

	// Verify the assertion
	credential, err := u.webAuthn.webAuthn.ValidateLogin(webAuthnUser, *webAuthnSessionData, response)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to validate passkey authentication")
		return nil, fmt.Errorf("invalid passkey: %w", err)
	}

	// Update the credential's sign count and last used time
	now := time.Now()
	passkeyCredential.SignCount = credential.Authenticator.SignCount
	passkeyCredential.LastUsedAt = &now
	if err := u.passkeyRepo.Update(passkeyCredential); err != nil {
		logger.WithField("error", err.Error()).Warn("Failed to update passkey credential")
	}

	// Clean up session
	delete(u.sessionStore, sessionKey)

	// Get user with department for complete response
	userWithDept, err := u.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		userWithDept = user
	}

	logger.WithFields(map[string]interface{}{
		"user_id": user.ID,
		"email":   user.Email,
	}).Info("Passkey authentication successful")

	return userWithDept, nil
}

// ListUserPasskeys returns all passkeys for a user
func (u *passkeyUsecase) ListUserPasskeys(userID uint) ([]*models.PasskeyCredential, error) {
	passkeys, err := u.passkeyRepo.GetByUserID(userID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to list user passkeys")
		return nil, fmt.Errorf("failed to get passkeys: %w", err)
	}
	return passkeys, nil
}

// DeletePasskey deletes a passkey
func (u *passkeyUsecase) DeletePasskey(userID, passkeyID uint) error {
	// Get the passkey to verify ownership
	passkey, err := u.passkeyRepo.GetByID(passkeyID)
	if err != nil {
		return fmt.Errorf("passkey not found")
	}

	if passkey.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	if err := u.passkeyRepo.Delete(passkeyID); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to delete passkey")
		return fmt.Errorf("failed to delete passkey: %w", err)
	}

	// Check if user has any remaining passkeys
	count, err := u.passkeyRepo.CountByUserID(userID)
	if err == nil && count == 0 {
		// Update user's passkey enabled status
		if err := u.userRepo.UpdatePasskeyStatus(userID, false); err != nil {
			logger.WithField("error", err.Error()).Warn("Failed to update user passkey status")
		}
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    userID,
		"passkey_id": passkeyID,
	}).Info("Passkey deleted successfully")

	return nil
}

// UpdatePasskeyName updates a passkey's name
func (u *passkeyUsecase) UpdatePasskeyName(userID, passkeyID uint, name string) error {
	// Get the passkey to verify ownership
	passkey, err := u.passkeyRepo.GetByID(passkeyID)
	if err != nil {
		return fmt.Errorf("passkey not found")
	}

	if passkey.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	passkey.Name = name
	if err := u.passkeyRepo.Update(passkey); err != nil {
		logger.WithField("error", err.Error()).Error("Failed to update passkey name")
		return fmt.Errorf("failed to update passkey: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"user_id":    userID,
		"passkey_id": passkeyID,
		"name":       name,
	}).Info("Passkey name updated")

	return nil
}

// formatTransports converts a slice of transports to a comma-separated string
func formatTransports(transports []protocol.AuthenticatorTransport) string {
	if len(transports) == 0 {
		return ""
	}

	strs := make([]string, len(transports))
	for i, t := range transports {
		strs[i] = string(t)
	}
	return strings.Join(strs, ",")
}
