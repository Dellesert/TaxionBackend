package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/user/handlers"
	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/services/user/usecase"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

	"github.com/gin-contrib/requestid"
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

	log.Info("Starting User service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.Migrate(&models.Department{}, &models.User{}); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize dependencies
	userRepo := repository.NewUserRepository(db)
	departmentRepo := repository.NewDepartmentRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize usecases
	userUsecase := usecase.NewUserUsecase(userRepo)
	authUsecase := usecase.NewAuthUsecase(userRepo, departmentRepo, jwtConfig)
	profileUsecase := usecase.NewProfileUsecase(userRepo, departmentRepo)
	adminUsecase := usecase.NewAdminUsecase(userRepo, departmentRepo)
	departmentUsecase := usecase.NewDepartmentUsecase(departmentRepo, userRepo)

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userUsecase)
	authHandler := handlers.NewAuthHandler(authUsecase)
	profileHandler := handlers.NewProfileHandler(profileUsecase)
	departmentHandler := handlers.NewDepartmentHandler(departmentUsecase)

	// Create Gin router
	router := gin.New()

	// Setup common middleware
	middleware.SetupCommonMiddleware(router)

	// Setup routes
	setupRoutes(router, userHandler, authHandler, profileHandler, departmentHandler, jwtConfig)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", getServerPort()),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("User service starting on port %s", getServerPort())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down User service...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("User service stopped")
}

// setupRoutes configures all routes for the user service
func setupRoutes(router *gin.Engine, userHandler *handlers.UserHandler, authHandler *handlers.AuthHandler, profileHandler *handlers.ProfileHandler, departmentHandler *handlers.DepartmentHandler, jwtConfig *middleware.JWTConfig) {
	// Health check endpoint
	router.GET("/health", healthHandler)

	// Public authentication routes (no JWT required)
	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/logout", authHandler.Logout)        // TODO: Add JWT middleware when implemented
		auth.POST("/refresh", authHandler.RefreshToken) // TODO: Add refresh token validation
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public authentication routes (alternative paths)
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)

		// Protected user routes (require JWT authentication)
		users := v1.Group("/users")
		users.Use(middleware.JWTMiddleware(jwtConfig)) // Apply JWT middleware to all user routes
		{
			users.GET("", userHandler.GetUsers)                                                          // GET /api/v1/users
			users.POST("", middleware.RequireRole("admin", "super_admin"), userHandler.CreateUser)       // POST /api/v1/users (admin only)
			users.GET("/:id", userHandler.GetUser)                                                       // GET /api/v1/users/:id
			users.PUT("/:id", userHandler.UpdateUser)                                                    // PUT /api/v1/users/:id
			users.DELETE("/:id", middleware.RequireRole("admin", "super_admin"), userHandler.DeleteUser) // DELETE /api/v1/users/:id (admin only)
		}

		// Protected profile routes (require JWT authentication)
		profile := v1.Group("/profile")
		profile.Use(middleware.JWTMiddleware(jwtConfig))
		{
			profile.GET("", profileHandler.GetMyProfile)            // GET /api/v1/profile (current user)
			profile.PUT("", profileHandler.UpdateMyProfile)         // PUT /api/v1/profile (current user)
			profile.PUT("/password", profileHandler.ChangePassword) // PUT /api/v1/profile/password
			profile.PUT("/status", profileHandler.UpdateStatus)     // PUT /api/v1/profile/status
			profile.GET("/:id", profileHandler.GetProfile)          // GET /api/v1/profile/:id (any user profile)
		}

		// Admin only routes
		admin := v1.Group("/admin")
		admin.Use(middleware.JWTMiddleware(jwtConfig))
		admin.Use(middleware.RequireRole("admin", "super_admin"))
		{
			// Department management
			admin.GET("/departments", departmentHandler.GetDepartments)                   // GET /api/v1/admin/departments
			admin.POST("/departments", departmentHandler.CreateDepartment)                // POST /api/v1/admin/departments
			admin.GET("/departments/:id", departmentHandler.GetDepartment)                // GET /api/v1/admin/departments/:id
			admin.PUT("/departments/:id", departmentHandler.UpdateDepartment)             // PUT /api/v1/admin/departments/:id
			admin.DELETE("/departments/:id", departmentHandler.DeleteDepartment)          // DELETE /api/v1/admin/departments/:id
			admin.GET("/departments/:id/users", departmentHandler.GetDepartmentWithUsers) // GET /api/v1/admin/departments/:id/users

			// User management (placeholder for now)
			admin.GET("/users/stats", getUserStatsHandler)          // GET /api/v1/admin/users/stats
			admin.PUT("/users/:id/status", updateUserStatusHandler) // PUT /api/v1/admin/users/:id/status
			admin.PUT("/users/:id/role", updateUserRoleHandler)     // PUT /api/v1/admin/users/:id/role
		}
	}
}

// Placeholder handlers for admin user management
func getUserStatsHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":      "Get user stats not implemented yet",
		"request_id": requestID,
	})
}

func updateUserStatusHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":      "Update user status not implemented yet",
		"request_id": requestID,
	})
}

func updateUserRoleHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":      "Update user role not implemented yet",
		"request_id": requestID,
	})
}

// healthHandler handles health check requests
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "user-service",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// getServerPort returns the server port from environment or default
func getServerPort() string {
	if port := os.Getenv("USER_SERVICE_PORT"); port != "" {
		return port
	}
	return "8081" // Default port for user service
}
