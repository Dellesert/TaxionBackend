package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/calendar/handlers"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	"tachyon-messenger/services/calendar/usecase"
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

	log.Info("Starting Calendar service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.Migrate(&models.Event{}, &models.EventParticipant{}, &models.EventReminder{}); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize dependencies
	eventRepo := repository.NewEventRepository(db)
	participantRepo := repository.NewParticipantRepository(db)
	reminderRepo := repository.NewReminderRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize usecases
	calendarUsecase := usecase.NewCalendarUsecase(eventRepo, participantRepo, reminderRepo)

	// Initialize handlers
	calendarHandler := handlers.NewCalendarHandler(calendarUsecase)

	// Setup routes
	r := setupRoutes(calendarHandler, jwtConfig)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084" // Default port for calendar service
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Calendar service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Calendar service...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Calendar service stopped")
}

func setupRoutes(
	calendarHandler *handlers.CalendarHandler,
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
			"service":   "calendar-service",
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
		// Event endpoints
		protected.GET("/events", calendarHandler.GetUserEvents)
		protected.GET("/events/:id", calendarHandler.GetEvent)
		protected.POST("/events", calendarHandler.CreateEvent)
		protected.PUT("/events/:id", calendarHandler.UpdateEvent)
		protected.DELETE("/events/:id", calendarHandler.DeleteEvent)

		// Calendar view
		protected.GET("/calendar", calendarHandler.GetUserCalendar)

		// Event search and stats
		protected.GET("/events/search", calendarHandler.SearchEvents)
		protected.GET("/events/stats", calendarHandler.GetEventStats)

		// Time conflict checking
		protected.POST("/events/check-conflict", calendarHandler.CheckTimeConflict)

		// Participant management
		protected.POST("/events/:id/participants", calendarHandler.InviteParticipants)
		protected.DELETE("/events/:id/participants/:user_id", calendarHandler.RemoveParticipant)
		protected.PUT("/events/:id/status", calendarHandler.UpdateParticipantStatus)

		// Reminder management
		protected.POST("/events/:id/reminders", calendarHandler.SetReminder)
		protected.DELETE("/events/:id/reminders/:reminder_id", calendarHandler.RemoveReminder)
	}

	return r
}
