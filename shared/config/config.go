package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Server   ServerConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	URL string
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL string
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// .env file might not exist, which is ok
		fmt.Println("No .env file found, using system environment variables")
	}

	config := &Config{
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Redis: RedisConfig{
			URL: os.Getenv("REDIS_URL"),
		},
		JWT: JWTConfig{
			Secret: os.Getenv("JWT_SECRET"),
		},
		Server: ServerConfig{
			Port: os.Getenv("SERVER_PORT"),
		},
	}

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// validateConfig validates that required configuration fields are present
func validateConfig(config *Config) error {
	if config.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if config.Redis.URL == "" {
		return fmt.Errorf("REDIS_URL is required")
	}

	if config.JWT.Secret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}

	if config.Server.Port == "" {
		config.Server.Port = "8080" // Default port
	}

	return nil
}
