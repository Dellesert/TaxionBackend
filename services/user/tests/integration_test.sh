#!/bin/bash

# Integration test script for User Service
# This script tests the actual running service

set -e

BASE_URL="http://localhost:8081"
TEST_EMAIL="integration-test@example.com"
TEST_PASSWORD="password123"
ACCESS_TOKEN=""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_service() {
    log_info "Checking if User Service is running..."
    
    if curl -s "$BASE_URL/health" > /dev/null; then
        log_info "✓ User Service is running"
    else
        log_error "✗ User Service is not running at $BASE_URL"
        log_error "Please start the service with: make run"
        exit 1
    fi
}

test_health_endpoint() {
    log_info "Testing health endpoint..."
    
    response=$(curl -s "$BASE_URL/health")
    status=$(echo "$response" | jq -r '.status' 2>/dev/null || echo "error")
    
    if [ "$status" = "healthy" ]; then
        log_info "✓ Health check passed"
    else
        log_error "✗ Health check failed"
        echo "Response: $response"
        exit 1
    fi
}

test_user_registration() {
    log_info "Testing user registration..."
    
    # Clean up any existing test user first
    cleanup_test_user
    
    response=$(curl -s -X POST "$BASE_URL/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"name\": \"Integration Test User\",
            \"password\": \"$TEST_PASSWORD\",
            \"role\": \"employee\",
            \"position\": \"Test Engineer\"
        }")
    
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL-2\",
            \"name\": \"Integration Test User 2\",
            \"password\": \"$TEST_PASSWORD\",
            \"role\": \"employee\"
        }")
    
    if [ "$http_code" = "201" ]; then
        log_info "✓ User registration successful"
    else
        log_error "✗ User registration failed (HTTP $http_code)"
        echo "Response: $response"
        exit 1
    fi
}

test_user_login() {
    log_info "Testing user login..."
    
    response=$(curl -s -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"$TEST_PASSWORD\"
        }")
    
    ACCESS_TOKEN=$(echo "$response" | jq -r '.tokens.access_token' 2>/dev/null)
    
    if [ "$ACCESS_TOKEN" != "null" ] && [ "$ACCESS_TOKEN" != "" ]; then
        log_info "✓ User login successful"
        log_info "Access token obtained: ${ACCESS_TOKEN:0:20}..."
    else
        log_error "✗ User login failed"
        echo "Response: $response"
        exit 1
    fi
}

test_protected_endpoints() {
    log_info "Testing protected endpoints..."
    
    if [ -z "$ACCESS_TOKEN" ]; then
        log_error "No access token available"
        exit 1
    fi
    
    # Test getting user list
    response=$(curl -s -X GET "$BASE_URL/api/v1/users?limit=5" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json")
    
    users_count=$(echo "$response" | jq '.users | length' 2>/dev/null || echo "0")
    
    if [ "$users_count" -gt "0" ]; then
        log_info "✓ Protected endpoint (get users) working"
        log_info "Found $users_count users"
    else
        log_error "✗ Protected endpoint failed"
        echo "Response: $response"
        exit 1
    fi
}

test_jwt_validation() {
    log_info "Testing JWT validation..."
    
    # Test with invalid token
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/api/v1/users" \
        -H "Authorization: Bearer invalid-token" \
        -H "Content-Type: application/json")
    
    if [ "$http_code" = "401" ]; then
        log_info "✓ JWT validation working (rejected invalid token)"
    else
        log_error "✗ JWT validation failed (should reject invalid token)"
        exit 1
    fi
    
    # Test without token
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/api/v1/users" \
        -H "Content-Type: application/json")
    
    if [ "$http_code" = "401" ]; then
        log_info "✓ JWT validation working (rejected missing token)"
    else
        log_error "✗ JWT validation failed (should reject missing token)"
        exit 1
    fi
}

test_error_handling() {
    log_info "Testing error handling..."
    
    # Test duplicate email registration
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"name\": \"Duplicate User\",
            \"password\": \"$TEST_PASSWORD\"
        }")
    
    if [ "$http_code" = "409" ]; then
        log_info "✓ Duplicate email handling working"
    else
        log_error "✗ Duplicate email should return 409 Conflict"
        exit 1
    fi
    
    # Test invalid login
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\": \"$TEST_EMAIL\",
            \"password\": \"wrongpassword\"
        }")
    
    if [ "$http_code" = "401" ]; then
        log_info "✓ Invalid login handling working"
    else
        log_error "✗ Invalid login should return 401 Unauthorized"
        exit 1
    fi
}

cleanup_test_user() {
    log_info "Cleaning up test data..."
    # Note: In a real scenario, you might want to add a cleanup endpoint
    # or connect directly to the database for cleanup
}

run_performance_test() {
    log_info "Running basic performance test..."
    
    # Simple load test with curl
    start_time=$(date +%s)
    
    for i in {1..10}; do
        curl -s "$BASE_URL/health" > /dev/null
    done
    
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    log_info "✓ 10 health check requests completed in ${duration}s"
    
    if [ $duration -lt 5 ]; then
        log_info "✓ Performance test passed"
    else
        log_warn "⚠ Performance test slow (took ${duration}s for 10 requests)"
    fi
}

# Main test execution
main() {
    log_info "Starting User Service Integration Tests"
    log_info "==========================================="
    
    check_service
    test_health_endpoint
    test_user_registration
    test_user_login
    test_protected_endpoints
    test_jwt_validation
    test_error_handling
    run_performance_test
    cleanup_test_user
    
    log_info "==========================================="
    log_info "✅ All integration tests passed!"
    log_info "User Service is working correctly"
}

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    log_error "jq is required for this script. Please install it:"
    log_error "  Ubuntu/Debian: sudo apt-get install jq"
    log_error "  macOS: brew install jq"
    exit 1
fi

# Run main function
main "$@"