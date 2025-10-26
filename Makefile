default: dev

# Docker Services
DOCKER_COMPOSE = docker-compose -f docker-compose.yml --env-file ./.env

# Development Commands (Recommended)
dev:
	@echo "Starting RaidHub-Services development environment..."
	@echo "This provides hot reload, service orchestration, and monitoring"
	@echo "Access Tilt UI at: http://localhost:10350"
	@echo "Press Ctrl+C to stop"
	@tilt up

dev-up: dev
up: dev

dev-down:
	@echo "Stopping development environment..."
	@tilt down
down: dev-down

dev-logs:
	@echo "Viewing development logs..."
	@tilt logs

dev-migrate:
	@echo "Running database migration..."
	@tilt trigger migrate-db

dev-build:
	@echo "Building all services..."
	@tilt trigger build-all

dev-clean:
	@echo "Cleaning build artifacts..."
	@tilt trigger clean


# Legacy development workflow (without Tilt)
dev-legacy: services-infra bin


# Docker Compose Services (for production/deployment)
services:
	$(DOCKER_COMPOSE) up -d

services-infra:
	$(DOCKER_COMPOSE) up -d postgres rabbitmq clickhouse

down:
	$(DOCKER_COMPOSE) stop

clean:
	$(DOCKER_COMPOSE) down -v
	rm -rf bin/
	rm -rf logs/*.log
	rm -rf volumes/

# Individual infrastructure services
infra-%:
	$(DOCKER_COMPOSE) up -d $(subst infra-,,$@)

build: bin

logs:
	$(DOCKER_COMPOSE) logs -f

restart:
	$(DOCKER_COMPOSE) restart

stop:
	$(DOCKER_COMPOSE) stop

ps:
	$(DOCKER_COMPOSE) ps

# Database management
migrate:
	go run ./infrastructure/postgres/tools/migrate/

seed:
	go run ./infrastructure/postgres/tools/seed/

# Cron management
cron:
	@echo "Installing cron jobs..."
	@crontab ./infrastructure/cron/prod.crontab
	@echo "Cron jobs installed successfully"

# Go Binaries
GO_BUILD = go build

# Build from new locations (apps/)
APPS_DIR = ./apps/
TOOLS_DIR = ./tools/

# Dynamically discover all services in apps/ directory
SERVICES = $(shell find $(APPS_DIR) -maxdepth 1 -type d -name '*' ! -name '.' | sed 's|$(APPS_DIR)||' | sort)

.PHONY: bin tools
bin:
	@echo "Building all services..."
	@mkdir -p bin
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		$(GO_BUILD) -o ./bin/$$service $(APPS_DIR)$$service; \
	done

tools:
	@echo "Building tools..."
	@mkdir -p bin
	$(GO_BUILD) -o ./bin/tools $(TOOLS_DIR)
	@echo "Tools built"

# Build individual service
%:
	@if [ -d "$(APPS_DIR)$@" ]; then \
		echo "Building $@..."; \
		$(GO_BUILD) -o ./bin/$@ $(APPS_DIR)$@; \
	else \
		echo "Service $@ not found in $(APPS_DIR)"; \
		exit 1; \
	fi

# Configuration management
config:
	@echo "🔧 Generating service roles and permissions..."
	./infrastructure/generate-configs.sh
	@echo "✅ Database roles and permissions updated"

# Help
help:
	@echo "RaidHub-Services Development Commands"
	@echo "====================================="
	@echo ""
	@echo "Development (Recommended):"
	@echo "  make dev           Start development environment with Tilt (interactive)"
	@echo "  make dev-down      Stop Tilt development environment"
	@echo "  make dev-logs      View Tilt logs"
	@echo "  make dev-migrate   Run database migration via Tilt"
	@echo "  make dev-build     Build all services via Tilt"
	@echo "  make dev-clean     Clean build artifacts via Tilt"
	@echo ""
	@echo "Legacy Development:"
	@echo "  make dev-legacy    Start services with Docker Compose (no hot reload)"
	@echo "  make build         Build all binaries"
	@echo "  make <service>     Build a specific service (e.g., make hermes)"
	@echo "  ./bin/hermes -topic <type>  Run a specific queue topic"
	@echo ""
	@echo "Docker Services:"
	@echo "  make services      Start all services"
	@echo "  make services-infra Start infrastructure services only"
	@echo "  make down          Stop all services (keeps containers)"
	@echo "  make logs          View logs"
	@echo "  make restart       Restart services"
	@echo "  make ps            View running services"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean         Remove containers, networks, volumes, and binaries"
	@echo ""
	@echo "Individual Infrastructure:"
	@echo "  make infra-postgres    Start PostgreSQL only"
	@echo "  make infra-rabbitmq    Start RabbitMQ only"
	@echo "  make infra-clickhouse  Start ClickHouse only"
	@echo "  make infra-prometheus  Start Prometheus only"
	@echo ""
	@echo "System Management:"
	@echo "  make cron             Install cron jobs from prod.crontab"
	@echo "  make config           Generate service roles and permissions"
	@echo ""
	@echo "For best development experience, use 'make dev'"
	@echo "Tilt UI will be available at: http://localhost:10350"
	