# RaidHub Services

Microservices architecture for managing Destiny 2 raid completion data collection, processing, and analysis.

## Architecture

RaidHub Services follows a microservices architecture with clear separation between applications, queue workers, tools, and infrastructure.

### Main Components

- **Apps** (`apps/`): Main application services (Atlas, Hermes, Hades, Athena, etc.)
- **Queue Workers** (`queue-workers/`): Background processing workers for async tasks
- **Tools** (`tools/`): Utility scripts and one-off tools
- **Infrastructure** (`infrastructure/`): Database schemas, migrations, cron jobs, and service configs
- **Libraries** (`lib/`): Shared libraries for database connections, messaging, monitoring

## Documentation

- **[Architecture Overview](docs/ARCHITECTURE.md)** - Detailed architecture documentation
- **[Application Categorization](docs/APP_CATEGORIZATION.md)** - How apps are organized by execution model
- **[Naming Conventions](docs/NAMING_CONVENTIONS.md)** - Naming patterns for services and tools
- **[Logger Setup](docs/LOGGER_SETUP.md)** - How to set up logging in domain services

## Quick Start

### Prerequisites

- Docker Desktop
- Go 1.21+
- Make
- **Tilt** (for development) - Install from https://docs.tilt.dev/install.html

### Development Setup (Recommended)

The easiest way to get started with hot reload and service orchestration:

1. **Install Tilt** (if not already installed):

   ```bash
   # macOS
   brew install tilt-dev/tap/tilt
   # Or follow instructions at https://docs.tilt.dev/install.html
   ```

2. **Copy environment file:**

   ```bash
   cp example.env .env
   ```

3. **Edit `.env`** with your configuration values (especially `BUNGIE_API_KEY`)

4. **Start development environment:**
   ```bash
   make dev
   ```

This will:

- Start all infrastructure services (PostgreSQL, RabbitMQ, ClickHouse, Prometheus)
- Build and run Go services with hot reload
- Provide Tilt UI at http://localhost:10350 for monitoring
- Enable hot reload for all Go services when code changes

### Legacy Setup (Docker Compose Only)

For production-like setup without Tilt:

```bash
./bootstrap.sh
```

Or manually:

1. **Copy environment file:**

   ```bash
   cp example.env .env
   ```

2. **Edit `.env`** with your configuration values

3. **Start services:**

   ```bash
   make up
   ```

4. **Build binaries:**
   ```bash
   make apps
   ```

## Makefile Commands

### Development

```bash
make dev          # Start development environment with Tilt (hot reload)
make apps         # Build all application binaries
make tools        # Build all tool binaries
make build        # Build apps (alias for 'make apps')
```

### Docker Services

```bash
make up           # Start infrastructure services (postgres, rabbitmq, clickhouse, prometheus)
make down         # Stop all services
make restart      # Restart services
make stop         # Stop services
make ps           # View running services
```

### Database Management

```bash
make migrate      # Run all database migrations (postgres + clickhouse)
make migrate-postgres     # Run PostgreSQL migrations only
make migrate-clickhouse   # Run ClickHouse migrations only
make seed         # Populate seed data
```

### Configuration

```bash
make config       # Generate service configurations and database roles
make cron         # Install cron jobs from infrastructure/cron/prod.crontab
```

## Application Services

### Long-Running Services

These services run continuously and are managed via Docker Compose:

- **`hermes`** - Queue worker manager with self-scaling topics
- **`atlas`** - Intelligent PGCR crawler with adaptive scaling
- **`zeus`** - Bungie API reverse proxy with IPv6 load balancing

Start with: `make up` (starts infrastructure) + run binaries individually or via Tilt

### Scheduled Services (Cron Jobs)

These applications run on a schedule via system crontab:

- **`hera`** - Top player crawler for leaderboard maintenance
- **`hades`** - Missed PGCR recovery processor
- **`nemesis`** - Cheat detection and player account maintenance
- **`athena`** - Destiny 2 manifest downloader

Configure in: `infrastructure/cron/prod.crontab`

### Manual Tools

Utilities executed manually as needed:

- **`activity-history-update`** - Updates player activity history
- **`fix-sherpa-clears`** - Fixes sherpa and first clear data
- **`flag-restricted-pgcrs`** - Flags restricted PGCRs
- **`process-single-pgcr`** - Processes a single PGCR
- **`update-skull-hashes`** - Updates skull hashes

Execute via: `./bin/tools <command>`

## Running Services

### Development Mode (Recommended)

**With Tilt (hot reload, service orchestration):**

```bash
make dev    # Start all services with hot reload
```

Access services:

- **Tilt UI**: http://localhost:10350 (service monitoring and control)
- **Hermes**: http://localhost:8083/metrics (queue worker manager)
- **Atlas**: http://localhost:8080/metrics (PGCR crawler)
- **Zeus**: http://localhost:7777 (Bungie API proxy)

### Production Mode

**Infrastructure services:**

```bash
make up    # Start postgres, rabbitmq, clickhouse, prometheus
```

**Application binaries:**

```bash
# Build all binaries first
make apps

# Run long-running services
./bin/hermes     # Queue worker manager
./bin/atlas      # PGCR crawler
./bin/zeus       # Bungie API proxy
```

**Scheduled services** (via cron):

