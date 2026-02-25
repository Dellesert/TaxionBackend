package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/calendar/clients"
	"tachyon-messenger/services/calendar/handlers"
	"tachyon-messenger/services/calendar/models"
	"tachyon-messenger/services/calendar/repository"
	"tachyon-messenger/services/calendar/usecase"
	"tachyon-messenger/services/calendar/worker"
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

	log.Info("Starting Calendar service...")

	// Initialize Sentry
	if err := sharedsentry.Init(cfg.Sentry.DSN, "calendar-service"); err != nil {
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
	if err := db.Migrate(
		&models.Event{},
		&models.EventParticipant{},
		&models.EventReminder{},
		&models.Schedule{},
		&models.ScheduleEntry{},
		&models.ScheduleTemplate{},
		&models.ScheduleTemplateEntry{},
		&models.ScheduleAssignment{},
		&models.ScheduleViewer{},
		&models.ScheduleEditor{},
		&models.Absence{},
		&models.AbsenceSubstitution{},
		&models.ScheduleTypeCompatibility{},
	); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default schedule type compatibilities
	seedScheduleTypeCompatibilities(db)

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

	// Initialize authentication configuration (for session support)
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

	// Initialize dependencies
	eventRepo := repository.NewEventRepository(db)
	participantRepo := repository.NewParticipantRepository(db)
	reminderRepo := repository.NewReminderRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	absenceRepo := repository.NewAbsenceRepository(db)
	substitutionRepo := repository.NewSubstitutionRepository(db)

	// Initialize clients
	notificationClient := clients.NewNotificationClient()
	userClient := clients.NewUserClient()
	fileClient := clients.NewFileClient()

	// Initialize usecases
	calendarUsecase := usecase.NewCalendarUsecase(eventRepo, participantRepo, reminderRepo)
	scheduleUsecase := usecase.NewScheduleUsecase(scheduleRepo, eventRepo, absenceRepo)
	templateUsecase := usecase.NewScheduleTemplateUsecase(scheduleRepo)
	importUsecase := usecase.NewScheduleImportUsecase(scheduleRepo, eventRepo, absenceRepo, fileClient)
	absenceUsecase := usecase.NewAbsenceUsecase(absenceRepo, eventRepo, participantRepo, substitutionRepo, notificationClient, userClient)
	holidayUsecase := usecase.NewHolidayUsecase(redisClient)

	// Initialize notification worker
	notificationWorker := worker.NewNotificationWorker(eventRepo, participantRepo, notificationClient, userClient, redisClient)

	// Initialize handlers
	calendarHandler := handlers.NewCalendarHandler(calendarUsecase)
	scheduleHandler := handlers.NewScheduleHandler(scheduleUsecase)
	templateHandler := handlers.NewScheduleTemplateHandler(templateUsecase)
	importHandler := handlers.NewScheduleImportHandler(importUsecase, userClient)
	absenceHandler := handlers.NewAbsenceHandler(absenceUsecase)
	substitutionHandler := handlers.NewSubstitutionHandler(absenceUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "calendar-service", startTime)
	holidayHandler := handlers.NewHolidayHandler(holidayUsecase)

	// Setup routes
	r := setupRoutes(calendarHandler, scheduleHandler, templateHandler, importHandler, absenceHandler, substitutionHandler, metricsHandler, holidayHandler, jwtConfig)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084" // Default port for calendar service
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start background tasks
	startBackgroundTasks(calendarUsecase)

	// Start notification worker
	notificationWorker.Start()

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
	scheduleHandler *handlers.ScheduleHandler,
	templateHandler *handlers.ScheduleTemplateHandler,
	importHandler *handlers.ScheduleImportHandler,
	absenceHandler *handlers.AbsenceHandler,
	substitutionHandler *handlers.SubstitutionHandler,
	metricsHandler *handlers.MetricsHandler,
	holidayHandler *handlers.HolidayHandler,
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
			"service":   "calendar-service",
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

	// Internal events endpoints (no auth required - only accessible from internal network)
	internalEvents := r.Group("/internal/events")
	{
		internalEvents.GET("/today", calendarHandler.GetTodayEvents)
	}

	// API routes
	api := r.Group("/api/v1")

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
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

		// Schedule endpoints
		protected.GET("/schedules", scheduleHandler.GetSchedules)
		protected.GET("/schedules/daily-summary", scheduleHandler.GetDailySummary)
		protected.GET("/schedules/my-entries", scheduleHandler.GetMyScheduleEntries)
		protected.GET("/schedules/:id", scheduleHandler.GetSchedule)
		protected.POST("/schedules", scheduleHandler.CreateSchedule)
		protected.PUT("/schedules/:id", scheduleHandler.UpdateSchedule)
		protected.DELETE("/schedules/:id", scheduleHandler.DeleteSchedule)
		protected.POST("/schedules/:id/publish", scheduleHandler.PublishSchedule)

		// Schedule group members
		protected.GET("/schedules/:id/group-members", scheduleHandler.GetScheduleGroupMembers)

		// Schedule entry endpoints
		protected.GET("/schedules/:id/entries", scheduleHandler.GetScheduleEntries)
		protected.POST("/schedules/:id/entries", scheduleHandler.CreateScheduleEntry)
		protected.PUT("/schedules/:id/entries/:entry_id", scheduleHandler.UpdateScheduleEntry)
		protected.DELETE("/schedules/:id/entries/:entry_id", scheduleHandler.DeleteScheduleEntry)

		// Schedule import endpoints
		protected.POST("/schedules/import", importHandler.ImportSchedule)
		protected.GET("/schedules/import/formats", importHandler.GetSupportedFormats)

		// Schedule template endpoints
		protected.GET("/schedule-templates", templateHandler.GetTemplates)
		protected.GET("/schedule-templates/:id", templateHandler.GetTemplate)
		protected.POST("/schedule-templates", templateHandler.CreateTemplate)
		protected.PUT("/schedule-templates/:id", templateHandler.UpdateTemplate)
		protected.DELETE("/schedule-templates/:id", templateHandler.DeleteTemplate)

		// Template entry endpoints
		protected.GET("/schedule-templates/:id/entries", templateHandler.GetTemplateEntries)
		protected.POST("/schedule-templates/:id/entries", templateHandler.AddTemplateEntry)
		protected.DELETE("/schedule-templates/:id/entries/:entry_id", templateHandler.DeleteTemplateEntry)

		// Apply template
		protected.POST("/schedule-templates/:id/apply", templateHandler.ApplyTemplate)

		// Absence endpoints (read - all authenticated users)
		protected.GET("/absences", absenceHandler.GetAbsences)
		protected.GET("/absences/:id", absenceHandler.GetAbsence)
		protected.GET("/users/:id/absences", absenceHandler.GetUserAbsences)

		// Absence endpoints (write - admin, super_admin, department_head only)
		protected.POST("/absences", middleware.RequireDepartmentHeadOrAbove(), absenceHandler.CreateAbsence)
		protected.PUT("/absences/:id", middleware.RequireDepartmentHeadOrAbove(), absenceHandler.UpdateAbsence)
		protected.DELETE("/absences/:id", middleware.RequireDepartmentHeadOrAbove(), absenceHandler.DeleteAbsence)

		// Absence substitution endpoints (read - all authenticated users)
		protected.GET("/absences/:id/substitutions", substitutionHandler.GetSubstitutions)
		protected.GET("/users/:id/substitutions", substitutionHandler.GetUserSubstitutions)

		// Absence substitution endpoints (write - admin, super_admin, department_head only)
		protected.POST("/absences/:id/substitutions", middleware.RequireDepartmentHeadOrAbove(), substitutionHandler.CreateSubstitution)
		protected.PUT("/absences/:id/substitutions/:sub_id", middleware.RequireDepartmentHeadOrAbove(), substitutionHandler.UpdateSubstitution)
		protected.DELETE("/absences/:id/substitutions/:sub_id", middleware.RequireDepartmentHeadOrAbove(), substitutionHandler.DeleteSubstitution)

		// Holiday endpoints (production calendar)
		protected.GET("/calendar/holidays", holidayHandler.GetHolidays)
	}

	return r
}

