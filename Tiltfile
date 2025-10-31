# RaidHub-Services Tiltfile for Local Development
# This Tiltfile orchestrates all microservices and infrastructure for Destiny 2 data processing

# =============================================================================
# CONFIGURATION AND SETUP
# =============================================================================

# Load environment variables
load("ext://dotenv", "dotenv")
dotenv(".env")

# Docker Compose Services
docker_compose("docker-compose.yml")

# =============================================================================
# DOCKER SERVICES
# =============================================================================
# These services auto-start via docker_compose() above
# dc_resource() calls expose them in Tilt UI

dc_resource("postgres", labels=["infrastructure"])
dc_resource("rabbitmq", labels=["infrastructure"])
dc_resource("clickhouse", labels=["infrastructure"])
dc_resource("prometheus", labels=["infrastructure"])
dc_resource("loki", labels=["infrastructure"]) 
dc_resource("promtail", labels=["infrastructure"]) 
dc_resource("grafana", labels=["infrastructure"]) 

# =============================================================================
# HERMES / ATLAS / ZEUS
# =============================================================================

dc_resource("hermes", labels=["core"], resource_deps=["postgres", "rabbitmq", "clickhouse"]) 

dc_resource("atlas", labels=["core"], resource_deps=["postgres", "rabbitmq", "clickhouse"]) 

dc_resource("zeus", labels=["core"], resource_deps=["postgres"]) 

# =============================================================================
# CRON SERVICES
# =============================================================================



# =============================================================================
# TOOLS
# =============================================================================

local_resource(
    "tools",
    cmd="go run ./tools/",
    resource_deps=["postgres", "rabbitmq"],
    auto_init=False,
    labels=["tools"],
)

# =============================================================================
# DATABASE
# =============================================================================

local_resource(
    "migrate-postgres",
    cmd="make migrate-postgres",
    resource_deps=["postgres"],
    auto_init=False,
    labels=["database"],
)

local_resource(
    "migrate-clickhouse",
    cmd="make migrate-clickhouse",
    resource_deps=["clickhouse"],
    auto_init=False,
    labels=["database"],
)
# =============================================================================
# DEVELOPMENT NOTES
# =============================================================================

# Behavior:
# - Run 'tilt up' to automatically start Docker infrastructure (postgres, rabbitmq, clickhouse, prometheus)
# - Go services (hermes, atlas, zeus, etc.) are available but don't auto-start - manually start via Tilt UI
# - All Go services use 'go run' for hot reloading
# - Run 'tilt down' to stop all services
#
# Prerequisites:
# - Docker and Docker Compose installed
# - Go 1.21+ installed
# - .env file configured
