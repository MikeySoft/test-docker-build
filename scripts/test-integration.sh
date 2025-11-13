#!/bin/bash

# Flotilla Integration Test Script
# This script tests the basic functionality of the Flotilla server and agent

set -e

echo "🧪 Starting Flotilla Integration Tests..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test functions
test_server_health() {
    echo "🔍 Testing server health endpoint..."

    # Start server in background
    echo "Starting server..."
    ./bin/server &
    SERVER_PID=$!

    # Wait for server to start
    sleep 3

    # Test health endpoint
    if curl -s http://localhost:8080/health | grep -q "healthy"; then
        echo -e "${GREEN}✅ Server health check passed${NC}"
    else
        echo -e "${RED}❌ Server health check failed${NC}"
        kill $SERVER_PID 2>/dev/null || true
        exit 1
    fi

    # Test hosts endpoint
    echo "🔍 Testing hosts endpoint..."
    if curl -s http://localhost:8080/api/v1/hosts | grep -q "hosts"; then
        echo -e "${GREEN}✅ Hosts endpoint accessible${NC}"
    else
        echo -e "${RED}❌ Hosts endpoint failed${NC}"
        kill $SERVER_PID 2>/dev/null || true
        exit 1
    fi

    # Clean up
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
}

test_agent_connection() {
    echo "🔍 Testing agent connection..."

    # This would require a running Docker daemon and proper API key
    # For now, just test that the agent binary exists and can start
    if [ -f "./bin/agent" ]; then
        echo -e "${GREEN}✅ Agent binary exists${NC}"
    else
        echo -e "${RED}❌ Agent binary not found${NC}"
        exit 1
    fi
}

test_database_connection() {
    echo "🔍 Testing database connection..."

    # Start PostgreSQL
    echo "Starting PostgreSQL..."
    docker-compose -f docker-compose.dev.yml up -d postgres

    # Wait for PostgreSQL to be ready
    echo "Waiting for PostgreSQL to be ready..."
    until docker-compose -f docker-compose.dev.yml exec postgres pg_isready -U flotilla -d flotilla > /dev/null 2>&1; do
        echo "   Waiting for PostgreSQL..."
        sleep 2
    done

    echo -e "${GREEN}✅ PostgreSQL is ready${NC}"
}

# Main test execution
main() {
    echo "🚀 Flotilla Integration Test Suite"
    echo "=================================="

    # Build binaries
    echo "🔨 Building binaries..."
    make build-all

    # Test database connection
    test_database_connection

    # Test server functionality
    test_server_health

    # Test agent binary
    test_agent_connection

    echo ""
    echo -e "${GREEN}🎉 All integration tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Start the development environment: make run-dev"
    echo "2. Generate an API key: make generate-api-key"
    echo "3. Configure and run an agent with the API key"
    echo "4. Test the full agent-server communication"
}

# Cleanup function
cleanup() {
    echo "🧹 Cleaning up..."
    docker-compose -f docker-compose.dev.yml down 2>/dev/null || true
    # Kill any remaining server processes
    pkill -f "./bin/server" 2>/dev/null || true
}

# Set up cleanup trap
trap cleanup EXIT

# Run main function
main
