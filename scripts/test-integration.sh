#!/bin/bash
# File: scripts/test-integration.sh

set -e

echo "🧪 Starting Tachyon Messenger Integration Tests"
echo "=============================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test URLs
USER_SERVICE_URL="http://localhost:8081"
CHAT_SERVICE_URL="http://localhost:8082"
GATEWAY_URL="http://localhost:8080"

# Function to check if service is healthy
check_service() {
    local service_name=$1
    local url=$2
    
    echo -n "Checking $service_name... "
    
    if curl -s "$url/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ OK${NC}"
        return 0
    else
        echo -e "${RED}❌ FAILED${NC}"
        return 1
    fi
}

# Function to test user registration and login
test_user_flow() {
    echo -e "\n${YELLOW}Testing User Service Flow...${NC}"
    
    # Test user registration
    echo -n "Registering test user... "
    REGISTER_RESPONSE=$(curl -s -X POST "$USER_SERVICE_URL/auth/register" \
        -H "Content-Type: application/json" \
        -d '{
            "email": "test@integration.com",
            "name": "Integration Test User",
            "password": "testpass123"
        }')
    
    if echo "$REGISTER_RESPONSE" | grep -q "User registered successfully"; then
        echo -e "${GREEN}✅ OK${NC}"
    else
        echo -e "${RED}❌ FAILED${NC}"
        echo "Response: $REGISTER_RESPONSE"
        return 1
    fi
    
    # Test user login
    echo -n "Logging in test user... "
    LOGIN_RESPONSE=$(curl -s -X POST "$USER_SERVICE_URL/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "email": "test@integration.com",
            "password": "testpass123"
        }')
    
    if echo "$LOGIN_RESPONSE" | grep -q "access_token"; then
        echo -e "${GREEN}✅ OK${NC}"
        # Extract token for chat tests
        ACCESS_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.tokens.access_token' 2>/dev/null || echo "")
        export ACCESS_TOKEN
        return 0
    else
        echo -e "${RED}❌ FAILED${NC}"
        echo "Response: $LOGIN_RESPONSE"
        return 1
    fi
}

# Function to test chat service
test_chat_flow() {
    echo -e "\n${YELLOW}Testing Chat Service Flow...${NC}"
    
    if [ -z "$ACCESS_TOKEN" ]; then
        echo -e "${RED}❌ No access token available${NC}"
        return 1
    fi
    
    # Test chat creation
    echo -n "Creating test chat... "
    CHAT_RESPONSE=$(curl -s -X POST "$CHAT_SERVICE_URL/api/v1/chats" \
        -H "Authorization: Bearer $ACCESS_TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "name": "Integration Test Chat",
            "type": "group",
            "member_ids": []
        }')
    
    if echo "$CHAT_RESPONSE" | grep -q "Chat created successfully"; then
        echo -e "${GREEN}✅ OK${NC}"
        
        # Extract chat ID
        CHAT_ID=$(echo "$CHAT_RESPONSE" | jq -r '.chat.id' 2>/dev/null || echo "")
        
        # Test getting chats
        echo -n "Getting user chats... "
        CHATS_RESPONSE=$(curl -s -X GET "$CHAT_SERVICE_URL/api/v1/chats" \
            -H "Authorization: Bearer $ACCESS_TOKEN")
        
        if echo "$CHATS_RESPONSE" | grep -q "chats"; then
            echo -e "${GREEN}✅ OK${NC}"
        else
            echo -e "${RED}❌ FAILED${NC}"
            echo "Response: $CHATS_RESPONSE"
            return 1
        fi
        
    else
        echo -e "${RED}❌ FAILED${NC}"
        echo "Response: $CHAT_RESPONSE"
        return 1
    fi
}

# Function to test WebSocket (basic connectivity)
test_websocket() {
    echo -e "\n${YELLOW}Testing WebSocket Connection...${NC}"
    
    # Note: This is a basic test - WebSocket testing in shell is limited
    echo -n "Checking WebSocket endpoint... "
    
    # Test if WebSocket endpoint responds to HTTP upgrade request
    WS_RESPONSE=$(curl -s -w "%{http_code}" -o /dev/null \
        -H "Connection: Upgrade" \
        -H "Upgrade: websocket" \
        -H "Sec-WebSocket-Key: test" \
        -H "Sec-WebSocket-Version: 13" \
        "$CHAT_SERVICE_URL/api/v1/ws?token=$ACCESS_TOKEN")
    
    if [ "$WS_RESPONSE" = "401" ] || [ "$WS_RESPONSE" = "400" ] || [ "$WS_RESPONSE" = "101" ]; then
        echo -e "${GREEN}✅ OK (WebSocket endpoint responding)${NC}"
    else
        echo -e "${YELLOW}⚠️  Unknown response: $WS_RESPONSE${NC}"
    fi
}

# Main test execution
main() {
    echo "Waiting for services to be ready..."
    sleep 5
    
    # Check all services
    local all_healthy=true
    
    check_service "User Service" "$USER_SERVICE_URL" || all_healthy=false
    check_service "Chat Service" "$CHAT_SERVICE_URL" || all_healthy=false
    check_service "Gateway" "$GATEWAY_URL" || all_healthy=false
    
    if [ "$all_healthy" = false ]; then
        echo -e "\n${RED}❌ Some services are not healthy. Aborting tests.${NC}"
        exit 1
    fi
    
    # Run integration tests
    test_user_flow || exit 1
    test_chat_flow || exit 1
    test_websocket || exit 1
    
    echo -e "\n${GREEN}🎉 All integration tests passed!${NC}"
    echo "=============================================="
}

# Run main function
main "$@"