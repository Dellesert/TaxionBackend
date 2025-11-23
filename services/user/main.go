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
	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/email"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sharedredis "tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// migrateUserSettings handles UserSettings table migration with custom constraint handling
func migrateUserSettings(db *gorm.DB) error {
	// Use raw SQL to create the table if it doesn't exist
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS user_settings (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			show_setup_guide BOOLEAN NOT NULL DEFAULT TRUE,
			theme VARCHAR(20) DEFAULT 'light',
			language VARCHAR(10) DEFAULT 'ru',
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`

	if err := db.Exec(createTableSQL).Error; err != nil {
		return fmt.Errorf("failed to create user_settings table: %w", err)
	}

	// Drop old constraint if it exists (ignore errors)
	db.Exec("ALTER TABLE user_settings DROP CONSTRAINT IF EXISTS uni_user_settings_user_id")

	// Create unique index on user_id if it doesn't exist
	indexSQL := `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_user_settings_user_id
		ON user_settings(user_id)
	`

	if err := db.Exec(indexSQL).Error; err != nil {
		return fmt.Errorf("failed to create unique index on user_id: %w", err)
	}

	return nil
}

func main() {
	// Track service start time
	startTime := time.Now()

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

	// Run database migrations (including 2FA, passkey, system settings, invitations, password resets, SMTP settings, user settings, and subdepartments tables)
	// First migrate all models except UserSettings
	if err := db.Migrate(
		&models.Department{},
		&models.Subdepartment{},
		&models.User{},
		&models.TwoFactorCode{},
		&models.PasskeyCredential{},
		&models.SystemSettings{},
		&models.Invitation{},
		&models.PasswordReset{},
		&models.SMTPSettings{},
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Handle UserSettings migration separately with custom constraint handling
	if err := migrateUserSettings(db.DB); err != nil {
		log.Fatalf("Failed to migrate UserSettings: %v", err)
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

	// Initialize analytics client
	analyticsURL := os.Getenv("ANALYTICS_SERVICE_URL")
	if analyticsURL == "" {
		analyticsURL = "http://analytics-service:8086"
	}
	analyticsClient := analytics.NewClient(analyticsURL, log)
	log.Infof("Analytics client initialized with URL: %s", analyticsURL)

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	departmentRepo := repository.NewDepartmentRepository(db)
	subdepartmentRepo := repository.NewSubdepartmentRepository(db)
	twoFARepo := repository.NewTwoFARepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	invitationRepo := repository.NewInvitationRepository(db.DB)
	passwordResetRepo := repository.NewPasswordResetRepository(db.DB)
	smtpRepo := repository.NewSMTPRepository(db.DB)

	// Initialize email service with dynamic config loader
	// This allows the email service to reload SMTP settings from database on each send
	emailService := email.NewEmailServiceWithLoader(func() *email.EmailConfig {
		// Try to load SMTP settings from database first, fallback to env
		smtpSettings, err := smtpRepo.GetSettings()
		if err == nil && smtpSettings != nil {
			// Use database settings if available
			log.Debug("Loading SMTP settings from database")
			return email.LoadConfigFromDB(
				smtpSettings.Host,
				smtpSettings.Port,
				smtpSettings.Username,
				smtpSettings.Password,
				smtpSettings.FromEmail,
				smtpSettings.FromName,
			)
		}
		// Fallback to environment variables
		log.Debug("Loading SMTP settings from environment variables")
		return email.LoadConfigFromEnv()
	})

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Get session duration from database settings, fallback to env
	sessionDuration := time.Duration(cfg.Auth.SessionDuration) * time.Hour // Default from env
	dbSettings, err := settingsRepo.GetOrCreate()
	if err == nil && dbSettings.SessionDurationHours > 0 {
		// Use database settings if available
		sessionDuration = time.Duration(dbSettings.SessionDurationHours) * time.Hour
		log.Infof("Using session duration from database: %d hours", dbSettings.SessionDurationHours)
	} else {
		log.Infof("Using session duration from environment: %d hours", cfg.Auth.SessionDuration)
	}

	// Initialize authentication configuration (supports both JWT and session modes)
	authMode := sharedmodels.AuthMode(cfg.Auth.Mode)
	middleware.InitAuthConfig(authMode, jwtConfig, redisClient.Client, sessionDuration)

	log.Infof("Authentication initialized in %s mode", authMode)

	// Initialize WebAuthn service
	webAuthnService, err := usecase.NewWebAuthnService()
	if err != nil {
		log.Fatalf("Failed to initialize WebAuthn service: %v", err)
	}

	// Initialize usecases
	userUsecase := usecase.NewUserUsecase(userRepo, settingsRepo)
	authUsecase := usecase.NewAuthUsecase(userRepo, departmentRepo, settingsRepo, jwtConfig)
	profileUsecase := usecase.NewProfileUsecase(userRepo, departmentRepo)
	adminUsecase := usecase.NewAdminUsecase(userRepo, departmentRepo, settingsRepo)
	departmentUsecase := usecase.NewDepartmentUsecase(departmentRepo, userRepo)
	subdepartmentUsecase := usecase.NewSubdepartmentUsecase(subdepartmentRepo, departmentRepo, userRepo)
	initUsecase := usecase.NewInitUsecase(userRepo)
	twoFAUsecase := usecase.NewTwoFAUsecase(userRepo, twoFARepo, emailService, authUsecase)
	settingsUsecase := usecase.NewSettingsUsecase(settingsRepo, userRepo, passkeyRepo)
	passkeyUsecase := usecase.NewPasskeyUsecase(userRepo, passkeyRepo, settingsRepo, webAuthnService)
	invitationUsecase := usecase.NewInvitationUsecase(invitationRepo, userRepo, departmentRepo, emailService, authUsecase)
	passwordResetUsecase := usecase.NewPasswordResetUsecase(passwordResetRepo, userRepo, emailService, authUsecase)
	smtpUsecase := usecase.NewSMTPUsecase(smtpRepo)

	// Initialize super admin if not exists
	if err := initUsecase.InitializeSuperAdmin(); err != nil {
		log.Errorf("Failed to initialize super admin: %v", err)
		// Don't fail the startup, just log the error
	}

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userUsecase)
	authHandler := handlers.NewAuthHandler(authUsecase, analyticsClient)
	profileHandler := handlers.NewProfileHandler(profileUsecase)
	departmentHandler := handlers.NewDepartmentHandler(departmentUsecase)
	subdepartmentHandler := handlers.NewSubdepartmentHandler(subdepartmentUsecase, departmentUsecase)
	adminHandler := handlers.NewAdminHandler(adminUsecase, userUsecase)
	settingsHandler := handlers.NewSettingsHandler(settingsUsecase)
	sessionHandler := handlers.NewSessionHandler(analyticsClient)
	twoFAHandler := handlers.NewTwoFAHandler(twoFAUsecase, authUsecase, jwtConfig)
	passkeyHandler := handlers.NewPasskeyHandler(passkeyUsecase)
	invitationHandler := handlers.NewInvitationHandler(invitationUsecase)
	passwordResetHandler := handlers.NewPasswordResetHandler(passwordResetUsecase)
	smtpHandler := handlers.NewSMTPHandler(smtpUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "user-service", startTime)
	quickStartHandler := handlers.NewQuickStartHandler(departmentUsecase, subdepartmentUsecase, userUsecase)

	// Create Gin router
	router := gin.New()

	// Set max multipart memory for file uploads (32 MB)
	router.MaxMultipartMemory = 32 << 20

	// Setup common middleware (without CORS - Gateway handles it)
	middleware.SetupCommonMiddlewareWithoutCORS(router)

	// Add metrics middleware to track HTTP requests
	router.Use(metricsHandler.MetricsMiddleware())

	// Setup routes
	setupRoutes(router, userHandler, authHandler, profileHandler, departmentHandler, subdepartmentHandler, adminHandler, settingsHandler, sessionHandler, twoFAHandler, passkeyHandler, invitationHandler, passwordResetHandler, smtpHandler, metricsHandler, quickStartHandler, jwtConfig)

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
func setupRoutes(router *gin.Engine, userHandler *handlers.UserHandler, authHandler *handlers.AuthHandler, profileHandler *handlers.ProfileHandler, departmentHandler *handlers.DepartmentHandler, subdepartmentHandler *handlers.SubdepartmentHandler, adminHandler *handlers.AdminHandler, settingsHandler *handlers.SettingsHandler, sessionHandler *handlers.SessionHandler, twoFAHandler *handlers.TwoFAHandler, passkeyHandler *handlers.PasskeyHandler, invitationHandler *handlers.InvitationHandler, passwordResetHandler *handlers.PasswordResetHandler, smtpHandler *handlers.SMTPHandler, metricsHandler *handlers.MetricsHandler, quickStartHandler *handlers.QuickStartHandler, jwtConfig *middleware.JWTConfig) {
	// Health check endpoint
	router.GET("/health", healthHandler)

	// Internal metrics endpoints (no auth required - only accessible from internal network)
	internalMetrics := router.Group("/internal/metrics")
	{
		internalMetrics.GET("/database", metricsHandler.GetDatabaseMetrics)
		internalMetrics.GET("/redis", metricsHandler.GetRedisMetrics)
		internalMetrics.GET("/runtime", metricsHandler.GetRuntimeMetrics)
	}

	// Apple App Site Association for Passkeys
	wellKnown := router.Group("/.well-known")
	{
		wellKnown.GET("/apple-app-site-association", func(c *gin.Context) {
			c.Header("Content-Type", "application/json")
			c.JSON(200, gin.H{
				"webcredentials": gin.H{
					"apps": []string{"QNVQ55232N.com.anonymous.tachyon-messenger"},
				},
			})
		})
	}

	// Public password reset redirect page (HTML page for email links)
	router.GET("/reset-password/:token", passwordResetHandler.PasswordResetRedirect)

	// Public invitation redirect page (HTML page for email links)
	router.GET("/invite/:token", invitationHandler.InvitationRedirect)

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
			passkey.POST("/login/begin", passkeyHandler.BeginAuthentication)                          // Legacy: requires email
			passkey.POST("/login/discoverable/begin", passkeyHandler.BeginDiscoverableAuthentication) // New: no email required
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
		// Get single user by ID for notification-service
		internal.GET("/users/:id", userHandler.GetUser)
		// Get multiple users by IDs for task-service
		internal.GET("/users", userHandler.GetUsersByIDs)
		// Get users by department for poll-service
		internal.GET("/users/department/:department_id", userHandler.GetUsersByDepartment)
		// Session management (for admin/analytics service)
		internal.DELETE("/sessions/:session_id", sessionHandler.TerminateSessionInternal)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public authentication routes (alternative paths)
		v1.POST("/register", authHandler.Register)
		v1.POST("/login", authHandler.Login)

		// Public invitation routes (no auth required)
		invitations := v1.Group("/invitations")
		{
			invitations.GET("/validate/:token", invitationHandler.ValidateInvitation)
			invitations.POST("/accept/:token", invitationHandler.AcceptInvitation)
		}

		// Public password reset routes (no auth required)
		passwordResets := v1.Group("/password-resets")
		{
			passwordResets.POST("/request", passwordResetHandler.RequestPasswordReset)
			passwordResets.GET("/validate/:token", passwordResetHandler.ValidateResetToken)
			passwordResets.POST("/reset/:token", passwordResetHandler.ResetPassword)
		}

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
				v1Passkey.POST("/login/begin", passkeyHandler.BeginAuthentication)                          // Legacy: requires email
				v1Passkey.POST("/login/discoverable/begin", passkeyHandler.BeginDiscoverableAuthentication) // New: no email required
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

		// User settings routes (require authentication)
		userSettings := v1.Group("/user")
		userSettings.Use(middleware.AuthMiddleware())
		{
			userSettings.GET("/settings", settingsHandler.GetUserSettings)
			userSettings.PUT("/settings", settingsHandler.UpdateUserSettings)
		}

		// Session management routes (require authentication)
		sessions := v1.Group("/sessions")
		sessions.Use(middleware.AuthMiddleware())
		{
			sessions.GET("", sessionHandler.GetActiveSessions)            // GET /api/v1/sessions - get all active sessions
			sessions.DELETE("", sessionHandler.DeleteAllSessions)         // DELETE /api/v1/sessions - delete all other sessions
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

		// Subdepartment management routes
		subdepartments := v1.Group("/subdepartments")
		subdepartments.Use(middleware.AuthMiddleware())
		{
			subdepartments.GET("", subdepartmentHandler.GetSubdepartments)                                         // GET /api/v1/subdepartments?department_id=1 - get all or filter by department
			subdepartments.GET("/:id", subdepartmentHandler.GetSubdepartment)                                      // GET /api/v1/subdepartments/:id - get specific subdepartment
			subdepartments.POST("", middleware.RequireAdminRole(), subdepartmentHandler.CreateSubdepartment)       // POST /api/v1/subdepartments - create new subdepartment (admin only)
			subdepartments.PUT("/:id", middleware.RequireAdminRole(), subdepartmentHandler.UpdateSubdepartment)    // PUT /api/v1/subdepartments/:id - update subdepartment (admin only)
			subdepartments.DELETE("/:id", middleware.RequireAdminRole(), subdepartmentHandler.DeleteSubdepartment) // DELETE /api/v1/subdepartments/:id - delete subdepartment (admin only)
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
				v1AdminUsers.POST("/import", middleware.LogAdminAction("import_users"), adminHandler.ImportUsers)                                    // Import users from CSV
				v1AdminUsers.POST("/bulk-activate", middleware.LogAdminAction("bulk_activate_users"), adminHandler.BulkActivateUsers)                // Bulk activate users
				v1AdminUsers.POST("/bulk-deactivate", middleware.LogAdminAction("bulk_deactivate_users"), adminHandler.BulkDeactivateUsers)          // Bulk deactivate users
				v1AdminUsers.POST("/bulk-assign-department", middleware.LogAdminAction("bulk_assign_department"), adminHandler.BulkAssignDepartment) // Bulk assign department
				v1AdminUsers.PUT("/:id", middleware.LogAdminAction("update_user"), adminHandler.UpdateUser)
				v1AdminUsers.GET("/stats", middleware.LogAdminAction("get_user_stats"), adminHandler.GetUserStats)
				v1AdminUsers.PUT("/:id/role", middleware.LogAdminAction("update_user_role"), adminHandler.UpdateUserRole)
				v1AdminUsers.PUT("/:id/status", middleware.LogAdminAction("update_user_status"), adminHandler.UpdateUserStatus)
				v1AdminUsers.PUT("/:id/activate", middleware.LogAdminAction("activate_user"), adminHandler.ActivateUser)
				v1AdminUsers.PUT("/:id/deactivate", middleware.LogAdminAction("deactivate_user"), adminHandler.DeactivateUser)
				v1AdminUsers.PUT("/:id/2fa", middleware.LogAdminAction("update_user_2fa"), adminHandler.UpdateUser2FA)                     // Super admin only
				v1AdminUsers.POST("/:id/reset-password", middleware.LogAdminAction("reset_user_password"), adminHandler.ResetUserPassword) // Super admin only
				v1AdminUsers.DELETE("/:id", middleware.LogAdminAction("delete_user"), adminHandler.DeleteUser)
			}

			// Department management for admins
			v1AdminDepartments := v1Admin.Group("/departments")
			{
				v1AdminDepartments.GET("", middleware.LogAdminAction("list_departments"), departmentHandler.GetDepartments)
				v1AdminDepartments.POST("", middleware.LogAdminAction("create_department"), departmentHandler.CreateDepartment)
				v1AdminDepartments.POST("/import", middleware.LogAdminAction("import_departments"), departmentHandler.ImportDepartments)
				v1AdminDepartments.POST("/bulk-delete", middleware.LogAdminAction("bulk_delete_departments"), departmentHandler.BulkDeleteDepartments)
				v1AdminDepartments.GET("/:id", middleware.LogAdminAction("get_department"), departmentHandler.GetDepartment)
				v1AdminDepartments.PUT("/:id", middleware.LogAdminAction("update_department"), departmentHandler.UpdateDepartment)
				v1AdminDepartments.DELETE("/:id", middleware.LogAdminAction("delete_department"), departmentHandler.DeleteDepartment)
				v1AdminDepartments.GET("/:id/users", middleware.LogAdminAction("get_department_users"), departmentHandler.GetDepartmentWithUsers)
			}

			// Subdepartment management for admins
			v1AdminSubdepartments := v1Admin.Group("/subdepartments")
			{
				v1AdminSubdepartments.POST("/import", middleware.LogAdminAction("import_subdepartments"), subdepartmentHandler.ImportSubdepartments)
				v1AdminSubdepartments.POST("/bulk-delete", middleware.LogAdminAction("bulk_delete_subdepartments"), subdepartmentHandler.BulkDeleteSubdepartments)
			}

			// Quick Start import endpoint (admin only)
			v1AdminQuickStart := v1Admin.Group("/quick-start")
			{
				v1AdminQuickStart.POST("/import", middleware.LogAdminAction("quick_start_import"), quickStartHandler.ImportQuickStart)
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

			// SMTP settings endpoints (super admin only)
			v1AdminSMTP := v1Admin.Group("/smtp-settings")
			v1AdminSMTP.Use(middleware.SuperAdminOnlyMiddleware())
			{
				v1AdminSMTP.GET("", middleware.LogAdminAction("get_smtp_settings"), smtpHandler.GetSettings)
				v1AdminSMTP.PUT("", middleware.LogAdminAction("update_smtp_settings"), smtpHandler.UpdateSettings)
				v1AdminSMTP.POST("/test", middleware.LogAdminAction("test_smtp_connection"), smtpHandler.TestConnection)
			}

			// Invitation management endpoints (super admin only)
			v1AdminInvitations := v1Admin.Group("/invitations")
			v1AdminInvitations.Use(middleware.SuperAdminOnlyMiddleware())
			{
				v1AdminInvitations.POST("", middleware.LogAdminAction("create_invitation"), invitationHandler.CreateInvitation)
				v1AdminInvitations.GET("", middleware.LogAdminAction("list_invitations"), invitationHandler.ListInvitations)
				v1AdminInvitations.GET("/stats", middleware.LogAdminAction("get_invitation_stats"), invitationHandler.GetStats)
				v1AdminInvitations.GET("/:id", middleware.LogAdminAction("get_invitation"), invitationHandler.GetInvitation)
				v1AdminInvitations.POST("/:id/resend", middleware.LogAdminAction("resend_invitation"), invitationHandler.ResendInvitation)
				v1AdminInvitations.DELETE("/:id", middleware.LogAdminAction("cancel_invitation"), invitationHandler.CancelInvitation)
				v1AdminInvitations.POST("/bulk-send", middleware.LogAdminAction("bulk_send_invitations"), invitationHandler.BulkSendInvitations)
			}

			// Password reset management endpoints (admin and super admin)
			v1AdminPasswordResets := v1Admin.Group("/password-resets")
			{
				v1AdminPasswordResets.POST("/initiate", middleware.LogAdminAction("initiate_password_reset"), passwordResetHandler.InitiatePasswordReset)
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
			users.PUT("/:id/2fa", middleware.LogAdminAction("update_user_2fa"), adminHandler.UpdateUser2FA) // Super admin only
			// Removed old reset-password endpoint, use /api/v1/admin/password-resets/initiate instead
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

		// Subdepartment management for admins
		subdepartments := admin.Group("/subdepartments")
		{
			subdepartments.GET("", middleware.LogAdminAction("list_subdepartments"), subdepartmentHandler.GetSubdepartments)
			subdepartments.POST("", middleware.LogAdminAction("create_subdepartment"), subdepartmentHandler.CreateSubdepartment)
			subdepartments.GET("/:id", middleware.LogAdminAction("get_subdepartment"), subdepartmentHandler.GetSubdepartment)
			subdepartments.PUT("/:id", middleware.LogAdminAction("update_subdepartment"), subdepartmentHandler.UpdateSubdepartment)
			subdepartments.DELETE("/:id", middleware.LogAdminAction("delete_subdepartment"), subdepartmentHandler.DeleteSubdepartment)
		}

		// System administration endpoints (super admin only)
		system := admin.Group("/system")
		system.Use(middleware.SuperAdminOnlyMiddleware())
		{
			system.GET("/health", middleware.LogAdminAction("system_health_check"), systemHealthHandler)
			system.GET("/stats", middleware.LogAdminAction("system_stats"), systemStatsHandler)
			system.GET("/metrics", middleware.LogAdminAction("get_all_service_metrics"), adminHandler.GetAllServiceMetrics)
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

		// SMTP settings endpoints (super admin only)
		smtp := admin.Group("/smtp-settings")
		smtp.Use(middleware.SuperAdminOnlyMiddleware())
		{
			smtp.GET("", middleware.LogAdminAction("get_smtp_settings"), smtpHandler.GetSettings)
			smtp.PUT("", middleware.LogAdminAction("update_smtp_settings"), smtpHandler.UpdateSettings)
			smtp.POST("/test", middleware.LogAdminAction("test_smtp_connection"), smtpHandler.TestConnection)
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