// startBackgroundTasks starts background workers for processing reminders
func startBackgroundTasks(calendarUC usecase.CalendarUsecase) {
	log := logger.New(&logger.Config{
		Level:       "info",
		Format:      "json",
		Environment: os.Getenv("ENVIRONMENT"),
	})

	log.Info("Starting background tasks for calendar notifications")

	// Start reminder processor (runs every minute)
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := calendarUC.ProcessEventReminders(); err != nil {
					log.WithField("error", err.Error()).Error("Failed to process event reminders")
				}
			}
		}
	}()

	log.Info("Background tasks started successfully")
}

// seedScheduleTypeCompatibilities seeds default schedule type compatibilities
// Work schedule is compatible with on_duty (дежурство)
func seedScheduleTypeCompatibilities(db *database.DB) {
	compatibilities := []models.ScheduleTypeCompatibility{
		{ScheduleType: models.ScheduleTypeWork, CompatibleWith: models.ScheduleTypeOnDuty},
		{ScheduleType: models.ScheduleTypeOnDuty, CompatibleWith: models.ScheduleTypeWork},
	}

	for _, c := range compatibilities {
		// Insert only if not exists
		var count int64
		db.Model(&models.ScheduleTypeCompatibility{}).
			Where("schedule_type = ? AND compatible_with = ?", c.ScheduleType, c.CompatibleWith).
			Count(&count)

		if count == 0 {
			db.Create(&c)
		}
	}
}
