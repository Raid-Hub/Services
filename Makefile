.PHONY: seed tools migrate compose env rebuild-apps rebuild-app recreate-apps recreate-app apps config sync-dashboards infra
# Go Binaries (optional - for production tool binaries)
GO_BUILD = go build
BIN_DIR = ./bin/
TOOLS_DIR = ./tools/

tools:
	mkdir -p $(BIN_DIR)
	$(GO_BUILD) -o $(BIN_DIR) $(TOOLS_DIR)...

# Docker Services
DOCKER_COMPOSE = docker compose -f docker-compose.yml --env-file ./.env

# Development Commands (Recommended)
dev:
	@echo "Starting RaidHub-Services development environment..."
	@echo "This provides hot reload, service orchestration, and monitoring"
	@echo "Access Tilt UI at: http://localhost:10350"
	@echo "Press Ctrl+C to stop"
	tilt up --watch

up:
	$(DOCKER_COMPOSE) up -d

infra:
	@echo "Starting infrastructure services..."
	$(DOCKER_COMPOSE) up -d postgres rabbitmq clickhouse prometheus loki promtail grafana

down:
	$(DOCKER_COMPOSE) down

restart:
	$(DOCKER_COMPOSE) restart

stop:
	$(DOCKER_COMPOSE) stop

ps:
	$(DOCKER_COMPOSE) ps

# Rebuild and recreate app containers
apps:
	$(DOCKER_COMPOSE) build hermes atlas zeus
	$(DOCKER_COMPOSE) up -d --force-recreate hermes atlas zeus

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
	@echo "✅ Configuration files are now static - no generation needed"

# Sync Grafana dashboards from UI back to infrastructure files
sync-dashboards:
	@echo "📥 Syncing Grafana dashboards..."
	@./infrastructure/grafana/sync-dashboards.sh

# Environment file management
env:
	@if [ ! -f .env ]; then \
		echo "📝 Creating .env file from example.env..."; \
		cp example.env .env; \
		echo "✅ .env file created"; \
	else \
		echo "✅ .env file already exists"; \
	fi
	@if [ -f example.env ] && [ -f .env ]; then \
		missing_keys=(); \
		missing_values=(); \
		added_count=0; \
		while IFS='=' read -r key value || [ -n "$$key" ]; do \
			key=$$(echo "$$key" | xargs); \
			[[ "$$key" =~ ^# ]] && continue; \
			[[ -z "$$key" ]] && continue; \
			value=$${value%$$'\r'}; \
			value=$$(echo "$$value" | xargs); \
			if [ -z "$$value" ]; then \
				continue; \
			fi; \
			grep_result=$$(grep "^$$key=" .env 2>/dev/null || echo ""); \
			if [ -z "$$grep_result" ]; then \
				missing_keys+=("$$key"); \
				missing_values+=("$$key=$$value"); \
				added_count=$$((added_count + 1)); \
			fi; \
		done < example.env; \
		if [ $${#missing_keys[@]} -gt 0 ]; then \
			echo "" >> .env; \
			echo "# Keys automatically added by make env on $$(date)" >> .env; \
			for entry in "$${missing_values[@]}"; do \
				echo "$$entry" >> .env; \
			done; \
			echo "📝 Added $$added_count missing key(s) to .env"; \
			for key in "$${missing_keys[@]}"; do \
				echo "   + $$key"; \
			done; \
		else \
			echo "✅ All keys from example.env are present in .env"; \
		fi; \
	fi

clean:
	$(DOCKER_COMPOSE) down
	rm -rf $(BIN_DIR) volumes/ logs/ .raidhub/