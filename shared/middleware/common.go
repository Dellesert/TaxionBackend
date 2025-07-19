package middleware

import (
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns CORS middleware with default configuration
func CORSMiddleware() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"} // Configure this properly in production
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	config.AllowHeaders = []string{
		"Origin",
		"Content-Type",
		"Content-Length",
		"Accept-Encoding",
		"X-CSRF-Token",
		"Authorization",
		"X-Request-ID",
		"X-Requested-With",
	}
	config.ExposeHeaders = []string{"X-Request-ID"}
	config.AllowCredentials = true
	config.MaxAge = 12 * time.Hour

	return cors.New(config)
}

// RequestIDMiddleware generates and adds request ID to context
func RequestIDMiddleware() gin.HandlerFunc {
	return requestid.New()
}

// RecoveryMiddleware handles panics and returns proper error responses
func RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		requestID := requestid.Get(c)

		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"panic":      recovered,
			"path":       c.Request.URL.Path,
			"method":     c.Request.Method,
		}).Error("Panic recovered")

		c.JSON(500, gin.H{
			"error":      "Internal server error",
			"request_id": requestID,
		})
	})
}

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		requestID := param.Keys["request_id"]
		if requestID == nil {
			requestID = "unknown"
		}

		// Log structured request info
		logger.WithFields(map[string]interface{}{
			"request_id":  requestID,
			"method":      param.Method,
			"path":        param.Path,
			"status_code": param.StatusCode,
			"latency":     param.Latency,
			"client_ip":   param.ClientIP,
			"user_agent":  param.Request.UserAgent(),
			"body_size":   param.BodySize,
		}).Info("HTTP Request")

		return ""
	})
}

// LoggerMiddlewareWithRequestID logs HTTP requests with request ID extraction
func LoggerMiddlewareWithRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get request ID
		requestID := requestid.Get(c)

		// Build path with query string
		if raw != "" {
			path = path + "?" + raw
		}

		// Get client IP
		clientIP := c.ClientIP()

		// Log the request
		logFields := map[string]interface{}{
			"request_id":  requestID,
			"method":      c.Request.Method,
			"path":        path,
			"status_code": c.Writer.Status(),
			"latency":     latency,
			"client_ip":   clientIP,
			"user_agent":  c.Request.UserAgent(),
			"body_size":   c.Writer.Size(),
		}

		// Add user ID if available (for authenticated requests)
		if userID, exists := c.Get("user_id"); exists {
			logFields["user_id"] = userID
		}

		// Log based on status code
		statusCode := c.Writer.Status()
		switch {
		case statusCode >= 500:
			logger.WithFields(logFields).Error("HTTP Request - Server Error")
		case statusCode >= 400:
			logger.WithFields(logFields).Warn("HTTP Request - Client Error")
		default:
			logger.WithFields(logFields).Info("HTTP Request")
		}
	}
}

// SetupCommonMiddleware sets up all common middleware in the correct order
func SetupCommonMiddleware(r *gin.Engine) {
	// Recovery should be first to catch any panics
	r.Use(RecoveryMiddleware())

	// Request ID for tracking
	r.Use(RequestIDMiddleware())

	// CORS for cross-origin requests
	r.Use(CORSMiddleware())

	// Request logging
	r.Use(LoggerMiddlewareWithRequestID())
}
