default: bin

# Docker Services
DOCKER_COMPOSE = docker-compose -f docker-compose.yml --env-file ./.env

services:
	$(DOCKER_COMPOSE) up -d 

up:
	$(DOCKER_COMPOSE) up -d postgres rabbitmq clickhouse

down:
	$(DOCKER_COMPOSE) down

# Single service
postgres:
	$(DOCKER_COMPOSE) up -d postgres

rabbit:
	$(DOCKER_COMPOSE) up -d rabbitmq
	
clickhouse:
	$(DOCKER_COMPOSE) up -d clickhouse

## Optional services
prometheus:
	$(DOCKER_COMPOSE) up -d prometheus


# Go Binaries
GO_BUILD = go build
BINARIES = -o ./bin/
COMMANDS = ./cmd/

.PHONY: bin
bin:
	$(GO_BUILD) $(BINARIES) $(COMMANDS)...

# Build a specific cmd with make <name>
.DEFAULT:
	$(GO_BUILD) $(BINARIES)$@ $(COMMANDS)$@
	