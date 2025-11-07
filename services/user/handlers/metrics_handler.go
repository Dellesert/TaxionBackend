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

	stats, err := h.db.Stats()
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
		}).Error("Failed to get database stats")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get database metrics",
		})
		return
	}

	// Transform to consistent format
	response := gin.H{
		"status":                "healthy",
		"max_open_connections":  stats["max_open_connections"],
		"open_connections":      stats["open_connections"],
		"in_use":                stats["in_use"],
		"idle":                  stats["idle"],
		"wait_count":            stats["wait_count"],
		"wait_duration":         stats["wait_duration"],
		"max_idle_closed":       stats["max_idle_closed"],
		"max_lifetime_closed":   stats["max_lifetime_closed"],
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
