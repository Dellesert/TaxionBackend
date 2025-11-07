package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// PoolMetrics contains detailed connection pool metrics
type PoolMetrics struct {
	// Configuration
	MaxOpenConnections int `json:"max_open_connections"`
	MaxIdleConns       int `json:"max_idle_conns"`

	// Current state
	OpenConnections int `json:"open_connections"`
	InUse           int `json:"in_use"`
	Idle            int `json:"idle"`

	// Wait statistics
	WaitCount         int64         `json:"wait_count"`
	WaitDuration      time.Duration `json:"wait_duration"`
	WaitDurationMs    int64         `json:"wait_duration_ms"`
	AvgWaitDurationMs float64       `json:"avg_wait_duration_ms"`

	// Connection lifecycle statistics
	MaxIdleClosed      int64 `json:"max_idle_closed"`
	MaxIdleTimeClosed  int64 `json:"max_idle_time_closed"`
	MaxLifetimeClosed  int64 `json:"max_lifetime_closed"`
	TotalConnectionsClosed int64 `json:"total_connections_closed"`

	// Health indicators
	UtilizationPercent float64 `json:"utilization_percent"`
	IsHealthy          bool    `json:"is_healthy"`
	Timestamp          string  `json:"timestamp"`
}

// GetMetrics returns detailed connection pool metrics
func (db *DB) GetMetrics() (*PoolMetrics, error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()

	// Calculate additional metrics
	var avgWaitDuration float64
	if stats.WaitCount > 0 {
		avgWaitDuration = float64(stats.WaitDuration.Milliseconds()) / float64(stats.WaitCount)
	}

	var utilizationPercent float64
	if stats.MaxOpenConnections > 0 {
		utilizationPercent = (float64(stats.OpenConnections) / float64(stats.MaxOpenConnections)) * 100
	}

	totalClosed := stats.MaxIdleClosed + stats.MaxIdleTimeClosed + stats.MaxLifetimeClosed

	metrics := &PoolMetrics{
		MaxOpenConnections:     stats.MaxOpenConnections,
		OpenConnections:        stats.OpenConnections,
		InUse:                  stats.InUse,
		Idle:                   stats.Idle,
		WaitCount:              stats.WaitCount,
		WaitDuration:           stats.WaitDuration,
		WaitDurationMs:         stats.WaitDuration.Milliseconds(),
		AvgWaitDurationMs:      avgWaitDuration,
		MaxIdleClosed:          stats.MaxIdleClosed,
		MaxIdleTimeClosed:      stats.MaxIdleTimeClosed,
		MaxLifetimeClosed:      stats.MaxLifetimeClosed,
		TotalConnectionsClosed: totalClosed,
		UtilizationPercent:     utilizationPercent,
		IsHealthy:              db.Health() == nil,
		Timestamp:              time.Now().Format(time.RFC3339),
	}

	return metrics, nil
}

// LogMetrics logs current connection pool metrics
func (db *DB) LogMetrics() {
	metrics, err := db.GetMetrics()
	if err != nil {
		logrus.Errorf("Failed to get pool metrics: %v", err)
		return
	}

	logrus.WithFields(logrus.Fields{
		"max_open":              metrics.MaxOpenConnections,
		"open":                  metrics.OpenConnections,
		"in_use":                metrics.InUse,
		"idle":                  metrics.Idle,
		"utilization_percent":   fmt.Sprintf("%.2f%%", metrics.UtilizationPercent),
		"wait_count":            metrics.WaitCount,
		"avg_wait_ms":           fmt.Sprintf("%.2f", metrics.AvgWaitDurationMs),
		"total_closed":          metrics.TotalConnectionsClosed,
		"healthy":               metrics.IsHealthy,
	}).Info("Database connection pool metrics")
}

// MonitorPoolHealth continuously monitors pool health and logs warnings
func (db *DB) MonitorPoolHealth(interval time.Duration, stopChan <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logrus.Infof("Starting database pool health monitoring (interval: %v)", interval)

	for {
		select {
		case <-ticker.C:
			metrics, err := db.GetMetrics()
			if err != nil {
				logrus.Errorf("Failed to get pool metrics during monitoring: %v", err)
				continue
			}

			// Check for potential issues
			if metrics.UtilizationPercent > 90 {
				logrus.Warnf("High connection pool utilization: %.2f%% (%d/%d connections in use)",
					metrics.UtilizationPercent, metrics.OpenConnections, metrics.MaxOpenConnections)
			}

			if metrics.WaitCount > 0 && metrics.AvgWaitDurationMs > 100 {
				logrus.Warnf("High average wait time for connections: %.2fms (total waits: %d)",
					metrics.AvgWaitDurationMs, metrics.WaitCount)
			}

			if !metrics.IsHealthy {
				logrus.Error("Database connection pool is unhealthy!")
			}

			// Log metrics periodically
			db.LogMetrics()

		case <-stopChan:
			logrus.Info("Stopping database pool health monitoring")
			return
		}
	}
}

// GetConnectionConfig returns current pool configuration
func (db *DB) GetConnectionConfig() (map[string]interface{}, error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"max_idle_connections": stats.Idle,
	}, nil
}

// CheckPoolHealth performs health checks and returns issues if any
func (db *DB) CheckPoolHealth() []string {
	var issues []string

	metrics, err := db.GetMetrics()
	if err != nil {
		issues = append(issues, fmt.Sprintf("Failed to get metrics: %v", err))
		return issues
	}

	// Check if pool is exhausted
	if metrics.UtilizationPercent >= 100 {
		issues = append(issues, fmt.Sprintf("Connection pool exhausted: %d/%d connections in use",
			metrics.OpenConnections, metrics.MaxOpenConnections))
	}

	// Check for high wait times
	if metrics.WaitCount > 0 && metrics.AvgWaitDurationMs > 500 {
		issues = append(issues, fmt.Sprintf("High average wait time: %.2fms", metrics.AvgWaitDurationMs))
	}

	// Check database connectivity
	if !metrics.IsHealthy {
		issues = append(issues, "Database connection is unhealthy")
	}

	// Check for excessive connection churn
	if metrics.TotalConnectionsClosed > 1000 {
		issues = append(issues, fmt.Sprintf("High connection churn: %d connections closed",
			metrics.TotalConnectionsClosed))
	}

	return issues
}

// SetConnectionLimits dynamically adjusts connection pool limits
func (db *DB) SetConnectionLimits(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
		logrus.Infof("Updated MaxOpenConns to %d", maxOpen)
	}

	if maxIdle > 0 {
		sqlDB.SetMaxIdleConns(maxIdle)
		logrus.Infof("Updated MaxIdleConns to %d", maxIdle)
	}

	if maxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(maxLifetime)
		logrus.Infof("Updated ConnMaxLifetime to %v", maxLifetime)
	}

	if maxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(maxIdleTime)
		logrus.Infof("Updated ConnMaxIdleTime to %v", maxIdleTime)
	}

	return nil
}

// ResetStats resets connection pool statistics (requires recreating connection)
func (db *DB) ResetStats() error {
	// Note: sql.DB doesn't provide a way to reset stats directly
	// This is mainly for documentation purposes
	logrus.Info("Connection pool stats reset requested - stats are cumulative and cannot be reset without reconnecting")
	return nil
}

// GetDBStats returns raw sql.DBStats for advanced monitoring
func (db *DB) GetDBStats() (*sql.DBStats, error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()
	return &stats, nil
}
