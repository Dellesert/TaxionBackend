// File: services/gateway/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize logger
	log := logger.New(&logger.Config{
		Level:       "info",
		Format:      "json",
		Environment: os.Getenv("ENVIRONMENT"),
	})

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Info("Starting Gateway service...")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	// Setup common middleware
	middleware.SetupCommonMiddleware(router)

	// Setup routes
	setupRoutes(router, cfg)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Gateway server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Gateway server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Gateway server stopped")
}

// setupRoutes configures all routes for the gateway
func setupRoutes(router *gin.Engine, cfg *config.Config) {
	// Get proxy configuration
	proxyConfig := getProxyConfig()

	// Health check endpoints
	router.GET("/health", healthHandler)
	router.HEAD("/health", healthHandler)
	router.GET("/health/services", servicesHealthHandler)
	router.GET("/health/ready", readinessHandler)
	router.GET("/health/live", livenessHandler)

	// .well-known endpoints for iOS/Android Universal Links and Passkeys
	router.GET("/.well-known/apple-app-site-association", serveAppleAppSiteAssociation)
	router.GET("/.well-known/assetlinks.json", serveAssetLinks)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (placeholder for now)
		auth := v1.Group("/auth")
		{
			auth.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))

		}

		// User routes - proxy to user service
		users := v1.Group("/users")
		{
			users.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Profile routes - proxy to user service
		profile := v1.Group("/profile")
		{
			profile.Any("", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			profile.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// User settings routes - proxy to user service
		user := v1.Group("/user")
		{
			user.Any("", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			user.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Session management routes - proxy to user service
		sessions := v1.Group("/sessions")
		{
			sessions.Any("", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			sessions.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Department routes - proxy to user service
		departments := v1.Group("/departments")
		{
			departments.Any("", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			departments.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Subdepartment routes - proxy to user service
		subdepartments := v1.Group("/subdepartments")
		{
			subdepartments.Any("", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			subdepartments.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Chat routes - proxy to chat service
		chats := v1.Group("/chats")
		{
			chats.Any("/*path", proxyRequest(proxyConfig.ChatService.URL, proxyConfig.ChatService.Name))
		}

		// Message routes - proxy to chat service
		messages := v1.Group("/messages")
		{
			messages.Any("", proxyRequest(proxyConfig.ChatService.URL, proxyConfig.ChatService.Name))
			messages.Any("/*path", proxyRequest(proxyConfig.ChatService.URL, proxyConfig.ChatService.Name))
		}

		// Task routes - proxy to task service
		tasks := v1.Group("/tasks")
		{
			tasks.Any("/*path", proxyRequest(proxyConfig.TaskService.URL, proxyConfig.TaskService.Name))
		}

		// Task attachments - proxy to task service
		attachments := v1.Group("/attachments")
		{
			attachments.Any("/*path", proxyRequest(proxyConfig.TaskService.URL, proxyConfig.TaskService.Name))
		}

		// Task comments - proxy to task service
		comments := v1.Group("/comments")
		{
			comments.Any("/*path", proxyRequest(proxyConfig.TaskService.URL, proxyConfig.TaskService.Name))
		}

		// Task checklists - proxy to task service
		checklists := v1.Group("/checklists")
		{
			checklists.Any("/*path", proxyRequest(proxyConfig.TaskService.URL, proxyConfig.TaskService.Name))
		}

		// Checklist items - proxy to task service
		checklistItems := v1.Group("/checklist-items")
		{
			checklistItems.Any("/*path", proxyRequest(proxyConfig.TaskService.URL, proxyConfig.TaskService.Name))
		}

		// Calendar routes - proxy to calendar service
		calendar := v1.Group("/calendar")
		{
			calendar.Any("/*path", proxyRequest(proxyConfig.CalendarService.URL, proxyConfig.CalendarService.Name))
		}

		// Event routes - proxy to calendar service
		events := v1.Group("/events")
		{
			events.Any("", proxyRequest(proxyConfig.CalendarService.URL, proxyConfig.CalendarService.Name))
			events.Any("/*path", proxyRequest(proxyConfig.CalendarService.URL, proxyConfig.CalendarService.Name))
		}

		// Poll routes - proxy to poll service
		polls := v1.Group("/polls")
		{
			polls.Any("", proxyRequest(proxyConfig.PollService.URL, proxyConfig.PollService.Name))
			polls.Any("/*path", proxyRequest(proxyConfig.PollService.URL, proxyConfig.PollService.Name))
		}

		// Notification routes - proxy to notification service
		notifications := v1.Group("/notifications")
		{
			notifications.Any("/*path", proxyRequest(proxyConfig.NotificationService.URL, proxyConfig.NotificationService.Name))
		}

		// File routes - proxy to file service
		files := v1.Group("/files")
		{
			files.Any("/*path", proxyRequest(proxyConfig.FileService.URL, proxyConfig.FileService.Name))
		}

		// Analytics routes - proxy to analytics service
		analytics := v1.Group("/analytics")
		{
			analytics.Any("/*path", proxyRequest(proxyConfig.AnalyticsService.URL, proxyConfig.AnalyticsService.Name))
		}

		// Backup routes - proxy to backup service
		backups := v1.Group("/backups")
		{
			backups.Any("", proxyRequest(proxyConfig.BackupService.URL, proxyConfig.BackupService.Name))
			backups.Any("/*path", proxyRequest(proxyConfig.BackupService.URL, proxyConfig.BackupService.Name))
		}

		// WebSocket endpoint - proxy to chat service for real-time communication
		v1.GET("/ws", proxyRequest(proxyConfig.ChatService.URL, proxyConfig.ChatService.Name))

		// Invitation routes (public) - proxy to user service
		invitations := v1.Group("/invitations")
		{
			invitations.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Password reset routes (public) - proxy to user service
		passwordResets := v1.Group("/password-resets")
		{
			passwordResets.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Admin routes within /api/v1
		admin := v1.Group("/admin")
		{
			// System metrics endpoint - specific route handled by gateway
			admin.GET("/system/metrics", systemMetricsHandler)

			// All other specific admin patterns that should be proxied to user service
			// (we can't use wildcard /*path here because it conflicts with /system/metrics)
			admin.Any("/users", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/users/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/departments", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/departments/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/subdepartments", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/subdepartments/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/quick-start", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/quick-start/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/invitations", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/invitations/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/settings", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/settings/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/smtp-settings", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/smtp-settings/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/password-resets", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
			admin.Any("/password-resets/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Super Admin routes within /api/v1 - proxy to user service
		superadmin := v1.Group("/superadmin")
		{
			superadmin.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}
	}

	// Legacy WebSocket endpoint for backward compatibility
	router.GET("/ws", proxyRequest(proxyConfig.ChatService.URL, proxyConfig.ChatService.Name))

	// Public redirect pages for email links (no /api/v1 prefix)
	// These are HTML pages that redirect to the mobile app
	router.GET("/invite/:token", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
	router.GET("/reset-password/:token", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))

	// Admin routes (direct) - proxy to user service
	adminDirect := router.Group("/admin")
	{
		adminDirect.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
	}
}

// placeholderHandler creates a placeholder handler for development
func placeholderHandler(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.WithFields(map[string]interface{}{
			"action": action,
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
		}).Info("Placeholder handler called")

		c.JSON(http.StatusNotImplemented, gin.H{
			"message": fmt.Sprintf("Handler for '%s' not implemented yet", action),
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
		})
	}
}

// serveAppleAppSiteAssociation serves the apple-app-site-association file with correct Content-Type
func serveAppleAppSiteAssociation(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.File("./.well-known/apple-app-site-association")
}

// serveAssetLinks serves the assetlinks.json file with correct Content-Type
func serveAssetLinks(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	c.File("./.well-known/assetlinks.json")
}
