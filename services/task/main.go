package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/task/handlers"
	"tachyon-messenger/services/task/models"
	"tachyon-messenger/services/task/repository"
	"tachyon-messenger/services/task/usecase"
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

	log.Info("Starting Task service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.Migrate(&models.Task{}, &models.TaskComment{}, &models.TaskAssignee{}); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize dependencies
	taskRepo := repository.NewTaskRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize usecases
	taskUsecase := usecase.NewTaskUsecase(taskRepo, commentRepo)

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskUsecase)
	internalHandler := handlers.NewInternalHandler(taskUsecase)

	// Setup routes
	r := setupRoutes(taskHandler, internalHandler, jwtConfig)

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
	jwtConfig *middleware.JWTConfig,
) *gin.Engine {
	r := gin.New()

	// Global middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(requestid.New())

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

	// API routes
	api := r.Group("/api/v1")

	// Internal routes (no auth required - for inter-service communication)
	internal := api.Group("/internal")
	{
		internal.GET("/tasks/:id", internalHandler.GetTaskForChat)
	}

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware(jwtConfig))
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
	}

	return r
}
