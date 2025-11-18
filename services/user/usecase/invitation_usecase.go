package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/email"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"golang.org/x/crypto/bcrypt"
)

// InvitationUsecase defines the interface for invitation business logic
type InvitationUsecase interface {
	CreateInvitation(req *models.CreateInvitationRequest, createdByID uint) (*models.InvitationResponse, error)
	GetInvitation(id uint) (*models.InvitationResponse, error)
	GetInvitationByToken(token string) (*models.InvitationResponse, error)
	ValidateInvitation(token string) (*models.PublicInvitationResponse, error)
	AcceptInvitation(token string, req *models.AcceptInvitationRequest, ipAddress, userAgent string) (*sharedmodels.LoginResponse, error)
	ListInvitations(filters map[string]interface{}, page, pageSize int) (*models.InvitationListResponse, error)
	ResendInvitation(id uint, createdByID uint) (*models.InvitationResponse, error)
	CancelInvitation(id uint, createdByID uint) error
	GetStats() (*models.InvitationStatsResponse, error)
	ExpireOldInvitations() (int64, error)
	BulkSendInvitations(userIDs []uint, createdByID uint) (*models.BulkSendInvitationsResponse, error)
}

// invitationUsecase implements InvitationUsecase interface
type invitationUsecase struct {
	invitationRepo repository.InvitationRepository
	userRepo       repository.UserRepository
	departmentRepo repository.DepartmentRepository
	emailService   *email.EmailService
	authUsecase    AuthUsecase
}

// NewInvitationUsecase creates a new invitation usecase
func NewInvitationUsecase(
	invitationRepo repository.InvitationRepository,
	userRepo repository.UserRepository,
	departmentRepo repository.DepartmentRepository,
	emailService *email.EmailService,
	authUsecase AuthUsecase,
) InvitationUsecase {
	return &invitationUsecase{
		invitationRepo: invitationRepo,
		userRepo:       userRepo,
		departmentRepo: departmentRepo,
		emailService:   emailService,
		authUsecase:    authUsecase,
	}
}

// CreateInvitation creates a new invitation and sends email
func (u *invitationUsecase) CreateInvitation(req *models.CreateInvitationRequest, createdByID uint) (*models.InvitationResponse, error) {
	// Validate email format
	if err := u.authUsecase.ValidateEmail(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	existingUser, err := u.userRepo.GetByEmail(req.Email)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", req.Email)
	}

	// Check if there's already a pending invitation for this email
	hasPending, err := u.invitationRepo.HasPendingInvitation(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check pending invitation: %w", err)
	}
	if hasPending {
		return nil, fmt.Errorf("there is already a pending invitation for email %s", req.Email)
	}

	// Validate department if provided
	if req.DepartmentID != nil {
		_, err := u.departmentRepo.GetByID(*req.DepartmentID)
		if err != nil {
			return nil, fmt.Errorf("invalid department: %w", err)
		}
	}

	// Validate role
	if !isValidRole(req.Role) {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// Generate secure token
	token, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Calculate expiration time
	expiresAt := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)

	// Create invitation
	invitation := &models.Invitation{
		Token:        token,
		Email:        req.Email,
		Name:         strings.TrimSpace(req.Name),
		Role:         sharedmodels.Role(req.Role),
		DepartmentID: req.DepartmentID,
		Position:     strings.TrimSpace(req.Position),
		Phone:        strings.TrimSpace(req.Phone),
		Status:       models.InvitationStatusPending,
		ExpiresAt:    expiresAt,
		CreatedByID:  createdByID,
	}

	// Save invitation
	if err := u.invitationRepo.Create(invitation); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Get invitation with relations for response
	invitationWithRelations, err := u.invitationRepo.GetWithRelations(invitation.ID)
	if err != nil {
		// Fallback to invitation without relations
		invitationWithRelations = invitation
	}

	// Generate invite link
	inviteLink := generateInviteLink(token)

	// Send invitation email
	if err := u.sendInvitationEmail(invitationWithRelations, inviteLink); err != nil {
		logger.WithFields(map[string]interface{}{
			"invitation_id": invitation.ID,
			"email":         invitation.Email,
			"error":         err.Error(),
		}).Error("Failed to send invitation email")
		// Don't fail the request if email fails, just log it
	}

	// Prepare response
	response := invitationWithRelations.ToResponse()
	response.InviteLink = inviteLink

	return response, nil
}

