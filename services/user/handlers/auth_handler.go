package handlers

import (
	"net/http"
	"strings"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/analytics"
	sharedErrors "tachyon-messenger/shared/errors"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	authUsecase     usecase.AuthUsecase
	analyticsClient *analytics.Client
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authUsecase usecase.AuthUsecase, analyticsClient *analytics.Client) *AuthHandler {
	return &AuthHandler{
		authUsecase:     authUsecase,
		analyticsClient: analyticsClient,
	}
}

// Register handles user registration requests (DISABLED - use invitations instead)
// This endpoint is now restricted to super_admin only for manual user creation
func (h *AuthHandler) Register(c *gin.Context) {
	requestID := requestid.Get(c)

	// Check if user is authenticated and has super_admin role
	userRole, err := middleware.GetUserRoleFromContext(c)
	if err != nil || userRole != sharedmodels.RoleSuperAdmin {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      "Free registration is disabled. Use invitation system.",
		}).Warn("Unauthorized registration attempt")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Free registration is disabled. Please use the invitation system.",
			"request_id": requestID,
		})
		return
	}

	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for user registration")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Additional validation for required fields
	if strings.TrimSpace(req.Email) == "" {
		logger.WithField("request_id", requestID).Warn("Email is required for registration")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Email is required",
			"request_id": requestID,
		})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		logger.WithField("request_id", requestID).Warn("Name is required for registration")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Name is required",
			"request_id": requestID,
		})
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		logger.WithField("request_id", requestID).Warn("Password is required for registration")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Password is required",
			"request_id": requestID,
		})
		return
	}

	// Call usecase to register user
	user, err := h.authUsecase.Register(&req)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Error("Failed to register user")

		// Determine appropriate HTTP status code based on error
		statusCode := http.StatusInternalServerError
		errorMessage := "Failed to register user"

		if strings.Contains(err.Error(), "already exists") {
			statusCode = http.StatusConflict
			errorMessage = "User with this email already exists"
		} else if strings.Contains(err.Error(), "invalid email") ||
			strings.Contains(err.Error(), "invalid password") ||
			strings.Contains(err.Error(), "invalid role") ||
			strings.Contains(err.Error(), "invalid department") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    user.ID,
		"email":      user.Email,
	}).Info("User registered successfully")

	// Send analytics event
	h.analyticsClient.SendEvent(
		analytics.EventUserRegistration,
		analytics.CategoryUser,
		uint64(user.ID),
		map[string]interface{}{
			"email": user.Email,
			"role":  user.Role,
		},
	)

	c.JSON(http.StatusCreated, gin.H{
		"message":    "User registered successfully",
		"user":       user,
		"request_id": requestID,
	})
}

