package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"tachyon-messenger/services/user/models"
	"tachyon-messenger/services/user/repository"
	"tachyon-messenger/shared/analytics"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/middleware"
	sharedmodels "tachyon-messenger/shared/models"
	sessionpkg "tachyon-messenger/shared/session"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	qrTokenTTL    = 3 * time.Minute
	qrRedisPrefix = "qr_login:"

	qrStatusPending   = "pending"
	qrStatusConfirmed = "confirmed"
)

// QRLoginToken represents a QR login request stored in Redis
type QRLoginToken struct {
	Token             string `json:"token"`
	Status            string `json:"status"`
	IPAddress         string `json:"ip_address"`
	UserAgent         string `json:"user_agent"`
	CreatedAt         int64  `json:"created_at"`
	ExpiresAt         int64  `json:"expires_at"`
	ConfirmedByUserID *uint  `json:"confirmed_by_user_id,omitempty"`
	SessionID         string `json:"session_id,omitempty"`
}

// QRAuthHandler handles QR code login requests
type QRAuthHandler struct {
	redisClient     *redis.Client
	userRepo        repository.UserRepository
	analyticsClient *analytics.Client
}

// NewQRAuthHandler creates a new QR auth handler
func NewQRAuthHandler(redisClient *redis.Client, userRepo repository.UserRepository, analyticsClient *analytics.Client) *QRAuthHandler {
	return &QRAuthHandler{
		redisClient:     redisClient,
		userRepo:        userRepo,
		analyticsClient: analyticsClient,
	}
}

// GenerateQRToken generates a new QR login token
// POST /auth/qr/generate (public, no auth required)
func (h *QRAuthHandler) GenerateQRToken(c *gin.Context) {
	requestID := requestid.Get(c)

	// Generate cryptographically secure token
	token, err := generateQRToken()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to generate QR token")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось сгенерировать QR-токен",
			"request_id": requestID,
		})
		return
	}

	// Extract client info (desktop browser/Electron)
	ipAddress, userAgent := middleware.ExtractClientInfo(c)

	now := time.Now()
	qrToken := &QRLoginToken{
		Token:     token,
		Status:    qrStatusPending,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(qrTokenTTL).Unix(),
	}

	// Store in Redis
	data, err := json.Marshal(qrToken)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to marshal QR token")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось сгенерировать QR-токен",
			"request_id": requestID,
		})
		return
	}

	ctx := context.Background()
	key := qrRedisPrefix + token
	if err := h.redisClient.Set(ctx, key, data, qrTokenTTL).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to store QR token in Redis")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось сгенерировать QR-токен",
			"request_id": requestID,
		})
		return
	}

	logger.WithFields(map[string]interface{}{
		"request_id": requestID,
		"ip_address": ipAddress,
	}).Info("QR login token generated")

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": qrToken.ExpiresAt,
		"request_id": requestID,
	})
}

// GetQRTokenStatus checks the status of a QR login token (polling endpoint)
// GET /auth/qr/status/:token (public, no auth required)
func (h *QRAuthHandler) GetQRTokenStatus(c *gin.Context) {
	requestID := requestid.Get(c)
	token := c.Param("token")

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Токен обязателен",
			"request_id": requestID,
		})
		return
	}

	ctx := context.Background()
	key := qrRedisPrefix + token

	data, err := h.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":     "expired",
			"error":      "QR-токен истёк или не найден",
			"request_id": requestID,
		})
		return
	}
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get QR token from Redis")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось проверить статус QR-токена",
			"request_id": requestID,
		})
		return
	}

	var qrToken QRLoginToken
	if err := json.Unmarshal([]byte(data), &qrToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось разобрать QR-токен",
			"request_id": requestID,
		})
		return
	}

	response := gin.H{
		"status":     qrToken.Status,
		"request_id": requestID,
	}

	// If confirmed, include session data and user info
	if qrToken.Status == qrStatusConfirmed && qrToken.ConfirmedByUserID != nil {
		response["session"] = gin.H{
			"session_id": qrToken.SessionID,
			"expires_at": time.Now().Add(middleware.GetSessionDuration()).Unix(),
		}

		// Get user data for the response
		user, err := h.userRepo.GetWithDepartment(*qrToken.ConfirmedByUserID)
		if err == nil {
			response["user"] = userToQRResponse(user)
			response["auth_mode"] = middleware.GetAuthMode()
		}

		// Delete QR token after successful retrieval (one-time use)
		h.redisClient.Del(ctx, key)
	}

	c.JSON(http.StatusOK, response)
}

