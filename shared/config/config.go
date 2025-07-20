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
// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// ВРЕМЕННЫЙ DEBUG - показываем текущую директорию
	pwd, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", pwd)

	// Попробуем загрузить .env файл
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Failed to load .env from current dir: %v\n", err)

		// Попробуем загрузить из корня проекта
		if err := godotenv.Load("../../.env"); err != nil {
			fmt.Printf("Failed to load .env from ../../.env: %v\n", err)
		} else {
			fmt.Println("Successfully loaded .env from ../../.env")
		}
	} else {
		fmt.Println("Successfully loaded .env from current directory")
	}

	// Показываем что загрузилось
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret != "" {
		if len(jwtSecret) > 10 {
			fmt.Printf("JWT_SECRET loaded: %s...%s (length: %d)\n",
				jwtSecret[:5], jwtSecret[len(jwtSecret)-5:], len(jwtSecret))
		} else {
			fmt.Printf("JWT_SECRET is short: '%s' (length: %d)\n", jwtSecret, len(jwtSecret))
		}
	} else {
		fmt.Println("JWT_SECRET is empty!")
	}

	config := &Config{
		Database: DatabaseConfig{
			URL: os.Getenv("DATABASE_URL"),
		},
		Redis: RedisConfig{
			URL: os.Getenv("REDIS_URL"),
		},
		JWT: JWTConfig{
			Secret: jwtSecret,
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
