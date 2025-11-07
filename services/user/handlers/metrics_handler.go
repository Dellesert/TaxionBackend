// File: services/user/handlers/metrics_handler.go
package handlers

import (
	"net/http"

	"tachyon-messenger/shared/database"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles internal metrics endpoints
type MetricsHandler struct {
	db    *database.DB
	redis *redis.Client
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(db *database.DB, redisClient *redis.Client) *MetricsHandler {
	return &MetricsHandler{
		db:    db,
		redis: redisClient,
	}
}

// GetDatabaseMetrics returns PostgreSQL connection pool metrics
func (h *MetricsHandler) GetDatabaseMetrics(c *gin.Context) {
	requestID := requestid.Get(c)

	logger.WithField("request_id", requestID).Debug("Fetching database metrics")

	// Use new GetMetrics function for detailed metrics
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

	// Determine status based on health
	status := "healthy"
	if !metrics.IsHealthy {
		status = "unhealthy"
	}

	// Return detailed metrics
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
