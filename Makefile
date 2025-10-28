default: apps tools


# Go Binaries
GO_BUILD = go build

# Build from new locations (apps/)
BIN_DIR = ./bin/
APPS_DIR = ./apps/
TOOLS_DIR = ./tools/

.PHONY: seed apps tools
apps:
	@echo "Building all apps..."
	@mkdir -p $(BIN_DIR)
	@$(GO_BUILD) -o $(BIN_DIR) $(APPS_DIR)...
	@echo "✅ All apps built"

tools:
	@echo "Building tools..."
	@mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR) $(TOOLS_DIR)
	@echo "✅ All tools built"

# Docker Services
DOCKER_COMPOSE = docker-compose -f docker-compose.yml --env-file ./.env

# Development Commands (Recommended)
dev:
	@echo "Starting RaidHub-Services development environment..."
	@echo "This provides hot reload, service orchestration, and monitoring"
	@echo "Access Tilt UI at: http://localhost:10350"
	@echo "Press Ctrl+C to stop"
	@tilt up

up:
	$(DOCKER_COMPOSE) up -d

down:
	@echo "Stopping services..."
	@$(DOCKER_COMPOSE) down
	@echo "✅ All services stopped"

build: apps

restart:
	$(DOCKER_COMPOSE) restart

stop:
	$(DOCKER_COMPOSE) stop

ps:
	$(DOCKER_COMPOSE) ps

# Database management
migrate: migrate-postgres migrate-clickhouse
	@echo "✓ All migrations complete"

migrate-%:
	@case "$*" in \
		postgres) \
			echo "Running PostgreSQL migrations..."; \
			go run ./infrastructure/postgres/tools/migrate/; \
			;; \
		clickhouse) \
			echo "Running ClickHouse migrations..."; \
			go run ./infrastructure/clickhouse/tools/migrate/; \
			;; \
		*) \
			echo "Unknown database: $*"; \
			echo "Available options: postgres, clickhouse"; \
			exit 1; \
			;; \
	esac

seed:
	go run ./infrastructure/postgres/tools/seed/

# Cron management
cron:
	@echo "Installing cron jobs..."
	@crontab ./infrastructure/cron/prod.crontab
	@echo "Cron jobs installed successfully"


# Configuration management
config:
	@echo "🔧 Generating service roles and permissions..."
	./infrastructure/generate-configs.sh
	@echo "✅ Database roles and permissions updated"

