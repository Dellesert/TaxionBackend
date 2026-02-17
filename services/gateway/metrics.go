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
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// VPSMetrics represents host system metrics (CPU, RAM, Disk)
type VPSMetrics struct {
	// CPU metrics
	CPUUsagePercent float64   `json:"cpu_usage_percent"`
	CPUCores        int       `json:"cpu_cores"`
	CPUModelName    string    `json:"cpu_model_name"`
	LoadAvg1        float64   `json:"load_avg_1"`
	LoadAvg5        float64   `json:"load_avg_5"`
	LoadAvg15       float64   `json:"load_avg_15"`

	// Memory metrics
	MemoryTotal       uint64  `json:"memory_total"`
	MemoryUsed        uint64  `json:"memory_used"`
	MemoryFree        uint64  `json:"memory_free"`
	MemoryUsedPercent float64 `json:"memory_used_percent"`
	SwapTotal         uint64  `json:"swap_total"`
	SwapUsed          uint64  `json:"swap_used"`
	SwapUsedPercent   float64 `json:"swap_used_percent"`

	// Disk metrics
	DiskTotal       uint64  `json:"disk_total"`
	DiskUsed        uint64  `json:"disk_used"`
	DiskFree        uint64  `json:"disk_free"`
	DiskUsedPercent float64 `json:"disk_used_percent"`

	// Host info
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	Uptime          uint64 `json:"uptime"`
}

// SystemMetrics represents aggregated system metrics
type SystemMetrics struct {
	Timestamp   time.Time              `json:"timestamp"`
	Services    []ServiceHealth        `json:"services"`
	Database    *DatabaseMetrics       `json:"database,omitempty"`
	Redis       *RedisMetrics          `json:"redis,omitempty"`
	WebSocket   *WebSocketMetrics      `json:"websocket,omitempty"`
	Notificator *NotificationMetrics   `json:"notificator,omitempty"`
	VPS         *VPSMetrics            `json:"vps,omitempty"`
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

	// Collect VPS system metrics
	if vpsMetrics := collectVPSMetrics(); vpsMetrics != nil {
		metrics.VPS = vpsMetrics
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

// collectVPSMetrics collects host system metrics (CPU, RAM, Disk)
func collectVPSMetrics() *VPSMetrics {
	vps := &VPSMetrics{}

	// CPU metrics
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		vps.CPUUsagePercent = cpuPercent[0]
	}

	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		vps.CPUModelName = cpuInfo[0].ModelName
		vps.CPUCores = int(cpuInfo[0].Cores)
	}

	// Load average
	loadAvg, err := load.Avg()
	if err == nil {
		vps.LoadAvg1 = loadAvg.Load1
		vps.LoadAvg5 = loadAvg.Load5
		vps.LoadAvg15 = loadAvg.Load15
	}

	// Memory metrics
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		vps.MemoryTotal = memInfo.Total
		vps.MemoryUsed = memInfo.Used
		vps.MemoryFree = memInfo.Free
		vps.MemoryUsedPercent = memInfo.UsedPercent
	}

	// Swap metrics
	swapInfo, err := mem.SwapMemory()
	if err == nil {
		vps.SwapTotal = swapInfo.Total
		vps.SwapUsed = swapInfo.Used
		vps.SwapUsedPercent = swapInfo.UsedPercent
	}

	// Disk metrics (root partition)
	diskInfo, err := disk.Usage("/")
	if err == nil {
		vps.DiskTotal = diskInfo.Total
		vps.DiskUsed = diskInfo.Used
		vps.DiskFree = diskInfo.Free
		vps.DiskUsedPercent = diskInfo.UsedPercent
	}

	// Host info
	hostInfo, err := host.Info()
	if err == nil {
		vps.Hostname = hostInfo.Hostname
		vps.OS = hostInfo.OS
		vps.Platform = hostInfo.Platform
		vps.PlatformVersion = hostInfo.PlatformVersion
		vps.KernelVersion = hostInfo.KernelVersion
		vps.Uptime = hostInfo.Uptime
	}

	return vps
}

// VPSMetricsHistoryPoint represents a single point in metrics history
type VPSMetricsHistoryPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	DiskUsedPercent   float64   `json:"disk_used_percent"`
	LoadAvg1          float64   `json:"load_avg_1"`
}

