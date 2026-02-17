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

	dbMetrics, err := h.db.GetMetrics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get database metrics"})
		return
	}

	status := "healthy"
	if !dbMetrics.IsHealthy {
		status = "unhealthy"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":                   status,
		"max_open_connections":     dbMetrics.MaxOpenConnections,
		"open_connections":         dbMetrics.OpenConnections,
		"in_use":                   dbMetrics.InUse,
		"idle":                     dbMetrics.Idle,
		"wait_count":               dbMetrics.WaitCount,
		"wait_duration":            dbMetrics.WaitDuration.String(),
		"wait_duration_ms":         dbMetrics.WaitDurationMs,
		"avg_wait_duration_ms":     dbMetrics.AvgWaitDurationMs,
		"max_idle_closed":          dbMetrics.MaxIdleClosed,
		"max_idle_time_closed":     dbMetrics.MaxIdleTimeClosed,
		"max_lifetime_closed":      dbMetrics.MaxLifetimeClosed,
		"total_connections_closed": dbMetrics.TotalConnectionsClosed,
		"utilization_percent":      dbMetrics.UtilizationPercent,
		"is_healthy":               dbMetrics.IsHealthy,
		"timestamp":                dbMetrics.Timestamp,
	})
}

// GetRedisMetrics returns Redis connection pool metrics
func (h *MetricsHandler) GetRedisMetrics(c *gin.Context) {
	poolStats := h.redis.Stats()

	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
	})
}

// GetRuntimeMetrics returns Go runtime metrics
func (h *MetricsHandler) GetRuntimeMetrics(c *gin.Context) {
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
		c.Next()
		latency := time.Since(start)
		h.RecordRequest(c.Writer.Status(), latency)
	}
}
