package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/analytics/clients"
	"tachyon-messenger/services/analytics/handlers"
	"tachyon-messenger/services/analytics/migrations"
	"tachyon-messenger/services/analytics/models"
	"tachyon-messenger/services/analytics/repository"
	"tachyon-messenger/services/analytics/usecase"
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

	log.Info("Starting Analytics service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run GORM auto-migrations first to create tables
	log.Info("Running GORM auto-migrations...")
	if err := db.Migrate(
		&models.AnalyticsEvent{},
		&models.AggregatedMetric{},
		&models.UserActivity{},
		&models.DepartmentStats{},
		&models.LoginAttempt{},
		&models.KnownDevice{},
		&models.SecuritySession{},
		&models.SuspiciousActivity{},
	); err != nil {
		log.Fatalf("Failed to run GORM migrations: %v", err)
	}

	// Run custom SQL migrations after tables are created
	log.Info("Running custom migrations...")
	if err := migrations.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run custom migrations: %v", err)
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

	// Initialize repositories
	analyticsRepo := repository.NewAnalyticsRepository(db)
	eventsRepo := repository.NewEventsRepository(db)
	metricsRepo := repository.NewMetricsRepository(db)
	securityRepo := repository.NewSecurityRepository(db)

	// Initialize task service client
	taskServiceURL := os.Getenv("TASK_SERVICE_URL")
	if taskServiceURL == "" {
		taskServiceURL = "http://task-service:8083"
	}
	taskClient := clients.NewTaskClient(taskServiceURL, log)
	log.Infof("Task client initialized with URL: %s", taskServiceURL)

	// Initialize file service client
	fileServiceURL := os.Getenv("FILE_SERVICE_URL")
	if fileServiceURL == "" {
		fileServiceURL = "http://file-service:8088"
	}
	fileClient := clients.NewFileClient(fileServiceURL, log)
	log.Infof("File client initialized with URL: %s", fileServiceURL)

	// Initialize backup service client
	backupClient := clients.NewBackupClient()
	log.Info("Backup client initialized")

	// Initialize user service client
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "http://user-service:8081"
	}
	userClient := clients.NewUserClient(userServiceURL, log)
	log.Infof("User client initialized with URL: %s", userServiceURL)

	// Initialize usecases
	analyticsUsecase := usecase.NewAnalyticsUsecase(analyticsRepo, metricsRepo, eventsRepo, redisClient, taskClient, fileClient, backupClient, log)
	aggregatorUsecase := usecase.NewAggregatorUsecase(analyticsRepo, metricsRepo, eventsRepo, redisClient)
	securityUsecase := usecase.NewSecurityUsecase(securityRepo, userClient, log)

	// Initialize handlers
	dashboardHandler := handlers.NewDashboardHandler(analyticsUsecase)
	eventsHandler := handlers.NewEventsHandler(eventsRepo)
	usersHandler := handlers.NewUsersAnalyticsHandler(analyticsUsecase)
	messagesHandler := handlers.NewMessagesHandler(analyticsUsecase)
	tasksHandler := handlers.NewTasksHandler(analyticsUsecase)
	calendarHandler := handlers.NewCalendarHandler(analyticsUsecase)
	pollsHandler := handlers.NewPollsHandler(analyticsUsecase)
	filesHandler := handlers.NewFilesHandler(analyticsUsecase)
	systemHandler := handlers.NewSystemHandler(analyticsUsecase)
	securityHandler := handlers.NewSecurityHandler(securityUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "analytics-service", startTime)

	// Setup routes
	r := setupRoutes(
		dashboardHandler,
		eventsHandler,
		usersHandler,
		messagesHandler,
		tasksHandler,
		calendarHandler,
		pollsHandler,
		filesHandler,
		systemHandler,
		securityHandler,
		metricsHandler,
		jwtConfig,
	)

	// Start background aggregation tasks
	go startBackgroundTasks(aggregatorUsecase, log)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("ANALYTICS_SERVICE_PORT")
		if port == "" {
			port = "8086" // Default port for analytics service
		}
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Analytics service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Analytics service...")

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Analytics service forced to shutdown: %v", err)
	}

	log.Info("Analytics service exited")
}