// VPSMetricsHistory represents metrics history for different periods
type VPSMetricsHistory struct {
	Current *VPSMetrics              `json:"current"`
	Today   []VPSMetricsHistoryPoint `json:"today"`
	Week    []VPSMetricsHistoryPoint `json:"week"`
	Month   []VPSMetricsHistoryPoint `json:"month"`
}

const (
	// Redis keys for VPS metrics history
	vpsMetricsTodayKey = "vps:metrics:today"   // 5-minute intervals, last 24 hours (288 points)
	vpsMetricsWeekKey  = "vps:metrics:week"    // Hourly intervals, last 7 days (168 points)
	vpsMetricsMonthKey = "vps:metrics:month"   // Daily intervals, last 30 days (30 points)
)

// startVPSMetricsCollector starts a background worker to collect VPS metrics
func startVPSMetricsCollector(ctx context.Context) {
	logger.Info("Starting VPS metrics collector...")

	// Collect metrics immediately on startup
	collectAndStoreVPSMetrics(ctx)

	// Ticker for collecting metrics every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Ticker for aggregating hourly data (every hour)
	hourlyTicker := time.NewTicker(1 * time.Hour)
	defer hourlyTicker.Stop()

	// Ticker for aggregating daily data (every day at midnight)
	dailyTicker := time.NewTicker(24 * time.Hour)
	defer dailyTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("VPS metrics collector stopped")
			return
		case <-ticker.C:
			collectAndStoreVPSMetrics(ctx)
		case <-hourlyTicker.C:
			aggregateHourlyMetrics(ctx)
		case <-dailyTicker.C:
			aggregateDailyMetrics(ctx)
		}
	}
}

// collectAndStoreVPSMetrics collects current VPS metrics and stores them in Redis
func collectAndStoreVPSMetrics(ctx context.Context) {
	if redisClient == nil {
		return
	}

	vps := collectVPSMetrics()
	if vps == nil {
		return
	}

	point := VPSMetricsHistoryPoint{
		Timestamp:         time.Now().UTC(),
		CPUUsagePercent:   vps.CPUUsagePercent,
		MemoryUsedPercent: vps.MemoryUsedPercent,
		DiskUsedPercent:   vps.DiskUsedPercent,
		LoadAvg1:          vps.LoadAvg1,
	}

	data, err := json.Marshal(point)
	if err != nil {
		logger.WithError(err).Warn("Failed to marshal VPS metrics point")
		return
	}

	// Store in today's list (LPUSH to add to front, LTRIM to keep only last 288 items = 24 hours * 12 per hour)
	pipe := redisClient.Pipeline()
	pipe.LPush(ctx, vpsMetricsTodayKey, data)
	pipe.LTrim(ctx, vpsMetricsTodayKey, 0, 287) // Keep last 288 points (24 hours)
	pipe.Expire(ctx, vpsMetricsTodayKey, 25*time.Hour)
	_, err = pipe.Exec(ctx)
	if err != nil {
		logger.WithError(err).Warn("Failed to store VPS metrics in Redis")
	}
}

// aggregateHourlyMetrics aggregates the last hour's 5-minute metrics into an hourly average
func aggregateHourlyMetrics(ctx context.Context) {
	if redisClient == nil {
		return
	}

	// Get last 12 points (1 hour of 5-minute intervals)
	data, err := redisClient.LRange(ctx, vpsMetricsTodayKey, 0, 11).Result()
	if err != nil || len(data) == 0 {
		return
	}

	var totalCPU, totalMem, totalDisk, totalLoad float64
	var count float64

	for _, d := range data {
		var point VPSMetricsHistoryPoint
		if err := json.Unmarshal([]byte(d), &point); err != nil {
			continue
		}
		totalCPU += point.CPUUsagePercent
		totalMem += point.MemoryUsedPercent
		totalDisk += point.DiskUsedPercent
		totalLoad += point.LoadAvg1
		count++
	}

	if count == 0 {
		return
	}

	avgPoint := VPSMetricsHistoryPoint{
		Timestamp:         time.Now().UTC().Truncate(time.Hour),
		CPUUsagePercent:   totalCPU / count,
		MemoryUsedPercent: totalMem / count,
		DiskUsedPercent:   totalDisk / count,
		LoadAvg1:          totalLoad / count,
	}

	avgData, err := json.Marshal(avgPoint)
	if err != nil {
		return
	}

	// Store in weekly list
	pipe := redisClient.Pipeline()
	pipe.LPush(ctx, vpsMetricsWeekKey, avgData)
	pipe.LTrim(ctx, vpsMetricsWeekKey, 0, 167) // Keep last 168 points (7 days)
	pipe.Expire(ctx, vpsMetricsWeekKey, 8*24*time.Hour)
	pipe.Exec(ctx)
}

