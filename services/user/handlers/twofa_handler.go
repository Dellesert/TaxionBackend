package handlers

import (
	"net/http"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	sharedErrors "tachyon-messenger/shared/errors"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// TwoFAHandler handles HTTP requests for 2FA operations
type TwoFAHandler struct {
	twoFAUsecase usecase.TwoFAUsecase
	authUsecase  usecase.AuthUsecase
	jwtConfig    *middleware.JWTConfig
}

// NewTwoFAHandler creates a new 2FA handler
func NewTwoFAHandler(
	twoFAUsecase usecase.TwoFAUsecase,
	authUsecase usecase.AuthUsecase,
	jwtConfig *middleware.JWTConfig,
) *TwoFAHandler {
	return &TwoFAHandler{
		twoFAUsecase: twoFAUsecase,
		authUsecase:  authUsecase,
		jwtConfig:    jwtConfig,
	}
}

// SendCode handles sending 2FA code via email
func (h *TwoFAHandler) SendCode(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.Send2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for 2FA send code")

		apiErr := sharedErrors.BadRequestError("Invalid request body").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Additional validation
	if strings.TrimSpace(req.Email) == "" {
		logger.WithField("request_id", requestID).Warn("Email is required for 2FA")
		apiErr := sharedErrors.RequiredFieldError("email").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		logger.WithField("request_id", requestID).Warn("Password is required for 2FA")
		apiErr := sharedErrors.RequiredFieldError("password").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Extract client info
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// Send 2FA code
	err := h.twoFAUsecase.SendCode(req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Warn("Failed to send 2FA code")

		// Determine appropriate error based on error message
		var apiErr *sharedErrors.APIError

		if strings.Contains(err.Error(), "invalid email or password") {
			apiErr = sharedErrors.InvalidCredentialsError()
		} else if strings.Contains(err.Error(), "deactivated") {
			apiErr = sharedErrors.AccountDeactivatedError()
		} else if strings.Contains(err.Error(), "super admin access is restricted") {
			apiErr = sharedErrors.SuperAdminWebOnlyError()
		} else if strings.Contains(err.Error(), "two factor authentication is not enabled") ||
			strings.Contains(err.Error(), "2FA not enabled") {
			apiErr = sharedErrors.NewAPIError(http.StatusBadRequest, sharedErrors.Auth2FANotEnabled,
				"Two-factor authentication is not enabled for this account")
		} else {
			apiErr = sharedErrors.NewAPIError(http.StatusInternalServerError, sharedErrors.Auth2FASendFailed,
				"Failed to send verification code")
		}

		apiErr.WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"email":      req.Email,
	}).Info("2FA code sent successfully")

	// 2FA code expires in 5 minutes (300 seconds)
	codeExpiresIn := 300
	canResendAfter := 60

	response := gin.H{
		"message":          "Verification code sent to your email",
		"request_id":       requestID,
		"code_expires_in":  codeExpiresIn,
		"can_resend_after": canResendAfter,
	}

	c.JSON(http.StatusOK, response)
}

// VerifyCode handles verifying 2FA code and completing login
func (h *TwoFAHandler) VerifyCode(c *gin.Context) {
	requestID := requestid.Get(c)

	var req models.Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for 2FA verify code")

		apiErr := sharedErrors.BadRequestError("Invalid request body").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Additional validation
	if strings.TrimSpace(req.Email) == "" {
		logger.WithField("request_id", requestID).Warn("Email is required for 2FA verification")
		apiErr := sharedErrors.RequiredFieldError("email").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	if strings.TrimSpace(req.Code) == "" {
		logger.WithField("request_id", requestID).Warn("Code is required for 2FA verification")
		apiErr := sharedErrors.RequiredFieldError("code").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Verify 2FA code
	user, err := h.twoFAUsecase.VerifyCode(req.Email, req.Code)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Warn("Failed to verify 2FA code")

		// Determine appropriate error based on error message
		var apiErr *sharedErrors.APIError

		if strings.Contains(err.Error(), "invalid or expired") || strings.Contains(err.Error(), "expired") {
			apiErr = sharedErrors.NewAPIError(http.StatusUnauthorized, sharedErrors.Auth2FACodeExpired,
				"Verification code is invalid or expired")
		} else if strings.Contains(err.Error(), "deactivated") {
			apiErr = sharedErrors.AccountDeactivatedError()
		} else if strings.Contains(err.Error(), "user not found") {
			apiErr = sharedErrors.NewAPIError(http.StatusNotFound, sharedErrors.UserNotFound, "User not found")
		} else {
			apiErr = sharedErrors.NewAPIError(http.StatusUnauthorized, sharedErrors.Auth2FAInvalidCode,
				"Invalid verification code")
		}

		apiErr.WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Update user login status (set to online)
	// This is handled separately in the auth flow, but we can do it here too
	// For simplicity, we'll skip this and let the client make a status update if needed

	// Convert user to shared model format for response
	responseUser := convertUserToSharedModel(user)

	// Get current auth mode
	authMode := middleware.GetAuthMode()

	// Create response based on auth mode
	response := gin.H{
		"message":    "Login successful",
		"user":       responseUser,
		"auth_mode":  authMode,
		"request_id": requestID,
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	switch authMode {
	case sharedmodels.AuthModeSession:
		// Create session
		authConfig := middleware.GetAuthConfig()
		if authConfig != nil && authConfig.SessionStore != nil {
			ctx := c.Request.Context()
			session, err := authConfig.SessionStore.CreateSession(
				ctx,
				user.ID,
				user.Email,
				user.Role,
				ipAddress,
				userAgent,
			)
			if err != nil {
				logger.WithFields(map[string]interface{}{
					"request_id": requestID,
					"error":      err.Error(),
				}).Error("Failed to create session after 2FA")

				apiErr := sharedErrors.InternalError("Failed to create session").
					WithRequestID(requestID)
				c.JSON(apiErr.StatusCode, apiErr)
				return
			}

			// Set session cookie
			c.SetCookie(
				"session_id",
				session.SessionID,
				int(session.ExpiresAt.Unix()-session.CreatedAt.Unix()),
				"/",
				"",
				false, // secure - set to true in production with HTTPS
				true,  // httpOnly
			)

			response["session"] = gin.H{
				"session_id": session.SessionID,
				"expires_at": session.ExpiresAt.Unix(),
			}
		}

		// Add must_change_password flag for super admin
		if user.MustChangePassword {
			response["must_change_password"] = true
		}

	case sharedmodels.AuthModeJWT:
		fallthrough
	default:
		// Generate JWT tokens
		tokens, err := middleware.GenerateTokens(user.ID, user.Email, user.Role, h.jwtConfig)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"error":      err.Error(),
			}).Error("Failed to generate tokens after 2FA")

			apiErr := sharedErrors.InternalError("Failed to generate authentication tokens").
				WithRequestID(requestID)
			c.JSON(apiErr.StatusCode, apiErr)
			return
		}
		response["tokens"] = tokens
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("2FA verification successful, user logged in")

	c.JSON(http.StatusOK, response)
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
