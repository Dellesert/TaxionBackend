package config

import (
	"fmt"
	"os"
	"path/filepath"

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
	// Определяем режим запуска (Docker или локально)
	environment := os.Getenv("ENVIRONMENT")
	isDocker := os.Getenv("DOCKER") == "true" || environment == "production"

	pwd, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", pwd)
	fmt.Printf("Environment: %s, Docker: %v\n", environment, isDocker)

	// Пытаемся загрузить .env файлы в порядке приоритета
	loaded := false

	// 1. Сначала пробуем .env.local (для локальной разработки)
	if !isDocker {
		if err := tryLoadEnvFile(".env.local"); err == nil {
			fmt.Println("✅ Successfully loaded .env.local")
			loaded = true
		} else {
			fmt.Printf("⚠️  .env.local not found: %v\n", err)
		}
	}

	// 2. Если не загрузился .env.local, пробуем основной .env
	if !loaded {
		if err := tryLoadEnvFile(".env"); err == nil {
			fmt.Println("✅ Successfully loaded .env")
			loaded = true
		} else {
			fmt.Printf("⚠️  .env not found: %v\n", err)
		}
	}

	// 3. Пробуем найти .env файлы в корне проекта
	if !loaded {
		rootPaths := []string{"../../.env.local", "../../.env", "../.env.local", "../.env"}
		for _, path := range rootPaths {
			if err := tryLoadEnvFile(path); err == nil {
				fmt.Printf("✅ Successfully loaded %s\n", path)
				loaded = true
				break
			}
		}
	}

	if !loaded {
		fmt.Println("⚠️  No .env file loaded, using only environment variables")
	}

	// Получаем переменные окружения
	jwtSecret := os.Getenv("JWT_SECRET")
	databaseURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	serverPort := os.Getenv("SERVER_PORT")

	// DEBUG: показываем что загрузилось
	if jwtSecret != "" {
		if len(jwtSecret) > 10 {
			fmt.Printf("JWT_SECRET loaded: %s...%s (length: %d)\n",
				jwtSecret[:5], jwtSecret[len(jwtSecret)-5:], len(jwtSecret))
		} else {
			fmt.Printf("JWT_SECRET is short: '%s' (length: %d)\n", jwtSecret, len(jwtSecret))
		}
	} else {
		fmt.Println("❌ JWT_SECRET is empty!")
	}

	fmt.Printf("DATABASE_URL: %s\n", maskURL(databaseURL))
	fmt.Printf("REDIS_URL: %s\n", maskURL(redisURL))
	fmt.Printf("SERVER_PORT: %s\n", serverPort)

	config := &Config{
		Database: DatabaseConfig{
			URL: databaseURL,
		},
		Redis: RedisConfig{
			URL: redisURL,
		},
		JWT: JWTConfig{
			Secret: jwtSecret,
		},
		Server: ServerConfig{
			Port: serverPort,
		},
	}

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// tryLoadEnvFile пытается загрузить .env файл
func tryLoadEnvFile(filename string) error {
	// Проверяем существование файла
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file %s does not exist", filename)
	}

	// Загружаем файл
	if err := godotenv.Load(filename); err != nil {
		return fmt.Errorf("failed to load %s: %w", filename, err)
	}

	return nil
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

// maskURL masks sensitive parts of URLs for logging
func maskURL(url string) string {
	if url == "" {
		return "❌ not set"
	}

	// Простое маскирование - показываем только схему и хост
	if len(url) > 20 {
		return url[:10] + "***" + url[len(url)-7:]
	}

	return "***"
}

// GetProjectRoot пытается найти корень проекта
func GetProjectRoot() string {
	// Начинаем с текущей директории
	dir, _ := os.Getwd()

	// Поднимаемся вверх, ища go.mod
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Достигли корня файловой системы
		}
		dir = parent
	}

	// Если не нашли, возвращаем текущую директорию
	current, _ := os.Getwd()
	return current
}