// aggregateDailyMetrics aggregates the last day's hourly metrics into a daily average
func aggregateDailyMetrics(ctx context.Context) {
	if redisClient == nil {
		return
	}

	// Get last 24 points (24 hours)
	data, err := redisClient.LRange(ctx, vpsMetricsWeekKey, 0, 23).Result()
	if err != nil || len(data) == 0 {
		return
	}

	var totalCPU, totalMem, totalDisk, totalLoad float64
	var count float64

	for _, d := range data {
		var point VPSMetricsHistoryPoint
		if err := json.Unmarshal([]byte(d), &point); err != nil {
			continue
		}
		totalCPU += point.CPUUsagePercent
		totalMem += point.MemoryUsedPercent
		totalDisk += point.DiskUsedPercent
		totalLoad += point.LoadAvg1
		count++
	}

	if count == 0 {
		return
	}

	avgPoint := VPSMetricsHistoryPoint{
		Timestamp:         time.Now().UTC().Truncate(24 * time.Hour),
		CPUUsagePercent:   totalCPU / count,
		MemoryUsedPercent: totalMem / count,
		DiskUsedPercent:   totalDisk / count,
		LoadAvg1:          totalLoad / count,
	}

	avgData, err := json.Marshal(avgPoint)
	if err != nil {
		return
	}

	// Store in monthly list
	pipe := redisClient.Pipeline()
	pipe.LPush(ctx, vpsMetricsMonthKey, avgData)
	pipe.LTrim(ctx, vpsMetricsMonthKey, 0, 29) // Keep last 30 points (30 days)
	pipe.Expire(ctx, vpsMetricsMonthKey, 31*24*time.Hour)
	pipe.Exec(ctx)
}

// vpsMetricsHistoryHandler returns VPS metrics history for today, week, and month
func vpsMetricsHistoryHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	logger.WithField("request_id", requestID).Info("Fetching VPS metrics history")

	history := VPSMetricsHistory{
		Current: collectVPSMetrics(),
		Today:   []VPSMetricsHistoryPoint{},
		Week:    []VPSMetricsHistoryPoint{},
		Month:   []VPSMetricsHistoryPoint{},
	}

	if redisClient == nil {
		c.JSON(http.StatusOK, history)
		return
	}

	ctx := c.Request.Context()

	// Get today's metrics (5-minute intervals)
	todayData, err := redisClient.LRange(ctx, vpsMetricsTodayKey, 0, -1).Result()
	if err == nil {
		for _, d := range todayData {
			var point VPSMetricsHistoryPoint
			if err := json.Unmarshal([]byte(d), &point); err == nil {
				history.Today = append(history.Today, point)
			}
		}
	}

	// Get weekly metrics (hourly intervals)
	weekData, err := redisClient.LRange(ctx, vpsMetricsWeekKey, 0, -1).Result()
	if err == nil {
		for _, d := range weekData {
			var point VPSMetricsHistoryPoint
			if err := json.Unmarshal([]byte(d), &point); err == nil {
				history.Week = append(history.Week, point)
			}
		}
	}

	// Get monthly metrics (daily intervals)
	monthData, err := redisClient.LRange(ctx, vpsMetricsMonthKey, 0, -1).Result()
	if err == nil {
		for _, d := range monthData {
			var point VPSMetricsHistoryPoint
			if err := json.Unmarshal([]byte(d), &point); err == nil {
				history.Month = append(history.Month, point)
			}
		}
	}

	c.JSON(http.StatusOK, history)
}
