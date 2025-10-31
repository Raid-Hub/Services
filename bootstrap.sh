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

# Function to verify and add missing keys from example.env to .env
verify_env_keys() {
    if [ ! -f example.env ]; then
        echo "⚠️  example.env not found, skipping key verification"
        return
    fi
    
    if [ ! -f .env ]; then
        echo "⚠️  .env not found, skipping key verification"
        return
    fi
    
    missing_keys=()
    missing_values=()
    added_count=0
    
    # Extract keys from example.env and check/add to .env
    while IFS='=' read -r key value; do
        # Skip comments and empty lines
        [[ "$key" =~ ^# ]] && continue
        [[ -z "$key" ]] && continue
        
        # Remove leading/trailing whitespace
        key=$(echo "$key" | xargs)
        value="${value%$'\r'}"  # Remove carriage return if present
        value=$(echo "$value" | xargs)  # Trim whitespace
        
        # Skip if value in example.env is empty (don't add empty keys)
        if [ -z "${value}" ]; then
            continue
        fi
        
        # Check if key exists in .env (regardless of value)
        grep_result=$(grep "^${key}=" .env 2>/dev/null || echo "")
        
        if [ -z "$grep_result" ]; then
            # Key doesn't exist at all, add it
            missing_keys+=("$key")
            missing_values+=("${key}=${value}")
            added_count=$((added_count + 1))
        fi
        # If key exists (even with empty value), skip it
    done < example.env
    
    if [ ${#missing_keys[@]} -gt 0 ]; then
        # Add timestamp comment before adding keys
        echo "" >> .env
        echo "# Keys automatically added by bootstrap.sh on $(date)" >> .env
        for entry in "${missing_values[@]}"; do
            echo "$entry" >> .env
        done
        
        echo "📝 Added ${added_count} missing key(s) to .env"
        for key in "${missing_keys[@]}"; do
            echo "   + $key"
        done
    else
        echo "✅ All keys from example.env are present in .env"
    fi
}

# Check if .env exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from example.env..."
    cp example.env .env
    echo "✅ .env file created"
    echo ""
else
    echo "✅ .env file already exists"
fi

# Verify all keys from example.env are present in .env
verify_env_keys
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

# Generate service configurations from .env
echo "🔧 Generating service configurations..."
./infrastructure/generate-configs.sh 2>&1
echo ""

# Start infrastructure services (postgres, clickhouse, rabbitmq)
echo "🐳 Starting containerized infrastructure services..."
docker-compose -f docker-compose.yml --env-file ./.env up -d
echo "✅ Infrastructure services started"
echo ""

# Build all applications and tools
echo "🔨 Building applications and tools..."
if ! make 2>&1; then
    exit_code=$?
    echo "❌ Build failed with exit code $exit_code"
    exit 1
fi
echo ""

# Wait for databases to be ready
echo "⏳ Waiting for databases to be ready..."
max_attempts=30
attempt=0

# Wait for PostgreSQL
while [ $attempt -lt $max_attempts ]; do
    if docker-compose -f docker-compose.yml --env-file ./.env exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then
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
    if docker-compose -f docker-compose.yml --env-file ./.env ps clickhouse | grep -q "Up" && \
       docker-compose -f docker-compose.yml --env-file ./.env exec -T clickhouse clickhouse-client --query "SELECT 1" > /dev/null 2>&1; then
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

# Run manifest downloader to populate definitions
echo ""
echo "🔮 Running manifest downloader to populate weapon and feat definitions..."
if ./bin/manifest-downloader --out="./.raidhub/defs" -f; then
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
echo "📚 Useful commands:"
echo ""
echo "🚀 Development:"
echo "   ▶️  Start dev:    make dev        (Tilt UI at http://localhost:10350)"
echo "   🛑 Stop dev:      make down"
echo ""
echo "🔧 Service Management:"
echo "   🗺️  Run Atlas Crawler:           ./bin/atlas --workers 10"
echo "   🗺️  Run Async Queue Worker:      ./bin/hermes"
echo ""
echo "🗄️  Database:"
echo "   🔄 Migrate:       make migrate"
echo "   🌱 Seed:          make seed"
echo ""
echo "🛠️  Investigation:"
echo "   📊 Service logs:  docker-compose logs -f <service>"
echo "   🧹 Clean:         make clean      (removes volumes and data)"
echo ""
echo "⚠️  IMPORTANT: Set your BUNGIE_API_KEY in .env"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""