// Login handles user login requests
func (h *AuthHandler) Login(c *gin.Context) {
	requestID := requestid.Get(c)

	var req struct {
		Email      string `json:"email" binding:"required,email"`
		Password   string `json:"password" binding:"required"`
		DeviceInfo string `json:"device_info"` // Optional: device info from mobile apps (iOS workaround)
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for user login")

		apiErr := sharedErrors.BadRequestError("Invalid request body").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Additional validation
	if strings.TrimSpace(req.Email) == "" {
		logger.WithField("request_id", requestID).Warn("Email is required for login")
		apiErr := sharedErrors.RequiredFieldError("email").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		logger.WithField("request_id", requestID).Warn("Password is required for login")
		apiErr := sharedErrors.RequiredFieldError("password").WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// If device_info is provided in request body (iOS workaround), use it instead of User-Agent header
	if req.DeviceInfo != "" {
		userAgent = req.DeviceInfo
	}

	// Call usecase to authenticate user
	loginResponse, err := h.authUsecase.Login(req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Warn("Failed login attempt")

		// Send failed login attempt to analytics
		h.analyticsClient.SendLoginAttemptAsync(analytics.LoginAttemptRequest{
			Email:      req.Email,
			IPAddress:  ipAddress,
			UserAgent:  userAgent,
			Success:    false,
			FailReason: analytics.DetermineFailReason(err),
			AuthMode:   "password",
		})

		// Determine appropriate error based on error message
		var apiErr *sharedErrors.APIError

		if strings.Contains(err.Error(), "invalid email or password") {
			apiErr = sharedErrors.InvalidCredentialsError()
		} else if strings.Contains(err.Error(), "2FA is required") {
			apiErr = sharedErrors.TwoFactorRequiredError()
		} else if strings.Contains(err.Error(), "deactivated") {
			apiErr = sharedErrors.AccountDeactivatedError()
		} else if strings.Contains(err.Error(), "password login is disabled") {
			apiErr = sharedErrors.PasskeyOnlyError()
		} else if strings.Contains(err.Error(), "super admin access is restricted") {
			apiErr = sharedErrors.SuperAdminWebOnlyError()
		} else if strings.Contains(err.Error(), "email is required") {
			apiErr = sharedErrors.RequiredFieldError("email")
		} else if strings.Contains(err.Error(), "password is required") {
			apiErr = sharedErrors.RequiredFieldError("password")
		} else {
			// For any other error, keep it generic for security
			apiErr = sharedErrors.InternalError("Login failed")
		}

		apiErr.WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
		"auth_mode":  loginResponse.AuthMode,
	}).Info("User logged in successfully")

	// Send successful login attempt to analytics
	userID := uint64(loginResponse.User.ID)
	h.analyticsClient.SendLoginAttemptAsync(analytics.LoginAttemptRequest{
		Email:     loginResponse.User.Email,
		UserID:    &userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Success:   true,
		AuthMode:  string(loginResponse.AuthMode),
	})

	// Track device for security monitoring
	h.analyticsClient.TrackDeviceAsync(userID, userAgent, ipAddress)

	// Send analytics event (legacy)
	h.analyticsClient.SendEvent(
		analytics.EventUserLogin,
		analytics.CategoryUser,
		uint64(loginResponse.User.ID),
		map[string]interface{}{
			"email":      loginResponse.User.Email,
			"auth_mode":  loginResponse.AuthMode,
			"ip_address": ipAddress,
		},
	)

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		// Calculate MaxAge in seconds (time until expiration)
		maxAge := int(loginResponse.Session.ExpiresAt - time.Now().Unix())
		if maxAge < 0 {
			maxAge = 0
		}

		// Set cookie with SameSite=Lax for local development
		// For production with HTTPS, use SameSite=None with Secure=true
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			maxAge,
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			false, // httpOnly - set to false to allow JS to read it for X-Session-ID header
		)

		// Track session in analytics
		expiresAt := time.Unix(loginResponse.Session.ExpiresAt, 0)
		h.analyticsClient.TrackSessionAsync(
			uint64(loginResponse.User.ID),
			loginResponse.Session.SessionID,
			ipAddress,
			userAgent,
			expiresAt,
		)
	}

	response := gin.H{
		"message":    "Login successful",
		"user":       loginResponse.User,
		"auth_mode":  loginResponse.AuthMode,
		"request_id": requestID,
	}

	// Add tokens or session based on auth mode
	if loginResponse.Tokens.AccessToken != "" {
		response["tokens"] = loginResponse.Tokens
	}
	if loginResponse.Session != nil {
		response["session"] = loginResponse.Session
	}

	c.JSON(http.StatusOK, response)
}

