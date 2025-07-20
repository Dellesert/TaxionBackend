package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tachyon-messenger/services/chat/handlers"
	"tachyon-messenger/services/chat/migrations"
	"tachyon-messenger/services/chat/models"
	"tachyon-messenger/services/chat/repository"
	"tachyon-messenger/services/chat/usecase"
	"tachyon-messenger/services/chat/websocket"
	"tachyon-messenger/shared/config"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"

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
	); err != nil {
		log.Fatalf("Failed to run GORM migrations: %v", err)
	}

	log.Info("Database connected and migrations completed")

	// Set Gin mode based on environment
	if os.Getenv("ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize dependencies
	chatRepo := repository.NewChatRepository(db)
	messageRepo := repository.NewMessageRepository(db)

	// Create JWT config
	jwtConfig := middleware.DefaultJWTConfig(cfg.JWT.Secret)

	// Initialize usecases
	chatUsecase := usecase.NewChatUsecase(chatRepo, messageRepo)
	messageUsecase := usecase.NewMessageUsecase(messageRepo, chatRepo)

	// Initialize WebSocket hub С messageUsecase
	wsHub := websocket.NewHub(messageUsecase)
	go wsHub.Run()

	// Initialize handlers
	chatHandler := handlers.NewChatHandler(chatUsecase)
	messageHandler := handlers.NewMessageHandler(messageUsecase)
	wsHandler := handlers.NewWebSocketHandler(wsHub, messageUsecase)

	// Create Gin router
	router := gin.New()

	// Setup common middleware
	middleware.SetupCommonMiddleware(router)

	// Setup routes
	setupRoutes(router, chatHandler, messageHandler, wsHandler, jwtConfig)

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
func setupRoutes(router *gin.Engine, chatHandler *handlers.ChatHandler, messageHandler *handlers.MessageHandler, wsHandler *handlers.WebSocketHandler, jwtConfig *middleware.JWTConfig) {
	// Health check endpoint
	router.GET("/health", healthHandler)

	// WebSocket endpoint БЕЗ JWT middleware (обрабатывает аутентификацию самостоятельно)
	router.GET("/api/v1/ws", wsHandler.HandleWebSocket) // GET /api/v1/ws

	// API v1 routes с JWT middleware
	v1 := router.Group("/api/v1")
	v1.Use(middleware.JWTMiddleware(jwtConfig)) // JWT middleware только для этих routes
	{
		// Chat routes
		chats := v1.Group("/chats")
		{
			chats.GET("", chatHandler.GetChats)           // GET /api/v1/chats
			chats.POST("", chatHandler.CreateChat)        // POST /api/v1/chats
			chats.POST("/:id/join", chatHandler.JoinChat) // POST /api/v1/chats/:id/join
			chats.GET("/:id", chatHandler.GetChat)        // GET /api/v1/chats/:id
			chats.PUT("/:id", chatHandler.UpdateChat)     // PUT /api/v1/chats/:id
			chats.DELETE("/:id", chatHandler.DeleteChat)  // DELETE /api/v1/chats/:id

			// Chat members
			chats.GET("/:id/members", chatHandler.GetChatMembers)              // GET /api/v1/chats/:id/members
			chats.POST("/:id/members", chatHandler.AddChatMember)              // POST /api/v1/chats/:id/members
			chats.DELETE("/:id/members/:userId", chatHandler.RemoveChatMember) // DELETE /api/v1/chats/:id/members/:userId
		}

		// Message routes
		messages := v1.Group("/messages")
		{
			messages.GET("", messageHandler.GetMessages)          // GET /api/v1/messages
			messages.POST("", messageHandler.SendMessage)         // POST /api/v1/messages
			messages.GET("/:id", messageHandler.GetMessage)       // GET /api/v1/messages/:id
			messages.PUT("/:id", messageHandler.UpdateMessage)    // PUT /api/v1/messages/:id
			messages.DELETE("/:id", messageHandler.DeleteMessage) // DELETE /api/v1/messages/:id

			// Message by chat
			messages.GET("/chat/:chatId", messageHandler.GetMessagesByChat) // GET /api/v1/messages/chat/:chatId
		}
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
