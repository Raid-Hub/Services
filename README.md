# RaidHub Services

Microservices architecture for managing Destiny 2 raid completion data collection, processing, and analysis.

## Architecture

RaidHub Services follows a microservices architecture with clear separation between applications, queue workers, tools, and infrastructure.

### Main Components

- **Apps** (`apps/`): Main application services (Atlas, Hermes, Hades, Athena, etc.)
- **Queue Workers** (`queue-workers/`): Background processing workers for async tasks
- **Tools** (`tools/`): Utility scripts and one-off tools
- **Infrastructure** (`infrastructure/`): Database schemas, migrations, cron jobs, and service configs
- **Shared** (`shared/`): Shared libraries for database connections, messaging, monitoring

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture documentation.

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
   make tilt-dev
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
   make services-infra
   ```

4. **Build binaries:**
   ```bash
   make bin
   ```

## Makefile Commands

### Development (Recommended)

```bash
make tilt-dev      # Start development environment with Tilt (hot reload)
make tilt-down     # Stop Tilt development environment
make tilt-logs     # View Tilt logs
make tilt-migrate  # Run database migration via Tilt
make tilt-build    # Build all services via Tilt
make tilt-clean    # Clean build artifacts via Tilt
```

### Legacy Development

```bash
make dev-legacy    # Start services with Docker Compose (no hot reload)
make build         # Build all binaries
make <name>        # Build a specific service/worker
```

### Docker Services

```bash
make services      # Start all services
make services-infra # Start infrastructure services only
make up            # Start required services (postgres, rabbitmq, clickhouse)
make down          # Stop all services
make logs          # View logs
make restart       # Restart services
make ps            # View running services
```

### Individual Services

```bash
make postgres      # Start PostgreSQL only
make rabbit        # Start RabbitMQ only
make clickhouse    # Start ClickHouse only
make prometheus    # Start Prometheus only
```

## Running Services

### Development Mode (Recommended)

**With Tilt (hot reload, service orchestration):**

```bash
make tilt-dev    # Start all services with hot reload
```

Access services:

- **Tilt UI**: http://localhost:10350 (service monitoring and control)
- **Hermes**: http://localhost:8083/metrics (queue worker manager)
- **Atlas**: http://localhost:8080/metrics (PGCR crawler)
- **Zeus**: http://localhost:7777 (Bungie API proxy)

### Legacy Mode

**Long-running services** (start with `make up`):

```bash
make up          # Start hermes, atlas, zeus
make hermes      # Queue worker manager
make atlas       # PGCR crawler
make zeus        # Bungie API proxy
```

**Cron jobs** (run via crontab):

- `hera` - Top player crawl (daily at 6 AM)
- `hades` - Missed log processor (every 6 hours)
- `nemesis` - Player account maintenance (daily at 3 AM)
- `athena` - Manifest downloader (daily at 2 AM)

**Tools** (run as regular commands, located in `tools/` directory):

```bash
make tools                              # Build all tools
./bin/tools activity-history-update     # Update player activity history
./bin/tools fix-sherpa-clears          # Fix sherpa/first-clear data
./bin/tools flag-restricted-pgcrs      # Flag restricted PGCRs
./bin/tools process-single-pgcr        # Process a single PGCR
./bin/tools update-skull-hashes        # Update skull hashes
```

See `apps/README.md` and `docs/APP_CATEGORIZATION.md` for details on application categorization. See `docs/NAMING_CONVENTIONS.md` for naming conventions.

## Available Services

Once running, these services are available:

### Infrastructure Services

- **PostgreSQL**: `localhost:5432` (Database)
- **RabbitMQ**: `localhost:5672` (AMQP), `localhost:15672` (Management UI)
- **ClickHouse**: `localhost:9000` (Native), `localhost:8123` (HTTP)
- **Prometheus**: `localhost:9090` (Metrics)

### Application Services

- **Hermes**: `localhost:8083` (Queue worker manager)
- **Atlas**: `localhost:8080` (PGCR crawler)
- **Zeus**: `localhost:7777` (Bungie API proxy)

### Development Tools

- **Tilt UI**: `localhost:10350` (Service monitoring and control)

## Project Structure

```
RaidHub-Services/
├── apps/              # Main application services
├── queue-workers/     # Background processing workers
├── tools/             # Utility scripts
├── infrastructure/    # Infrastructure config (schemas, migrations, cron, etc.)
├── shared/            # Shared libraries
├── docker/            # Docker configurations
├── bin/               # Built binaries
├── volumes/           # Docker volumes
└── logs/              # Application logs
```

## Database Management

### Migrations

Run database migrations and seeding:

```bash
make migrate  # Run database migrations (multi-schema structure)
make seed     # Seed initial data (definitions, seasons, activities)
```

### Database Structure

- **Multi-Schema Architecture**: `core`, `definitions`, `clan`, `extended`, `raw`, `flagging`, `leaderboard`
- **Migrations**: `infrastructure/postgres/migrations/` (numbered migration files)
- **Seeds**: `infrastructure/postgres/seeds/` (JSON-based seed data)
- **ClickHouse Views**: `infrastructure/clickhouse/views/`

## Environment Variables

Key environment variables (see `example.env` for full list):

```bash
# Bungie API
BUNGIE_API_KEY=your_api_key

# PostgreSQL
POSTGRES_USER=username
POSTGRES_PASSWORD=password
POSTGRES_PORT=5432

# RabbitMQ
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=guest

```

## Development

### Adding a New Service

1. Create service directory in `apps/` or `queue-workers/`
2. Add `main.go` with your service logic
3. Build with `make <service-name>`
4. Run from `bin/<service-name>`

### Adding a New Database Migration

1. Create SQL file in `infrastructure/postgres/migrations/`
2. Follow naming convention: `XXX_description.sql` (where XXX is next number)
3. Use multi-schema structure (create schemas if needed)
4. Run with `make migrate`

## Documentation

- [Architecture Overview](docs/ARCHITECTURE.md) - Detailed architecture documentation

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 5432, 5672, 8080, 8083, 7777, 9090, 3000 are available
2. **API Key Missing**: Set `BUNGIE_API_KEY` in `.env`
3. **Docker Issues**: Ensure Docker is running and has sufficient resources
4. **Go Build Errors**: Check Go version and module dependencies
5. **Tilt Issues**: Ensure Tilt is installed and Docker is running

### Debugging

- Use Tilt UI (http://localhost:10350) to view service status and logs
- Check health check resources for infrastructure issues
- Monitor service dependencies in Tilt UI
- Use `tilt logs <service>` for detailed error messages

## Contributing

1. Follow the architecture principles in `docs/ARCHITECTURE.md`
2. Keep infrastructure and app code separate
3. Document new services and workers
4. Update documentation with significant changes

## License

See LICENSE file for details.
