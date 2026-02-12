package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/chat/client"
	"tachyon-messenger/services/chat/handlers"
	"tachyon-messenger/services/chat/migrations"
	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/repository"
	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/services/chat/websocket"
	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sharedredis "tachyon-messenger/shared/redis"

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

	log.Info("Starting Chat service...")

	// Connect to database
	dbConfig := database.DefaultConfig(cfg.Database.URL)
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run SQL migrations first
	migrationManager := migrations.NewMigrationManager(db, log)
	if err := migrationManager.RunMigrations(); err != nil {
		log.Fatalf("Failed to run SQL migrations: %v", err)
	}

	// Run GORM migrations for model sync (ensures all indexes and constraints)
	if err := db.Migrate(
		&models.Chat{},
		&models.ChatMember{},
		&models.Message{},
		&models.MessageReaction{},
		&models.MessageReadReceipt{},
		&models.MessageAttachment{},
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

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize dependencies
	chatRepo := repository.NewChatRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize authentication configuration (for session support in WebSocket)
	authMode := sharedmodels.AuthMode(cfg.Auth.Mode)
	sessionDuration := time.Duration(cfg.Auth.SessionDuration) * time.Hour
	middleware.InitAuthConfig(authMode, jwtConfig, redisClient.Client, sessionDuration)
	log.WithFields(map[string]interface{}{
		"auth_mode":        authMode,
		"session_duration": sessionDuration,
	}).Info("Authentication configuration initialized")

	// Initialize notification client with Redis for duplicate prevention
	notificationClient := client.NewNotificationClient(redisClient)

	// Initialize usecases
	chatUsecase := usecase.NewChatUsecase(chatRepo, messageRepo)
	messageUsecase := usecase.NewMessageUsecase(messageRepo, chatRepo, notificationClient)

	// Initialize WebSocket hub with messageUsecase and Redis for distributed presence tracking
	wsHub := websocket.NewHub(messageUsecase, redisClient)

	// Set WebSocket hub in usecases to enable broadcasting
	messageUsecase.SetWebSocketHub(wsHub)
	chatUsecase.SetWebSocketHub(wsHub)

	// Set chat repository in WebSocket hub for getting chat members
	wsHub.SetChatRepository(chatRepo)

	go wsHub.Run()

	// Initialize handlers
	chatHandler := handlers.NewChatHandler(chatUsecase)
	messageHandler := handlers.NewMessageHandler(messageUsecase, analyticsClient)
	wsHandler := handlers.NewWebSocketHandler(wsHub, messageUsecase)
	metricsHandler := handlers.NewMetricsHandler(wsHub, db, redisClient, "chat-service", startTime)
	internalHandler := handlers.NewInternalHandler(wsHub)

	// Create Gin router
	router := gin.New()

	// Setup common middleware (without CORS - Gateway handles it)
	middleware.SetupCommonMiddlewareWithoutCORS(router)

	// Add metrics middleware to track HTTP requests
	router.Use(metricsHandler.MetricsMiddleware())

	// Setup routes
	setupRoutes(router, chatHandler, messageHandler, wsHandler, metricsHandler, internalHandler, jwtConfig)

	// Create HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", getServerPort()),
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("Chat service starting on port %s", getServerPort())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down Chat service...")

	// Close WebSocket hub
	wsHub.Close()

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}

	log.Info("Chat service stopped")
}

