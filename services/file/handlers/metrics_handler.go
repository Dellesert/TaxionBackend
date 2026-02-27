package handlers

import (
	"net/http"
	"time"

	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/metrics"
	"tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles internal metrics endpoints
type MetricsHandler struct {
	db          *database.DB
	redis       *redis.Client
	serviceName string
	startTime   time.Time
	httpMetrics *metrics.HTTPMetrics
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(db *database.DB, redisClient *redis.Client, serviceName string, startTime time.Time) *MetricsHandler {
	return &MetricsHandler{
		db:          db,
		redis:       redisClient,
		serviceName: serviceName,
		startTime:   startTime,
		httpMetrics: metrics.NewHTTPMetrics(),
	}
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
			"error": "Не удалось получить метрики базы данных",
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

	poolStats := h.redis.Stats()

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
