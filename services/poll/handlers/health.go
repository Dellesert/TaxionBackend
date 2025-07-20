package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime,omitempty"`
}

var startTime = time.Now()

// HealthCheck handles health check requests
func HealthCheck(c *gin.Context) {
	uptime := time.Since(startTime).String()

	response := HealthResponse{
		Status:    "healthy",
		Service:   "poll-service",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
		Uptime:    uptime,
	}

	c.JSON(http.StatusOK, response)
}

// ReadinessCheck handles readiness check requests (can be extended with DB checks)
func ReadinessCheck(c *gin.Context) {
	// TODO: Add database connectivity check
	// TODO: Add external service dependency checks

	response := HealthResponse{
		Status:    "ready",
		Service:   "poll-service",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
	}

	c.JSON(http.StatusOK, response)
}

// LivenessCheck handles liveness check requests
func LivenessCheck(c *gin.Context) {
	response := HealthResponse{
		Status:    "alive",
		Service:   "poll-service",
		Version:   "1.0.0",
		Timestamp: time.Now().UTC(),
	}

	c.JSON(http.StatusOK, response)
}
