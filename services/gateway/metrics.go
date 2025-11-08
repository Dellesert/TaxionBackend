// File: services/gateway/metrics.go
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// SystemMetrics represents aggregated system metrics
type SystemMetrics struct {
	Timestamp   time.Time              `json:"timestamp"`
	Services    []ServiceHealth        `json:"services"`
	Database    *DatabaseMetrics       `json:"database,omitempty"`
	Redis       *RedisMetrics          `json:"redis,omitempty"`
	WebSocket   *WebSocketMetrics      `json:"websocket,omitempty"`
	Notificator *NotificationMetrics   `json:"notificator,omitempty"`
}

// DatabaseMetrics represents PostgreSQL connection pool metrics
type DatabaseMetrics struct {
	Status              string `json:"status"`
	MaxOpenConnections  int    `json:"max_open_connections"`
	OpenConnections     int    `json:"open_connections"`
	InUse               int    `json:"in_use"`
	Idle                int    `json:"idle"`
	WaitCount           int64  `json:"wait_count"`
	WaitDuration        string `json:"wait_duration"`
	MaxIdleClosed       int64  `json:"max_idle_closed"`
	MaxLifetimeClosed   int64  `json:"max_lifetime_closed"`
}

// RedisMetrics represents Redis connection pool metrics
type RedisMetrics struct {
	Status     string `json:"status"`
	TotalConns uint32 `json:"total_conns"`
	IdleConns  uint32 `json:"idle_conns"`
	StaleConns uint32 `json:"stale_conns"`
	Hits       uint64 `json:"hits"`
	Misses     uint64 `json:"misses"`
	Timeouts   uint64 `json:"timeouts"`
}

// WebSocketMetrics represents WebSocket hub metrics
type WebSocketMetrics struct {
	Status           string    `json:"status"`
	ConnectedClients int       `json:"connected_clients"`
	ActiveChatRooms  int       `json:"active_chat_rooms"`
	MessagesSent     int64     `json:"messages_sent"`
	MessagesReceived int64     `json:"messages_received"`
	Uptime           time.Time `json:"uptime"`
}

// NotificationMetrics represents notification worker metrics
type NotificationMetrics struct {
	Status              string `json:"status"`
	MainQueueLength     int64  `json:"main_queue_length"`
	RetryQueueLength    int64  `json:"retry_queue_length"`
	ScheduledQueueLength int64 `json:"scheduled_queue_length"`
	DeadLetterQueueLength int64 `json:"dead_letter_queue_length"`
	ProcessingTasksCount int64 `json:"processing_tasks_count"`
	ActiveWorkersCount   int64 `json:"active_workers_count"`
}

// systemMetricsHandler aggregates metrics from all services
func systemMetricsHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	proxyConfig := getProxyConfig()

	logger.WithField("request_id", requestID).Info("Fetching system metrics")

	metrics := SystemMetrics{
		Timestamp: time.Now().UTC(),
		Services:  []ServiceHealth{},
	}

	// Get services health (reuse existing health check)
	services := []ServiceConfig{
		proxyConfig.UserService,
		proxyConfig.ChatService,
		proxyConfig.TaskService,
		proxyConfig.CalendarService,
		proxyConfig.PollService,
		proxyConfig.NotificationService,
		proxyConfig.FileService,
		proxyConfig.AnalyticsService,
		proxyConfig.BackupService,
	}

	for _, service := range services {
		metrics.Services = append(metrics.Services, checkServiceHealth(service))
	}

	// Fetch database metrics from user-service
	if dbMetrics := fetchDatabaseMetrics(proxyConfig.UserService.URL); dbMetrics != nil {
		metrics.Database = dbMetrics
	}

	// Fetch Redis metrics from user-service
	if redisMetrics := fetchRedisMetrics(proxyConfig.UserService.URL); redisMetrics != nil {
		metrics.Redis = redisMetrics
	}

	// Fetch WebSocket metrics from chat-service
	if wsMetrics := fetchWebSocketMetrics(proxyConfig.ChatService.URL); wsMetrics != nil {
		metrics.WebSocket = wsMetrics
	}

	// Fetch notification worker metrics from notification-service
	if notifMetrics := fetchNotificationMetrics(proxyConfig.NotificationService.URL); notifMetrics != nil {
		metrics.Notificator = notifMetrics
	}

	c.JSON(http.StatusOK, metrics)
}

// fetchDatabaseMetrics fetches database connection pool metrics from a service
func fetchDatabaseMetrics(serviceURL string) *DatabaseMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := serviceURL + "/internal/metrics/database"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithError(err).Warn("Failed to create database metrics request")
		return nil
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Warn("Failed to fetch database metrics")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var metrics DatabaseMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		logger.WithError(err).Warn("Failed to decode database metrics")
		return nil
	}

	return &metrics
}

// fetchRedisMetrics fetches Redis connection pool metrics from a service
func fetchRedisMetrics(serviceURL string) *RedisMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := serviceURL + "/internal/metrics/redis"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithError(err).Warn("Failed to create redis metrics request")
		return nil
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Warn("Failed to fetch redis metrics")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var metrics RedisMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		logger.WithError(err).Warn("Failed to decode redis metrics")
		return nil
	}

	return &metrics
}

// fetchWebSocketMetrics fetches WebSocket hub metrics from chat service
func fetchWebSocketMetrics(serviceURL string) *WebSocketMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := serviceURL + "/internal/metrics/websocket"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithError(err).Warn("Failed to create websocket metrics request")
		return nil
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Warn("Failed to fetch websocket metrics")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var metrics WebSocketMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		logger.WithError(err).Warn("Failed to decode websocket metrics")
		return nil
	}

	return &metrics
}

// fetchNotificationMetrics fetches notification worker metrics
func fetchNotificationMetrics(serviceURL string) *NotificationMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	url := serviceURL + "/internal/metrics/worker"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		logger.WithError(err).Warn("Failed to create notification metrics request")
		return nil
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.WithError(err).Warn("Failed to fetch notification metrics")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var metrics NotificationMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		logger.WithError(err).Warn("Failed to decode notification metrics")
		return nil
	}

	return &metrics
}