func setupRoutes(
	dashboardHandler *handlers.DashboardHandler,
	eventsHandler *handlers.EventsHandler,
	usersHandler *handlers.UsersAnalyticsHandler,
	messagesHandler *handlers.MessagesHandler,
	tasksHandler *handlers.TasksHandler,
	calendarHandler *handlers.CalendarHandler,
	pollsHandler *handlers.PollsHandler,
	filesHandler *handlers.FilesHandler,
	systemHandler *handlers.SystemHandler,
	securityHandler *handlers.SecurityHandler,
	metricsHandler *handlers.MetricsHandler,
	jwtConfig *middleware.JWTConfig,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())
	r.Use(requestid.New())
	r.Use(middleware.CORSMiddleware())

	// Add metrics middleware to track HTTP requests
	r.Use(metricsHandler.MetricsMiddleware())

	// Health check endpoint (no auth required)
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "analytics-service",
			"version":   "1.0.0",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
	r.GET("/health", healthHandler)
	r.HEAD("/health", healthHandler)

	// Internal metrics endpoints (no auth required - only accessible from internal network)
	internalMetrics := r.Group("/internal/metrics")
	{
		internalMetrics.GET("/database", metricsHandler.GetDatabaseMetrics)
		internalMetrics.GET("/redis", metricsHandler.GetRedisMetrics)
		internalMetrics.GET("/runtime", metricsHandler.GetRuntimeMetrics)
	}

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Analytics routes (require admin or super admin authentication)
		analytics := v1.Group("/analytics")
		analytics.Use(middleware.AuthMiddleware())
		analytics.Use(middleware.RequireAdminRole())
		{
			// Dashboard - main analytics page
			analytics.GET("/dashboard", dashboardHandler.GetDashboard)

			// Users analytics
			users := analytics.Group("/users")
			{
				users.GET("/activity", usersHandler.GetUserActivity)
				users.GET("/top-active", usersHandler.GetTopActiveUsers)
				users.GET("/registrations", usersHandler.GetRegistrations)
			}

			// Messages analytics
			messages := analytics.Group("/messages")
			{
				messages.GET("/stats", messagesHandler.GetStats)
				messages.GET("/timeline", messagesHandler.GetTimeline)
				messages.GET("/top-chats", messagesHandler.GetTopChats)
			}

			// Tasks analytics
			tasks := analytics.Group("/tasks")
			{
				tasks.GET("/stats", tasksHandler.GetStats)
				tasks.GET("/completion-rate", tasksHandler.GetCompletionRate)
				tasks.GET("/top-performers", tasksHandler.GetTopPerformers)
				tasks.GET("/departments", tasksHandler.GetDepartmentStats)
				tasks.GET("/trends", tasksHandler.GetTaskTrends)
				tasks.GET("/priority-distribution", tasksHandler.GetPriorityDistribution)
			}

			// Calendar analytics
			calendar := analytics.Group("/calendar")
			{
				calendar.GET("/stats", calendarHandler.GetStats)
				calendar.GET("/attendance", calendarHandler.GetAttendance)
			}

			// Polls analytics
			polls := analytics.Group("/polls")
			{
				polls.GET("/stats", pollsHandler.GetStats)
				polls.GET("/participation", pollsHandler.GetParticipation)
			}

			// Files analytics
			files := analytics.Group("/files")
			{
				files.GET("/stats", filesHandler.GetStats)
				files.GET("/storage", filesHandler.GetStorage)
			}

			// Departments analytics
			departments := analytics.Group("/departments")
			{
				departments.GET("/activity", dashboardHandler.GetDepartmentActivity)
				departments.GET("/comparison", dashboardHandler.GetDepartmentComparison)
			}

			// System analytics
			system := analytics.Group("/system")
			{
				system.GET("/performance", systemHandler.GetPerformance)
				system.GET("/errors", systemHandler.GetErrors)
				system.GET("/api-usage", systemHandler.GetAPIUsage)
			}

			// Security analytics
			security := analytics.Group("/security")
			{
				security.GET("/dashboard", securityHandler.GetDashboard)
				security.GET("/login-attempts", securityHandler.GetLoginAttempts)
				security.GET("/failed-logins", securityHandler.GetFailedLogins)
				security.GET("/login-stats", securityHandler.GetLoginStats)
				security.GET("/top-failed-ips", securityHandler.GetTopFailedIPs)
				security.GET("/suspicious-activities", securityHandler.GetSuspiciousActivities)
				security.GET("/suspicious-activities/unresolved", securityHandler.GetUnresolvedSuspiciousActivities)
				security.POST("/suspicious-activities/:id/resolve", securityHandler.ResolveSuspiciousActivity)
				security.GET("/active-sessions", securityHandler.GetActiveSessions)
				security.GET("/users/:user_id/sessions", securityHandler.GetUserActiveSessions)
				security.DELETE("/sessions/:session_id", securityHandler.TerminateSession)
				security.GET("/users/:user_id/devices", securityHandler.GetUserKnownDevices)
				security.DELETE("/devices/:device_id", securityHandler.RemoveKnownDevice)
				security.POST("/devices/:device_id/trust", securityHandler.TrustDevice)
			}

			// Reports
			reports := analytics.Group("/reports")
			{
				reports.POST("/generate", dashboardHandler.GenerateReport)
				reports.GET("/:id", dashboardHandler.GetReport)
			}

			// Export
			analytics.GET("/export", dashboardHandler.ExportData)
		}

		// Internal endpoints (for other services to send events)
		internal := v1.Group("/analytics")
		{
			// Events collection - can be called by other services without strict auth
			internal.POST("/events", eventsHandler.CreateEvent)
			internal.POST("/events/batch", eventsHandler.CreateEventsBatch)

			// Security events collection - for auth service to send login attempts and track sessions
			internal.POST("/security/login-attempt", securityHandler.RecordLoginAttempt)
			internal.POST("/security/track-session", securityHandler.TrackSession)
			internal.POST("/security/track-device", securityHandler.TrackDevice)
			internal.POST("/security/sessions/:session_id/deactivate", securityHandler.DeactivateSession)
		}
	}

	return r
}

