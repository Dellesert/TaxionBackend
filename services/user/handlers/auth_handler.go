package handlers

import (
	"net/http"
	"strings"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/logger"

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

// Register handles user registration requests
func (h *AuthHandler) Register(c *gin.Context) {
	requestID := requestid.Get(c)

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

	// Call usecase to authenticate user
	loginResponse, err := h.authUsecase.Login(req.Email, req.Password)
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
		} else if strings.Contains(err.Error(), "deactivated") {
			statusCode = http.StatusForbidden
			errorMessage = "Account is deactivated"
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
	}).Info("User logged in successfully")

	c.JSON(http.StatusOK, gin.H{
		"message":    "Login successful",
		"user":       loginResponse.User,
		"tokens":     loginResponse.Tokens,
		"request_id": requestID,
	})
}

// Logout handles user logout requests (placeholder for future implementation)
func (h *AuthHandler) Logout(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Info("Logout endpoint called")

	// TODO: Implement logout logic
	// - Blacklist the current JWT token
	// - Update user status to offline
	// - Clear any session data

	c.JSON(http.StatusOK, gin.H{
		"message":    "Logout successful",
		"request_id": requestID,
	})
}

// RefreshToken handles token refresh requests (placeholder for future implementation)
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Info("Refresh token endpoint called")

	// TODO: Implement token refresh logic
	// - Validate refresh token
	// - Generate new access token
	// - Optionally rotate refresh token

	c.JSON(http.StatusNotImplemented, gin.H{
		"error":      "Token refresh not implemented yet",
		"request_id": requestID,
	})
}
