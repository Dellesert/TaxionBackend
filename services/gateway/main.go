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

	// Health check endpoint
	router.GET("/health", healthHandler)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Authentication routes (placeholder for now)
		auth := v1.Group("/auth")
		{
			auth.POST("/login", placeholderHandler("login"))
			auth.POST("/register", placeholderHandler("register"))
			auth.POST("/refresh", placeholderHandler("refresh"))
			auth.POST("/logout", placeholderHandler("logout"))
		}

		// User routes - proxy to user service
		users := v1.Group("/users")
		{
			users.Any("/*path", proxyRequest(proxyConfig.UserService.URL, proxyConfig.UserService.Name))
		}

		// Chat routes (placeholder for now)
		chats := v1.Group("/chats")
		{
			chats.GET("", placeholderHandler("get chats"))
			chats.POST("", placeholderHandler("create chat"))
			chats.GET("/:id", placeholderHandler("get chat"))
			chats.PUT("/:id", placeholderHandler("update chat"))
			chats.DELETE("/:id", placeholderHandler("delete chat"))
		}

		// Task routes (placeholder for now)
		tasks := v1.Group("/tasks")
		{
			tasks.GET("", placeholderHandler("get tasks"))
			tasks.POST("", placeholderHandler("create task"))
			tasks.GET("/:id", placeholderHandler("get task"))
			tasks.PUT("/:id", placeholderHandler("update task"))
			tasks.DELETE("/:id", placeholderHandler("delete task"))
		}

		// Calendar routes (placeholder for now)
		calendar := v1.Group("/calendar")
		{
			calendar.GET("/events", placeholderHandler("get events"))
			calendar.POST("/events", placeholderHandler("create event"))
			calendar.GET("/events/:id", placeholderHandler("get event"))
			calendar.PUT("/events/:id", placeholderHandler("update event"))
			calendar.DELETE("/events/:id", placeholderHandler("delete event"))
		}

		// Poll routes (placeholder for now)
		polls := v1.Group("/polls")
		{
			polls.GET("", placeholderHandler("get polls"))
			polls.POST("", placeholderHandler("create poll"))
			polls.GET("/:id", placeholderHandler("get poll"))
			polls.PUT("/:id", placeholderHandler("update poll"))
			polls.DELETE("/:id", placeholderHandler("delete poll"))
		}

		// File routes (placeholder for now)
		files := v1.Group("/files")
		{
			files.POST("/upload", placeholderHandler("upload file"))
			files.GET("/:id", placeholderHandler("get file"))
			files.DELETE("/:id", placeholderHandler("delete file"))
		}

		// Analytics routes (placeholder for now)
		analytics := v1.Group("/analytics")
		{
			analytics.GET("/dashboard", placeholderHandler("get dashboard"))
			analytics.GET("/reports", placeholderHandler("get reports"))
		}
	}

	// WebSocket endpoint placeholder
	router.GET("/ws", placeholderHandler("websocket"))
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
