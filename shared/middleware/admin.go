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

// RequireDepartmentHeadOrAbove creates middleware that requires department_head, admin, or super_admin role
func RequireDepartmentHeadOrAbove() gin.HandlerFunc {
	return RequireRole(models.RoleDepartmentHead, models.RoleAdmin, models.RoleSuperAdmin)
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

		// Skip validation for GET and DELETE requests
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" {
			c.Next()
			return
		}

		// Skip validation for requests without body (Content-Length: 0 or no body)
		// This allows POST requests that don't need a body (like /activate endpoints)
		if c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		// Check Content-Type for POST/PUT requests with body
		contentType := c.GetHeader("Content-Type")
		// Allow application/json and multipart/form-data (for file uploads)
		isJSON := contentType == "application/json"
		isMultipart := len(contentType) >= 19 && contentType[:19] == "multipart/form-data"

		if !isJSON && !isMultipart {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":      "Invalid Content-Type",
				"message":    "Content-Type must be application/json or multipart/form-data for this request",
				"request_id": requestID,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAdminOrDepartmentHead creates middleware that requires admin or department head of specified department
// departmentIDParam is the URL parameter name containing the department ID (e.g., "id")
func RequireAdminOrDepartmentHead(departmentIDParam string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := requestid.Get(c)

		// Get user info from context
		userRole, roleExists := c.Get("user_role")
		userID, userIDExists := c.Get("user_id")

		if !roleExists || !userIDExists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":      "Authentication required",
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

		// Admins and super admins have full access
		if role == models.RoleAdmin || role == models.RoleSuperAdmin {
			c.Next()
			return
		}

		// For department heads, check if they are the head of this specific department
		if role == models.RoleDepartmentHead {
			// Get department ID from URL parameter
			departmentIDStr := c.Param(departmentIDParam)
			if departmentIDStr == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":      "Department ID required",
					"request_id": requestID,
				})
				c.Abort()
				return
			}

			// Store department ID for handler to use
			c.Set("requested_department_id", departmentIDStr)
			c.Set("requires_department_head_check", true)

			// Handler must verify the user is actually the head of this department
			c.Next()
			return
		}

		// Other roles don't have access
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"user_role":  role,
			"method":     c.Request.Method,
			"path":       c.Request.URL.Path,
		}).Warn("Unauthorized department access attempt")

		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Insufficient permissions",
			"message":    "Only admins and department heads can perform this action",
			"request_id": requestID,
		})
		c.Abort()
	}
}
