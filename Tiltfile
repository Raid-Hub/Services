# RaidHub-Services Tiltfile for Local Development
# This Tiltfile orchestrates all microservices and infrastructure for Destiny 2 data processing

# =============================================================================
# CONFIGURATION AND SETUP
# =============================================================================

# Load environment variables
load("ext://dotenv", "dotenv")
dotenv(".env")

# Development environment setup
local_resource(
    "dev-setup",
    cmd='echo "Setting up development environment..."',
    deps=[".env"],
    labels=["setup", "configuration"],
)

# Docker Compose Services
docker_compose("docker-compose.yml")

# Build all Go binaries first
local_resource(
    "build-binaries", cmd="make bin", deps=["dev-setup"], labels=["build", "setup"]
)

# =============================================================================
# INFRASTRUCTURE SERVICES (Docker Containers)
# =============================================================================
# Individual Docker Compose services with proper labeling

# Database Services
dc_resource("postgres", labels=["database", "infrastructure"])
dc_resource("rabbitmq", labels=["message-queue", "infrastructure"])
dc_resource("clickhouse", labels=["database", "infrastructure"])

# Monitoring Services
dc_resource("prometheus", labels=["monitoring", "metrics"])

# =============================================================================
# CORE MICROSERVICES (Always Running)
# =============================================================================
# These services are essential for the application to function

local_resource(
    "hermes",
    cmd="./bin/hermes",
    deps=["postgres", "rabbitmq"],
    resource_deps=["postgres", "rabbitmq", "build-binaries"],
    labels=["core-service", "microservice"],
)

local_resource(
    "atlas",
    cmd="./bin/atlas",
    deps=["postgres", "rabbitmq", "clickhouse"],
    resource_deps=["postgres", "rabbitmq", "clickhouse", "build-binaries"],
    labels=["core-service", "microservice"],
)

local_resource(
    "zeus",
    cmd="./bin/zeus",
    resource_deps=["build-binaries"],
    labels=["core-service", "microservice", "proxy"],
)

# =============================================================================
# CRON SERVICES (Manual Start)
# =============================================================================
# These services run on a schedule in production but can be started manually for development

local_resource(
    "hades",
    cmd="./bin/hades",
    resource_deps=["build-binaries", "postgres", "rabbitmq"],
    auto_init=False,
    labels=["cron-service", "maintenance"],
)

local_resource(
    "athena",
    cmd="./bin/athena",
    resource_deps=["build-binaries"],
    auto_init=False,
    labels=["cron-service", "manifest"],
)

local_resource(
    "hera",
    cmd="./bin/hera",
    resource_deps=["build-binaries", "postgres", "rabbitmq"],
    auto_init=False,
    labels=["cron-service", "maintenance"],
)

local_resource(
    "nemesis",
    cmd="./bin/nemesis",
    resource_deps=["build-binaries", "postgres", "rabbitmq"],
    auto_init=False,
    labels=["cron-service", "maintenance"],
)

# =============================================================================
# DEVELOPMENT TOOLS AND UTILITIES
# =============================================================================
# Tools for maintenance, debugging, and development tasks
local_resource(
    "tools",
    cmd="./bin/tools",
    deps=["postgres", "rabbitmq"],
    resource_deps=["postgres", "rabbitmq", "build-binaries"],
    auto_init=False,
    labels=["development", "tools", "utilities"],
)

# =============================================================================
# HEALTH MONITORING AND DIAGNOSTICS
# =============================================================================
# Health checks and monitoring for all infrastructure services
local_resource(
    "postgres-health",
    cmd="pg_isready -h localhost -p 5432 -U " + os.getenv("POSTGRES_USER", "username"),
    resource_deps=["postgres", "build-binaries"],
    labels=["health-check", "monitoring", "postgres"],
)

# Health check for RabbitMQ
local_resource(
    "rabbitmq-health",
    cmd="curl -f http://localhost:15672/api/overview || exit 1",
    resource_deps=["rabbitmq"],
    labels=["health-check", "monitoring", "rabbitmq"],
)

# Health check for ClickHouse
local_resource(
    "clickhouse-health",
    cmd="curl -f http://localhost:8123/ping || exit 1",
    resource_deps=["clickhouse"],
    labels=["health-check", "monitoring", "clickhouse"],
)

# =============================================================================
# DATABASE AND MAINTENANCE COMMANDS
# =============================================================================
# Database migrations, builds, and cleanup operations
local_resource(
    "migrate-db",
    cmd="make migrate",
    deps=["postgres"],
    auto_init=False,
    labels=["database", "migration", "maintenance"],
)


# Clean build artifacts
local_resource(
    "clean", cmd="rm -rf bin/", auto_init=False, labels=["cleanup", "maintenance"]
)

# =============================================================================
# ENVIRONMENT AND CONFIGURATION
# =============================================================================
# Environment variable management and configuration watching
local_resource(
    "env-watcher",
    cmd='echo "Watching .env file for changes..."',
    deps=[".env"],
    labels=["configuration", "environment", "watcher"],
)

# =============================================================================
# DEVELOPMENT NOTES
# =============================================================================

# This Tiltfile provides organized development environment with:
# 1. CONFIGURATION: Environment setup and binary building
# 2. INFRASTRUCTURE: Docker services (PostgreSQL, RabbitMQ, ClickHouse, etc.)
# 3. CORE SERVICES: Essential microservices (Hermes, Atlas, Zeus)
# 4. DEVELOPMENT TOOLS: Maintenance and debugging utilities
# 5. HEALTH MONITORING: Service health checks and diagnostics
# 6. SERVICE ACCESS: Port forwarding and URL management
# 7. DATABASE COMMANDS: Migrations, builds, and cleanup
# 8. ENVIRONMENT: Configuration management and watching
#
# Note: Background/cron services (Hades, Hera, Nemesis, Athena) are NOT included
# as they should be run via actual cron jobs, not continuously in development.
#
# Usage:
# - Run 'tilt up' to start all services
# - Use 'tilt down' to stop all services
# - Individual services can be started/stopped via Tilt UI
# - For cron services, run them manually: ./bin/hades, etc.
#
# Prerequisites:
# - Docker and Docker Compose installed
# - Go 1.21+ installed
# - .env file configured with required variables
# - BUNGIE_API_KEY must be set for API access
