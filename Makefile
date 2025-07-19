.PHONY: build up down clean logs help

# Default target
help:
	@echo "Available commands:"
	@echo "  build  - Build all Go services"
	@echo "  up     - Start docker-compose services"
	@echo "  down   - Stop docker-compose services"
	@echo "  clean  - Clean up containers and volumes"
	@echo "  logs   - View docker-compose logs"

# Build all Go services
build:
	@echo "Building all services..."
	@go mod tidy
	@go build -o bin/ ./cmd/...

# Start docker-compose services
up:
	@echo "Starting infrastructure services..."
	@docker-compose up -d

# Stop docker-compose services
down:
	@echo "Stopping infrastructure services..."
	@docker-compose down

# Clean up containers, volumes and build artifacts
clean:
	@echo "Cleaning up..."
	@docker-compose down -v --remove-orphans
	@docker system prune -f
	@rm -rf bin/

# View docker-compose logs
logs:
	@docker-compose logs -f