// startBackgroundTasks starts the background aggregation tasks
func startBackgroundTasks(aggregator *usecase.AggregatorUsecase, log *logger.Logger) {
	// Hourly aggregation
	hourlyTicker := time.NewTicker(1 * time.Hour)
	defer hourlyTicker.Stop()

	// Daily aggregation (at midnight)
	dailyTicker := time.NewTicker(24 * time.Hour)
	defer dailyTicker.Stop()

	// Initial aggregation on startup (after 1 minute delay)
	time.Sleep(1 * time.Minute)
	log.Info("Running initial hourly aggregation...")
	if err := aggregator.AggregateHourlyMetrics(); err != nil {
		log.Errorf("Initial hourly aggregation failed: %v", err)
	}

	for {
		select {
		case <-hourlyTicker.C:
			log.Info("Running hourly aggregation...")
			if err := aggregator.AggregateHourlyMetrics(); err != nil {
				log.Errorf("Hourly aggregation failed: %v", err)
			}

		case <-dailyTicker.C:
			log.Info("Running daily aggregation...")
			if err := aggregator.AggregateDailyMetrics(); err != nil {
				log.Errorf("Daily aggregation failed: %v", err)
			}

			// Also cleanup old events
			log.Info("Cleaning up old events...")
			if err := aggregator.CleanupOldEvents(365); err != nil {
				log.Errorf("Cleanup failed: %v", err)
			}
		}
	}
}