// LoginSuperAdmin handles super admin login requests (web dashboard only)
func (h *AuthHandler) LoginSuperAdmin(c *gin.Context) {
	requestID := requestid.Get(c)

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for super admin login")

		apiErr := sharedErrors.BadRequestError("Invalid request body").
			WithRequestID(requestID).
			WithDetails(err.Error())
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// Call usecase to authenticate super admin
	loginResponse, err := h.authUsecase.LoginSuperAdmin(req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Warn("Failed super admin login attempt")

		// Send failed login attempt to analytics
		h.analyticsClient.SendLoginAttemptAsync(analytics.LoginAttemptRequest{
			Email:        req.Email,
			IPAddress:    ipAddress,
			UserAgent:    userAgent,
			Success:      false,
			FailReason:   analytics.DetermineFailReason(err),
			AuthMode:     "password",
			IsSuperAdmin: true,
		})

		// Determine appropriate error based on error message
		var apiErr *sharedErrors.APIError

		if strings.Contains(err.Error(), "2FA is required") {
			apiErr = sharedErrors.TwoFactorRequiredError()
		} else if strings.Contains(err.Error(), "invalid email or password") {
			apiErr = sharedErrors.InvalidCredentialsError()
		} else if strings.Contains(err.Error(), "deactivated") {
			apiErr = sharedErrors.AccountDeactivatedError()
		} else if strings.Contains(err.Error(), "password login is disabled") {
			apiErr = sharedErrors.PasskeyOnlyError()
		} else {
			apiErr = sharedErrors.InvalidCredentialsError()
		}

		apiErr.WithRequestID(requestID)
		c.JSON(apiErr.StatusCode, apiErr)
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
		"auth_mode":  loginResponse.AuthMode,
	}).Info("Super admin logged in successfully")

	// Send successful login attempt to analytics
	userID := uint64(loginResponse.User.ID)
	h.analyticsClient.SendLoginAttemptAsync(analytics.LoginAttemptRequest{
		Email:        loginResponse.User.Email,
		UserID:       &userID,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Success:      true,
		AuthMode:     string(loginResponse.AuthMode),
		IsSuperAdmin: true,
	})

	// Track device for security monitoring
	h.analyticsClient.TrackDeviceAsync(userID, userAgent, ipAddress)

	// Send analytics event (legacy)
	h.analyticsClient.SendEvent(
		analytics.EventUserLogin,
		analytics.CategoryUser,
		uint64(loginResponse.User.ID),
		map[string]interface{}{
			"email":      loginResponse.User.Email,
			"auth_mode":  loginResponse.AuthMode,
			"ip_address": ipAddress,
			"role":       "super_admin",
		},
	)

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		// Calculate MaxAge in seconds (time until expiration)
		maxAge := int(loginResponse.Session.ExpiresAt - time.Now().Unix())
		if maxAge < 0 {
			maxAge = 0
		}

		// Set cookie with SameSite=Lax for local development
		// For production with HTTPS, use SameSite=None with Secure=true
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			maxAge,
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			false, // httpOnly - set to false to allow JS to read it for X-Session-ID header
		)

		// Track session in analytics
		expiresAt := time.Unix(loginResponse.Session.ExpiresAt, 0)
		h.analyticsClient.TrackSessionAsync(
			uint64(loginResponse.User.ID),
			loginResponse.Session.SessionID,
			ipAddress,
			userAgent,
			expiresAt,
		)
	}

	response := gin.H{
		"message":              "Login successful",
		"user":                 loginResponse.User,
		"must_change_password": loginResponse.MustChangePassword,
		"auth_mode":            loginResponse.AuthMode,
		"request_id":           requestID,
	}

	// Add tokens or session based on auth mode
	if loginResponse.Tokens.AccessToken != "" {
		response["tokens"] = loginResponse.Tokens
	}
	if loginResponse.Session != nil {
		response["session"] = loginResponse.Session
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout requests
func (h *AuthHandler) Logout(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get user ID from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Failed to get user ID for logout")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User not authenticated",
			"request_id": requestID,
		})
		return
	}

	// Get session ID if exists
	sessionID := ""
	if sid, exists := c.Get("session_id"); exists {
		if s, ok := sid.(string); ok {
			sessionID = s
		}
	}

	// Call usecase to handle logout
	err = h.authUsecase.Logout(userID, sessionID)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to logout user")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to logout",
			"request_id": requestID,
		})
		return
	}

	// Clear session cookie if exists
	c.SetCookie(
		"session_id",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    userID,
	}).Info("User logged out successfully")

	// Send analytics event
	h.analyticsClient.SendEvent(
		analytics.EventUserLogout,
		analytics.CategoryUser,
		uint64(userID),
		map[string]interface{}{
			"session_id": sessionID,
		},
	)

	// Deactivate session in analytics if session ID exists
	if sessionID != "" {
		h.analyticsClient.DeactivateSessionAsync(sessionID)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Logout successful",
		"request_id": requestID,
	})
}

// RefreshToken handles token refresh requests (placeholder for future implementation)
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Info("Refresh token endpoint called")

	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for token refresh")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Call usecase to refresh token
	tokens, err := h.authUsecase.RefreshToken(req.RefreshToken)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Token refresh failed")

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	logger.WithField("request_id", requestID).Info("Token refreshed successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Token refreshed successfully",
		"request_id": requestID,
		"tokens":     tokens,
	})
}
