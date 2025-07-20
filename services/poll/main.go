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

	"tachyon-messenger/services/poll/handlers"
	"tachyon-messenger/services/poll/models"
	"tachyon-messenger/services/poll/repository"
	"tachyon-messenger/services/poll/usecase"
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

	// Initialize JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

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

	// Setup routes
	r := setupRoutes(pollHandler, jwtConfig)

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
	jwtConfig *middleware.JWTConfig,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(requestid.New())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health endpoint (no auth required)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "poll-service",
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	})

	// API routes
	api := r.Group("/api/v1")

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware(jwtConfig))
	{
		// Poll CRUD operations
		protected.GET("/polls", pollHandler.GetPolls)
		protected.GET("/polls/:id", pollHandler.GetPoll)
		protected.POST("/polls", pollHandler.CreatePoll)
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