// ConfirmQRLogin confirms a QR login from an authenticated mobile device
// POST /auth/qr/confirm (requires auth - mobile user must be logged in)
func (h *QRAuthHandler) ConfirmQRLogin(c *gin.Context) {
	requestID := requestid.Get(c)

	// Get authenticated user info from middleware context
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "Требуется аутентификация",
			"request_id": requestID,
		})
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Токен обязателен",
			"request_id": requestID,
		})
		return
	}

	// Get user to check permissions
	user, err := h.userRepo.GetByID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось получить данные пользователя",
			"request_id": requestID,
		})
		return
	}

	// Block super_admin from QR login (same restriction as regular login)
	if user.Role == sharedmodels.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Доступ суперадмина ограничен только веб-панелью",
			"request_id": requestID,
		})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "Аккаунт пользователя деактивирован",
			"request_id": requestID,
		})
		return
	}

	ctx := context.Background()
	key := qrRedisPrefix + req.Token

	// Get QR token from Redis
	data, err := h.redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "QR-токен истёк или не найден",
			"request_id": requestID,
		})
		return
	}
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get QR token from Redis")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось обработать QR-вход",
			"request_id": requestID,
		})
		return
	}

	var qrToken QRLoginToken
	if err := json.Unmarshal([]byte(data), &qrToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось разобрать QR-токен",
			"request_id": requestID,
		})
		return
	}

	// Token must be in pending status
	if qrToken.Status != qrStatusPending {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "QR-токен уже использован",
			"request_id": requestID,
		})
		return
	}

	// Create a new session for the desktop device
	authConfig := middleware.GetAuthConfig()
	if authConfig == nil || authConfig.SessionStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Управление сессиями недоступно",
			"request_id": requestID,
		})
		return
	}

	// Use the desktop's IP and User-Agent that were captured when QR was generated
	newSession, evictedSessionIDs, err := authConfig.SessionStore.CreateSession(
		ctx,
		user.ID,
		user.Email,
		user.Role,
		qrToken.IPAddress,
		qrToken.UserAgent,
	)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"user_id":    userID,
			"error":      err.Error(),
		}).Error("Failed to create session for QR login")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Не удалось создать сессию",
			"request_id": requestID,
		})
		return
	}

	// Notify evicted sessions via WebSocket
	sessionpkg.NotifyEvictedSessions(evictedSessionIDs, "session_limit_exceeded")

	// Update QR token status to confirmed
	qrToken.Status = qrStatusConfirmed
	qrToken.ConfirmedByUserID = &userID
	qrToken.SessionID = newSession.SessionID

	updatedData, _ := json.Marshal(qrToken)
	// Keep it in Redis for 30 seconds so the desktop polling can pick it up
	h.redisClient.Set(ctx, key, updatedData, 30*time.Second)

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"user_id":        userID,
		"new_session_id": newSession.SessionID,
		"desktop_ip":     qrToken.IPAddress,
	}).Info("QR login confirmed successfully")

	// Track in analytics
	h.analyticsClient.SendEvent(
		analytics.EventUserLogin,
		analytics.CategoryUser,
		uint64(userID),
		map[string]interface{}{
			"email":      user.Email,
			"auth_mode":  "qr_login",
			"ip_address": qrToken.IPAddress,
		},
	)

	// Track session in analytics
	expiresAt := newSession.ExpiresAt
	h.analyticsClient.TrackSessionAsync(
		uint64(userID),
		newSession.SessionID,
		qrToken.IPAddress,
		qrToken.UserAgent,
		expiresAt,
	)

	c.JSON(http.StatusOK, gin.H{
		"message":    "QR login confirmed successfully",
		"request_id": requestID,
	})
}

// generateQRToken generates a cryptographically secure token
func generateQRToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// userToQRResponse converts user service model to shared model for QR login response
func userToQRResponse(user *models.User) *sharedmodels.User {
	sharedUser := &sharedmodels.User{
		BaseModel:    user.BaseModel,
		Email:        user.Email,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		Avatar:       user.Avatar,
		Phone:        user.Phone,
		Position:     user.Position,
		LastActiveAt: user.LastActiveAt,
		IsActive:     user.IsActive,
	}

	if user.Department != nil {
		sharedUser.Department = user.Department.Name
	}

	return sharedUser
}
