package middleware

import (
	"net/http"

	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/models"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// RequireAdminRole creates middleware that requires admin or super_admin role
func RequireAdminRole() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin, models.RoleSuperAdmin)
}

// RequireSuperAdminRole creates middleware that requires super_admin role only
func RequireSuperAdminRole() gin.HandlerFunc {
	return RequireRole(models.RoleSuperAdmin)
}

// RequireManagerOrAbove creates middleware that requires manager, admin, or super_admin role
func RequireManagerOrAbove() gin.HandlerFunc {
	return RequireRole(models.RoleManager, models.RoleAdmin, models.RoleSuperAdmin)
}

// AdminOnlyMiddleware is a more specific admin middleware with better error messages
func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)

		// Check if user is authenticated
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authentication required",
				"message":    "Please log in to access admin features",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		role, ok := userRole.(models.Role)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Invalid authentication data",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// Check if user has admin privileges
		if role != models.RoleAdmin && role != models.RoleSuperAdmin {
			userID, _ := c.Get("user_id")
			userEmail, _ := c.Get("user_email")

			// Log unauthorized access attempt
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"user_email": userEmail,
				"user_role":  role,
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
			}).Warn("Unauthorized admin access attempt")

			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Admin access required",
				"message":    "This action requires administrator privileges",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// User has admin privileges, continue
		c.Next()
	}
}

// SuperAdminOnlyMiddleware requires super admin role specifically
func SuperAdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)

		// Check if user is authenticated
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authentication required",
				"message":    "Please log in to access super admin features",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		role, ok := userRole.(models.Role)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Invalid authentication data",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// Check if user has super admin privileges
		if role != models.RoleSuperAdmin {
			userID, _ := c.Get("user_id")
			userEmail, _ := c.Get("user_email")

			// Log unauthorized access attempt
			logger.WithFields(map[string]interface{}{
				"request_id": requestID,
				"user_id":    userID,
				"user_email": userEmail,
				"user_role":  role,
				"method":     c.Request.Method,
				"path":       c.Request.URL.Path,
			}).Warn("Unauthorized super admin access attempt")

			c.JSON(http.StatusForbidden, gin.H{
				"error":      "Super admin access required",
				"message":    "This action requires super administrator privileges",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		// User has super admin privileges, continue
		c.Next()
	}
}

// LogAdminAction middleware logs admin actions for audit purposes
func LogAdminAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)
		userID, _ := c.Get("user_id")
		userEmail, _ := c.Get("user_email")
		userRole, _ := c.Get("user_role")

		// Log the admin action before processing
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"admin_id":    userID,
			"admin_email": userEmail,
			"admin_role":  userRole,
			"action":      action,
			"method":      c.Request.Method,
			"path":        c.Request.URL.Path,
			"user_agent":  c.Request.UserAgent(),
			"client_ip":   c.ClientIP(),
		}).Info("Admin action initiated")

		c.Next()

		// Log completion status
		statusCode := c.Writer.Status()
		if statusCode >= 200 && statusCode < 300 {
			logger.WithFields(map[string]interface{}{
				"request_id":  requestID,
				"admin_id":    userID,
				"action":      action,
				"status_code": statusCode,
			}).Info("Admin action completed successfully")
		} else {
			logger.WithFields(map[string]interface{}{
				"request_id":  requestID,
				"admin_id":    userID,
				"action":      action,
				"status_code": statusCode,
			}).Warn("Admin action failed")
		}
	}
}

// ValidateAdminRequest middleware validates that admin requests have valid structure
func ValidateAdminRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)

		// Skip validation for GET requests
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
			c.Next()
			return
		}

		// Check Content-Type for POST/PUT requests
		contentType := c.GetHeader("Content-Type")
		if contentType != "application/json" && c.Request.Method != "DELETE" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid Content-Type",
				"message":    "Content-Type must be application/json for this request",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
