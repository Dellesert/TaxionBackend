// File: services/notification/worker/notification_worker.go
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tachyon-messenger/services/notification/models"
	"tachyon-messenger/services/notification/usecase"
	"tachyon-messenger/shared/logger"
	"tachyon-messenger/shared/redis"

	goredis "github.com/redis/go-redis/v9"
)

// NotificationTask represents a notification task to be processed
type NotificationTask struct {
	ID                    string                                `json:"id"`
	Type                  TaskType                              `json:"type"`
	Notification          *models.CreateNotificationRequest     `json:"notification,omitempty"`
	BulkNotification      *models.BulkCreateNotificationRequest `json:"bulk_notification,omitempty"`
	TemplatedNotification *usecase.TemplatedNotificationRequest `json:"templated_notification,omitempty"`
	SystemAnnouncement    *usecase.SystemAnnouncementRequest    `json:"system_announcement,omitempty"`
	Priority              models.NotificationPriority           `json:"priority"`
	CreatedAt             time.Time                             `json:"created_at"`
	ScheduledAt           *time.Time                            `json:"scheduled_at,omitempty"`
	AttemptCount          int                                   `json:"attempt_count"`
	LastError             string                                `json:"last_error,omitempty"`
	MaxRetries            int                                   `json:"max_retries"`
}

// TaskType represents the type of notification task
type TaskType string

const (
	TaskTypeSingle       TaskType = "single"       // Single notification
	TaskTypeBulk         TaskType = "bulk"         // Bulk notifications
	TaskTypeTemplated    TaskType = "templated"    // Templated notification
	TaskTypeAnnouncement TaskType = "announcement" // System announcement
	TaskTypeScheduled    TaskType = "scheduled"    // Scheduled notification
	TaskTypeRetry        TaskType = "retry"        // Retry failed notification
)

