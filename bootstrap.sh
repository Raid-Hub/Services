#!/bin/bash

# RaidHub Services Bootstrap Script
# This script sets up the development environment and installs dependencies
# It is as idempotent as possible, so it can be run multiple times without issues

set -e

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  🚀 RaidHub Services Bootstrap"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Function to detect OS
detect_os() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "macos"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "linux"
    else
        echo "unknown"
    fi
}

# Function to install Go
install_go() {
    local os=$(detect_os)
    echo "📦 Installing Go..."
    
    if [[ "$os" == "macos" ]]; then
        if command -v brew &> /dev/null; then
            brew install go
        else
            echo "❌ Homebrew not found. Please install Go manually from https://golang.org/dl/"
            exit 1
        fi
    elif [[ "$os" == "linux" ]]; then
        # Try package manager first
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y golang-go
        elif command -v yum &> /dev/null; then
            sudo yum install -y golang
        elif command -v dnf &> /dev/null; then
            sudo dnf install -y golang
        else
            echo "❌ Package manager not found. Please install Go manually from https://golang.org/dl/"
            exit 1
        fi
    else
        echo "❌ Unsupported OS. Please install Go manually from https://golang.org/dl/"
        exit 1
    fi
    
    echo "✅ Go installed"
}

# Function to install Docker
install_docker() {
    local os=$(detect_os)
    echo "📦 Installing Docker..."
    
    if [[ "$os" == "macos" ]]; then
        if command -v brew &> /dev/null; then
            brew install --cask docker
            echo "⚠️  Please start Docker Desktop manually after installation"
        else
            echo "❌ Homebrew not found. Please install Docker Desktop manually from https://www.docker.com/products/docker-desktop/"
            exit 1
        fi
    elif [[ "$os" == "linux" ]]; then
        # Install Docker using official script
        curl -fsSL https://get.docker.com -o get-docker.sh
        sudo sh get-docker.sh
        sudo usermod -aG docker $USER
        rm get-docker.sh
        echo "⚠️  Please log out and back in for Docker group changes to take effect"
    else
        echo "❌ Unsupported OS. Please install Docker manually from https://www.docker.com/products/docker-desktop/"
        exit 1
    fi
    
    echo "✅ Docker installed"
}

# Function to install Tilt
install_tilt() {
    echo "📦 Installing Tilt..."
    curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash
    echo "✅ Tilt installed"
}

# Setup .env file using make env
echo "📋 Setting up .env file..."
make env
echo ""

echo "📋 Checking dependencies..."
echo ""

# Check and install Go
if ! command -v go &> /dev/null; then
    echo "❌ Go not found"
    install_go
else
    echo "✅ Go is installed"
fi

# Check and install Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker not found"
    install_docker
else
    echo "✅ Docker is installed"
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running"
    echo "   On macOS: Start Docker Desktop"
    echo "   On Linux: sudo systemctl start docker"
    exit 1
else
    echo "✅ Docker is running"
fi

# Function to detect docker-compose command (standalone or plugin)
detect_docker_compose() {
    if command -v docker-compose &> /dev/null; then
        echo "docker-compose"
    elif docker compose version > /dev/null 2>&1; then
        echo "docker compose"
    else
        echo ""
    fi
}

# Check for docker-compose (standalone or plugin)
DOCKER_COMPOSE_CMD=$(detect_docker_compose)
if [ -z "$DOCKER_COMPOSE_CMD" ]; then
    echo "❌ docker-compose not found"
    echo "   Docker Compose V2 (plugin) is recommended: https://docs.docker.com/compose/install/"
    echo "   Or install standalone: https://docs.docker.com/compose/install/standalone/"
    exit 1
else
    echo "✅ docker-compose is available ($DOCKER_COMPOSE_CMD)"
fi

# Check and install Tilt
if ! command -v tilt &> /dev/null; then
    echo "❌ Tilt not found"
    install_tilt
else
    echo "✅ Tilt is installed"
fi

# Create necessary directories
echo ""
echo "📁 Creating necessary directories..."
mkdir -p volumes logs bin
echo "✅ Directories created"
echo ""

# Configuration files are now static - no generation needed
echo "✅ Using static configuration files"
echo ""

# Stop any running services
echo "🛑 Stopping any running services..."
$DOCKER_COMPOSE_CMD down
echo "✅ Services stopped"
echo ""

# Start infrastructure services (postgres, clickhouse, rabbitmq, prometheus, loki, promtail, grafana)
# Exclude app services (atlas, hermes, zeus)
echo "🐳 Starting containerized infrastructure services..."
$DOCKER_COMPOSE_CMD --env-file ./.env up -d postgres rabbitmq clickhouse prometheus loki promtail grafana
echo "✅ Infrastructure services started"
echo ""

