// File: services/notification/handlers/metrics_handler.go
package handlers

import (
	"context"
	"net/http"

	"tachyon-messenger/services/notification/worker"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// MetricsHandler handles internal metrics endpoints
type MetricsHandler struct {
	worker       *worker.Worker
	redisClient  *redis.Client
	workerConfig *worker.WorkerConfig
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(w *worker.Worker, redisClient *redis.Client, workerConfig *worker.WorkerConfig) *MetricsHandler {
	return &MetricsHandler{
		worker:       w,
		redisClient:  redisClient,
		workerConfig: workerConfig,
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