// Worker represents the notification worker
type Worker struct {
	id                string
	ctx               context.Context
	cancel            context.CancelFunc
	notificationUC    usecase.NotificationUsecase
	redisClient       *redis.Client
	taskChan          chan *NotificationTask
	retryTaskChan     chan *NotificationTask
	scheduledTaskChan chan *NotificationTask
	wg                sync.WaitGroup
	config            *WorkerConfig
	isRunning         bool
	mu                sync.RWMutex
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	WorkerID             string        `json:"worker_id"`
	ConcurrentWorkers    int           `json:"concurrent_workers"`
	TaskChannelSize      int           `json:"task_channel_size"`
	RetryChannelSize     int           `json:"retry_channel_size"`
	ScheduledChannelSize int           `json:"scheduled_channel_size"`
	RedisKeyPrefix       string        `json:"redis_key_prefix"`
	QueueName            string        `json:"queue_name"`
	RetryQueueName       string        `json:"retry_queue_name"`
	ScheduledQueueName   string        `json:"scheduled_queue_name"`
	ProcessingTimeout    time.Duration `json:"processing_timeout"`
	RetryDelay           time.Duration `json:"retry_delay"`
	MaxRetries           int           `json:"max_retries"`
	HealthCheckInterval  time.Duration `json:"health_check_interval"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
}

// DefaultWorkerConfig returns default worker configuration
func DefaultWorkerConfig() *WorkerConfig {
	hostname, _ := os.Hostname()
	return &WorkerConfig{
		WorkerID:             fmt.Sprintf("notification-worker-%s-%d", hostname, time.Now().Unix()),
		ConcurrentWorkers:    5,
		TaskChannelSize:      1000,
		RetryChannelSize:     500,
		ScheduledChannelSize: 500,
		RedisKeyPrefix:       "tachyon:notification",
		QueueName:            "notifications:queue",
		RetryQueueName:       "notifications:retry",
		ScheduledQueueName:   "notifications:scheduled",
		ProcessingTimeout:    30 * time.Second,
		RetryDelay:           30 * time.Second,
		MaxRetries:           3,
		HealthCheckInterval:  30 * time.Second,
		CleanupInterval:      5 * time.Minute,
	}
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	notificationUC usecase.NotificationUsecase,
	redisClient *redis.Client,
	config *WorkerConfig,
) *Worker {
	if config == nil {
		config = DefaultWorkerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		id:                config.WorkerID,
		ctx:               ctx,
		cancel:            cancel,
		notificationUC:    notificationUC,
		redisClient:       redisClient,
		taskChan:          make(chan *NotificationTask, config.TaskChannelSize),
		retryTaskChan:     make(chan *NotificationTask, config.RetryChannelSize),
		scheduledTaskChan: make(chan *NotificationTask, config.ScheduledChannelSize),
		config:            config,
		isRunning:         false,
	}
}

// Start starts the notification worker
func (w *Worker) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.isRunning {
		return fmt.Errorf("worker is already running")
	}

	logger.WithField("worker_id", w.id).Info("Starting notification worker")

	// Register worker in Redis
	if err := w.registerWorker(); err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}

	// Start background goroutines
	w.startBackgroundTasks()

	// Start worker goroutines
	for i := 0; i < w.config.ConcurrentWorkers; i++ {
		w.wg.Add(1)
		go w.taskProcessor(i + 1)
	}

	// Start retry processor
	w.wg.Add(1)
	go w.retryProcessor()

	// Start scheduled task processor
	w.wg.Add(1)
	go w.scheduledTaskProcessor()

	// Start queue consumers
	w.wg.Add(1)
	go w.queueConsumer()

	w.isRunning = true

	logger.WithField("worker_id", w.id).Info("Notification worker started successfully")
	return nil
}

// Stop stops the notification worker gracefully
func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isRunning {
		return nil
	}

	logger.WithField("worker_id", w.id).Info("Stopping notification worker")

	// Cancel context to signal all goroutines to stop
	w.cancel()

	// Close channels
	close(w.taskChan)
	close(w.retryTaskChan)
	close(w.scheduledTaskChan)

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.WithField("worker_id", w.id).Info("All worker goroutines stopped")
	case <-time.After(30 * time.Second):
		logger.WithField("worker_id", w.id).Warn("Worker shutdown timeout exceeded")
	}

	// Unregister worker from Redis
	if err := w.unregisterWorker(); err != nil {
		logger.WithFields(map[string]interface{}{
			"worker_id": w.id,
			"error":     err.Error(),
		}).Error("Failed to unregister worker")
	}

	w.isRunning = false
	logger.WithField("worker_id", w.id).Info("Notification worker stopped")
	return nil
}

// AddTask adds a notification task to the queue
func (w *Worker) AddTask(task *NotificationTask) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	// Set default values
	if task.ID == "" {
		task.ID = generateTaskID()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.MaxRetries == 0 {
		task.MaxRetries = w.config.MaxRetries
	}

	// Serialize task
	taskData, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to serialize task: %w", err)
	}

	// Determine which queue to use
	queueName := w.config.QueueName
	if task.ScheduledAt != nil && task.ScheduledAt.After(time.Now()) {
		queueName = w.config.ScheduledQueueName
	}

	// Add to Redis queue
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	if err := w.redisClient.LPush(ctx, queueName, taskData).Err(); err != nil {
		return fmt.Errorf("failed to add task to queue: %w", err)
	}

	logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"task_type": task.Type,
		"queue":     queueName,
	}).Info("Task added to queue")

	return nil
}

// ProcessNotification processes a single notification task
func (w *Worker) ProcessNotification(task *NotificationTask) error {
	startTime := time.Now()

	logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"task_type":   task.Type,
		"attempt":     task.AttemptCount + 1,
		"max_retries": task.MaxRetries,
	}).Info("Processing notification task")

	var err error

	switch task.Type {
	case TaskTypeSingle:
		if task.Notification == nil {
			return fmt.Errorf("notification data is required for single task")
		}
		_, err = w.notificationUC.SendNotification(task.Notification)

	case TaskTypeBulk:
		if task.BulkNotification == nil {
			return fmt.Errorf("bulk notification data is required for bulk task")
		}
		err = w.notificationUC.SendBulkNotification(task.BulkNotification)

	case TaskTypeTemplated:
		if task.TemplatedNotification == nil {
			return fmt.Errorf("templated notification data is required for templated task")
		}
		_, err = w.notificationUC.SendTemplatedNotification(task.TemplatedNotification)

	case TaskTypeAnnouncement:
		if task.SystemAnnouncement == nil {
			return fmt.Errorf("system announcement data is required for announcement task")
		}
		err = w.notificationUC.SendSystemAnnouncement(task.SystemAnnouncement)

	case TaskTypeScheduled:
		// Process scheduled task based on its original type
		if task.Notification != nil {
			_, err = w.notificationUC.SendNotification(task.Notification)
		} else if task.BulkNotification != nil {
			err = w.notificationUC.SendBulkNotification(task.BulkNotification)
		} else {
			return fmt.Errorf("no valid notification data for scheduled task")
		}

	case TaskTypeRetry:
		// Same processing as original task type
		return w.ProcessNotification(&NotificationTask{
			ID:                    task.ID,
			Type:                  TaskTypeSingle, // Assume single for retry
			Notification:          task.Notification,
			BulkNotification:      task.BulkNotification,
			TemplatedNotification: task.TemplatedNotification,
			SystemAnnouncement:    task.SystemAnnouncement,
			Priority:              task.Priority,
			CreatedAt:             task.CreatedAt,
			AttemptCount:          task.AttemptCount,
			MaxRetries:            task.MaxRetries,
		})

	default:
		return fmt.Errorf("unknown task type: %s", task.Type)
	}

	duration := time.Since(startTime)

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id":     task.ID,
			"task_type":   task.Type,
			"attempt":     task.AttemptCount + 1,
			"duration_ms": duration.Milliseconds(),
			"error":       err.Error(),
		}).Error("Failed to process notification task")

		return err
	}

	logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"task_type":   task.Type,
		"attempt":     task.AttemptCount + 1,
		"duration_ms": duration.Milliseconds(),
	}).Info("Successfully processed notification task")

	return nil
}

// taskProcessor processes tasks from the task channel
func (w *Worker) taskProcessor(workerNum int) {
	defer w.wg.Done()

	logger.WithFields(map[string]interface{}{
		"worker_id":  w.id,
		"worker_num": workerNum,
	}).Info("Task processor started")

	for {
		select {
		case task, ok := <-w.taskChan:
			if !ok {
				logger.WithField("worker_num", workerNum).Info("Task channel closed, stopping processor")
				return
			}

			w.processTaskWithTimeout(task)

		case <-w.ctx.Done():
			logger.WithField("worker_num", workerNum).Info("Context cancelled, stopping processor")
			return
		}
	}
}

// retryProcessor processes retry tasks
func (w *Worker) retryProcessor() {
	defer w.wg.Done()

	logger.WithField("worker_id", w.id).Info("Retry processor started")

	for {
		select {
		case task, ok := <-w.retryTaskChan:
			if !ok {
				logger.Info("Retry channel closed, stopping retry processor")
				return
			}

			// Wait for retry delay
			if task.AttemptCount > 0 {
				delay := w.calculateRetryDelay(task.AttemptCount)
				logger.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"delay":   delay,
					"attempt": task.AttemptCount + 1,
				}).Info("Waiting before retry")

				select {
				case <-time.After(delay):
					// Continue with retry
				case <-w.ctx.Done():
					return
				}
			}

			w.processTaskWithTimeout(task)

		case <-w.ctx.Done():
			logger.Info("Context cancelled, stopping retry processor")
			return
		}
	}
}

// scheduledTaskProcessor processes scheduled tasks
func (w *Worker) scheduledTaskProcessor() {
	defer w.wg.Done()

	logger.WithField("worker_id", w.id).Info("Scheduled task processor started")

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.processScheduledTasks()

		case task, ok := <-w.scheduledTaskChan:
			if !ok {
				logger.Info("Scheduled channel closed, stopping scheduled processor")
				return
			}

			// Check if it's time to process the task
			if task.ScheduledAt != nil && task.ScheduledAt.After(time.Now()) {
				// Not yet time, put it back in the scheduled queue
				w.addToScheduledQueue(task)
				continue
			}

			// Process immediately
			w.processTaskWithTimeout(task)

		case <-w.ctx.Done():
			logger.Info("Context cancelled, stopping scheduled processor")
			return
		}
	}
}

// queueConsumer consumes tasks from Redis queues
func (w *Worker) queueConsumer() {
	defer w.wg.Done()

	logger.WithField("worker_id", w.id).Info("Queue consumer started")

	for {
		select {
		case <-w.ctx.Done():
			logger.Info("Context cancelled, stopping queue consumer")
			return

		default:
			// Try to get task from main queue
			if task := w.consumeFromQueue(w.config.QueueName); task != nil {
				select {
				case w.taskChan <- task:
					// Task sent to processing channel
				case <-w.ctx.Done():
					return
				default:
					// Channel is full, put task back to queue
					w.addToQueue(w.config.QueueName, task)
				}
				continue
			}

			// Try to get task from retry queue
			if task := w.consumeFromQueue(w.config.RetryQueueName); task != nil {
				select {
				case w.retryTaskChan <- task:
					// Task sent to retry channel
				case <-w.ctx.Done():
					return
				default:
					// Channel is full, put task back to queue
					w.addToQueue(w.config.RetryQueueName, task)
				}
				continue
			}

			// No tasks available, wait a bit
			time.Sleep(1 * time.Second)
		}
	}
}

// processTaskWithTimeout processes a task with timeout
func (w *Worker) processTaskWithTimeout(task *NotificationTask) {
	ctx, cancel := context.WithTimeout(w.ctx, w.config.ProcessingTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- w.ProcessNotification(task)
	}()

	select {
	case err := <-done:
		if err != nil {
			w.handleTaskError(task, err)
		} else {
			w.handleTaskSuccess(task)
		}

	case <-ctx.Done():
		err := fmt.Errorf("task processing timeout")
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"timeout": w.config.ProcessingTimeout,
		}).Error("Task processing timed out")

		w.handleTaskError(task, err)
	}
}

// handleTaskSuccess handles successful task completion
func (w *Worker) handleTaskSuccess(task *NotificationTask) {
	logger.WithFields(map[string]interface{}{
		"task_id":   task.ID,
		"task_type": task.Type,
		"attempts":  task.AttemptCount + 1,
	}).Info("Task completed successfully")

	// Remove from processing set
	w.removeFromProcessingSet(task.ID)
}

// handleTaskError handles task processing errors
func (w *Worker) handleTaskError(task *NotificationTask, err error) {
	task.AttemptCount++
	task.LastError = err.Error()

	if task.AttemptCount >= task.MaxRetries {
		logger.WithFields(map[string]interface{}{
			"task_id":     task.ID,
			"task_type":   task.Type,
			"attempts":    task.AttemptCount,
			"max_retries": task.MaxRetries,
			"error":       err.Error(),
		}).Error("Task failed permanently after max retries")

		// Move to dead letter queue
		w.addToDeadLetterQueue(task)
		w.removeFromProcessingSet(task.ID)
		return
	}

	logger.WithFields(map[string]interface{}{
		"task_id":     task.ID,
		"task_type":   task.Type,
		"attempts":    task.AttemptCount,
		"max_retries": task.MaxRetries,
		"error":       err.Error(),
	}).Warn("Task failed, scheduling retry")

	// Add to retry queue
	task.Type = TaskTypeRetry
	w.addToQueue(w.config.RetryQueueName, task)
	w.removeFromProcessingSet(task.ID)
}

// consumeFromQueue consumes a task from Redis queue
func (w *Worker) consumeFromQueue(queueName string) *NotificationTask {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	result, err := w.redisClient.BRPop(ctx, 1*time.Second, queueName).Result()
	if err != nil {
		if err != goredis.Nil {
			logger.WithFields(map[string]interface{}{
				"queue": queueName,
				"error": err.Error(),
			}).Error("Failed to consume from queue")
		}
		return nil
	}

	if len(result) < 2 {
		return nil
	}

	var task NotificationTask
	if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
		logger.WithFields(map[string]interface{}{
			"queue": queueName,
			"error": err.Error(),
			"data":  result[1],
		}).Error("Failed to unmarshal task")
		return nil
	}

	// Add to processing set for tracking
	w.addToProcessingSet(&task)

	return &task
}

// addToQueue adds a task to Redis queue
func (w *Worker) addToQueue(queueName string, task *NotificationTask) {
	taskData, err := json.Marshal(task)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		}).Error("Failed to marshal task for queue")
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	if err := w.redisClient.LPush(ctx, queueName, taskData).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"queue":   queueName,
			"error":   err.Error(),
		}).Error("Failed to add task to queue")
	}
}

// addToScheduledQueue adds a task to scheduled queue with delay
func (w *Worker) addToScheduledQueue(task *NotificationTask) {
	if task.ScheduledAt == nil {
		w.addToQueue(w.config.QueueName, task)
		return
	}

	taskData, err := json.Marshal(task)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		}).Error("Failed to marshal scheduled task")
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	score := float64(task.ScheduledAt.Unix())
	if err := w.redisClient.ZAdd(ctx, w.config.ScheduledQueueName, goredis.Z{
		Score:  score,
		Member: taskData,
	}).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id":      task.ID,
			"scheduled_at": task.ScheduledAt,
			"error":        err.Error(),
		}).Error("Failed to add scheduled task")
	}
}

// addToDeadLetterQueue adds a failed task to dead letter queue
func (w *Worker) addToDeadLetterQueue(task *NotificationTask) {
	deadLetterQueue := w.config.RedisKeyPrefix + ":dead_letter"
	taskData, err := json.Marshal(task)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		}).Error("Failed to marshal dead letter task")
		return
	}

	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	if err := w.redisClient.LPush(ctx, deadLetterQueue, taskData).Err(); err != nil {
		logger.WithFields(map[string]interface{}{
			"task_id": task.ID,
			"error":   err.Error(),
		}).Error("Failed to add task to dead letter queue")
	}
}

// processScheduledTasks processes tasks that are ready to be executed
func (w *Worker) processScheduledTasks() {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	now := time.Now().Unix()
	result, err := w.redisClient.ZRangeByScore(ctx, w.config.ScheduledQueueName, &goredis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", now),
		Offset: 0,
		Count:  100, // Process up to 100 scheduled tasks at once
	}).Result()

	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to get scheduled tasks")
		return
	}

	for _, taskData := range result {
		var task NotificationTask
		if err := json.Unmarshal([]byte(taskData), &task); err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
				"data":  taskData,
			}).Error("Failed to unmarshal scheduled task")
			continue
		}

		// Remove from scheduled queue
		w.redisClient.ZRem(ctx, w.config.ScheduledQueueName, taskData)

		// Add to main processing queue
		task.Type = TaskTypeScheduled
		select {
		case w.scheduledTaskChan <- &task:
			// Task sent to scheduled channel
		default:
			// Channel is full, add back to main queue
			w.addToQueue(w.config.QueueName, &task)
		}
	}

	if len(result) > 0 {
		logger.WithFields(map[string]interface{}{
			"count": len(result),
		}).Info("Processed scheduled tasks")
	}
}

// addToProcessingSet adds task to processing set for tracking
func (w *Worker) addToProcessingSet(task *NotificationTask) {
	processingKey := w.config.RedisKeyPrefix + ":processing"
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	taskInfo := map[string]interface{}{
		"id":         task.ID,
		"type":       task.Type,
		"worker_id":  w.id,
		"started_at": time.Now().Unix(),
	}

	taskData, _ := json.Marshal(taskInfo)
	w.redisClient.HSet(ctx, processingKey, task.ID, taskData)
	w.redisClient.Expire(ctx, processingKey, w.config.ProcessingTimeout*2)
}

// removeFromProcessingSet removes task from processing set
func (w *Worker) removeFromProcessingSet(taskID string) {
	processingKey := w.config.RedisKeyPrefix + ":processing"
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	w.redisClient.HDel(ctx, processingKey, taskID)
}

// registerWorker registers worker in Redis
func (w *Worker) registerWorker() error {
	workerKey := w.config.RedisKeyPrefix + ":workers"
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	workerInfo := map[string]interface{}{
		"id":                 w.id,
		"started_at":         time.Now().Unix(),
		"concurrent_workers": w.config.ConcurrentWorkers,
		"status":             "running",
	}

	workerData, err := json.Marshal(workerInfo)
	if err != nil {
		return err
	}

	return w.redisClient.HSet(ctx, workerKey, w.id, workerData).Err()
}

// unregisterWorker unregisters worker from Redis
func (w *Worker) unregisterWorker() error {
	workerKey := w.config.RedisKeyPrefix + ":workers"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return w.redisClient.HDel(ctx, workerKey, w.id).Err()
}

// startBackgroundTasks starts background maintenance tasks
func (w *Worker) startBackgroundTasks() {
	// Health check task
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.updateWorkerHealth()
			case <-w.ctx.Done():
				return
			}
		}
	}()

	// Cleanup task
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.cleanupOldTasks()
			case <-w.ctx.Done():
				return
			}
		}
	}()
}

// updateWorkerHealth updates worker health status in Redis
func (w *Worker) updateWorkerHealth() {
	workerKey := w.config.RedisKeyPrefix + ":workers"
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	workerInfo := map[string]interface{}{
		"id":                   w.id,
		"last_heartbeat":       time.Now().Unix(),
		"concurrent_workers":   w.config.ConcurrentWorkers,
		"status":               "running",
		"task_queue_size":      len(w.taskChan),
		"retry_queue_size":     len(w.retryTaskChan),
		"scheduled_queue_size": len(w.scheduledTaskChan),
	}

	workerData, _ := json.Marshal(workerInfo)
	w.redisClient.HSet(ctx, workerKey, w.id, workerData)
}

// cleanupOldTasks cleans up old completed and failed tasks
func (w *Worker) cleanupOldTasks() {
	// Clean up processing set (remove stuck tasks)
	processingKey := w.config.RedisKeyPrefix + ":processing"
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	// Get all processing tasks
	result, err := w.redisClient.HGetAll(ctx, processingKey).Result()
	if err != nil {
		logger.WithField("error", err.Error()).Error("Failed to get processing tasks for cleanup")
		return
	}

	now := time.Now().Unix()
	stuckTasks := []string{}

	for taskID, taskDataStr := range result {
		var taskInfo map[string]interface{}
		if err := json.Unmarshal([]byte(taskDataStr), &taskInfo); err != nil {
			continue
		}

		startedAt, ok := taskInfo["started_at"].(float64)
		if !ok {
			continue
		}

		// If task has been processing for more than timeout * 2, consider it stuck
		if now-int64(startedAt) > int64(w.config.ProcessingTimeout.Seconds()*2) {
			stuckTasks = append(stuckTasks, taskID)
		}
	}

	// Remove stuck tasks
	if len(stuckTasks) > 0 {
		w.redisClient.HDel(ctx, processingKey, stuckTasks...)
		logger.WithFields(map[string]interface{}{
			"stuck_tasks": len(stuckTasks),
		}).Info("Cleaned up stuck tasks")
	}

	// Clean up old dead letter tasks (older than 7 days)
	deadLetterQueue := w.config.RedisKeyPrefix + ":dead_letter"
	deadLetterTasks, err := w.redisClient.LRange(ctx, deadLetterQueue, 0, -1).Result()
	if err != nil {
		return
	}

	cutoffTime := time.Now().Add(-7 * 24 * time.Hour)
	tasksToRemove := 0

	for _, taskDataStr := range deadLetterTasks {
		var task NotificationTask
		if err := json.Unmarshal([]byte(taskDataStr), &task); err != nil {
			continue
		}

		if task.CreatedAt.Before(cutoffTime) {
			tasksToRemove++
		} else {
			break // Tasks are ordered by creation time
		}
	}

	if tasksToRemove > 0 {
		w.redisClient.LTrim(ctx, deadLetterQueue, int64(tasksToRemove), -1)
		logger.WithFields(map[string]interface{}{
			"removed_tasks": tasksToRemove,
		}).Info("Cleaned up old dead letter tasks")
	}
}

// calculateRetryDelay calculates retry delay with exponential backoff
func (w *Worker) calculateRetryDelay(attempt int) time.Duration {
	baseDelay := w.config.RetryDelay

	// Exponential backoff: baseDelay * 2^(attempt-1)
	multiplier := 1 << (attempt - 1) // 2^(attempt-1)
	if multiplier > 16 {
		multiplier = 16 // Cap at 16x base delay
	}

	delay := time.Duration(multiplier) * baseDelay

	// Add jitter (±25%)
	jitter := time.Duration(float64(delay) * 0.25 * (2*float64(time.Now().UnixNano()%1000)/1000 - 1))
	delay += jitter

	// Ensure minimum delay of 1 second
	if delay < time.Second {
		delay = time.Second
	}

	// Cap maximum delay at 10 minutes
	if delay > 10*time.Minute {
		delay = 10 * time.Minute
	}

	return delay
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%d", time.Now().UnixNano(), time.Now().UnixNano()%1000)
}

// IsRunning returns whether the worker is currently running
func (w *Worker) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isRunning
}

// GetStats returns worker statistics
func (w *Worker) GetStats() WorkerStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WorkerStats{
		WorkerID:           w.id,
		IsRunning:          w.isRunning,
		TaskQueueSize:      len(w.taskChan),
		RetryQueueSize:     len(w.retryTaskChan),
		ScheduledQueueSize: len(w.scheduledTaskChan),
		ConcurrentWorkers:  w.config.ConcurrentWorkers,
	}
}

// WorkerStats represents worker statistics
type WorkerStats struct {
	WorkerID           string `json:"worker_id"`
	IsRunning          bool   `json:"is_running"`
	TaskQueueSize      int    `json:"task_queue_size"`
	RetryQueueSize     int    `json:"retry_queue_size"`
	ScheduledQueueSize int    `json:"scheduled_queue_size"`
	ConcurrentWorkers  int    `json:"concurrent_workers"`
}

// GracefulShutdown handles graceful shutdown with signal handling
func (w *Worker) GracefulShutdown() {
	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		logger.WithFields(map[string]interface{}{
			"worker_id": w.id,
			"signal":    sig.String(),
		}).Info("Received shutdown signal")

		if err := w.Stop(); err != nil {
			logger.WithFields(map[string]interface{}{
				"worker_id": w.id,
				"error":     err.Error(),
			}).Error("Error during worker shutdown")
		}
	}()
}

// QueueManager provides methods to interact with the notification queues
type QueueManager struct {
	redisClient *redis.Client
	config      *WorkerConfig
}

// NewQueueManager creates a new queue manager
func NewQueueManager(redisClient *redis.Client, config *WorkerConfig) *QueueManager {
	return &QueueManager{
		redisClient: redisClient,
		config:      config,
	}
}

// GetQueueStats returns statistics about the queues
func (qm *QueueManager) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	stats := &QueueStats{}

	// Get main queue length
	mainQueueLen, err := qm.redisClient.LLen(ctx, qm.config.QueueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get main queue length: %w", err)
	}
	stats.MainQueueLength = mainQueueLen

	// Get retry queue length
	retryQueueLen, err := qm.redisClient.LLen(ctx, qm.config.RetryQueueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get retry queue length: %w", err)
	}
	stats.RetryQueueLength = retryQueueLen

	// Get scheduled queue length
	scheduledQueueLen, err := qm.redisClient.ZCard(ctx, qm.config.ScheduledQueueName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled queue length: %w", err)
	}
	stats.ScheduledQueueLength = scheduledQueueLen

	// Get dead letter queue length
	deadLetterQueue := qm.config.RedisKeyPrefix + ":dead_letter"
	deadLetterLen, err := qm.redisClient.LLen(ctx, deadLetterQueue).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get dead letter queue length: %w", err)
	}
	stats.DeadLetterQueueLength = deadLetterLen

	// Get processing tasks count
	processingKey := qm.config.RedisKeyPrefix + ":processing"
	processingLen, err := qm.redisClient.HLen(ctx, processingKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get processing tasks count: %w", err)
	}
	stats.ProcessingTasksCount = processingLen

	// Get active workers count
	workersKey := qm.config.RedisKeyPrefix + ":workers"
	workersLen, err := qm.redisClient.HLen(ctx, workersKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get workers count: %w", err)
	}
	stats.ActiveWorkersCount = workersLen

	return stats, nil
}

// PurgeQueues removes all tasks from queues (use with caution)
func (qm *QueueManager) PurgeQueues(ctx context.Context) error {
	queues := []string{
		qm.config.QueueName,
		qm.config.RetryQueueName,
		qm.config.ScheduledQueueName,
		qm.config.RedisKeyPrefix + ":dead_letter",
	}

	for _, queue := range queues {
		if err := qm.redisClient.Del(ctx, queue).Err(); err != nil {
			return fmt.Errorf("failed to purge queue %s: %w", queue, err)
		}
	}

	logger.Info("All notification queues purged")
	return nil
}

// RequeueDeadLetterTasks moves tasks from dead letter queue back to main queue
func (qm *QueueManager) RequeueDeadLetterTasks(ctx context.Context, limit int) (int, error) {
	deadLetterQueue := qm.config.RedisKeyPrefix + ":dead_letter"

	requeued := 0
	for i := 0; i < limit; i++ {
		taskData, err := qm.redisClient.RPop(ctx, deadLetterQueue).Result()
		if err != nil {
			if err == goredis.Nil {
				break // No more tasks
			}
			return requeued, fmt.Errorf("failed to get task from dead letter queue: %w", err)
		}

		// Reset attempt count and error
		var task NotificationTask
		if err := json.Unmarshal([]byte(taskData), &task); err != nil {
			logger.WithField("error", err.Error()).Error("Failed to unmarshal dead letter task")
			continue
		}

		task.AttemptCount = 0
		task.LastError = ""
		task.Type = TaskTypeSingle // Reset to single type

		// Add back to main queue
		if newTaskData, err := json.Marshal(task); err == nil {
			if err := qm.redisClient.LPush(ctx, qm.config.QueueName, newTaskData).Err(); err != nil {
				logger.WithFields(map[string]interface{}{
					"task_id": task.ID,
					"error":   err.Error(),
				}).Error("Failed to requeue dead letter task")
				continue
			}
			requeued++
		}
	}

	if requeued > 0 {
		logger.WithField("requeued_count", requeued).Info("Requeued dead letter tasks")
	}

	return requeued, nil
}

// QueueStats represents queue statistics
type QueueStats struct {
	MainQueueLength       int64 `json:"main_queue_length"`
	RetryQueueLength      int64 `json:"retry_queue_length"`
	ScheduledQueueLength  int64 `json:"scheduled_queue_length"`
	DeadLetterQueueLength int64 `json:"dead_letter_queue_length"`
	ProcessingTasksCount  int64 `json:"processing_tasks_count"`
	ActiveWorkersCount    int64 `json:"active_workers_count"`
}

// Helper functions for creating different types of tasks

// CreateSingleNotificationTask creates a task for single notification
func CreateSingleNotificationTask(req *models.CreateNotificationRequest, priority models.NotificationPriority) *NotificationTask {
	return &NotificationTask{
		ID:           generateTaskID(),
		Type:         TaskTypeSingle,
		Notification: req,
		Priority:     priority,
		CreatedAt:    time.Now(),
		MaxRetries:   3,
	}
}

// CreateBulkNotificationTask creates a task for bulk notifications
func CreateBulkNotificationTask(req *models.BulkCreateNotificationRequest, priority models.NotificationPriority) *NotificationTask {
	return &NotificationTask{
		ID:               generateTaskID(),
		Type:             TaskTypeBulk,
		BulkNotification: req,
		Priority:         priority,
		CreatedAt:        time.Now(),
		MaxRetries:       3,
	}
}

// CreateTemplatedNotificationTask creates a task for templated notification
func CreateTemplatedNotificationTask(req *usecase.TemplatedNotificationRequest, priority models.NotificationPriority) *NotificationTask {
	return &NotificationTask{
		ID:                    generateTaskID(),
		Type:                  TaskTypeTemplated,
		TemplatedNotification: req,
		Priority:              priority,
		CreatedAt:             time.Now(),
		MaxRetries:            3,
	}
}

// CreateScheduledNotificationTask creates a task for scheduled notification
func CreateScheduledNotificationTask(req *models.CreateNotificationRequest, scheduledAt time.Time, priority models.NotificationPriority) *NotificationTask {
	return &NotificationTask{
		ID:           generateTaskID(),
		Type:         TaskTypeScheduled,
		Notification: req,
		Priority:     priority,
		CreatedAt:    time.Now(),
		ScheduledAt:  &scheduledAt,
		MaxRetries:   3,
	}
}

// CreateSystemAnnouncementTask creates a task for system announcement
func CreateSystemAnnouncementTask(req *usecase.SystemAnnouncementRequest, priority models.NotificationPriority) *NotificationTask {
	return &NotificationTask{
		ID:                 generateTaskID(),
		Type:               TaskTypeAnnouncement,
		SystemAnnouncement: req,
		Priority:           priority,
		CreatedAt:          time.Now(),
		MaxRetries:         3,
	}
}
