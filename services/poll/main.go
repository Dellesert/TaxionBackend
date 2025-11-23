// File: services/poll/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/poll/clients"
	"tachyon-messenger/services/poll/handlers"
	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/services/poll/repository"
	"tachyon-messenger/services/poll/usecase"
	"tachyon-messenger/services/poll/worker"
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

	log.Info("Starting Poll service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(
		&models.Poll{},
		&models.PollOption{},
		&models.PollVote{},
		&models.PollParticipant{},
		&models.PollComment{},
	); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	log.Info("Database migrations completed successfully")

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

	// Initialize authentication configuration (for session support)
	authMode := sharedmodels.AuthMode(cfg.Auth.Mode)
	sessionDuration := time.Duration(cfg.Auth.SessionDuration) * time.Hour
	middleware.InitAuthConfig(authMode, jwtConfig, redisClient.Client, sessionDuration)
	log.WithFields(map[string]interface{}{
		"auth_mode":        authMode,
		"session_duration": sessionDuration,
	}).Info("Authentication configuration initialized")

	// Initialize repositories
	pollRepo := repository.NewPollRepository(db)
	optionRepo := repository.NewPollOptionRepository(db)
	voteRepo := repository.NewPollVoteRepository(db)
	participantRepo := repository.NewPollParticipantRepository(db)
	commentRepo := repository.NewPollCommentRepository(db)

	// Initialize usecases
	pollUsecase := usecase.NewPollUsecase(pollRepo, optionRepo, voteRepo, participantRepo, commentRepo)

	// Initialize handlers
	pollHandler := handlers.NewPollHandler(pollUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "poll-service", startTime)

	// Start notification worker for poll reminders
	notificationClient := clients.NewNotificationClient()
	userClient := clients.NewUserClient()
	notificationWorker := worker.NewNotificationWorker(pollRepo, participantRepo, voteRepo, notificationClient, userClient)
	notificationWorker.Start()

	// Setup routes
	r := setupRoutes(pollHandler, metricsHandler, jwtConfig)

	// Start background scheduler for auto-closing expired polls
	checkInterval := time.Duration(cfg.Poll.AutoCloseCheckInterval) * time.Minute
	log.WithFields(map[string]interface{}{
		"interval_minutes": cfg.Poll.AutoCloseCheckInterval,
	}).Info("Starting poll auto-close scheduler")

	ctx, cancelScheduler := context.WithCancel(context.Background())
	defer cancelScheduler()

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Info("Poll auto-close scheduler stopped")
				return
			case <-ticker.C:
				startTime := time.Now()
				closedCount, err := pollUsecase.AutoCloseExpiredPolls()
				duration := time.Since(startTime)

				if err != nil {
					log.WithFields(map[string]interface{}{
						"error":        err.Error(),
						"duration_ms":  duration.Milliseconds(),
					}).Error("Failed to auto-close expired polls")
				} else if closedCount > 0 {
					log.WithFields(map[string]interface{}{
						"closed_count": closedCount,
						"duration_ms":  duration.Milliseconds(),
					}).Info("Auto-closed expired polls")
				} else {
					// Log successful check even if no polls were closed (for monitoring)
					log.WithFields(map[string]interface{}{
						"closed_count": 0,
						"duration_ms":  duration.Milliseconds(),
					}).Debug("Poll auto-close check completed - no expired polls")
				}
			}
		}
	}()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085" // Default port for poll service
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Poll service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Poll service...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Poll service stopped")
}

func setupRoutes(
	pollHandler *handlers.PollHandler,
	metricsHandler *handlers.MetricsHandler,
	jwtConfig *middleware.JWTConfig,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(requestid.New())

	// Add metrics middleware to track HTTP requests
	r.Use(metricsHandler.MetricsMiddleware())

	// CORS is handled by Gateway - no need for CORS middleware here

	// Health endpoint (no auth required)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "poll-service",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// Internal metrics endpoints (no auth required - only accessible from internal network)
	internalMetrics := r.Group("/internal/metrics")
	{
		internalMetrics.GET("/database", metricsHandler.GetDatabaseMetrics)
		internalMetrics.GET("/redis", metricsHandler.GetRedisMetrics)
		internalMetrics.GET("/runtime", metricsHandler.GetRuntimeMetrics)
	}

	// API routes
	api := r.Group("/api/v1")

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		// Poll viewing - all authenticated users
		protected.GET("/polls", pollHandler.GetPolls)
		protected.GET("/polls/:id", pollHandler.GetPoll)

		// Poll creation - only department_head, admin, super_admin can create polls
		protected.POST("/polls", middleware.RequireDepartmentHeadOrAbove(), pollHandler.CreatePoll)

		// Poll update/delete - handled in usecase (creator, admin, super_admin)
		protected.PUT("/polls/:id", pollHandler.UpdatePoll)
		protected.DELETE("/polls/:id", pollHandler.DeletePoll)

		// Poll search and stats
		protected.GET("/polls/search", pollHandler.SearchPolls)
		protected.GET("/polls/stats", pollHandler.GetPollStats)

		// Poll status management
		protected.PATCH("/polls/:id/status", pollHandler.UpdatePollStatus)

		// Voting
		protected.POST("/polls/:id/vote", pollHandler.VotePoll)
		protected.GET("/polls/:id/my-votes", pollHandler.GetMyVotes)
		protected.GET("/polls/:id/results", pollHandler.GetPollResults)
		protected.GET("/polls/:id/voters", pollHandler.GetPollVoters)

		// Participant management
		protected.POST("/polls/:id/participants", pollHandler.AddParticipants)
		protected.DELETE("/polls/:id/participants/:user_id", pollHandler.RemoveParticipant)

		// Comment management
		protected.GET("/polls/:id/comments", pollHandler.GetComments)
		protected.POST("/polls/:id/comments", pollHandler.CreateComment)
		protected.DELETE("/polls/:id/comments/:comment_id", pollHandler.DeleteComment)
	}

	return r
}
