package main

// NOTE: This is an updated version of main.go with auth config initialization
// Replace main.go with this file after reviewing the changes

/*
Key changes:
1. Added Redis client initialization
2. Added unified auth configuration (supports both JWT and session modes)
3. Updated middleware from JWTMiddleware to AuthMiddleware
4. Added settings handler and routes
5. Added admin endpoints for auth mode switching

Instructions:
- Review this file
- Replace services/user/main.go with this content
- Ensure .env has AUTH_MODE=jwt or AUTH_MODE=session
- Ensure .env has SESSION_DURATION_HOURS (default: 168 = 7 days)
*/

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
	"tachyon-messenger/shared/email"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sharedredis "tachyon-messenger/shared/redis"

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

	// Run database migrations (including 2FA, passkey, and system settings tables)
	if err := db.Migrate(
		&models.Department{},
		&models.User{},
		&models.TwoFactorCode{},
		&models.PasskeyCredential{},
		&models.SystemSettings{},
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Connect to Redis
	redisConfig := sharedredis.DefaultConfig(cfg.Redis.URL)
	redisClient, err := sharedredis.ConnectRedis(redisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	log.Info("Redis connected successfully")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	departmentRepo := repository.NewDepartmentRepository(db)
	twoFARepo := repository.NewTwoFARepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	// Initialize email service for 2FA
	emailConfig := email.LoadConfigFromEnv()
	emailService := email.NewEmailService(emailConfig)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize authentication configuration (supports both JWT and session modes)
	authMode := sharedmodels.AuthMode(cfg.Auth.Mode)
	sessionDuration := time.Duration(cfg.Auth.SessionDuration) * time.Hour
	middleware.InitAuthConfig(authMode, jwtConfig, redisClient.Client, sessionDuration)

	log.Infof("Authentication initialized in %s mode", authMode)

	// Initialize WebAuthn service
	webAuthnService, err := usecase.NewWebAuthnService()
	if err != nil {
		log.Fatalf("Failed to initialize WebAuthn service: %v", err)
	}

	// Initialize usecases
	userUsecase := usecase.NewUserUsecase(userRepo)
	authUsecase := usecase.NewAuthUsecase(userRepo, departmentRepo, settingsRepo, jwtConfig)
	profileUsecase := usecase.NewProfileUsecase(userRepo, departmentRepo)
	adminUsecase := usecase.NewAdminUsecase(userRepo, departmentRepo)
	departmentUsecase := usecase.NewDepartmentUsecase(departmentRepo, userRepo)
	initUsecase := usecase.NewInitUsecase(userRepo)
	twoFAUsecase := usecase.NewTwoFAUsecase(userRepo, twoFARepo, emailService, authUsecase)
	settingsUsecase := usecase.NewSettingsUsecase(settingsRepo, userRepo, passkeyRepo)
	passkeyUsecase := usecase.NewPasskeyUsecase(userRepo, passkeyRepo, settingsRepo, webAuthnService)

	// Initialize super admin if not exists
	if err := initUsecase.InitializeSuperAdmin(); err != nil {
		log.Errorf("Failed to initialize super admin: %v", err)
		// Don't fail the startup, just log the error
	}

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userUsecase)
	authHandler := handlers.NewAuthHandler(authUsecase)
	profileHandler := handlers.NewProfileHandler(profileUsecase)
	departmentHandler := handlers.NewDepartmentHandler(departmentUsecase)
	adminHandler := handlers.NewAdminHandler(adminUsecase, userUsecase)
	settingsHandler := handlers.NewSettingsHandler(settingsUsecase)
	sessionHandler := handlers.NewSessionHandler()
	twoFAHandler := handlers.NewTwoFAHandler(twoFAUsecase, authUsecase, jwtConfig)
	passkeyHandler := handlers.NewPasskeyHandler(passkeyUsecase)

	// Create Gin router
	router := gin.New()

	// Set max multipart memory for file uploads (32 MB)
	router.MaxMultipartMemory = 32 << 20

	// Setup common middleware (without CORS - Gateway handles it)
	middleware.SetupCommonMiddlewareWithoutCORS(router)

	// Setup routes
	setupRoutes(router, userHandler, authHandler, profileHandler, departmentHandler, adminHandler, settingsHandler, sessionHandler, twoFAHandler, passkeyHandler, jwtConfig)

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
func setupRoutes(router *gin.Engine, userHandler *handlers.UserHandler, authHandler *handlers.AuthHandler, profileHandler *handlers.ProfileHandler, departmentHandler *handlers.DepartmentHandler, adminHandler *handlers.AdminHandler, settingsHandler *handlers.SettingsHandler, sessionHandler *handlers.SessionHandler, twoFAHandler *handlers.TwoFAHandler, passkeyHandler *handlers.PasskeyHandler, jwtConfig *middleware.JWTConfig) {
	// Health check endpoint
	router.GET("/health", healthHandler)

	// Public authentication routes (no auth required for login/register)
	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/login/superadmin", authHandler.LoginSuperAdmin)
		auth.POST("/logout", middleware.AuthMiddleware(), authHandler.Logout) // Requires auth
		auth.POST("/refresh", authHandler.RefreshToken)

		// 2FA endpoints (public - no auth required)
		auth.POST("/2fa/send", twoFAHandler.SendCode)
		auth.POST("/2fa/verify", twoFAHandler.VerifyCode)

		// Passkey endpoints
		passkey := auth.Group("/passkey")
		{
			// Public endpoints (no auth required for login)
			passkey.POST("/login/begin", passkeyHandler.BeginAuthentication)
			passkey.POST("/login/finish", passkeyHandler.FinishAuthentication)

			// Protected endpoints (require auth for registration and management)
			passkey.POST("/register/begin", middleware.AuthMiddleware(), passkeyHandler.BeginRegistration)
			passkey.POST("/register/finish", middleware.AuthMiddleware(), passkeyHandler.FinishRegistration)
			passkey.GET("", middleware.AuthMiddleware(), passkeyHandler.ListPasskeys)
			passkey.DELETE("/:id", middleware.AuthMiddleware(), passkeyHandler.DeletePasskey)
			passkey.PATCH("/:id", middleware.AuthMiddleware(), passkeyHandler.UpdatePasskeyName)
		}
	}

	// Internal routes (for inter-service communication, no auth required)
	internal := router.Group("/internal")
	{
		// Status update endpoint for chat-service
		internal.PUT("/users/:id/status", userHandler.UpdateUser)
		// Get multiple users by IDs for task-service
		internal.GET("/users", userHandler.GetUsersByIDs)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public authentication routes (alternative paths)
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)

		v1Auth := v1.Group("/auth")
		{
			v1Auth.POST("/register", authHandler.Register)
			v1Auth.POST("/login", authHandler.Login)
			v1Auth.POST("/login/superadmin", authHandler.LoginSuperAdmin)
			v1Auth.POST("/logout", middleware.AuthMiddleware(), authHandler.Logout) // Requires auth
			v1Auth.POST("/refresh", authHandler.RefreshToken)

			// 2FA endpoints (public - no auth required)
			v1Auth.POST("/2fa/send", twoFAHandler.SendCode)
			v1Auth.POST("/2fa/verify", twoFAHandler.VerifyCode)

			// Passkey endpoints (v1)
			v1Passkey := v1Auth.Group("/passkey")
			{
				// Public endpoints (no auth required for login)
				v1Passkey.POST("/login/begin", passkeyHandler.BeginAuthentication)
				v1Passkey.POST("/login/finish", passkeyHandler.FinishAuthentication)

				// Protected endpoints (require auth for registration and management)
				v1Passkey.POST("/register/begin", middleware.AuthMiddleware(), passkeyHandler.BeginRegistration)
				v1Passkey.POST("/register/finish", middleware.AuthMiddleware(), passkeyHandler.FinishRegistration)
				v1Passkey.GET("", middleware.AuthMiddleware(), passkeyHandler.ListPasskeys)
				v1Passkey.DELETE("/:id", middleware.AuthMiddleware(), passkeyHandler.DeletePasskey)
				v1Passkey.PATCH("/:id", middleware.AuthMiddleware(), passkeyHandler.UpdatePasskeyName)
			}
		}

		// Super admin password change endpoint (protected, super admin only)
		v1.PUT("/superadmin/change-password",
			middleware.AuthMiddleware(), // Use unified auth
			middleware.RequireRole("super_admin"),
			profileHandler.ChangeSuperAdminPassword)

		// Super admin 2FA status update endpoint (protected, super admin only)
		v1.PUT("/superadmin/2fa/status",
			middleware.AuthMiddleware(), // Use unified auth
			middleware.RequireRole("super_admin"),
			profileHandler.UpdateSuperAdmin2FAStatus)

		// Protected user routes (require authentication)
		users := v1.Group("/users")
		users.Use(middleware.AuthMiddleware()) // Apply unified auth middleware
		{
			users.GET("", userHandler.GetUsers)
			users.POST("", middleware.RequireRole("admin", "super_admin"), userHandler.CreateUser)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.DELETE("/:id", middleware.RequireRole("admin", "super_admin"), userHandler.DeleteUser)
		}

		// Protected profile routes (require authentication)
		profile := v1.Group("/profile")
		profile.Use(middleware.AuthMiddleware())
		{
			profile.GET("", profileHandler.GetMyProfile)
			profile.PUT("", profileHandler.UpdateMyProfile)
			profile.PUT("/password", profileHandler.ChangePassword)
			profile.PUT("/status", profileHandler.UpdateStatus)
			profile.GET("/:id", profileHandler.GetProfile)
		}

		// Session management routes (require authentication)
		sessions := v1.Group("/sessions")
		sessions.Use(middleware.AuthMiddleware())
		{
			sessions.GET("", sessionHandler.GetActiveSessions)           // GET /api/v1/sessions - get all active sessions
			sessions.DELETE("", sessionHandler.DeleteAllSessions)        // DELETE /api/v1/sessions - delete all other sessions
			sessions.DELETE("/:session_id", sessionHandler.DeleteSession) // DELETE /api/v1/sessions/:id - delete specific session
		}

		// Department management routes
		departments := v1.Group("/departments")
		departments.Use(middleware.AuthMiddleware())
		{
			departments.GET("", departmentHandler.GetDepartments)
			departments.GET("/:id", departmentHandler.GetDepartment)
			departments.GET("/:id/users", departmentHandler.GetDepartmentWithUsers)
			departments.POST("", middleware.RequireAdminRole(), departmentHandler.CreateDepartment)
			departments.DELETE("/:id", middleware.RequireAdminRole(), departmentHandler.DeleteDepartment)
			departments.PUT("/:id", middleware.RequireAdminOrDepartmentHead("id"), departmentHandler.UpdateDepartment)
		}

		// Admin routes within /api/v1 for gateway compatibility
		v1Admin := v1.Group("/admin")
		v1Admin.Use(middleware.AuthMiddleware()) // Use unified auth
		v1Admin.Use(middleware.AdminOnlyMiddleware())
		v1Admin.Use(middleware.ValidateAdminRequest())
		{
			// User management endpoints
			v1AdminUsers := v1Admin.Group("/users")
			{
				v1AdminUsers.GET("", middleware.LogAdminAction("list_users"), adminHandler.GetUsers)
				v1AdminUsers.POST("", middleware.LogAdminAction("create_user"), adminHandler.CreateUser)
				v1AdminUsers.POST("/import", middleware.LogAdminAction("import_users"), adminHandler.ImportUsers) // Import users from CSV
				v1AdminUsers.PUT("/:id", middleware.LogAdminAction("update_user"), adminHandler.UpdateUser)
				v1AdminUsers.GET("/stats", middleware.LogAdminAction("get_user_stats"), adminHandler.GetUserStats)
				v1AdminUsers.PUT("/:id/role", middleware.LogAdminAction("update_user_role"), adminHandler.UpdateUserRole)
				v1AdminUsers.PUT("/:id/status", middleware.LogAdminAction("update_user_status"), adminHandler.UpdateUserStatus)
				v1AdminUsers.PUT("/:id/activate", middleware.LogAdminAction("activate_user"), adminHandler.ActivateUser)
				v1AdminUsers.PUT("/:id/deactivate", middleware.LogAdminAction("deactivate_user"), adminHandler.DeactivateUser)
				v1AdminUsers.PUT("/:id/2fa", middleware.LogAdminAction("update_user_2fa"), adminHandler.UpdateUser2FA)                         // Super admin only
				v1AdminUsers.POST("/:id/reset-password", middleware.LogAdminAction("reset_user_password"), adminHandler.ResetUserPassword) // Super admin only
				v1AdminUsers.DELETE("/:id", middleware.LogAdminAction("delete_user"), adminHandler.DeleteUser)
			}

			// Department management for admins
			v1AdminDepartments := v1Admin.Group("/departments")
			{
				v1AdminDepartments.GET("", middleware.LogAdminAction("list_departments"), departmentHandler.GetDepartments)
				v1AdminDepartments.POST("", middleware.LogAdminAction("create_department"), departmentHandler.CreateDepartment)
				v1AdminDepartments.GET("/:id", middleware.LogAdminAction("get_department"), departmentHandler.GetDepartment)
				v1AdminDepartments.PUT("/:id", middleware.LogAdminAction("update_department"), departmentHandler.UpdateDepartment)
				v1AdminDepartments.DELETE("/:id", middleware.LogAdminAction("delete_department"), departmentHandler.DeleteDepartment)
				v1AdminDepartments.GET("/:id/users", middleware.LogAdminAction("get_department_users"), departmentHandler.GetDepartmentWithUsers)
			}

			// System settings endpoints (super admin only)
			v1AdminSettings := v1Admin.Group("/settings")
			v1AdminSettings.Use(middleware.SuperAdminOnlyMiddleware())
			{
				// New settings endpoints
				v1AdminSettings.GET("/auth", middleware.LogAdminAction("get_auth_settings"), settingsHandler.GetSettings)
				v1AdminSettings.GET("/auth/presets", middleware.LogAdminAction("get_security_presets"), settingsHandler.GetPresets)
				v1AdminSettings.PUT("/auth/preset", middleware.LogAdminAction("apply_security_preset"), settingsHandler.ApplyPreset)
				v1AdminSettings.PUT("/auth/custom", middleware.LogAdminAction("update_custom_settings"), settingsHandler.UpdateCustomSettings)
				v1AdminSettings.GET("/auth/summary", middleware.LogAdminAction("get_security_summary"), settingsHandler.GetSummary)

				// Legacy endpoints (deprecated but kept for backward compatibility)
				v1AdminSettings.GET("/auth/mode", middleware.LogAdminAction("get_auth_mode_legacy"), settingsHandler.GetAuthMode)
				v1AdminSettings.PUT("/auth/mode", middleware.LogAdminAction("set_auth_mode_legacy"), settingsHandler.SetAuthMode)
			}
		}
	}

	// Admin routes with specific middleware and logging
	admin := router.Group("/admin")
	admin.Use(middleware.AuthMiddleware())       // Require authentication (unified)
	admin.Use(middleware.AdminOnlyMiddleware())  // Require admin role
	admin.Use(middleware.ValidateAdminRequest()) // Validate request format
	{
		// User management endpoints
		users := admin.Group("/users")
		{
			users.GET("", middleware.LogAdminAction("list_users"), adminHandler.GetUsers)
			users.POST("", middleware.LogAdminAction("create_user"), adminHandler.CreateUser)
			users.PUT("/:id", middleware.LogAdminAction("update_user"), adminHandler.UpdateUser)
			users.GET("/stats", middleware.LogAdminAction("get_user_stats"), adminHandler.GetUserStats)
			users.PUT("/:id/role", middleware.LogAdminAction("update_user_role"), adminHandler.UpdateUserRole)
			users.PUT("/:id/status", middleware.LogAdminAction("update_user_status"), adminHandler.UpdateUserStatus)
			users.PUT("/:id/activate", middleware.LogAdminAction("activate_user"), adminHandler.ActivateUser)
			users.PUT("/:id/deactivate", middleware.LogAdminAction("deactivate_user"), adminHandler.DeactivateUser)
			users.PUT("/:id/2fa", middleware.LogAdminAction("update_user_2fa"), adminHandler.UpdateUser2FA)                         // Super admin only
			users.POST("/:id/reset-password", middleware.LogAdminAction("reset_user_password"), adminHandler.ResetUserPassword) // Super admin only
		}

		// Department management for admins
		departments := admin.Group("/departments")
		{
			departments.GET("", middleware.LogAdminAction("list_departments"), departmentHandler.GetDepartments)
			departments.POST("", middleware.LogAdminAction("create_department"), departmentHandler.CreateDepartment)
			departments.GET("/:id", middleware.LogAdminAction("get_department"), departmentHandler.GetDepartment)
			departments.PUT("/:id", middleware.LogAdminAction("update_department"), departmentHandler.UpdateDepartment)
			departments.DELETE("/:id", middleware.LogAdminAction("delete_department"), departmentHandler.DeleteDepartment)
			departments.GET("/:id/users", middleware.LogAdminAction("get_department_users"), departmentHandler.GetDepartmentWithUsers)
		}

		// System administration endpoints (super admin only)
		system := admin.Group("/system")
		system.Use(middleware.SuperAdminOnlyMiddleware())
		{
			system.GET("/health", middleware.LogAdminAction("system_health_check"), systemHealthHandler)
			system.GET("/stats", middleware.LogAdminAction("system_stats"), systemStatsHandler)
		}

		// Settings endpoints (super admin only)
		settings := admin.Group("/settings")
		settings.Use(middleware.SuperAdminOnlyMiddleware())
		{
			// New settings endpoints
			settings.GET("/auth", middleware.LogAdminAction("get_auth_settings"), settingsHandler.GetSettings)
			settings.GET("/auth/presets", middleware.LogAdminAction("get_security_presets"), settingsHandler.GetPresets)
			settings.PUT("/auth/preset", middleware.LogAdminAction("apply_security_preset"), settingsHandler.ApplyPreset)
			settings.PUT("/auth/custom", middleware.LogAdminAction("update_custom_settings"), settingsHandler.UpdateCustomSettings)
			settings.GET("/auth/summary", middleware.LogAdminAction("get_security_summary"), settingsHandler.GetSummary)

			// Legacy endpoints (deprecated but kept for backward compatibility)
			settings.GET("/auth/mode", middleware.LogAdminAction("get_auth_mode_legacy"), settingsHandler.GetAuthMode)
			settings.PUT("/auth/mode", middleware.LogAdminAction("set_auth_mode_legacy"), settingsHandler.SetAuthMode)
		}
	}
}

// System administration handlers
func systemHealthHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"service":     "user-service",
		"timestamp":   time.Now().UTC(),
		"version":     "1.0.0",
		"environment": os.Getenv("ENVIRONMENT"),
		"request_id":  requestID,
	})
}

func systemStatsHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	c.JSON(http.StatusOK, gin.H{
		"uptime":     "24h",
		"memory":     "512MB",
		"cpu":        "5%",
		"requests":   "1000",
		"timestamp":  time.Now().UTC(),
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
