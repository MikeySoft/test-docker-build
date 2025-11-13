#!/bin/bash

# Flotilla Development Environment Startup Script

set -e

echo "🚀 Starting Flotilla Development Environment..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if .env file exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from template..."
    cp env.example .env
    echo "✅ Created .env file. You may need to modify it for your environment."
fi

# Start PostgreSQL
echo "🐘 Starting PostgreSQL..."
docker-compose -f docker-compose.dev.yml up -d

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
until docker-compose -f docker-compose.dev.yml exec postgres pg_isready -U flotilla -d flotilla > /dev/null 2>&1; do
    echo "   Waiting for PostgreSQL..."
    sleep 2
done

echo "✅ PostgreSQL is ready!"

# Build the server
echo "🔨 Building management server..."
make build-server

# Start the server
echo "🌐 Starting management server..."
echo "   Server will be available at: http://localhost:8080"
echo "   Health check: http://localhost:8080/health"
echo ""
echo "Press Ctrl+C to stop the development environment"
echo ""

# Start the server in the foreground
./bin/server
