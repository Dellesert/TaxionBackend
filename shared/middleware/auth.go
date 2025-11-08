package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"tachyon-messenger/shared/models"
	"tachyon-messenger/shared/session"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Mode            models.AuthMode
	JWTConfig       *JWTConfig
	SessionStore    *session.SessionStore
	SessionDuration time.Duration
}

// Global auth config with thread-safe access
var (
	globalAuthConfig *AuthConfig
	authConfigMutex  sync.RWMutex
)

// InitAuthConfig initializes global authentication configuration
func InitAuthConfig(mode models.AuthMode, jwtConfig *JWTConfig, redisClient *redis.Client, sessionDuration time.Duration) {
	authConfigMutex.Lock()
	defer authConfigMutex.Unlock()

	var sessionStore *session.SessionStore
	if mode == models.AuthModeSession || redisClient != nil {
		sessionStore = session.NewSessionStore(redisClient, sessionDuration)
	}

	globalAuthConfig = &AuthConfig{
		Mode:            mode,
		JWTConfig:       jwtConfig,
		SessionStore:    sessionStore,
		SessionDuration: sessionDuration,
	}
}

// GetAuthConfig returns current authentication configuration
func GetAuthConfig() *AuthConfig {
	authConfigMutex.RLock()
	defer authConfigMutex.RUnlock()
	return globalAuthConfig
}

// SetAuthMode updates authentication mode (for admin panel)
func SetAuthMode(mode models.AuthMode) error {
	authConfigMutex.Lock()
	defer authConfigMutex.Unlock()

	if globalAuthConfig == nil {
		return fmt.Errorf("auth config not initialized")
	}

	if mode != models.AuthModeJWT && mode != models.AuthModeSession {
		return fmt.Errorf("invalid auth mode: %s", mode)
	}

	// Ensure session store is available for session mode
	if mode == models.AuthModeSession && globalAuthConfig.SessionStore == nil {
		return fmt.Errorf("session store not available")
	}

	globalAuthConfig.Mode = mode
	return nil
}

// GetAuthMode returns current authentication mode
func GetAuthMode() models.AuthMode {
	authConfigMutex.RLock()
	defer authConfigMutex.RUnlock()

	if globalAuthConfig == nil {
		return models.AuthModeJWT // Default
	}
	return globalAuthConfig.Mode
}

// UpdateSessionDuration updates the session duration dynamically (called when settings change)
func UpdateSessionDuration(newDuration time.Duration) error {
	authConfigMutex.Lock()
	defer authConfigMutex.Unlock()

	if globalAuthConfig == nil {
		return fmt.Errorf("auth config not initialized")
	}

	if globalAuthConfig.SessionStore == nil {
		return fmt.Errorf("session store not available")
	}

	globalAuthConfig.SessionDuration = newDuration
	globalAuthConfig.SessionStore.UpdateSessionDuration(newDuration)
	return nil
}

// GetSessionDuration returns current session duration
func GetSessionDuration() time.Duration {
	authConfigMutex.RLock()
	defer authConfigMutex.RUnlock()

	if globalAuthConfig == nil || globalAuthConfig.SessionStore == nil {
		return 7 * 24 * time.Hour // Default 7 days
	}
	return globalAuthConfig.SessionStore.GetSessionDuration()
}

// AuthMiddleware creates unified authentication middleware that supports both JWT and session
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := GetAuthConfig()
		if config == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Authentication not configured",
			})
			c.Abort()
			return
		}

		// Try both authentication methods based on current mode
		switch config.Mode {
		case models.AuthModeSession:
			// Try session authentication first
			if authenticateWithSession(c, config) {
				c.Next()
				return
			}
			// Fallback to JWT for backward compatibility during transition
			if authenticateWithJWT(c, config) {
				c.Next()
				return
			}

		case models.AuthModeJWT:
			// Try JWT authentication first
			if authenticateWithJWT(c, config) {
				c.Next()
				return
			}
			// No fallback for JWT mode

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid authentication mode",
			})
			c.Abort()
			return
		}

		// Authentication failed
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Authentication required",
		})
		c.Abort()
	}
}

// authenticateWithJWT authenticates request using JWT token
func authenticateWithJWT(c *gin.Context, config *AuthConfig) bool {
	if config.JWTConfig == nil {
		return false
	}

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	// Check Bearer token format
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return false
	}

	tokenString := tokenParts[1]

	// Validate token
	claims, err := ValidateToken(tokenString, config.JWTConfig)
	if err != nil {
		return false
	}

	// Set user data in context
	c.Set("user_id", claims.UserID)
	c.Set("user_email", claims.Email)
	c.Set("user_role", claims.Role)
	c.Set("claims", claims)
	c.Set("auth_method", "jwt")

	return true
}

// authenticateWithSession authenticates request using session
func authenticateWithSession(c *gin.Context, config *AuthConfig) bool {
	if config.SessionStore == nil {
		return false
	}

	// Try to get session ID from cookie
	sessionID, err := c.Cookie("session_id")
	if err != nil || sessionID == "" {
		// Try to get from header as fallback
		sessionID = c.GetHeader("X-Session-ID")
		if sessionID == "" {
			return false
		}
	}

	// Get session from store
	ctx := context.Background()
	session, err := config.SessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return false
	}

	// Update session activity
	config.SessionStore.UpdateSessionActivity(ctx, sessionID)

	// Set user data in context
	c.Set("user_id", session.UserID)
	c.Set("user_email", session.Email)
	c.Set("user_role", session.Role)
	c.Set("session_id", sessionID)
	c.Set("session", session)
	c.Set("auth_method", "session")

	return true
}

// ExtractClientInfo extracts IP address and user agent from request
func ExtractClientInfo(c *gin.Context) (ipAddress, userAgent string) {
	// Get IP address
	ipAddress = c.ClientIP()

	// Get user agent
	userAgent = c.GetHeader("User-Agent")

	return ipAddress, userAgent
}
