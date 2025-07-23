// File: services/notification/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/notification/email"
	"tachyon-messenger/services/notification/handlers"
	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/services/notification/repository"
	"tachyon-messenger/services/notification/usecase"
	"tachyon-messenger/services/notification/worker"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	"tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize logger
	log := logger.New(&logger.Config{
		Level:       getLogLevel(),
		Format:      getLogFormat(),
		Environment: os.Getenv("ENVIRONMENT"),
	})

	log.Info("Starting Notification service...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Connect to database
	db, err := database.Connect(database.DefaultConfig(cfg.Database.URL))
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run GORM migrations
	if err := db.Migrate(
		&models.Notification{},
		&models.NotificationDelivery{},
		&models.EmailTemplate{},
		&models.UserNotificationPreference{},
		&models.NotificationTemplate{},
	); err != nil {
		log.Fatalf("Failed to run GORM migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Connect to Redis
	redisClient, err := redis.ConnectRedis(redis.DefaultConfig(cfg.Redis.URL))
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	log.Info("Redis connected successfully")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize email sender
	var emailSender email.EmailSender
	if isEmailEnabled() {
		emailConfig := email.GetSMTPConfigFromEnv()
		emailSender, err = email.NewEmailSender(emailConfig)
		if err != nil {
			log.Warnf("Failed to initialize email sender: %v", err)
			log.Info("Email notifications will be disabled")
		} else {
			// Load default email templates
			templateLoader := email.NewTemplateLoader(emailSender)
			if err := templateLoader.LoadDefaultTemplates(); err != nil {
				log.Warnf("Failed to load default email templates: %v", err)
			} else {
				log.Info("Email sender initialized with default templates")
			}
		}
	} else {
		log.Info("Email notifications disabled by configuration")
	}

	// Initialize repositories
	notificationRepo := repository.NewNotificationRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize usecases
	notificationUC := usecase.NewNotificationUsecase(notificationRepo, emailSender)

	// Initialize background worker
	workerConfig := worker.DefaultWorkerConfig()
	workerConfig.WorkerID = fmt.Sprintf("notification-worker-%s", getServerPort())
	workerConfig.ConcurrentWorkers = getConcurrentWorkers()

	notificationWorker := worker.NewNotificationWorker(notificationUC, redisClient, workerConfig)

	// Start background worker
	notificationWorker.GracefulShutdown() // Setup signal handling
	if err := notificationWorker.Start(); err != nil {
		log.Fatalf("Failed to start notification worker: %v", err)
	}
	log.Info("Notification worker started successfully")

	// Initialize handlers
	notificationHandler := handlers.NewNotificationHandler(notificationUC)

	// Create Gin router
	router := gin.New()

	// Setup common middleware
	setupCommonMiddleware(router)

	// Setup routes
	setupRoutes(router, notificationHandler, jwtConfig, notificationWorker, redisClient, workerConfig, notificationUC)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", getServerPort()),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Notification service starting on port %s", getServerPort())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Start background tasks
	startBackgroundTasks(notificationUC, notificationWorker)

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Notification service...")

	// Stop background worker first
	if err := notificationWorker.Stop(); err != nil {
		log.Errorf("Error stopping notification worker: %v", err)
	}

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Notification service stopped")
}

// setupCommonMiddleware sets up common middleware for the router
func setupCommonMiddleware(router *gin.Engine) {
	// Recovery middleware
	router.Use(gin.Recovery())

	// Logger middleware
	if os.Getenv("ENVIRONMENT") != "production" {
		router.Use(gin.Logger())
	}

	// Request ID middleware
	router.Use(requestid.New())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Security headers
	router.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})
}

