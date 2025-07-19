package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"tachyon-messenger/shared/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
	Issuer               string
}

// DefaultJWTConfig returns default JWT configuration
func DefaultJWTConfig(secret string) *JWTConfig {
	return &JWTConfig{
		Secret:               secret,
		AccessTokenDuration:  15 * time.Minute,
		RefreshTokenDuration: 7 * 24 * time.Hour, // 7 days
		Issuer:               "tachyon-messenger",
	}
}

// GenerateTokens generates access and refresh token pair
func GenerateTokens(userID uint, email string, role models.Role, config *JWTConfig) (*models.TokenPair, error) {
	// Generate access token
	accessToken, err := generateToken(userID, email, role, config.AccessTokenDuration, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := generateToken(userID, email, role, config.RefreshTokenDuration, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateToken generates a JWT token with specified duration
func generateToken(userID uint, email string, role models.Role, duration time.Duration, config *JWTConfig) (string, error) {
	now := time.Now()
	claims := &models.Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    config.Issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.Secret))
}

// ValidateToken validates and parses JWT token
func ValidateToken(tokenString string, config *JWTConfig) (*models.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &models.Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*models.Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// ExtractUserID extracts user ID from JWT token
func ExtractUserID(tokenString string, config *JWTConfig) (uint, error) {
	claims, err := ValidateToken(tokenString, config)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

// JWTMiddleware creates JWT authentication middleware
func JWTMiddleware(config *JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check Bearer token format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := ValidateToken(tokenString, config)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Set user data in context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

// RequireRole creates middleware that requires specific role
func RequireRole(allowedRoles ...models.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found in context",
			})
			c.Abort()
			return
		}

		role, ok := userRole.(models.Role)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid role type in context",
			})
			c.Abort()
			return
		}

		// Check if user role is in allowed roles
		for _, allowedRole := range allowedRoles {
			if role == allowedRole {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "Insufficient permissions",
		})
		c.Abort()
	}
}

// GetUserIDFromContext extracts user ID from gin context
func GetUserIDFromContext(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user ID not found in context")
	}

	id, ok := userID.(uint)
	if !ok {
		return 0, fmt.Errorf("invalid user ID type in context")
	}

	return id, nil
}

// GetUserRoleFromContext extracts user role from gin context
func GetUserRoleFromContext(c *gin.Context) (models.Role, error) {
	userRole, exists := c.Get("user_role")
	if !exists {
		return "", fmt.Errorf("user role not found in context")
	}

	role, ok := userRole.(models.Role)
	if !ok {
		return "", fmt.Errorf("invalid role type in context")
	}

	return role, nil
}
