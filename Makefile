# File: Makefile (корень проекта)
.PHONY: help build up down clean logs restart status test

# Default target
help:
	@echo "🚀 Tachyon Messenger - Docker Commands"
	@echo ""
	@echo "Development:"
	@echo "  up          - Start all services with Docker Compose"
	@echo "  down        - Stop all services"
	@echo "  restart     - Restart all services"
	@echo "  build       - Build all Docker images"
	@echo "  rebuild     - Clean build all images"
	@echo ""
	@echo "Monitoring:"
	@echo "  logs        - View logs from all services"
	@echo "  logs-user   - View User Service logs"
	@echo "  logs-chat   - View Chat Service logs"
	@echo "  status      - Show service status"
	@echo ""
	@echo "Database:"
	@echo "  db-shell    - Connect to PostgreSQL shell"
	@echo "  redis-shell - Connect to Redis shell"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean       - Stop and remove containers, networks"
	@echo "  clean-all   - Clean everything including volumes"
	@echo ""
	@echo "Testing:"
	@echo "  test        - Run integration tests"
	@echo "  test-user   - Test User Service endpoints"
	@echo "  test-chat   - Test Chat Service endpoints"

# Development commands
up:
	@echo "🚀 Starting Tachyon Messenger services..."
	@docker-compose up -d
	@echo "✅ Services started! Check status with: make status"

down:
	@echo "🛑 Stopping Tachyon Messenger services..."
	@docker-compose down
	@echo "✅ Services stopped"

restart: down up

build:
	@echo "🔨 Building Docker images..."
	@docker-compose build --parallel
	@echo "✅ Build completed"

rebuild:
	@echo "🔨 Rebuilding Docker images (no cache)..."
	@docker-compose build --no-cache --parallel
	@echo "✅ Rebuild completed"

# Monitoring commands
logs:
	@echo "📋 Showing logs from all services..."
	@docker-compose logs -f --tail=100

logs-user:
	@echo "📋 Showing User Service logs..."
	@docker-compose logs -f --tail=100 user-service

logs-chat:
	@echo "📋 Showing Chat Service logs..."
	@docker-compose logs -f --tail=100 chat-service

logs-gateway:
	@echo "📋 Showing Gateway logs..."
	@docker-compose logs -f --tail=100 gateway

status:
	@echo "📊 Service Status:"
	@docker-compose ps
	@echo ""
	@echo "🌐 Service URLs:"
	@echo "  Gateway:      http://localhost:8080"
	@echo "  User Service: http://localhost:8081"
	@echo "  Chat Service: http://localhost:8082"
	@echo "  PostgreSQL:   localhost:5432"
	@echo "  Redis:        localhost:6379"

# Database commands
db-shell:
	@echo "🐘 Connecting to PostgreSQL..."
	@docker-compose exec postgres psql -U tachyon_user -d tachyon_messenger

redis-shell:
	@echo "🔴 Connecting to Redis..."
	@docker-compose exec redis redis-cli -a redis_password

# Cleanup commands
clean:
	@echo "🧹 Cleaning up containers and networks..."
	@docker-compose down --remove-orphans
	@docker system prune -f
	@echo "✅ Cleanup completed"

clean-all:
	@echo "🧹 Cleaning up everything including volumes..."
	@docker-compose down -v --remove-orphans
	@docker system prune -a -f --volumes
	@echo "✅ Complete cleanup finished"

# Testing commands
test:
	@echo "🧪 Running integration tests..."
	@./scripts/test-integration.sh

test-user:
	@echo "🧪 Testing User Service..."
	@curl -s http://localhost:8081/health | jq || echo "❌ User Service not responding"

test-chat:
	@echo "🧪 Testing Chat Service..."
	@curl -s http://localhost:8082/health | jq || echo "❌ Chat Service not responding"

test-gateway:
	@echo "🧪 Testing Gateway..."
	@curl -s http://localhost:8080/health | jq || echo "❌ Gateway not responding"

# Development helpers
dev-logs:
	@echo "📋 Development logs (following)..."
	@docker-compose logs -f user-service chat-service

dev-restart-user:
	@echo "🔄 Restarting User Service..."
	@docker-compose restart user-service

dev-restart-chat:
	@echo "🔄 Restarting Chat Service..."
	@docker-compose restart chat-service

# Initialize development environment
init:
	@echo "🎯 Initializing development environment..."
	@make build
	@make up
	@echo "⏳ Waiting for services to start..."
	@sleep 10
	@make status
	@echo "✅ Development environment ready!"