// setupRoutes configures all routes for the notification service
func setupRoutes(
	router *gin.Engine,
	notificationHandler *handlers.NotificationHandler,
	jwtConfig *middleware.JWTConfig,
	notificationWorker *worker.Worker,
	redisClient *redis.Client,
	workerConfig *worker.WorkerConfig,
	notificationUC usecase.NotificationUsecase,
) {
	// Health check endpoint
	router.GET("/health", healthHandler)

	// API v1 routes
	v1 := router.Group("/api/v1")

	// Protected notification routes (require JWT authentication)
	notifications := v1.Group("/notifications")
	notifications.Use(middleware.JWTMiddleware(jwtConfig))
	{
		// User notification endpoints
		notifications.GET("", notificationHandler.GetNotifications)            // GET /api/v1/notifications
		notifications.GET("/search", notificationHandler.SearchNotifications)  // GET /api/v1/notifications/search
		notifications.GET("/stats", notificationHandler.GetNotificationStats)  // GET /api/v1/notifications/stats
		notifications.GET("/unread-count", notificationHandler.GetUnreadCount) // GET /api/v1/notifications/unread-count
		notifications.GET("/:id", notificationHandler.GetNotificationByID)     // GET /api/v1/notifications/:id

		// Mark as read endpoints
		notifications.PUT("/:id/read", notificationHandler.MarkAsRead)     // PUT /api/v1/notifications/:id/read
		notifications.PUT("/read", notificationHandler.MarkMultipleAsRead) // PUT /api/v1/notifications/read
		notifications.PUT("/read-all", notificationHandler.MarkAllAsRead)  // PUT /api/v1/notifications/read-all

		// User preferences endpoints
		notifications.GET("/preferences", notificationHandler.GetUserPreferences)         // GET /api/v1/notifications/preferences
		notifications.PUT("/preferences/:type", notificationHandler.UpdateUserPreference) // PUT /api/v1/notifications/preferences/:type
	}

	// Admin routes (require admin role)
	admin := v1.Group("/admin")
	admin.Use(middleware.JWTMiddleware(jwtConfig))
	admin.Use(middleware.RequireRole("admin", "super_admin"))
	{
		// Notification management
		adminNotifications := admin.Group("/notifications")
		{
			adminNotifications.POST("/send", createSendNotificationHandler(notificationWorker))           // POST /api/v1/admin/notifications/send
			adminNotifications.POST("/send-bulk", createSendBulkNotificationHandler(notificationWorker))  // POST /api/v1/admin/notifications/send-bulk
			adminNotifications.POST("/announcement", createSystemAnnouncementHandler(notificationWorker)) // POST /api/v1/admin/notifications/announcement
			adminNotifications.DELETE("/cleanup", createCleanupHandler(notificationUC))                   // DELETE /api/v1/admin/notifications/cleanup
		}

		// Worker management
		adminWorker := admin.Group("/worker")
		{
			adminWorker.GET("/stats", createWorkerStatsHandler(notificationWorker))                // GET /api/v1/admin/worker/stats
			adminWorker.GET("/queues", createQueueStatsHandler(redisClient, workerConfig))         // GET /api/v1/admin/worker/queues
			adminWorker.POST("/queues/purge", createPurgeQueuesHandler(redisClient, workerConfig)) // POST /api/v1/admin/worker/queues/purge
			adminWorker.POST("/queues/requeue", createRequeueHandler(redisClient, workerConfig))   // POST /api/v1/admin/worker/queues/requeue
		}

		// System statistics
		admin.GET("/stats", createSystemStatsHandler(notificationUC)) // GET /api/v1/admin/stats
	}

	// Internal endpoints (for service-to-service communication)
	internal := v1.Group("/internal")
	{
		internal.POST("/notifications/task", createAddTaskHandler(notificationWorker))            // POST /api/v1/internal/notifications/task
		internal.POST("/notifications/scheduled", createScheduledTaskHandler(notificationWorker)) // POST /api/v1/internal/notifications/scheduled
	}
}

// Health check handler
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "notification-service",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   getServiceVersion(),
	})
}

// startBackgroundTasks starts background maintenance tasks
func startBackgroundTasks(notificationUC usecase.NotificationUsecase, notificationWorker *worker.Worker) {
	// Initialize logger for background tasks
	log := logger.New(&logger.Config{
		Level:       getLogLevel(),
		Format:      getLogFormat(),
		Environment: os.Getenv("ENVIRONMENT"),
	})

	// Start scheduled notification processor
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := notificationUC.ProcessScheduledNotifications(); err != nil {
					log.WithField("error", err.Error()).Error("Failed to process scheduled notifications")
				}
			}
		}
	}()

	// Start failed delivery retry processor
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := notificationUC.RetryFailedDeliveries(); err != nil {
					log.WithField("error", err.Error()).Error("Failed to retry failed deliveries")
				}
			}
		}
	}()

	// Start old notification cleanup (daily)
	go func() {
		ticker := time.NewTicker(24 * time.Hour) // Run daily
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Clean up notifications older than 30 days
				cutoffDate := time.Now().Add(-30 * 24 * time.Hour)
				deletedCount, err := notificationUC.DeleteOldNotifications(cutoffDate)
				if err != nil {
					log.WithField("error", err.Error()).Error("Failed to cleanup old notifications")
				} else if deletedCount > 0 {
					log.WithField("deleted_count", deletedCount).Info("Cleaned up old notifications")
				}
			}
		}
	}()

	log.Info("Background tasks started")
}

// Configuration helper functions

func getServerPort() string {
	// Сначала проверяем специфичную переменную для notification service
	port := os.Getenv("NOTIFICATION_SERVICE_PORT")
	if port != "" {
		return port
	}

	// Затем общую переменную SERVER_PORT
	port = os.Getenv("SERVER_PORT")
	if port != "" {
		return port
	}

	// По умолчанию используем 8087 для notification service
	return "8087"
}

func getLogLevel() string {
	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}
	return level
}