```bash
# Install cron jobs
make cron

# Or run manually
./bin/hera       # Top player crawl
./bin/hades      # Missed log processor
./bin/nemesis    # Cheat detection maintenance
./bin/athena     # Manifest downloader
```

**Manual tools:**

```bash
./bin/tools activity-history-update
./bin/tools fix-sherpa-clears
./bin/tools flag-restricted-pgcrs
./bin/tools process-single-pgcr
./bin/tools update-skull-hashes
```

## Available Services

### Infrastructure Services

- **PostgreSQL**: `localhost:5432` (Database)
- **RabbitMQ**: `localhost:5672` (AMQP), `localhost:15672` (Management UI)
- **ClickHouse**: `localhost:9000` (Native), `localhost:8123` (HTTP)
- **Prometheus**: `localhost:9090` (Metrics)

### Application Services

- **Hermes**: `localhost:8083/metrics` (Queue worker manager)
- **Atlas**: `localhost:8080/metrics` (PGCR crawler)
- **Zeus**: `localhost:7777` (Bungie API proxy)

### Development Tools

- **Tilt UI**: `localhost:10350` (Service monitoring and control)

## Project Structure

```
RaidHub-Services/
├── apps/              # Main application services
├── queue-workers/     # Background processing workers
├── tools/             # Utility scripts
├── lib/               # Shared libraries and business logic
├── infrastructure/    # Infrastructure config (schemas, migrations, cron, etc.)
├── docs/              # Documentation
├── bin/               # Built binaries
├── volumes/           # Docker volumes
└── logs/              # Application logs
```

## Database Management

### Migrations

Run database migrations and seeding:

```bash
make migrate  # Run all migrations (postgres + clickhouse)
make seed     # Seed initial data (definitions, seasons, activities)
```

### Database Structure

- **Multi-Schema Architecture**: `core`, `definitions`, `clan`, `extended`, `raw`, `flagging`, `leaderboard`
- **Migrations**: `infrastructure/postgres/migrations/` (numbered migration files)
- **Seeds**: `infrastructure/postgres/seeds/` (JSON-based seed data)
- **ClickHouse Views**: `infrastructure/clickhouse/views/`

## Environment Configuration

Key environment variables (see `example.env` for full list):

```bash
# Bungie API
BUNGIE_API_KEY=your_api_key
ZEUS_API_KEYS=key1,key2,key3  # Comma-separated for Zeus rotation

# IPv6 (required for Zeus)
IPV6=2001:db8::1  # Base IPv6 address for load balancing

# PostgreSQL
POSTGRES_USER=username
POSTGRES_PASSWORD=password
POSTGRES_PORT=5432

# RabbitMQ
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest

# ClickHouse
CLICKHOUSE_USER=username
CLICKHOUSE_PASSWORD=password

# Webhooks
ATLAS_WEBHOOK_URL=discord_webhook_url
HADES_WEBHOOK_URL=discord_webhook_url
```

## Development

### Adding a New Service

1. Create service directory in `apps/`
2. Add `main.go` with your service logic
3. Build with `make apps`
4. Run from `bin/<service-name>`

### Adding a New Database Migration

1. Create SQL file in `infrastructure/postgres/migrations/`
2. Follow naming convention: `XXX_description.sql` (where XXX is next number)
3. Use multi-schema structure (create schemas if needed)
4. Run with `make migrate`

### Adding a New Tool

1. Create tool directory in `tools/`
2. Add your tool logic (will be built as subcommand)
3. Build with `make tools`
4. Run with `./bin/tools <your-tool-name>`

## Queue Worker System

The system uses RabbitMQ with self-scaling topic managers:

- **`player_crawl`** - Player profile data processing
- **`activity_history_crawl`** - Player activity history updates
- **`character_fill`** - Missing character data completion
- **`clan_crawl`** - Clan information processing
- **`pgcr_blocked_retry`** - Retry mechanism for blocked PGCRs
- **`instance_store`** - Primary PGCR data storage
- **`instance_cheat_check`** - Post-storage cheat detection

Workers automatically scale based on queue depth and processing metrics.

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 5432, 5672, 8080, 8083, 7777, 9090, 15672 are available
2. **API Key Missing**: Set `BUNGIE_API_KEY` in `.env`
3. **Zeus Requires IPv6**: Set `IPV6` in `.env` for production use
4. **Docker Issues**: Ensure Docker is running and has sufficient resources
5. **Go Build Errors**: Check Go version and module dependencies
6. **Tilt Issues**: Ensure Tilt is installed and Docker is running

### Debugging

- Use Tilt UI (http://localhost:10350) to view service status and logs
- Check individual service logs via `docker-compose logs <service>`
- Monitor queue depths in RabbitMQ Management UI (http://localhost:15672)
- Check Prometheus metrics (http://localhost:9090) for performance issues

## Contributing

1. Follow the architecture principles in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
2. Use appropriate naming conventions from [docs/NAMING_CONVENTIONS.md](docs/NAMING_CONVENTIONS.md)
3. Follow application categorization in [docs/APP_CATEGORIZATION.md](docs/APP_CATEGORIZATION.md)
4. Set up proper logging using [docs/LOGGER_SETUP.md](docs/LOGGER_SETUP.md)
5. Keep infrastructure and application code separate
6. Document new services and workers
7. Update documentation with significant changes

## License

See LICENSE file for details.
