package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"tachyon-messenger/shared/logger"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
)

// ServiceHealth represents health status of a service
type ServiceHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "healthy", "unhealthy", "unknown"
	URL       string    `json:"url"`
	Latency   string    `json:"latency"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// GatewayHealth represents overall gateway health
type GatewayHealth struct {
	Status    string          `json:"status"`
	Service   string          `json:"service"`
	Version   string          `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	Services  []ServiceHealth `json:"services,omitempty"`
}

// healthHandler handles basic gateway health check
func healthHandler(c *gin.Context) {
	health := GatewayHealth{
		Status:    "healthy",
		Service:   "gateway",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
	}

	c.JSON(http.StatusOK, health)
}

// servicesHealthHandler checks health of all downstream services
func servicesHealthHandler(c *gin.Context) {
	requestID := requestid.Get(c)
	proxyConfig := getProxyConfig()

	logger.WithField("request_id", requestID).Info("Checking health of all services")

	// List of services to check
	services := []ServiceConfig{
		proxyConfig.UserService,
		// Add other services here when they're implemented
		{Name: "chat-service", URL: getEnvOrDefault("CHAT_SERVICE_URL", "http://localhost:8082")},
		{Name: "task-service", URL: getEnvOrDefault("TASK_SERVICE_URL", "http://localhost:8083")},
		{Name: "calendar-service", URL: getEnvOrDefault("CALENDAR_SERVICE_URL", "http://localhost:8084")},
		{Name: "poll-service", URL: getEnvOrDefault("POLL_SERVICE_URL", "http://localhost:8085")},
		{Name: "analytics-service", URL: getEnvOrDefault("ANALYTICS_SERVICE_URL", "http://localhost:8086")},
		{Name: "notification-service", URL: getEnvOrDefault("NOTIFICATION_SERVICE_URL", "http://localhost:8087")},
		{Name: "file-service", URL: getEnvOrDefault("FILE_SERVICE_URL", "http://localhost:8088")},
	}

	// Check health of each service
	serviceHealths := make([]ServiceHealth, len(services))
	healthyCount := 0

	for i, service := range services {
		serviceHealths[i] = checkServiceHealth(service)
		if serviceHealths[i].Status == "healthy" {
			healthyCount++
		}
	}

	// Determine overall status
	var overallStatus string
	if healthyCount == len(services) {
		overallStatus = "healthy"
	} else if healthyCount > 0 {
		overallStatus = "degraded"
	} else {
		overallStatus = "unhealthy"
	}

	health := GatewayHealth{
		Status:    overallStatus,
		Service:   "gateway",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
		Services:  serviceHealths,
	}

	// Return appropriate HTTP status
	var httpStatus int
	switch overallStatus {
	case "healthy":
		httpStatus = http.StatusOK
	case "degraded":
		httpStatus = http.StatusPartialContent // 206
	case "unhealthy":
		httpStatus = http.StatusServiceUnavailable // 503
	default:
		httpStatus = http.StatusInternalServerError
	}

	logger.WithFields(map[string]interface{}{
		"request_id":     requestID,
		"overall_status": overallStatus,
		"healthy_count":  healthyCount,
		"total_services": len(services),
	}).Info("Services health check completed")

	c.JSON(httpStatus, health)
}

// checkServiceHealth performs health check for a single service
func checkServiceHealth(service ServiceConfig) ServiceHealth {
	startTime := time.Now()

	health := ServiceHealth{
		Name:      service.Name,
		URL:       service.URL,
		Timestamp: time.Now().UTC(),
	}

	// Create health check URL
	healthURL := fmt.Sprintf("%s/health", service.URL)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		health.Status = "unhealthy"
		health.Error = fmt.Sprintf("Failed to create request: %v", err)
		health.Latency = time.Since(startTime).String()
		return health
	}

	// Add headers
	req.Header.Set("User-Agent", "gateway-health-checker/1.0")
	req.Header.Set("Accept", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		health.Status = "unhealthy"
		health.Error = fmt.Sprintf("Request failed: %v", err)
		health.Latency = time.Since(startTime).String()

		logger.WithFields(map[string]interface{}{
			"service": service.Name,
			"url":     healthURL,
			"error":   err.Error(),
		}).Warn("Service health check failed")

		return health
	}
	defer resp.Body.Close()

	// Calculate latency
	latency := time.Since(startTime)
	health.Latency = latency.String()

	// Check response status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		health.Status = "healthy"

		logger.WithFields(map[string]interface{}{
			"service":     service.Name,
			"status_code": resp.StatusCode,
			"latency":     latency,
		}).Debug("Service health check successful")
	} else {
		health.Status = "unhealthy"
		health.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)

		logger.WithFields(map[string]interface{}{
			"service":     service.Name,
			"status_code": resp.StatusCode,
			"latency":     latency,
		}).Warn("Service returned non-2xx status")
	}

	return health
}

// readinessHandler checks if gateway is ready to serve traffic
func readinessHandler(c *gin.Context) {
	// For now, just check if we can reach our configuration
	proxyConfig := getProxyConfig()

	ready := true
	var issues []string

	// Check if essential services are configured
	if proxyConfig.UserService.URL == "" {
		ready = false
		issues = append(issues, "user-service URL not configured")
	}

	response := gin.H{
		"ready":     ready,
		"service":   "gateway",
		"timestamp": time.Now().UTC(),
	}

	if !ready {
		response["issues"] = issues
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// livenessHandler checks if gateway is alive
func livenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"alive":     true,
		"service":   "gateway",
		"timestamp": time.Now().UTC(),
	})
}
