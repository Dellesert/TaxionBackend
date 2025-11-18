// File: services/notification/handlers/metrics_handler.go
package handlers

import (
	"context"
	"net/http"
	"time"

	"tachyon-messenger/services/notification/worker"
	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/metrics"
	"tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles internal metrics endpoints
type MetricsHandler struct {
	worker       *worker.Worker
	db           *database.DB
	redisClient  *redis.Client
	workerConfig *worker.WorkerConfig
	serviceName  string
	startTime    time.Time
	httpMetrics  *metrics.HTTPMetrics
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(w *worker.Worker, db *database.DB, redisClient *redis.Client, workerConfig *worker.WorkerConfig, serviceName string, startTime time.Time) *MetricsHandler {
	return &MetricsHandler{
		worker:       w,
		db:           db,
		redisClient:  redisClient,
		workerConfig: workerConfig,
		serviceName:  serviceName,
		startTime:    startTime,
		httpMetrics:  metrics.NewHTTPMetrics(),
	}
}

// GetWorkerMetrics returns notification worker metrics
func (h *MetricsHandler) GetWorkerMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching worker metrics")

	// Get worker stats
	stats := h.worker.GetStats()

	// Create queue manager to get queue stats
	queueManager := worker.NewQueueManager(h.redisClient, h.workerConfig)
	queueStats, err := queueManager.GetQueueStats(context.Background())
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get queue stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get worker metrics",
		})
		return
	}

	response := gin.H{
		"status":                   "healthy",
		"main_queue_length":        queueStats.MainQueueLength,
		"retry_queue_length":       queueStats.RetryQueueLength,
		"scheduled_queue_length":   queueStats.ScheduledQueueLength,
		"dead_letter_queue_length": queueStats.DeadLetterQueueLength,
		"processing_tasks_count":   queueStats.ProcessingTasksCount,
		"active_workers_count":     queueStats.ActiveWorkersCount,
		"worker_is_running":        stats.IsRunning,
		"concurrent_workers":       stats.ConcurrentWorkers,
		"task_queue_size":          stats.TaskQueueSize,
	}

	c.JSON(http.StatusOK, response)
}

// GetDatabaseMetrics returns PostgreSQL connection pool metrics
func (h *MetricsHandler) GetDatabaseMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching database metrics")

	metrics, err := h.db.GetMetrics()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get database metrics")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get database metrics",
		})
		return
	}

	status := "healthy"
	if !metrics.IsHealthy {
		status = "unhealthy"
	}

	response := gin.H{
		"status":                   status,
		"max_open_connections":     metrics.MaxOpenConnections,
		"open_connections":         metrics.OpenConnections,
		"in_use":                   metrics.InUse,
		"idle":                     metrics.Idle,
		"wait_count":               metrics.WaitCount,
		"wait_duration":            metrics.WaitDuration.String(),
		"wait_duration_ms":         metrics.WaitDurationMs,
		"avg_wait_duration_ms":     metrics.AvgWaitDurationMs,
		"max_idle_closed":          metrics.MaxIdleClosed,
		"max_idle_time_closed":     metrics.MaxIdleTimeClosed,
		"max_lifetime_closed":      metrics.MaxLifetimeClosed,
		"total_connections_closed": metrics.TotalConnectionsClosed,
		"utilization_percent":      metrics.UtilizationPercent,
		"is_healthy":               metrics.IsHealthy,
		"timestamp":                metrics.Timestamp,
	}

	c.JSON(http.StatusOK, response)
}

// GetRedisMetrics returns Redis connection pool metrics
func (h *MetricsHandler) GetRedisMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching redis metrics")

	poolStats := h.redisClient.Stats()

	response := gin.H{
		"status":      "healthy",
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
	}

	c.JSON(http.StatusOK, response)
}

// GetRuntimeMetrics returns Go runtime metrics
func (h *MetricsHandler) GetRuntimeMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching runtime metrics")

	runtimeMetrics := metrics.GetRuntimeMetrics()
	httpSnapshot := h.httpMetrics.GetSnapshot()

	serviceMetrics := metrics.ServiceMetrics{
		ServiceName: h.serviceName,
		Runtime:     runtimeMetrics,
		HTTP:        httpSnapshot,
		Uptime:      time.Since(h.startTime).String(),
		StartTime:   h.startTime,
	}

	c.JSON(http.StatusOK, serviceMetrics)
}

// RecordRequest records an HTTP request for metrics
func (h *MetricsHandler) RecordRequest(statusCode int, latency time.Duration) {
	h.httpMetrics.RecordRequest(statusCode, latency)
}

// MetricsMiddleware creates middleware to track HTTP requests
func (h *MetricsHandler) MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		latency := time.Since(start)
		h.RecordRequest(c.Writer.Status(), latency)
	}
}
