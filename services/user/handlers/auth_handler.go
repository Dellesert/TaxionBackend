package handlers

import (
	"net/http"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for authentication
type AuthHandler struct {
	authUsecase usecase.AuthUsecase
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authUsecase usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{
		authUsecase: authUsecase,
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
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Warn("Invalid request body for user login")

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Additional validation
	if strings.TrimSpace(req.Email) == "" {
		logger.WithField("request_id", requestID).Warn("Email is required for login")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Email is required",
			"request_id": requestID,
		})
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		logger.WithField("request_id", requestID).Warn("Password is required for login")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Password is required",
			"request_id": requestID,
		})
		return
	}

	// Extract client info for session tracking
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	// Call usecase to authenticate user
	loginResponse, err := h.authUsecase.Login(req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"email":      req.Email,
			"error":      err.Error(),
		}).Warn("Failed login attempt")

		// Determine appropriate HTTP status code based on error
		statusCode := http.StatusUnauthorized
		errorMessage := "Invalid credentials"

		if strings.Contains(err.Error(), "invalid email or password") {
			statusCode = http.StatusUnauthorized
			errorMessage = "Invalid email or password"
		} else if strings.Contains(err.Error(), "2FA is required") ||
			strings.Contains(err.Error(), "2FA") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Account is deactivated"
		} else if strings.Contains(err.Error(), "Passkey") ||
			strings.Contains(err.Error(), "password login is disabled") {
			statusCode = http.StatusForbidden
			errorMessage = err.Error()
		} else if strings.Contains(err.Error(), "email is required") ||
			strings.Contains(err.Error(), "password is required") {
			statusCode = http.StatusBadRequest
			errorMessage = err.Error()
		} else {
			// For any other error, keep it generic for security
			statusCode = http.StatusInternalServerError
			errorMessage = "Login failed"
		}

		c.JSON(statusCode, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
		"auth_mode":  loginResponse.AuthMode,
	}).Info("User logged in successfully")

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			int(loginResponse.Session.ExpiresAt),
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			true,  // httpOnly
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

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request body",
			"details":    err.Error(),
			"request_id": requestID,
		})
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

		// Check if error is 2FA required
		errorMessage := "Invalid credentials"
		if strings.Contains(err.Error(), "2FA") || strings.Contains(err.Error(), "two-factor") {
			errorMessage = err.Error()
		}

		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      errorMessage,
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"user_id":    loginResponse.User.ID,
		"email":      loginResponse.User.Email,
		"auth_mode":  loginResponse.AuthMode,
	}).Info("Super admin logged in successfully")

	// Set session cookie if in session mode
	if loginResponse.Session != nil {
		c.SetCookie(
			"session_id",
			loginResponse.Session.SessionID,
			int(loginResponse.Session.ExpiresAt),
			"/",
			"",
			false, // secure - set to true in production with HTTPS
			true,  // httpOnly
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