func getLogFormat() string {
	format := os.Getenv("LOG_FORMAT")
	if format == "" {
		if os.Getenv("ENVIRONMENT") == "production" {
			format = "json"
		} else {
			format = "text"
		}
	}
	return format
}

func getServiceVersion() string {
	version := os.Getenv("SERVICE_VERSION")
	if version == "" {
		version = "1.0.0"
	}
	return version
}

func getConcurrentWorkers() int {
	workers := os.Getenv("NOTIFICATION_CONCURRENT_WORKERS")
	if workers == "" {
		return 5 // Default
	}

	var count int
	if _, err := fmt.Sscanf(workers, "%d", &count); err == nil && count > 0 {
		return count
	}
	return 5
}

func isEmailEnabled() bool {
	enabled := os.Getenv("EMAIL_ENABLED")
	return enabled != "false" && enabled != "0"
}

// Admin handler creators

func createSendNotificationHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.CreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// Handle priority - if nil, use medium as default
		priority := models.NotificationPriorityMedium
		if req.Priority != nil {
			priority = *req.Priority
		}

		task := worker.CreateSingleNotificationTask(&req, priority)
		if err := w.AddTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to queue notification",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "Notification queued for processing",
			"task_id": task.ID,
		})
	}
}

func createSendBulkNotificationHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.BulkCreateNotificationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		priority := models.NotificationPriorityMedium
		if req.Priority != nil {
			priority = *req.Priority
		}

		task := worker.CreateBulkNotificationTask(&req, priority)
		if err := w.AddTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to queue bulk notification",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message":    "Bulk notification queued for processing",
			"task_id":    task.ID,
			"user_count": len(req.UserIDs),
		})
	}
}

func createSystemAnnouncementHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req usecase.SystemAnnouncementRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		task := worker.CreateSystemAnnouncementTask(&req, req.Priority)
		if err := w.AddTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to queue system announcement",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "System announcement queued for processing",
			"task_id": task.ID,
		})
	}
}

func createCleanupHandler(notificationUC usecase.NotificationUsecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		daysStr := c.DefaultQuery("days", "30")
		var days int
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err != nil || days < 1 {
			days = 30
		}

		cutoffDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		deletedCount, err := notificationUC.DeleteOldNotifications(cutoffDate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to cleanup notifications",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Cleanup completed",
			"deleted_count": deletedCount,
			"cutoff_date":   cutoffDate.Format(time.RFC3339),
		})
	}
}

func createWorkerStatsHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := w.GetStats()
		c.JSON(http.StatusOK, gin.H{
			"worker_stats": stats,
		})
	}
}

func createQueueStatsHandler(redisClient *redis.Client, workerConfig *worker.WorkerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueManager := worker.NewQueueManager(redisClient, workerConfig)
		stats, err := queueManager.GetQueueStats(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to get queue stats",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"queue_stats": stats,
		})
	}
}

func createPurgeQueuesHandler(redisClient *redis.Client, workerConfig *worker.WorkerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		queueManager := worker.NewQueueManager(redisClient, workerConfig)
		if err := queueManager.PurgeQueues(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to purge queues",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "All queues purged successfully",
		})
	}
}

func createRequeueHandler(redisClient *redis.Client, workerConfig *worker.WorkerConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		limitStr := c.DefaultQuery("limit", "100")
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 {
			limit = 100
		}

		queueManager := worker.NewQueueManager(redisClient, workerConfig)
		requeued, err := queueManager.RequeueDeadLetterTasks(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to requeue tasks",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":        "Tasks requeued successfully",
			"requeued_count": requeued,
		})
	}
}

func createSystemStatsHandler(notificationUC usecase.NotificationUsecase) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := notificationUC.GetSystemStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to get system stats",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"system_stats": stats,
		})
	}
}

func createAddTaskHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var task worker.NotificationTask
		if err := c.ShouldBindJSON(&task); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid task data",
				"details": err.Error(),
			})
			return
		}

		if err := w.AddTask(&task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to add task",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message": "Task added successfully",
			"task_id": task.ID,
		})
	}
}

func createScheduledTaskHandler(w *worker.Worker) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Notification *models.CreateNotificationRequest `json:"notification"`
			ScheduledAt  time.Time                         `json:"scheduled_at"`
			Priority     models.NotificationPriority       `json:"priority"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		if req.Notification == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Notification data is required",
			})
			return
		}

		if req.ScheduledAt.Before(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Scheduled time must be in the future",
			})
			return
		}

		task := worker.CreateScheduledNotificationTask(req.Notification, req.ScheduledAt, req.Priority)
		if err := w.AddTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to schedule notification",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"message":      "Notification scheduled successfully",
			"task_id":      task.ID,
			"scheduled_at": req.ScheduledAt.Format(time.RFC3339),
		})
	}
}
