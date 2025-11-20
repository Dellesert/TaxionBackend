#!/bin/bash

# ==============================================
# Tachyon Deployment Script
# ==============================================
# Automated deployment script for production environment
# Uses GitHub Container Registry (ghcr.io) for private images

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env.production"
CREDENTIALS_FILE="${GITHUB_CREDENTIALS_FILE:-$HOME/.github-credentials}"

# Functions
print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    print_success "Docker is installed"

    # Check Docker Compose
    if ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed"
        exit 1
    fi
    print_success "Docker Compose is installed"

    # Check and login to GitHub Container Registry
    check_ghcr_login

    # Check environment files
    if [ ! -f "$ENV_FILE" ]; then
        print_error "Environment file $ENV_FILE not found"
        exit 1
    fi
    print_success "Environment file found: $ENV_FILE"
}

# Check GHCR login and authenticate if needed
check_ghcr_login() {
    # Check if already logged in
    if docker info 2>&1 | grep -q "ghcr.io"; then
        print_success "Already logged in to GitHub Container Registry"
        return 0
    fi

    print_warning "Not logged in to GitHub Container Registry"

    # Try to load credentials from file
    if [ -f "$CREDENTIALS_FILE" ]; then
        print_info "Loading credentials from $CREDENTIALS_FILE"
        source "$CREDENTIALS_FILE"
    fi

    # Check if credentials are available
    if [ -z "$GITHUB_TOKEN" ] || [ -z "$GITHUB_USERNAME" ]; then
        print_error "GitHub credentials not found"
        print_info "Please set GITHUB_TOKEN and GITHUB_USERNAME environment variables"
        print_info "Or create $CREDENTIALS_FILE with:"
        echo ""
        echo "  GITHUB_TOKEN=ghp_your_token_here"
        echo "  GITHUB_USERNAME=your-username"
        echo ""
        print_info "Then run: chmod 600 $CREDENTIALS_FILE"
        exit 1
    fi

    # Attempt login
    print_info "Logging in to GitHub Container Registry..."
    if echo "$GITHUB_TOKEN" | docker login ghcr.io -u "$GITHUB_USERNAME" --password-stdin 2>&1 | grep -q "Login Succeeded"; then
        print_success "Successfully logged in to GitHub Container Registry"
    else
        print_error "Failed to login to GitHub Container Registry"
        print_info "Check your GITHUB_TOKEN and GITHUB_USERNAME"
        exit 1
    fi
}

# Pull latest images
pull_images() {
    print_header "Pulling Latest Images"

    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" pull

    print_success "Images pulled successfully"
}

# Deploy services
deploy() {
    print_header "Deploying Services"

    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

    print_success "Services deployed successfully"
}

# Health check
health_check() {
    print_header "Running Health Checks"

    sleep 10

    services=(
        "tachyon-gateway-prod:8080"
        "tachyon-user-service-prod:8081"
        "tachyon-chat-service-prod:8082"
        "tachyon-task-service-prod:8083"
        "tachyon-calendar-service-prod:8084"
        "tachyon-poll-service-prod:8085"
        "tachyon-analytics-service-prod:8086"
        "tachyon-notification-service-prod:8087"
        "tachyon-file-service-prod:8088"
        "tachyon-backup-service-prod:8089"
    )

    all_healthy=true

    for service in "${services[@]}"; do
        container_name="${service%%:*}"
        port="${service##*:}"

        if docker ps --format '{{.Names}}' | grep -q "^${container_name}$"; then
            if docker exec "$container_name" wget --no-verbose --tries=1 --spider "http://localhost:${port}/health" &> /dev/null; then
                print_success "$container_name is healthy"
            else
                print_error "$container_name health check failed"
                all_healthy=false
            fi
        else
            print_error "$container_name is not running"
            all_healthy=false
        fi
    done

    if [ "$all_healthy" = true ]; then
        print_success "All services are healthy!"
    else
        print_error "Some services are not healthy. Check logs with: docker compose -f $COMPOSE_FILE logs"
        exit 1
    fi
}

# Show running services
show_status() {
    print_header "Service Status"
    docker compose -f "$COMPOSE_FILE" ps
}

# Main execution
main() {
    print_header "Tachyon Production Deployment"

    check_prerequisites
    pull_images
    deploy
    health_check
    show_status

    print_header "Deployment Complete!"
    print_success "All services are running and healthy"
    print_info "Gateway is available at: http://localhost:8080"
}

# Parse arguments
case "${1:-deploy}" in
    deploy)
        main
        ;;
    pull)
        check_prerequisites
        pull_images
        ;;
    status)
        show_status
        ;;
    health)
        health_check
        ;;
    logs)
        docker compose -f "$COMPOSE_FILE" logs -f "${2:-}"
        ;;
    down)
        print_header "Stopping Services"
        docker compose -f "$COMPOSE_FILE" down
        print_success "Services stopped"
        ;;
    restart)
        print_header "Restarting Services"
        docker compose -f "$COMPOSE_FILE" restart "${2:-}"
        print_success "Services restarted"
        ;;
    *)
        echo "Usage: $0 {deploy|pull|status|health|logs [service]|down|restart [service]}"
        exit 1
        ;;
esac
