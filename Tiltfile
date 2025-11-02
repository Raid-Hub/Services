# Simplified Tiltfile for viewing logs
# All services are managed via docker-compose, Tilt just provides a nice UI for logs

# Load docker-compose with matching project name
# Disable file watching to prevent rebuilds - we only want log viewing
docker_compose(
    "docker-compose.yml",
    project_name="raidhub",
)

watch_settings(ignore=['*', '!apps/**', '!lib/**'])

# Explicitly expose all services with correct names for log viewing
# Infrastructure services
dc_resource("postgres", labels=["database"])
dc_resource("rabbitmq", labels=["messaging"])
dc_resource("clickhouse", labels=["database"])
dc_resource("prometheus", labels=["metrics-logging"])
dc_resource("loki", labels=["metrics-logging"])
dc_resource("promtail", labels=["metrics-logging"])
dc_resource("grafana", labels=["metrics-logging"])

# Application services
dc_resource("atlas", labels=["app"])
dc_resource("zeus", labels=["app"])
dc_resource("hermes", labels=["app"]) 

# =============================================================================
# TOOLS
# =============================================================================
# Automatically discover and create local resources for all tools

tool_deps = ["postgres", "rabbitmq", "clickhouse"]

# Explicitly list all tools to avoid auto-discovery issues
tools = [
    "activity-history-update",
    "cheat-detection",
    "fix-sherpa-clears",
    "flag-restricted-pgcrs",
    "leaderboard-clan-crawl",
    "manifest-downloader",
    "seed",
    "update-skull-hashes",
]

for tool in tools:
    tool_path = "tools/" + tool
    local_resource(
        tool,
        cmd="go run ./%s" % tool_path,
        auto_init=False,
        labels=["tools"],
    )

views = [
    "individual_global_leaderboard",
    "individual_raid_leaderboard",
    "world_first_contest_leaderboard",
    "team_activity_version_leaderboard",
]
for view in views:
    local_resource(
        "refresh %s" % view,
        cmd="go run ./tools/refresh-view %s" % view,
        auto_init=False,
        labels=["views"],
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