// setupRoutes configures all routes for the chat service
func setupRoutes(router *gin.Engine, chatHandler *handlers.ChatHandler, messageHandler *handlers.MessageHandler, wsHandler *handlers.WebSocketHandler, metricsHandler *handlers.MetricsHandler, internalHandler *handlers.InternalHandler, jwtConfig *middleware.JWTConfig) {
	// Health check endpoint
	router.Any("/health", healthHandler)

	// Internal endpoints (no auth required - only accessible from internal network)
	internal := router.Group("/api/v1/internal")
	{
		// Metrics endpoints
		metrics := internal.Group("/metrics")
		{
			metrics.GET("/websocket", metricsHandler.GetWebSocketMetrics)
			metrics.GET("/database", metricsHandler.GetDatabaseMetrics)
			metrics.GET("/redis", metricsHandler.GetRedisMetrics)
			metrics.GET("/runtime", metricsHandler.GetRuntimeMetrics)
		}

		// WebSocket broadcast endpoints
		ws := internal.Group("/ws")
		{
			ws.POST("/broadcast/user", internalHandler.BroadcastToUser)
			ws.POST("/disconnect-session", internalHandler.DisconnectSession)
		}
	}

	// WebSocket endpoint БЕЗ JWT middleware (обрабатывает аутентификацию самостоятельно)
	router.GET("/api/v1/ws", wsHandler.HandleWebSocket) // GET /api/v1/ws

	// API v1 routes с unified auth middleware (supports both session and JWT)
	v1 := router.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware()) // Unified auth middleware (session or JWT)
	{
		// Chat routes
		chats := v1.Group("/chats")
		{
			chats.GET("", chatHandler.GetChats)                      // GET /api/v1/chats
			chats.GET("/saved", chatHandler.GetSavedChat)            // GET /api/v1/chats/saved
			chats.GET("/pinned", chatHandler.GetPinnedChats)         // GET /api/v1/chats/pinned
			chats.GET("/unread-count", chatHandler.GetTotalUnreadCount) // GET /api/v1/chats/unread-count
			chats.POST("", chatHandler.CreateChat)                   // POST /api/v1/chats
			chats.POST("/direct/:userId", chatHandler.GetOrCreateDirectChat)  // POST /api/v1/chats/direct/:userId
			chats.POST("/task/:taskId", chatHandler.GetOrCreateTaskChat)      // POST /api/v1/chats/task/:taskId
			chats.POST("/:id/join", chatHandler.JoinChat)            // POST /api/v1/chats/:id/join
			chats.PUT("/:id/favorite", chatHandler.ToggleFavorite)   // PUT /api/v1/chats/:id/favorite
			chats.PUT("/:id/pinned", chatHandler.TogglePinned)       // PUT /api/v1/chats/:id/pinned
			chats.GET("/:id", chatHandler.GetChat)                   // GET /api/v1/chats/:id
			chats.PUT("/:id", chatHandler.UpdateChat)                // PUT /api/v1/chats/:id
			chats.DELETE("/:id", chatHandler.DeleteChat)             // DELETE /api/v1/chats/:id

			// Chat members
			chats.GET("/:id/members", chatHandler.GetChatMembers)                // GET /api/v1/chats/:id/members
			chats.POST("/:id/members", chatHandler.AddChatMember)                // POST /api/v1/chats/:id/members
			chats.PUT("/:id/members/:userId", chatHandler.UpdateChatMemberRole)  // PUT /api/v1/chats/:id/members/:userId
			chats.DELETE("/:id/members/:userId", chatHandler.RemoveChatMember)   // DELETE /api/v1/chats/:id/members/:userId

			// Chat attachments
			chats.GET("/:id/attachments", chatHandler.GetChatAttachments)        // GET /api/v1/chats/:id/attachments
		}

		// Message routes
		messages := v1.Group("/messages")
		{
			messages.GET("", messageHandler.GetMessages)          // GET /api/v1/messages (DEPRECATED - use /chats/:id/messages/latest)
			messages.POST("", messageHandler.SendMessage)         // POST /api/v1/messages
			messages.POST("/bulk-delete", messageHandler.BulkDeleteMessages)   // POST /api/v1/messages/bulk-delete
			messages.POST("/bulk-forward", messageHandler.BulkForwardMessages) // POST /api/v1/messages/bulk-forward
			messages.GET("/:id", messageHandler.GetMessage)       // GET /api/v1/messages/:id
			messages.PUT("/:id", messageHandler.UpdateMessage)    // PUT /api/v1/messages/:id
			messages.DELETE("/:id", messageHandler.DeleteMessage) // DELETE /api/v1/messages/:id
			messages.POST("/:id/restore", messageHandler.RestoreMessage) // POST /api/v1/messages/:id/restore
			messages.POST("/:id/pin", messageHandler.PinMessage)         // POST /api/v1/messages/:id/pin
			messages.POST("/:id/unpin", messageHandler.UnpinMessage)     // POST /api/v1/messages/:id/unpin

			// Message by chat (DEPRECATED - use new endpoints below)
			messages.GET("/chat/:chatId", messageHandler.GetMessagesByChat)         // GET /api/v1/messages/chat/:chatId (DEPRECATED)
			messages.POST("/chat/:chatId/read", messageHandler.MarkChatAsRead)      // POST /api/v1/messages/chat/:chatId/read
			messages.POST("/:id/read", messageHandler.MarkAsRead)                   // POST /api/v1/messages/:id/read
		}

		// NEW refactored message endpoints (under chats) - using :id for consistency
		chats.GET("/:id/messages/latest", messageHandler.GetLatestMessages)                     // GET /api/v1/chats/:id/messages/latest
		chats.GET("/:id/messages/before/:messageId", messageHandler.GetMessagesBeforeID)        // GET /api/v1/chats/:id/messages/before/:messageId
		chats.GET("/:id/messages/after/:messageId", messageHandler.GetMessagesAfterID)          // GET /api/v1/chats/:id/messages/after/:messageId
		chats.GET("/:id/messages/context/:messageId", messageHandler.GetMessageContext)         // GET /api/v1/chats/:id/messages/context/:messageId
		chats.GET("/:id/messages/search", messageHandler.SearchMessages)                        // GET /api/v1/chats/:id/messages/search?q=query

		// Chat-specific routes
		chats.POST("/:id/read", messageHandler.MarkChatAsRead)            // POST /api/v1/chats/:id/read (mark all messages as read)
		chats.POST("/:id/clear-history", messageHandler.ClearChatHistory) // POST /api/v1/chats/:id/clear-history
	}
}

// healthHandler handles health check requests
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "chat-service",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// getServerPort returns the server port from environment or default
func getServerPort() string {
	if port := os.Getenv("CHAT_SERVICE_PORT"); port != "" {
		return port
	}
	return "8082" // Default port for chat service
}
