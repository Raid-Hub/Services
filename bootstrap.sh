#!/bin/bash

# RaidHub Services Bootstrap Script
# This script sets up the development environment and installs dependencies
# It is as idempotent as possible, so it can be run multiple times without issues

set -e

echo "🚀 RaidHub Services Bootstrap"
echo "============================="
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

# Check if .env exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from example.env..."
    cp example.env .env
    echo "✅ .env file created. Please update it with your configuration."
    echo "⚠️  IMPORTANT: Edit .env and set your BUNGIE_API_KEY before continuing!"
else
    echo "✅ .env file already exists"
fi

# Check and install Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed."
    install_go
else
    echo "✅ Go is installed ($(go version))"
fi

# Check and install Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed."
    install_docker
else
    echo "✅ Docker is installed"
fi

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    echo "   On macOS: Start Docker Desktop"
    echo "   On Linux: sudo systemctl start docker"
    exit 1
fi

echo "✅ Docker is running"

# Check and install Tilt
if ! command -v tilt &> /dev/null; then
    echo "❌ Tilt is not installed."
    install_tilt
else
    echo "✅ Tilt is installed"
fi

# Create necessary directories
echo ""
echo "📁 Creating necessary directories..."
mkdir -p volumes
mkdir -p logs
mkdir -p bin
echo "✅ Directories created"

# Generate service configurations from .env
echo ""
echo "🔧 Generating service configurations..."
./infrastructure/generate-configs.sh
echo "✅ Service configurations generated"

# Build all applications and tools
echo ""
echo "🔨 Building applications and tools..."
make bin
make tools
echo "✅ All binaries built successfully"

# Run database migrations
echo ""
echo "🗄️  Running database migrations..."
go run ./infrastructure/postgres/tools/migrate.go
echo "✅ Database migrations completed"

# Verify binaries were created
echo ""
echo "🔍 Verifying build artifacts..."
if [ ! -f "./bin/hermes" ] || [ ! -f "./bin/atlas" ] || [ ! -f "./bin/zeus" ]; then
    echo "❌ Critical services failed to build"
    exit 1
fi

if [ ! -f "./bin/tools" ]; then
    echo "❌ Tools binary failed to build"
    exit 1
fi

echo "✅ All build artifacts verified"

# Stop any existing services and start fresh
echo ""
echo "🐳 Stopping any existing services..."
docker-compose -f docker-compose.yml --env-file ./.env down --remove-orphans 2>/dev/null || true

echo "🐳 Starting services with Docker Compose..."
docker-compose -f docker-compose.yml --env-file ./.env up -d
echo "✅ Services started successfully"

echo ""
echo "🎉 Bootstrap complete!"
echo "⚠️  Remember to set your BUNGIE_API_KEY in the .env file!"
echo "📊 You can view logs with: docker-compose logs -f"
echo "🛑 Stop services with: make down"

