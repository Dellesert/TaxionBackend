package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tachyon-messenger/services/backup/handlers"
	"tachyon-messenger/services/backup/models"
	"tachyon-messenger/services/backup/repository"
	"tachyon-messenger/services/backup/usecase"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedsentry "tachyon-messenger/shared/sentry"
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

	log.Info("Starting Backup service...")

	// Initialize Sentry
	if err := sharedsentry.Init(cfg.Sentry.DSN, "backup-service"); err != nil {
		log.Warnf("Sentry initialization failed: %v", err)
	}
	defer sharedsentry.Flush()

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	log.Info("Running GORM auto-migrations...")
	if err := db.Migrate(&models.Backup{}); err != nil {
		log.Fatalf("Failed to run GORM migrations: %v", err)
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

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize authentication configuration
	authMode := sharedmodels.AuthMode(cfg.Auth.Mode)
	sessionDuration := time.Duration(cfg.Auth.SessionDuration) * time.Hour
	middleware.InitAuthConfig(authMode, jwtConfig, redisClient.Client, sessionDuration)
	log.WithFields(map[string]interface{}{
		"auth_mode":        authMode,
		"session_duration": sessionDuration,
	}).Info("Authentication configuration initialized")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Get backup directory
	backupDir := os.Getenv("BACKUP_DIR")
	if backupDir == "" {
		backupDir = "/app/backups"
	}

	// Parse database URL for backup commands
	dbURL := cfg.Database.URL
	dbHost, dbPort, dbName, dbUser, dbPassword := parseDatabaseURL(dbURL)

	// Initialize repository
	backupRepo := repository.NewBackupRepository(db)

	// Initialize usecase
	backupUsecase := usecase.NewBackupUsecase(
		backupRepo,
		backupDir,
		dbHost,
		dbPort,
		dbName,
		dbUser,
		dbPassword,
		log,
	)

	// Initialize handler
	backupHandler := handlers.NewBackupHandler(backupUsecase)

	// Setup routes
	r := setupRoutes(backupHandler, jwtConfig)

	// Start automatic backup scheduler
	go startBackupScheduler(backupUsecase, log)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8089" // Default port for backup service
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Backup service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Backup service...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Backup service forced to shutdown: %v", err)
	}

	log.Info("Backup service exited")
}

func setupRoutes(
	backupHandler *handlers.BackupHandler,
	jwtConfig *middleware.JWTConfig,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(requestid.New())

	// Health endpoint (no auth required)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "backup-service",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// API routes
	api := r.Group("/api/v1")

	// Protected routes (require super_admin authentication)
	backups := api.Group("/backups")
	backups.Use(middleware.AuthMiddleware())
	backups.Use(middleware.RequireSuperAdminRole())
	{
		backups.POST("", backupHandler.CreateBackup)
		backups.GET("", backupHandler.ListBackups)
		backups.GET("/stats", backupHandler.GetStats)
		backups.GET("/:id", backupHandler.GetBackup)
		backups.POST("/:id/restore", backupHandler.RestoreBackup)
		backups.DELETE("/:id", backupHandler.DeleteBackup)
		backups.GET("/:id/download", backupHandler.DownloadBackup)
	}

	return r
}

// parseDatabaseURL extracts connection parameters from PostgreSQL URL
func parseDatabaseURL(dbURL string) (host, port, dbName, user, password string) {
	// Format: postgres://user:password@host:port/dbname?params
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")

	// Split at @
	parts := strings.Split(dbURL, "@")
	if len(parts) != 2 {
		return "postgres", "5432", "tachyon_messenger", "tachyon_user", ""
	}

	// Extract user and password
	userPass := parts[0]
	if idx := strings.Index(userPass, ":"); idx > 0 {
		user = userPass[:idx]
		password = userPass[idx+1:]
	} else {
		user = userPass
	}

	// Extract host, port, and dbname
	hostPart := parts[1]
	if idx := strings.Index(hostPart, "/"); idx > 0 {
		hostPort := hostPart[:idx]
		if pidx := strings.Index(hostPort, ":"); pidx > 0 {
			host = hostPort[:pidx]
			port = hostPort[pidx+1:]
		} else {
			host = hostPort
			port = "5432"
		}

		dbPart := hostPart[idx+1:]
		if qidx := strings.Index(dbPart, "?"); qidx > 0 {
			dbName = dbPart[:qidx]
		} else {
			dbName = dbPart
		}
	}

	return
}

// startBackupScheduler starts automatic backup scheduler
func startBackupScheduler(backupUsecase *usecase.BackupUsecase, log *logger.Logger) {
	// Create automatic backup every 24 hours
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Create initial backup after 5 minutes
	time.Sleep(5 * time.Minute)
	log.Info("Creating initial automatic backup...")
	if _, err := backupUsecase.CreateBackup(1, models.BackupTypeAutomatic, "Automatic daily backup"); err != nil {
		log.Errorf("Failed to create initial automatic backup: %v", err)
	}

	for range ticker.C {
		log.Info("Creating automatic backup...")
		if _, err := backupUsecase.CreateBackup(1, models.BackupTypeAutomatic, "Automatic daily backup"); err != nil {
			log.Errorf("Failed to create automatic backup: %v", err)
		}
	}
}
