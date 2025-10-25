# RaidHub-Services Tiltfile for Local Development
# This Tiltfile orchestrates all microservices and infrastructure for Destiny 2 data processing

# =============================================================================
# CONFIGURATION AND SETUP
# =============================================================================

# Load environment variables
load("ext://dotenv", "dotenv")
dotenv(".env")


# Shared environment configuration
def get_shared_env():
    return {
        "POSTGRES_HOST": "localhost",
        "POSTGRES_PORT": "5432",
        "POSTGRES_USER": os.getenv("POSTGRES_USER", "username"),
        "POSTGRES_PASSWORD": os.getenv("POSTGRES_PASSWORD", "password"),
        "POSTGRES_DB": "raidhub",
        "RABBITMQ_HOST": "localhost",
        "RABBITMQ_PORT": "5672",
        "RABBITMQ_USER": os.getenv("RABBITMQ_USER", "guest"),
        "RABBITMQ_PASSWORD": os.getenv("RABBITMQ_PASSWORD", "guest"),
        "CLICKHOUSE_HOST": "localhost",
        "CLICKHOUSE_PORT": "9000",
        "CLICKHOUSE_USER": os.getenv("CLICKHOUSE_USER", "default"),
        "CLICKHOUSE_PASSWORD": os.getenv("CLICKHOUSE_PASSWORD", ""),
        "BUNGIE_API_KEY": os.getenv("BUNGIE_API_KEY"),
    }


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
dc_resource("grafana", labels=["monitoring", "dashboard"])

# Networking Services
dc_resource("cloudflared", labels=["networking", "tunnel"], auto_init=False)

# =============================================================================
# CORE MICROSERVICES (Always Running)
# =============================================================================
# These services are essential for the application to function

local_resource(
    "hermes",
    cmd="go run ./apps/hermes",
    deps=["postgres", "rabbitmq"],
    env=get_shared_env(),
    resource_deps=["postgres", "rabbitmq", "build-binaries"],
    labels=["core-service", "microservice"],
)

local_resource(
    "atlas",
    cmd="go run ./apps/atlas",
    deps=["postgres", "rabbitmq", "clickhouse"],
    env=get_shared_env(),
    resource_deps=["postgres", "rabbitmq", "clickhouse", "build-binaries"],
    labels=["core-service", "microservice"],
)

local_resource(
    "zeus",
    cmd="go run ./apps/zeus",
    env={
        "BUNGIE_API_KEY": os.getenv("BUNGIE_API_KEY"),
        "ZEUS_API_KEYS": os.getenv("ZEUS_API_KEYS", ""),
        "IPV6": os.getenv("IPV6", ""),
    },
    resource_deps=["build-binaries"],
    labels=["core-service", "microservice", "proxy"],
)

# =============================================================================
# DEVELOPMENT TOOLS AND UTILITIES
# =============================================================================
# Tools for maintenance, debugging, and development tasks
local_resource(
    "tools",
    cmd="go run ./tools",
    deps=["postgres", "rabbitmq"],
    env=get_shared_env(),
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
# SERVICE ACCESS AND PORT FORWARDING
# =============================================================================
# Easy access to all service endpoints and monitoring interfaces
local_resource(
    "service-urls",
    cmd='echo "Service URLs:" && echo "PostgreSQL: localhost:5432" && echo "RabbitMQ UI: http://localhost:15672" && echo "ClickHouse: localhost:9000/8123" && echo "Prometheus: http://localhost:9090" && echo "Grafana: http://localhost:3000" && echo "Hermes Metrics: http://localhost:8083/metrics" && echo "Atlas Metrics: http://localhost:8080/metrics" && echo "Zeus Proxy: http://localhost:7777"',
    labels=["service-access", "urls", "port-forwarding"],
)

# =============================================================================
# DATABASE AND MAINTENANCE COMMANDS
# =============================================================================
# Database migrations, builds, and cleanup operations
local_resource(
    "migrate-db",
    cmd="go run ./infrastructure/postgres/tools/migrate.go",
    deps=["postgres"],
    auto_init=False,
    labels=["database", "migration", "maintenance"],
)

# Build all services command
local_resource(
    "build-all", cmd="make bin", auto_init=False, labels=["build", "maintenance"]
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
# - For cron services, run them manually: go run ./apps/hades, etc.
#
# Prerequisites:
# - Docker and Docker Compose installed
# - Go 1.21+ installed
# - .env file configured with required variables
# - BUNGIE_API_KEY must be set for API access
