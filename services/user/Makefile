.PHONY: build run test test-auth test-db clean help lint

# Default target
help:
	@echo "Available commands:"
	@echo "  build     - Build the user service"
	@echo "  run       - Run the user service"
	@echo "  test      - Run all tests"
	@echo "  test-auth - Run authentication tests only"
	@echo "  test-db   - Test database connection and setup"
	@echo "  lint      - Run code linting"
	@echo "  clean     - Clean build artifacts"

# Build the user service
build:
	@echo "Building user service..."
	@go mod tidy
	@go build -o bin/user-service ./

# Run the user service
run: build
	@echo "Starting user service..."
	@./bin/user-service

# Run all tests
test:
	@echo "Running all tests..."
	@go mod tidy
	@go test -v ./tests/...

# Run authentication tests only
test-auth:
	@echo "Running authentication tests..."
	@go mod tidy
	@go test -v ./tests/ -run TestUser

# Test database connection and setup
test-db:
	@echo "Testing database connection..."
	@go run ./cmd/test-db/main.go

# Run code linting
lint:
	@echo "Running code linting..."
	@golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf bin/
	@go clean

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@go mod tidy
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/stretchr/testify@latest

# Run with hot reload (requires air)
dev:
	@echo "Starting development server with hot reload..."
	@air

# Database operations
db-migrate:
	@echo "Running database migrations..."
	@go run -tags migrate ./cmd/migrate.go

db-reset:
	@echo "Resetting database (WARNING: This will delete all data)..."
	@go run -tags reset ./cmd/reset.go

# Docker operations
docker-build:
	@echo "Building Docker image..."
	@docker build -t tachyon-user-service .

docker-run:
	@echo "Running in Docker..."
	@docker run -p 8081:8081 --env-file ../../.env tachyon-user-service

# Integration tests (requires running infrastructure)
integration-test:
	@echo "Running integration tests..."
	@./tests/integration_test.sh