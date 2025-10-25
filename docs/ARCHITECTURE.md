# RaidHub Services Architecture

## Overview

RaidHub Services is a microservices architecture built in Go, managing data collection, processing, and analysis for Destiny 2 raid completion tracking.

## Folder Structure

```
RaidHub-Services/
├── apps/                        # Main application services
│   ├── atlas/                   # PGCR crawler
│   ├── hermes/                  # Queue worker manager
│   ├── hades/                   # Missed PGCR collector
│   ├── athena/                  # Manifest downloader
│   ├── zeus/                    #
│   ├── bob/                     #
│   ├── hera/                    #
│   ├── nemesis/                 # Cheat detection service
│   └── pan/                     # Player analytics
├── queue-workers/               # Background processing workers
│   ├── activity_history/        # Activity history processor
│   ├── character_fill/          # Character data filler
│   ├── clan_crawl/              # Clan crawler
│   ├── pgcr_blocked/            # Blocked PGCR handler
│   ├── pgcr_cheat_check/        # Cheat check processor
│   ├── pgcr_clickhouse/         # ClickHouse writer
│   ├── pgcr_exists/             # PGCR existence checker
│   └── player_crawl/            # Player data crawler
├── tools/                       # Utilities and one-off tools
│   ├── main.go                 # Command dispatcher
│   ├── activity-history-update/ # Activity history updater
│   ├── fix-sherpa-clears/      # Sherpa clears fixer
│   ├── flag-restricted-pgcrs/  # PGCR flagger
│   ├── process-single-pgcr/    # Single PGCR processor
│   └── update-skull-hashes/    # Skull hash updater
├── shared/                      # Shared libraries
│   ├── config/                  # Configuration management
│   ├── database/                # Database connection logic
│   │   ├── postgres/            # PostgreSQL connection
│   │   └── clickhouse/          # ClickHouse connection
│   ├── messaging/               # Message queue libraries
│   │   └── rabbit/              # RabbitMQ connection
│   ├── monitoring/              # Monitoring and metrics
│   └── utils/                   # Common utilities
├── infrastructure/              # Infrastructure tooling (NO app code)
│   ├── postgres/                # PostgreSQL infrastructure
│   │   ├── schemas/             # Database schemas
│   │   ├── migrations/          # Migration files
│   │   ├── seeds/               # Seed data
│   │   ├── views/               # Database views
│   │   └── tools/               # Migration tools
│   ├── clickhouse/              # ClickHouse infrastructure
│   │   ├── migrations/          # ClickHouse migrations
│   │   ├── seeds/               # Seed data
│   │   ├── views/               # ClickHouse views
│   │   └── tools/               # Migration tools
│   ├── cron/                    # Cron job infrastructure
│   │   ├── crontab/             # Crontab definitions
│   │   └── scripts/             # Cron job scripts
│   ├── cloudflared/             # Cloudflare tunnel config
│   ├── prometheus/              # Prometheus config
│   └── grafana/                 # Grafana dashboards
├── docker/                      # Docker configs
├── docs/                        # Documentation
├── bin/                         # Built binaries
├── volumes/                     # Docker volumes
└── logs/                        # Log files
```

## Key Architectural Principles

### 1. Clear Separation of Concerns

- **`apps/`** - Main application services
- **`queue-workers/`** - Background processing workers
- **`tools/`** - Utilities and one-off tools
- **`shared/`** - Shared libraries used by apps
- **`infrastructure/`** - Infrastructure tooling (NO app code)

### 2. Infrastructure vs App Code

- **`infrastructure/`** contains ONLY tooling and configuration
  - Database schemas, migrations, seeds, views
  - Cron job definitions and wrapper scripts
  - Service configuration files
  - Migration and seeding tools
- **`shared/`** contains ONLY application code libraries
  - Database connection logic
  - Message queue connection logic
  - Configuration management
  - Monitoring libraries
  - Common utilities

### 3. Database Organization

- **PostgreSQL**: Schemas, migrations, seeds, views in `infrastructure/postgres/`
- **ClickHouse**: Migrations, seeds, views in `infrastructure/clickhouse/`
- Connection logic in `shared/database/` for use by apps
- Separate migrate/seed tools for each database

### 4. Cron Job Management

- Crontab definitions in `infrastructure/cron/crontab/`
- Wrapper scripts in `infrastructure/cron/scripts/`
- Environment-specific crontabs (dev/staging/prod)
- No hardcoded credentials - use environment variables

## Services

### Main Applications (`apps/`)

#### Atlas

PGCR (Post-Game Carnage Report) crawler service that fetches and processes raid completion data from the Bungie API.

#### Hermes

Queue worker manager that coordinates all background processing workers.

#### Hades

Missed PGCR collector that handles retrieval of PGCs that were missed during initial crawling.

#### Athena

Manifest downloader that fetches and processes Destiny 2 manifest data.

#### Nemesis

Cheat detection service that analyzes player behavior to detect potential cheating.

#### Pan

Player analytics service for aggregating player statistics.

### Queue Workers (`queue-workers/`)

Background processing workers that handle asynchronous tasks:

- **activity_history**: Processes player activity history
- **character_fill**: Fills missing character data
- **clan_crawl**: Crawls clan information
- **pgcr_blocked**: Handles blocked PGCRs
- **pgcr_cheat_check**: Performs cheat checking on PGCRs
- **pgcr_clickhouse**: Writes PGCR data to ClickHouse
- **pgcr_exists**: Checks if PGCRs exist in the database
- **player_crawl**: Crawls player data

## Infrastructure

### Docker Services

- **PostgreSQL**: Primary relational database
- **RabbitMQ**: Message queue for async processing
- **ClickHouse**: Analytics database
- **Prometheus**: Metrics collection
- **Grafana**: Monitoring dashboards
- **Cloudflared**: Secure tunnel for remote access

### Database Management

- Migrations are managed through custom Go tools
- Schemas define table structures
- Seeds populate initial data
- Views provide analytical queries

### Monitoring

- Prometheus collects metrics from all services
- Grafana provides dashboards for visualization
- Custom metrics track PGCR crawling, queue processing, and system health

## Development Workflow

### Setup

1. Run `./bootstrap.sh` to set up the environment
2. Update `.env` with your configuration
3. Services will be available via Docker Compose

### Building

```bash
make bin        # Build all binaries
make <service>  # Build a specific service
make dev        # Build and start all services
```

### Running

```bash
make up         # Start required services (postgres, rabbitmq, clickhouse)
make services   # Start all services
make logs       # View logs
make down       # Stop all services
```

### Database

```bash
make migrate    # Run database migrations (includes schemas and seeds)
```

## Environment Variables

See `example.env` for all required environment variables.

Key variables:

- `BUNGIE_API_KEY`: Bungie API key for data access
- Database credentials (POSTGRES*\*, RABBITMQ*\_, CLICKHOUSE\_\_)
- Monitoring credentials (PROMETHEUS*\*, GRAFANA*\*)
- Cloudflare tunnel configuration

## Future Improvements

The architecture is designed to support future enhancements:

1. Enhanced monitoring and alerting
2. Additional service integrations
3. Performance optimizations
4. Extended queue worker capabilities

## Contributing

1. Follow the architecture principles
2. Keep infrastructure and app code separate
3. Document new services and workers
4. Update this document with significant changes
