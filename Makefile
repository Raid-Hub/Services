default: apps tools

.PHONY: seed apps tools migrate
# Go Binaries
GO_BUILD = go build
BIN_DIR = ./bin/
APPS_DIR = ./apps/
TOOLS_DIR = ./tools/

apps:
	mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR) $(APPS_DIR)...

tools:
	mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR) $(TOOLS_DIR)...

# Docker Services
DOCKER_COMPOSE = docker-compose -f docker-compose.yml --env-file ./.env

# Development Commands (Recommended)
dev:
	@echo "Starting RaidHub-Services development environment..."
	@echo "This provides hot reload, service orchestration, and monitoring"
	@echo "Access Tilt UI at: http://localhost:10350"
	@echo "Press Ctrl+C to stop"
	tilt up

up:
	$(DOCKER_COMPOSE) up -d

down:
	$(DOCKER_COMPOSE) down

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
			go run ./infrastructure/postgres/migrate/; \
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
	go run ./tools/seed/


cron:
	crontab ./infrastructure/cron/prod.crontab


config:
	./infrastructure/generate-configs.sh

clean:
	rm -rf $(BIN_DIR)
	rm -rf volumes/
