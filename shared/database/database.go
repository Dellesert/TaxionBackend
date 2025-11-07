package database

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the database connection
type DB struct {
	*gorm.DB
}

// Config holds database configuration options
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultConfig returns default database configuration
func DefaultConfig(dsn string) *Config {
	return &Config{
		DSN:             dsn,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 10,
		LogLevel:        logger.Info,
	}
}

// ConfigFromEnv creates database configuration from environment variables
func ConfigFromEnv(dsn string) *Config {
	config := DefaultConfig(dsn)

	// Read connection pool settings from environment
	if maxOpen := os.Getenv("DB_MAX_OPEN_CONNS"); maxOpen != "" {
		if val, err := strconv.Atoi(maxOpen); err == nil {
			config.MaxOpenConns = val
			logrus.Infof("DB Connection Pool: MaxOpenConns set to %d from environment", val)
		}
	}

	if maxIdle := os.Getenv("DB_MAX_IDLE_CONNS"); maxIdle != "" {
		if val, err := strconv.Atoi(maxIdle); err == nil {
			config.MaxIdleConns = val
			logrus.Infof("DB Connection Pool: MaxIdleConns set to %d from environment", val)
		}
	}

	if connMaxLifetime := os.Getenv("DB_CONN_MAX_LIFETIME"); connMaxLifetime != "" {
		if val, err := time.ParseDuration(connMaxLifetime); err == nil {
			config.ConnMaxLifetime = val
			logrus.Infof("DB Connection Pool: ConnMaxLifetime set to %v from environment", val)
		}
	}

	if connMaxIdleTime := os.Getenv("DB_CONN_MAX_IDLE_TIME"); connMaxIdleTime != "" {
		if val, err := time.ParseDuration(connMaxIdleTime); err == nil {
			config.ConnMaxIdleTime = val
			logrus.Infof("DB Connection Pool: ConnMaxIdleTime set to %v from environment", val)
		}
	}

	// Log level configuration
	if logLevel := os.Getenv("DB_LOG_LEVEL"); logLevel != "" {
		switch logLevel {
		case "silent":
			config.LogLevel = logger.Silent
		case "error":
			config.LogLevel = logger.Error
		case "warn":
			config.LogLevel = logger.Warn
		case "info":
			config.LogLevel = logger.Info
		default:
			config.LogLevel = logger.Info
		}
		logrus.Infof("DB Connection Pool: LogLevel set to %s from environment", logLevel)
	}

	return config
}

// Connect establishes a connection to PostgreSQL database
func Connect(config *Config) (*DB, error) {
	logrus.Info("Connecting to PostgreSQL database...")

	// Configure GORM logger
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(config.LogLevel),
	}

	// Open database connection
	db, err := gorm.Open(postgres.Open(config.DSN), gormConfig)
	if err != nil {
		logrus.Errorf("Failed to connect to database: %v", err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		logrus.Errorf("Failed to get underlying sql.DB: %v", err)
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	logrus.WithFields(logrus.Fields{
		"max_open_conns":     config.MaxOpenConns,
		"max_idle_conns":     config.MaxIdleConns,
		"conn_max_lifetime":  config.ConnMaxLifetime,
		"conn_max_idle_time": config.ConnMaxIdleTime,
	}).Info("Database connection pool configured")

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		logrus.Errorf("Failed to ping database: %v", err)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logrus.Info("Successfully connected to PostgreSQL database")
	return &DB{db}, nil
}

// Close gracefully closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Close()
}

// Migrate runs database migrations for provided models
func (db *DB) Migrate(models ...interface{}) error {
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate model %T: %w", model, err)
		}
	}
	return nil
}

// Health checks database connection health
func (db *DB) Health() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return sqlDB.Ping()
}

// Stats returns database connection statistics
func (db *DB) Stats() (map[string]interface{}, error) {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration,
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}, nil
}
