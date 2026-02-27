package middleware

import (
	"fmt"
	"os"
	"strings"
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// CORSMiddleware returns custom CORS middleware to avoid duplicate headers
func CORSMiddleware() gin.HandlerFunc {
	// Read CORS origins from environment variable
	corsOrigins := os.Getenv("CORS_ORIGINS")
	var allowedOrigins []string

	if corsOrigins != "" {
		// Split by comma and trim spaces
		origins := strings.Split(corsOrigins, ",")
		for _, origin := range origins {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(origin))
		}
		logger.WithField("origins", allowedOrigins).Info("CORS configured from environment variable")
	} else {
		// Fallback to default origins
		allowedOrigins = []string{"http://localhost:8093", "http://localhost:3000"}
		logger.Warn("CORS_ORIGINS not set, using default origins")
	}

	// Create a map for fast origin lookup
	allowedOriginsMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		allowedOriginsMap[origin] = true
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Handle preflight OPTIONS requests first
		if c.Request.Method == "OPTIONS" {
			// Check if origin is allowed
			if origin != "" && allowedOriginsMap[origin] {
				// Set CORS headers for allowed origin
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID, X-Requested-With, X-Session-ID")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
				c.Header("Access-Control-Expose-Headers", "X-Request-ID")
				c.Header("Access-Control-Max-Age", "43200")
				c.AbortWithStatus(204)
			} else {
				// Origin not allowed - return 403 Forbidden
				logger.WithFields(map[string]interface{}{
					"origin":          origin,
					"allowed_origins": allowedOrigins,
				}).Warn("CORS preflight request from non-allowed origin")
				c.AbortWithStatus(403)
			}
			return
		}

		// For non-OPTIONS requests, set CORS headers if origin is allowed
		if origin != "" && allowedOriginsMap[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Request-ID, X-Requested-With, X-Session-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			c.Header("Access-Control-Max-Age", "43200")
		}

		c.Next()
	}
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

		// Capture panic in Sentry
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			hub.RecoverWithContext(c.Request.Context(), recovered)
		} else if recovered != nil {
			sentry.CaptureException(fmt.Errorf("panic: %v", recovered))
		}

		c.JSON(500, gin.H{
			"error":      "Внутренняя ошибка сервера",
			"request_id": requestID,
		})
	})
}

// SentryMiddleware returns the sentrygin middleware for capturing errors and traces
func SentryMiddleware() gin.HandlerFunc {
	return sentrygin.New(sentrygin.Options{
		Repanic: true,
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
// This version includes CORS - use for Gateway only
func SetupCommonMiddleware(r *gin.Engine) {
	// Sentry must be before recovery to capture panics
	r.Use(SentryMiddleware())

	// Recovery should be first to catch any panics
	r.Use(RecoveryMiddleware())

	// Request ID for tracking
	r.Use(RequestIDMiddleware())

	// CORS for cross-origin requests (Gateway only)
	r.Use(CORSMiddleware())

	// Request logging
	r.Use(LoggerMiddlewareWithRequestID())
}

// SetupCommonMiddlewareWithoutCORS sets up common middleware without CORS
// Use this for microservices - Gateway handles CORS
func SetupCommonMiddlewareWithoutCORS(r *gin.Engine) {
	// Sentry must be before recovery to capture panics
	r.Use(SentryMiddleware())

	// Recovery should be first to catch any panics
	r.Use(RecoveryMiddleware())

	// Request ID for tracking
	r.Use(RequestIDMiddleware())

	// Request logging
	r.Use(LoggerMiddlewareWithRequestID())
}