// GetInvitation retrieves an invitation by ID
func (u *invitationUsecase) GetInvitation(id uint) (*models.InvitationResponse, error) {
	invitation, err := u.invitationRepo.GetWithRelations(id)
	if err != nil {
		return nil, err
	}
	return invitation.ToResponse(), nil
}

// GetInvitationByToken retrieves an invitation by token
func (u *invitationUsecase) GetInvitationByToken(token string) (*models.InvitationResponse, error) {
	invitation, err := u.invitationRepo.GetByTokenWithRelations(token)
	if err != nil {
		return nil, err
	}
	return invitation.ToResponse(), nil
}

// ValidateInvitation validates an invitation token and returns public information
func (u *invitationUsecase) ValidateInvitation(token string) (*models.PublicInvitationResponse, error) {
	invitation, err := u.invitationRepo.GetByTokenWithRelations(token)
	if err != nil {
		return nil, fmt.Errorf("invalid invitation token")
	}

	// Check if invitation is valid
	if !invitation.IsValid() {
		if invitation.IsExpired() {
			return nil, fmt.Errorf("invitation has expired")
		}
		return nil, fmt.Errorf("invitation is no longer valid (status: %s)", invitation.Status)
	}

	return invitation.ToPublicResponse(), nil
}

// AcceptInvitation accepts an invitation and creates a user account
func (u *invitationUsecase) AcceptInvitation(token string, req *models.AcceptInvitationRequest, ipAddress, userAgent string) (*sharedmodels.LoginResponse, error) {
	// Validate invitation
	invitation, err := u.invitationRepo.GetByTokenWithRelations(token)
	if err != nil {
		return nil, fmt.Errorf("invalid invitation token")
	}

	// Check if invitation is valid
	if !invitation.IsValid() {
		if invitation.IsExpired() {
			return nil, fmt.Errorf("invitation has expired")
		}
		return nil, fmt.Errorf("invitation is no longer valid (status: %s)", invitation.Status)
	}

	// Validate passwords match
	if req.Password != req.ConfirmPassword {
		return nil, fmt.Errorf("passwords do not match")
	}

	// Validate password strength
	if err := u.authUsecase.ValidatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("invalid password: %w", err)
	}

	// Check if user already exists
	existingUser, err := u.userRepo.GetByEmail(invitation.Email)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var user *models.User

	if existingUser != nil {
		// User already exists
		if existingUser.IsActive {
			// User is already active - cannot reactivate
			return nil, fmt.Errorf("account is already activated")
		}

		// User exists but is inactive - reactivate with new password
		existingUser.HashedPassword = &hashedPassword
		existingUser.IsActive = true
		existingUser.Status = sharedmodels.StatusOnline
		now := time.Now()
		existingUser.PasswordChangedAt = &now

		// Update user
		if err := u.userRepo.Update(existingUser); err != nil {
			return nil, fmt.Errorf("failed to activate user: %w", err)
		}

		user = existingUser
	} else {
		// Create new user
		now := time.Now()
		user = &models.User{
			Email:             invitation.Email,
			Name:              invitation.Name,
			HashedPassword:    &hashedPassword,
			PasswordChangedAt: &now,
			Role:              invitation.Role,
			DepartmentID:      invitation.DepartmentID,
			Position:          invitation.Position,
			Phone:             invitation.Phone,
			IsActive:          true,
			Status:            sharedmodels.StatusOnline,
		}

		// Save user
		if err := u.userRepo.Create(user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Update invitation status
	now := time.Now()
	invitation.Status = models.InvitationStatusAccepted
	invitation.AcceptedAt = &now
	invitation.UserID = &user.ID
	if err := u.invitationRepo.Update(invitation); err != nil {
		logger.WithFields(map[string]interface{}{
			"invitation_id": invitation.ID,
			"user_id":       user.ID,
			"error":         err.Error(),
		}).Error("Failed to update invitation status after user creation")
		// Don't fail the request, user is already created
	}

	// Cancel any other pending invitations for this email
	if err := u.invitationRepo.CancelPendingInvitationsByEmail(invitation.Email); err != nil {
		logger.WithFields(map[string]interface{}{
			"email": invitation.Email,
			"error": err.Error(),
		}).Warn("Failed to cancel other pending invitations")
	}

	// Get user with department for complete response
	userWithDept, err := u.userRepo.GetWithDepartment(user.ID)
	if err != nil {
		userWithDept = user
	}

	// Convert user to shared model format
	responseUser := convertUserToSharedModel(userWithDept)

	// Get current auth mode
	authMode := middleware.GetAuthMode()

	// Create login response with session
	response := &sharedmodels.LoginResponse{
		User:     *responseUser,
		AuthMode: authMode,
	}

	// Create session (stateful authentication)
	if authMode == sharedmodels.AuthModeSession {
		authConfig := middleware.GetAuthConfig()
		if authConfig != nil && authConfig.SessionStore != nil {
			ctx := context.Background()
			session, err := authConfig.SessionStore.CreateSession(
				ctx,
				user.ID,
				user.Email,
				user.Role,
				ipAddress,
				userAgent,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create session: %w", err)
			}

			response.Session = &sharedmodels.SessionResponse{
				SessionID: session.SessionID,
				ExpiresAt: session.ExpiresAt.Unix(),
			}
		} else {
			return nil, fmt.Errorf("session store not available")
		}
	}

	// Send notification to super admin
	go u.sendActivationNotificationToSuperAdmin(invitation, user)

	return response, nil
}

// ListInvitations retrieves a paginated list of invitations
func (u *invitationUsecase) ListInvitations(filters map[string]interface{}, page, pageSize int) (*models.InvitationListResponse, error) {
	// Set default pagination values
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	invitations, total, err := u.invitationRepo.List(filters, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations: %w", err)
	}

	// Convert to response
	invitationResponses := make([]*models.InvitationResponse, len(invitations))
	for i, inv := range invitations {
		invitationResponses[i] = inv.ToResponse()
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	return &models.InvitationListResponse{
		Invitations: invitationResponses,
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		TotalPages:  totalPages,
	}, nil
}

// ResendInvitation resends an invitation with a new token
func (u *invitationUsecase) ResendInvitation(id uint, createdByID uint) (*models.InvitationResponse, error) {
	invitation, err := u.invitationRepo.GetWithRelations(id)
	if err != nil {
		return nil, err
	}

	// Verify that the creator is the same
	if invitation.CreatedByID != createdByID {
		return nil, fmt.Errorf("unauthorized: you can only resend invitations you created")
	}

	// Can only resend pending or expired invitations
	if invitation.Status != models.InvitationStatusPending && invitation.Status != models.InvitationStatusExpired {
		return nil, fmt.Errorf("can only resend pending or expired invitations")
	}

	// Check if user was already created and is active
	existingUser, err := u.userRepo.GetByEmail(invitation.Email)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil && existingUser.IsActive {
		return nil, fmt.Errorf("user with email %s is already active", invitation.Email)
	}

	// Generate new token
	newToken, err := generateSecureToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Update invitation
	invitation.Token = newToken
	invitation.Status = models.InvitationStatusPending
	invitation.ExpiresAt = time.Now().Add(7 * 24 * time.Hour) // Reset to 7 days from now
	invitation.AcceptedAt = nil

	if err := u.invitationRepo.Update(invitation); err != nil {
		return nil, fmt.Errorf("failed to update invitation: %w", err)
	}

	// Generate new invite link
	inviteLink := generateInviteLink(newToken)

	// Resend invitation email
	if err := u.sendInvitationEmail(invitation, inviteLink); err != nil {
		logger.WithFields(map[string]interface{}{
			"invitation_id": invitation.ID,
			"email":         invitation.Email,
			"error":         err.Error(),
		}).Error("Failed to resend invitation email")
		// Don't fail the request if email fails
	}

	response := invitation.ToResponse()
	response.InviteLink = inviteLink

	return response, nil
}

// CancelInvitation cancels an invitation
func (u *invitationUsecase) CancelInvitation(id uint, createdByID uint) error {
	invitation, err := u.invitationRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Verify that the creator is the same
	if invitation.CreatedByID != createdByID {
		return fmt.Errorf("unauthorized: you can only cancel invitations you created")
	}

	// Can only cancel pending invitations
	if invitation.Status != models.InvitationStatusPending {
		return fmt.Errorf("can only cancel pending invitations")
	}

	// Update status
	invitation.Status = models.InvitationStatusCancelled
	return u.invitationRepo.Update(invitation)
}

// GetStats retrieves invitation statistics
func (u *invitationUsecase) GetStats() (*models.InvitationStatsResponse, error) {
	return u.invitationRepo.GetStats()
}

// ExpireOldInvitations marks old pending invitations as expired
func (u *invitationUsecase) ExpireOldInvitations() (int64, error) {
	return u.invitationRepo.ExpireOldInvitations()
}

// Helper functions

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken() (string, error) {
	b := make([]byte, 64) // 64 bytes = 128 hex characters
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateInviteLink generates an invitation link
// This link points to the backend redirect page that handles platform detection
func generateInviteLink(token string) string {
	backendURL := os.Getenv("BACKEND_URL")
	if backendURL == "" {
		// Fallback to user service URL
		backendURL = os.Getenv("USER_SERVICE_URL")
		if backendURL == "" {
			backendURL = "http://localhost:8081"
		}
	}
	return fmt.Sprintf("%s/invite/%s", strings.TrimSuffix(backendURL, "/"), token)
}

// sendInvitationEmail sends an invitation email
func (u *invitationUsecase) sendInvitationEmail(invitation *models.Invitation, inviteLink string) error {
	if u.emailService == nil {
		return fmt.Errorf("email service not configured")
	}

	// Generate deep link
	deepLink := fmt.Sprintf("tachyon://invite/%s", invitation.Token)

	// Use new email template method
	return u.emailService.SendInvitationEmail(
		invitation.Email,
		invitation.Name,
		invitation.Token,
		deepLink,
	)
}

// sendActivationNotificationToSuperAdmin sends notification to super admin when user activates account
func (u *invitationUsecase) sendActivationNotificationToSuperAdmin(invitation *models.Invitation, user *models.User) {
	if u.emailService == nil {
		return
	}

	// Get super admin email
	superAdminEmail := os.Getenv("SUPER_ADMIN_EMAIL")
	if superAdminEmail == "" {
		logger.Warn("SUPER_ADMIN_EMAIL not configured, skipping activation notification")
		return
	}

	subject := "Новый пользователь активирован"
	htmlBody := renderActivationNotificationTemplate(invitation, user)

	if err := u.emailService.SendEmail(superAdminEmail, subject, htmlBody); err != nil {
		logger.WithFields(map[string]interface{}{
			"invitation_id": invitation.ID,
			"user_id":       user.ID,
			"error":         err.Error(),
		}).Error("Failed to send activation notification to super admin")
	}
}

// renderInvitationEmailTemplate renders the invitation email template
func renderInvitationEmailTemplate(invitation *models.Invitation, inviteLink string) string {
	departmentName := "Не указан"
	if invitation.Department != nil {
		departmentName = invitation.Department.Name
	}

	roleName := translateRole(string(invitation.Role))

	// Extract token from invite link for deep link
	token := invitation.Token
	deepLink := fmt.Sprintf("tachyon://invite/%s", token)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Приглашение в Tachyon Messenger</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #E94444;
            margin: 0;
            font-size: 28px;
        }
        .content {
            margin: 20px 0;
        }
        .info-box {
            background-color: #f8f9fa;
            border-left: 4px solid #E94444;
            padding: 15px;
            margin: 20px 0;
        }
        .info-box h3 {
            margin-top: 0;
            color: #2c3e50;
        }
        .info-row {
            margin: 10px 0;
        }
        .info-label {
            font-weight: bold;
            color: #555;
        }
        .code-box {
            background-color: #f8f9fa;
            border: 2px dashed #E94444;
            border-radius: 8px;
            padding: 20px;
            text-align: center;
            margin: 20px 0;
        }
        .code {
            font-size: 32px;
            font-weight: bold;
            color: #E94444;
            letter-spacing: 4px;
            font-family: 'Courier New', monospace;
            word-break: break-all;
        }
        .button {
            display: inline-block;
            background-color: #E94444;
            color: #ffffff !important;
            padding: 15px 30px;
            text-decoration: none;
            border-radius: 8px;
            margin: 10px 5px;
            font-weight: bold;
            text-align: center;
        }
        .button:hover {
            background-color: #d93636;
        }
        .button-secondary {
            background-color: #6c757d;
        }
        .button-secondary:hover {
            background-color: #5a6268;
        }
        .expiry-notice {
            background-color: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
            color: #856404;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e9ecef;
            color: #6c757d;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📱 Приглашение в Tachyon Messenger</h1>
        </div>

        <div class="content">
            <p>Здравствуйте, <strong>%s</strong>!</p>

            <p>Вы приглашены присоединиться к корпоративному мессенджеру <strong>Tachyon</strong>.</p>

            <div class="info-box">
                <h3>Ваши данные:</h3>
                <div class="info-row">
                    <span class="info-label">Email:</span> %s
                </div>
                <div class="info-row">
                    <span class="info-label">Роль:</span> %s
                </div>
                <div class="info-row">
                    <span class="info-label">Отдел:</span> %s
                </div>
                %s
            </div>

            <h3 style="text-align: center; color: #2c3e50;">Ваш код приглашения:</h3>
            <div class="code-box">
                <div class="code">%s</div>
                <p style="margin: 10px 0 0 0; font-size: 14px; color: #6c757d;">Используйте этот код в мобильном приложении</p>
            </div>

            <p style="text-align: center; margin: 30px 0 10px 0;"><strong>Выберите способ активации:</strong></p>

            <div style="text-align: center;">
                <a href="%s" class="button" target="_blank" rel="noopener">🔗 Открыть в приложении</a>
                <a href="%s" class="button button-secondary" target="_blank" rel="noopener">🌐 Открыть в браузере</a>
            </div>

            <p style="text-align: center; font-size: 13px; color: #6c757d; margin-top: 10px;">
                Нажмите на кнопку выше, чтобы открыть приложение
            </p>

            <div style="background-color: #f8f9fa; padding: 15px; border-radius: 8px; margin: 20px 0; border: 1px solid #dee2e6;">
                <p style="margin: 0 0 10px 0;"><strong>📱 Для мобильного приложения:</strong></p>
                <p style="margin: 5px 0; font-size: 13px;">Скопируйте ссылку ниже и откройте её в приложении Tachyon Messenger:</p>
                <div style="background: white; padding: 10px; border: 1px solid #dee2e6; border-radius: 4px; margin: 10px 0;">
                    <code style="color: #E94444; word-break: break-all; font-size: 13px;">%s</code>
                </div>
                <p style="margin: 10px 0 0 0; font-size: 12px; color: #6c757d;">
                    Или используйте код приглашения выше
                </p>
            </div>

            <div class="expiry-notice">
                <strong>⚠️ Важно:</strong> Приглашение действительно до <strong>%s</strong>
            </div>

            <p style="font-size: 13px; color: #6c757d; text-align: center;">
                Если у вас не установлено приложение, скачайте его в App Store или Google Play
            </p>
        </div>

        <div class="footer">
            <p>С уважением,<br>Команда Tachyon Messenger</p>
            <p style="font-size: 12px; color: #adb5bd;">
                Это автоматическое сообщение. Если вы не ожидали этого приглашения, проигнорируйте его.
            </p>
        </div>
    </div>
</body>
</html>
`, invitation.Name, invitation.Email, roleName, departmentName,
		formatPosition(invitation.Position),
		token,
		deepLink,
		inviteLink,
		deepLink,
		invitation.ExpiresAt.Format("02.01.2006 15:04"))
}

// renderActivationNotificationTemplate renders the activation notification template for super admin
func renderActivationNotificationTemplate(invitation *models.Invitation, user *models.User) string {
	departmentName := "Не указан"
	if invitation.Department != nil {
		departmentName = invitation.Department.Name
	}

	roleName := translateRole(string(user.Role))

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Новый пользователь активирован</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
        }
        .header h1 {
            color: #28a745;
            margin: 0;
            font-size: 28px;
        }
        .success-icon {
            font-size: 48px;
            text-align: center;
            margin-bottom: 20px;
        }
        .info-box {
            background-color: #f8f9fa;
            border-left: 4px solid #28a745;
            padding: 15px;
            margin: 20px 0;
        }
        .info-row {
            margin: 10px 0;
        }
        .info-label {
            font-weight: bold;
            color: #555;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #e9ecef;
            color: #6c757d;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="success-icon">✅</div>

        <div class="header">
            <h1>Новый пользователь активирован</h1>
        </div>

        <p>Пользователь успешно активировал свой аккаунт в системе Tachyon Messenger.</p>

        <div class="info-box">
            <h3>Информация о пользователе:</h3>
            <div class="info-row">
                <span class="info-label">Имя:</span> %s
            </div>
            <div class="info-row">
                <span class="info-label">Email:</span> %s
            </div>
            <div class="info-row">
                <span class="info-label">Роль:</span> %s
            </div>
            <div class="info-row">
                <span class="info-label">Отдел:</span> %s
            </div>
            %s
            <div class="info-row">
                <span class="info-label">Дата активации:</span> %s
            </div>
        </div>

        <div class="footer">
            <p>С уважением,<br>Система Tachyon Messenger</p>
        </div>
    </div>
</body>
</html>
`, user.Name, user.Email, roleName, departmentName,
		formatPosition(user.Position),
		time.Now().Format("02.01.2006 15:04"))
}

// Helper function to format position
func formatPosition(position string) string {
	if position != "" {
		return fmt.Sprintf(`<div class="info-row"><span class="info-label">Должность:</span> %s</div>`, position)
	}
	return ""
}

// Helper function to translate role to Russian
func translateRole(role string) string {
	translations := map[string]string{
		"super_admin":     "Суперадминистратор",
		"admin":           "Администратор",
		"department_head": "Руководитель отдела",
		"employee":        "Сотрудник",
	}
	if translated, ok := translations[role]; ok {
		return translated
	}
	return role
}

// hashPassword hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// BulkSendInvitations sends invitations to multiple inactive users
func (u *invitationUsecase) BulkSendInvitations(userIDs []uint, createdByID uint) (*models.BulkSendInvitationsResponse, error) {
	response := &models.BulkSendInvitationsResponse{
		TotalUsers:      len(userIDs),
		SuccessCount:    0,
		ErrorCount:      0,
		SentInvitations: []*models.InvitationResponse{},
		Errors:          []models.BulkInvitationError{},
	}

	// Process each user
	for _, userID := range userIDs {
		// Get user
		user, err := u.userRepo.GetByID(userID)
		if err != nil {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.BulkInvitationError{
				UserID:  userID,
				Message: fmt.Sprintf("User not found: %v", err),
			})
			continue
		}

		// Check if user is inactive (only send invitations to inactive users)
		if user.IsActive {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.BulkInvitationError{
				UserID:  userID,
				Email:   user.Email,
				Message: "User is already active",
			})
			continue
		}

		// Check if there's already a pending invitation
		hasPending, err := u.invitationRepo.HasPendingInvitation(user.Email)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"user_id": userID,
				"email":   user.Email,
				"error":   err.Error(),
			}).Warn("Failed to check pending invitation, proceeding anyway")
		}

		// If there's already a pending invitation, skip
		if hasPending {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.BulkInvitationError{
				UserID:  userID,
				Email:   user.Email,
				Message: "Pending invitation already exists",
			})
			continue
		}

		// Generate secure token
		token, err := generateSecureToken()
		if err != nil {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.BulkInvitationError{
				UserID:  userID,
				Email:   user.Email,
				Message: fmt.Sprintf("Failed to generate token: %v", err),
			})
			continue
		}

		// Calculate expiration time (7 days)
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		// Create invitation
		invitation := &models.Invitation{
			Token:        token,
			Email:        user.Email,
			Name:         user.Name,
			Role:         user.Role,
			DepartmentID: user.DepartmentID,
			Position:     user.Position,
			Phone:        user.Phone,
			Status:       models.InvitationStatusPending,
			ExpiresAt:    expiresAt,
			CreatedByID:  createdByID,
		}

		// Save invitation
		if err := u.invitationRepo.Create(invitation); err != nil {
			response.ErrorCount++
			response.Errors = append(response.Errors, models.BulkInvitationError{
				UserID:  userID,
				Email:   user.Email,
				Message: fmt.Sprintf("Failed to create invitation: %v", err),
			})
			continue
		}

		// Get invitation with relations for response
		invitationWithRelations, err := u.invitationRepo.GetWithRelations(invitation.ID)
		if err != nil {
			// Fallback to invitation without relations
			invitationWithRelations = invitation
		}

		// Generate invite link
		inviteLink := generateInviteLink(token)

		// Send invitation email
		if err := u.sendInvitationEmail(invitationWithRelations, inviteLink); err != nil {
			logger.WithFields(map[string]interface{}{
				"invitation_id": invitation.ID,
				"user_id":       userID,
				"email":         user.Email,
				"error":         err.Error(),
			}).Error("Failed to send invitation email in bulk operation")
			// Don't fail the operation, just log it
		}

		// Success
		response.SuccessCount++
		invitationResponse := invitationWithRelations.ToResponse()
		invitationResponse.InviteLink = inviteLink
		response.SentInvitations = append(response.SentInvitations, invitationResponse)
	}

	return response, nil
}
