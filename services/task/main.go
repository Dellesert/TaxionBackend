package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/task/clients"
	"tachyon-messenger/services/task/handlers"
	"tachyon-messenger/services/task/migrations"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
	"tachyon-messenger/services/task/usecase"
	"tachyon-messenger/services/task/worker"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sharedredis "tachyon-messenger/shared/redis"
	"tachyon-messenger/shared/analytics"

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

	log.Info("Starting Task service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run custom SQL migrations first (for existing data)
	log.Info("Running custom migrations...")
	if err := migrations.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run custom migrations: %v", err)
	}

	// Run GORM auto-migrations
	log.Info("Running GORM auto-migrations...")
	if err := db.Migrate(
		&models.Task{},
		&models.TaskComment{},
		&models.TaskAssignee{},
		&models.TaskActivity{},
		&models.TaskAttachment{},
		&models.TaskChecklist{},
		&models.TaskChecklistItem{},
		&sharedmodels.DeletedRecord{}, // For sync tracking
	); err != nil {
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

	// Initialize analytics client
	analyticsURL := os.Getenv("ANALYTICS_SERVICE_URL")
	if analyticsURL == "" {
		analyticsURL = "http://analytics-service:8086"
	}
	analyticsClient := analytics.NewClient(analyticsURL, log)
	log.Infof("Analytics client initialized with URL: %s", analyticsURL)

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

	// Initialize repositories
	taskRepo := repository.NewTaskRepository(db)
	commentRepo := repository.NewCommentRepository(db)
	activityRepo := repository.NewActivityRepository(db)
	attachmentRepo := repository.NewAttachmentRepository(db)
	checklistRepo := repository.NewChecklistRepository(db)
	syncRepo := repository.NewSyncRepository(db)

	// Initialize user client
	userClient := clients.NewUserClient()

	// Initialize usecases (order matters due to dependencies)
	activityUsecase := usecase.NewActivityUsecase(activityRepo, taskRepo, userClient)
	attachmentUsecase := usecase.NewAttachmentUsecase(attachmentRepo, taskRepo)
	checklistUsecase := usecase.NewChecklistUsecase(checklistRepo, taskRepo)

	// Set activity usecase for attachment usecase (to avoid circular dependency)
	attachmentUsecase.SetActivityUsecase(activityUsecase)

	// Initialize task usecase with attachment and sync support
	taskUsecase := usecase.NewTaskUsecaseWithSync(taskRepo, commentRepo, activityRepo, attachmentRepo, checklistRepo, attachmentUsecase, syncRepo)

	// Set task usecase for checklist usecase (to avoid circular dependency)
	checklistUsecase.SetTaskUsecase(taskUsecase)

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskUsecase, analyticsClient)
	internalHandler := handlers.NewInternalHandler(taskUsecase)
	activityHandler := handlers.NewActivityHandler(activityUsecase)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentUsecase, taskUsecase)
	checklistHandler := handlers.NewChecklistHandler(checklistUsecase)
	metricsHandler := handlers.NewMetricsHandler(db, redisClient, "task-service", startTime)

	// Start notification worker for deadline checks and reminders
	notificationClient := clients.NewNotificationClient()
	notificationWorker := worker.NewNotificationWorker(taskRepo, notificationClient, userClient)
	notificationWorker.Start()

	// Setup routes
	r := setupRoutes(taskHandler, internalHandler, activityHandler, attachmentHandler, checklistHandler, metricsHandler, jwtConfig)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083" // Default port for task service
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Task service starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Task service...")

	// Give outstanding requests a deadline for completion
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Task service stopped")
}

