package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/search/handlers"
	"tachyon-messenger/services/search/migrations"
	"tachyon-messenger/services/search/repository"
	"tachyon-messenger/services/search/usecase"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sharedredis "tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

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

	log.Info("Starting Search service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run custom SQL migrations
	log.Info("Running search migrations...")
	if err := migrations.RunMigrations(db); err != nil {
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

	// Create JWT config and initialize auth
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)
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

	// Initialize layers
	searchRepo := repository.NewSearchRepository(db)
	searchUsecase := usecase.NewSearchUsecase(searchRepo, redisClient)

	// Initialize handlers
	searchHandler := handlers.NewSearchHandler(searchUsecase)
	indexHandler := handlers.NewIndexHandler(searchUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "search-service", startTime)

	// Setup routes
	r := setupRoutes(searchHandler, indexHandler, metricsHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Search service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Search service...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Search service stopped")
}

func setupRoutes(
	searchHandler *handlers.SearchHandler,
	indexHandler *handlers.IndexHandler,
	metricsHandler *handlers.MetricsHandler,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(requestid.New())

	// Add metrics middleware
	r.Use(metricsHandler.MetricsMiddleware())

	// Health endpoint (no auth required)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "search-service",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// Internal metrics endpoints (no auth required)
	internalMetrics := r.Group("/internal/metrics")
	{
		internalMetrics.GET("/database", metricsHandler.GetDatabaseMetrics)
		internalMetrics.GET("/redis", metricsHandler.GetRedisMetrics)
		internalMetrics.GET("/runtime", metricsHandler.GetRuntimeMetrics)
	}

	// API routes
	api := r.Group("/api/v1")

	// Internal indexing endpoints (no auth - inter-service only)
	internal := api.Group("/internal/search")
	{
		internal.POST("/index", indexHandler.IndexDocument)
		internal.POST("/bulk-index", indexHandler.BulkIndex)
		internal.DELETE("/index", indexHandler.DeleteDocument)
	}

	// Protected search endpoint (requires auth)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/search", searchHandler.Search)
	}

	return r
}