# Build app services (but don't start them)
echo "🔨 Building app services (atlas, hermes)..."
$DOCKER_COMPOSE_CMD --env-file ./.env build atlas hermes
echo "✅ App services built (not started)"
echo "ℹ️  App services (atlas, hermes) can be started with 'make dev' or manually."
echo ""

# Wait for databases to be ready
echo "⏳ Waiting for databases to be ready..."
max_attempts=30
attempt=0

# Wait for PostgreSQL
while [ $attempt -lt $max_attempts ]; do
    if $DOCKER_COMPOSE_CMD --env-file ./.env exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo "✅ PostgreSQL is ready"
        break
    fi
    attempt=$((attempt + 1))
    echo "  Waiting for PostgreSQL... ($attempt/$max_attempts)"
    sleep 2
done
echo ""

if [ $attempt -eq $max_attempts ]; then
    echo "❌ PostgreSQL failed to start within $max_attempts attempts"
    exit 1
fi

# Wait for ClickHouse
attempt=0
while [ $attempt -lt $max_attempts ]; do
    # Check if container is running and ClickHouse is responding
    if $DOCKER_COMPOSE_CMD --env-file ./.env ps clickhouse | grep -q "Up" && \
       $DOCKER_COMPOSE_CMD --env-file ./.env exec -T clickhouse clickhouse-client --query "SELECT 1" > /dev/null 2>&1; then
        echo "✅ ClickHouse is ready"
        break
    fi
    attempt=$((attempt + 1))
    echo "  Waiting for ClickHouse... ($attempt/$max_attempts)"
    sleep 2
done
echo ""

if [ $attempt -eq $max_attempts ]; then
    echo "❌ ClickHouse failed to start within $max_attempts attempts"
    exit 1
fi

# Run database migrations
echo ""
echo "🗄️  Running database migrations..."
if make migrate 2>&1; then
    echo "✅ Migrations completed"
else
    exit_code=$?
    echo "❌ Migrations failed with exit code $exit_code"
fi

# Run database seeding
echo ""
echo "🌱 Seeding database..."
if make seed 2>&1; then
    echo "✅ Seeding completed"
else
    exit_code=$?
    echo "⚠️  Seeding failed with exit code $exit_code (non-critical)"
fi

# Start zeus (needed for manifest downloader)
echo ""
echo "⚡ Starting Zeus API proxy (required for manifest downloader)..."
$DOCKER_COMPOSE_CMD --env-file ./.env up -d zeus
echo "✅ Zeus started"
echo ""

# Run manifest downloader to populate definitions
echo "🔮 Running manifest downloader to populate weapon and feat definitions..."
if go run ./tools/manifest-downloader -f; then
    echo "✅ Manifest downloader completed"
else
    exit_code=$?
    echo "⚠️  Manifest downloader failed with exit code $exit_code (non-critical)"
fi

# Final summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 Bootstrap complete!"
echo ""
echo "📚 How to run everything:"
echo ""
echo "🚀 Development (Recommended):"
echo "   ▶️  Start all services:    make dev        (Tilt UI at http://localhost:10350)"
echo "                              This starts infrastructure + apps with hot reload"
echo "   🛑 Stop all services:      make down"
echo ""
echo "🐳 Manual Docker Compose:"
echo "   ▶️  Start infrastructure:  make infra"
echo "   ▶️  Start Zeus:            docker compose up -d zeus"
echo "   ▶️  Start apps:            docker compose up -d atlas hermes"
echo "   ▶️  Start everything:      make up"
echo "   🛑 Stop everything:        make down"
echo ""
echo "🔧 Individual Services (Built Binaries):"
echo "   🗺️  Run Atlas Crawler:           ./bin/atlas --workers 10"
echo "   🗺️  Run Async Queue Worker:      ./bin/hermes"
echo "   ⚡ Run Zeus API Proxy:           ./bin/zeus"
echo ""
echo "🗄️  Database:"
echo "   🔄 Run migrations:    make migrate"
echo "   🌱 Seed database:     make seed"
echo ""
echo "🛠️  Investigation:"
echo "   📊 View logs:         docker compose logs -f <service>"
echo "   📋 List services:     docker compose ps"
echo "   🧹 Clean everything:  make clean      (removes volumes and data)"
echo ""
echo "⚠️  IMPORTANT: Set your BUNGIE_API_KEY in .env"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""