func setupRoutes(
	taskHandler *handlers.TaskHandler,
	internalHandler *handlers.InternalHandler,
	activityHandler *handlers.ActivityHandler,
	attachmentHandler *handlers.AttachmentHandler,
	checklistHandler *handlers.ChecklistHandler,
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
			"service":   "task-service",
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

	// Internal routes (no auth required - for inter-service communication)
	internal := api.Group("/internal")
	{
		internal.GET("/tasks/stats", internalHandler.GetTaskStats)
		internal.GET("/tasks/:id", internalHandler.GetTaskForChat)

		// Analytics endpoints
		analytics := internal.Group("/tasks/analytics")
		{
			analytics.GET("/departments", internalHandler.GetDepartmentTaskStats)
			analytics.GET("/top-performers", internalHandler.GetTopPerformers)
			analytics.GET("/trends", internalHandler.GetTaskTrends)
			analytics.GET("/priority-distribution", internalHandler.GetPriorityDistribution)
		}
	}

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(middleware.AuthMiddleware())
	{
		// Task endpoints - viewing tasks (all authenticated users)
		protected.GET("/tasks", taskHandler.GetTasks)
		protected.GET("/tasks/:id", taskHandler.GetTask)

		// Creating tasks - access control handled in usecase
		// (employees can create tasks for themselves, department_head+ can assign to others)
		protected.POST("/tasks", taskHandler.CreateTask)

		// Updating/Deleting tasks - handled in usecase (creator, admin, super_admin)
		protected.PUT("/tasks/:id", taskHandler.UpdateTask)
		protected.DELETE("/tasks/:id", taskHandler.DeleteTask)

		// Status updates - handled in usecase (assignee, creator, department_head, admin, super_admin)
		protected.PATCH("/tasks/:id/status", taskHandler.UpdateTaskStatus)

		// Task statistics
		protected.GET("/tasks/stats", taskHandler.GetTaskStats)

		// Task assignments - only department_head, admin, super_admin can assign tasks
		protected.POST("/tasks/:id/assign", middleware.RequireDepartmentHeadOrAbove(), taskHandler.AssignTask)
		protected.DELETE("/tasks/:id/assign", middleware.RequireDepartmentHeadOrAbove(), taskHandler.UnassignTask)

		// Task comments
		protected.POST("/tasks/:id/comments", taskHandler.AddComment)
		protected.GET("/tasks/:id/comments", taskHandler.GetTaskComments)

		// Comment management
		protected.PUT("/comments/:id", taskHandler.UpdateComment)
		protected.DELETE("/comments/:id", taskHandler.DeleteComment)

		// Task hierarchy
		protected.POST("/tasks/:id/subtasks", taskHandler.CreateSubtask)
		protected.GET("/tasks/:id/subtasks", taskHandler.GetSubtasks)
		protected.GET("/tasks/:id/hierarchy", taskHandler.GetTaskHierarchy)

		// Task delegation
		protected.POST("/tasks/:id/delegate", middleware.RequireDepartmentHeadOrAbove(), taskHandler.DelegateTask)
		protected.GET("/tasks/:id/delegation-chain", taskHandler.GetDelegationChain)

		// First-view tracking
		protected.POST("/tasks/:id/view", taskHandler.MarkTaskAsViewed)

		// Progress management
		protected.PATCH("/tasks/:id/progress", taskHandler.UpdateTaskProgress)

		// Task activities
		protected.GET("/tasks/:id/activities", activityHandler.GetTaskActivities)

		// Task attachments
		protected.POST("/tasks/:id/attachments", attachmentHandler.UploadAttachment)
		protected.GET("/tasks/:id/attachments", attachmentHandler.GetTaskAttachments)
		protected.DELETE("/attachments/:id", attachmentHandler.DeleteAttachment)

		// Task checklists
		protected.POST("/tasks/:id/checklists", checklistHandler.CreateChecklist)
		protected.GET("/tasks/:id/checklists", checklistHandler.GetTaskChecklists)
		protected.PUT("/checklists/:id", checklistHandler.UpdateChecklist)
		protected.DELETE("/checklists/:id", checklistHandler.DeleteChecklist)

		// Checklist items
		protected.POST("/checklists/:id/items", checklistHandler.CreateChecklistItem)
		protected.PUT("/checklist-items/:id", checklistHandler.UpdateChecklistItem)
		protected.PATCH("/checklist-items/:id/toggle", checklistHandler.ToggleChecklistItem)
		protected.DELETE("/checklist-items/:id", checklistHandler.DeleteChecklistItem)

		// Task permissions endpoints (NEW)
		protected.GET("/tasks/:id/permissions", taskHandler.GetTaskPermissions)
		protected.POST("/tasks/:id/emergency-complete", taskHandler.EmergencyCompleteTask)
	}

	return r
}
