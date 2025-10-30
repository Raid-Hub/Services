# RaidHub Services Architecture

## Overview

RaidHub Services is a microservices architecture built in Go, managing data collection, processing, and analysis for Destiny 2 raid completion tracking. The system is designed to handle high-throughput PGCR (Post-Game Carnage Report) crawling, intelligent queue-based processing, and comprehensive cheat detection.

## Folder Structure

```
RaidHub-Services/
├── apps/                        # Main application services
│   ├── atlas/                   # Intelligent PGCR crawler with adaptive scaling
│   ├── zeus/                    # Bungie API reverse proxy with IPv6 load balancing
│   ├── hermes/                  # Queue worker manager with self-scaling topics
│   ├── athena/                  # Destiny 2 manifest downloader
│   ├── hades/                   # Missed PGCR recovery processor
│   ├── hera/                    # Top player crawler for leaderboard maintenance
│   └── nemesis/                 # Cheat detection and player account maintenance
├── queue-workers/               # Queue worker topic definitions
│   ├── activity_history.go      # Player activity history processing
│   ├── character_fill.go        # Character data completion
│   ├── clan_crawl.go            # Clan information crawler
│   ├── pgcr_blocked.go          # Retry mechanism for blocked PGCRs
│   ├── instance_cheat_check.go  # Post-storage cheat detection
│   ├── instance_store.go        # Primary PGCR data storage
│   ├── pgcr_crawl.go            # General PGCR processing (legacy)
│   └── player_crawl.go          # Player profile data crawler
├── lib/                         # Shared libraries and business logic
│   ├── database/                # Database connection singletons
│   │   ├── postgres/            # PostgreSQL connection management
│   │   └── clickhouse/          # ClickHouse connection management
│   ├── messaging/               # RabbitMQ messaging infrastructure
│   │   ├── processing/          # Topic managers and workers
│   │   ├── routing/             # Queue routing constants
│   │   ├── rabbit/              # RabbitMQ connection singleton
│   │   └── messages/            # Message type definitions
│   ├── services/                # Domain-specific business logic
│   │   ├── pgcr_processing/     # PGCR fetch and processing logic
│   │   ├── instance_storage/    # Multi-database storage orchestration
│   │   ├── cheat_detection/     # Comprehensive cheat detection system
│   │   ├── player/              # Player data management
│   │   ├── character/           # Character data operations
│   │   ├── clan/                # Clan data operations
│   │   ├── instance/            # Instance data queries
│   │   └── stats/               # Statistical calculations
│   ├── web/                     # External API clients
│   │   ├── bungie/              # Bungie.net API client
│   │   ├── discord/             # Discord webhook client
│   │   └── gm_report/           # Grandmaster reporting
│   ├── monitoring/              # Prometheus metrics
│   ├── utils/                   # Common utilities
│   └── env/                     # Environment configuration
├── tools/                       # Utilities and one-off maintenance tools
│   ├── main.go                 # Tool dispatcher
│   ├── activity-history-update/ # Batch activity history updates
│   ├── fix-sherpa-clears/      # Data correction utilities
│   ├── flag-restricted-pgcrs/  # Batch PGCR flagging
│   ├── process-single-pgcr/    # Individual PGCR processing
│   └── update-skull-hashes/    # Manifest hash updates
├── infrastructure/              # Infrastructure configuration (NO application code)
│   ├── postgres/                # PostgreSQL infrastructure
│   │   ├── migrations/          # Schema migrations
│   │   ├── init/                # Database initialization scripts
│   │   └── tools/               # Migration and seeding utilities
│   ├── clickhouse/              # ClickHouse analytics database
│   │   ├── migrations/          # ClickHouse schema migrations
│   │   ├── views/               # Materialized views for analytics
│   │   └── tools/               # ClickHouse utilities
│   ├── cron/                    # Scheduled task configuration
│   ├── prometheus/              # Monitoring configuration
│   └── rabbitmq/                # Message queue configuration
├── docs/                        # Architecture and API documentation
├── bin/                         # Built application binaries
├── volumes/                     # Docker persistent volumes
└── logs/                        # Application log files
```

## Key Architectural Principles

### 1. Microservices with Clear Boundaries

- **`apps/`** - Independent, deployable application services
- **`queue-workers/`** - Topic-based asynchronous processing definitions
- **`lib/`** - Shared business logic and infrastructure libraries
- **`tools/`** - Maintenance utilities and one-off operations
- **`infrastructure/`** - Pure infrastructure configuration (NO application code)

### 2. Event-Driven Architecture

- **Message Queues**: RabbitMQ with topic-based routing for async processing
- **Self-Scaling Workers**: Automatic scaling based on queue depth and processing metrics
- **Side Effects**: Storage operations trigger downstream processing via message queues
- **Failure Recovery**: Dedicated retry mechanisms and missed item recovery

### 3. Infrastructure vs Application Code

- **`infrastructure/`** contains ONLY configuration and tooling:
  - Database schemas, migrations, and views
  - Service configuration templates
  - Cron job definitions
  - Migration and deployment tools
- **`lib/`** contains ONLY application logic:
  - Database connection management
  - Business domain logic
  - External API clients
  - Message queue processing framework
  - Monitoring and utilities

### 4. Domain-Driven Design

- **Services** organized by business domain (player, instance, cheat_detection)
- **Clear boundaries** between domains with well-defined interfaces
- **Dependency injection** through singleton pattern for shared resources
- **Separation of concerns** between data access, business logic, and external integrations

## Application Services

### Long-Running Services

These services run continuously and are started with `make up`:

#### Atlas - Intelligent PGCR Crawler

**Purpose**: Crawls PostGame Carnage Reports from the Bungie API with intelligent scaling and recovery mechanisms.

**Key Features**:

- **Adaptive Worker Scaling**: Dynamically adjusts worker count based on 404 rates and lag metrics
- **Offload Workers**: Handles problematic PGCRs with exponential backoff retry logic
- **Gap Detection**: Automatically identifies and handles missing PGCR sequences
- **Rate Limiting**: Respects Bungie API limits with intelligent throttling
- **Monitoring**: Comprehensive Prometheus metrics and Discord alerting

**Configuration**:

- Default workers: 25 (configurable via flags)
- Buffer distance: 10,000 IDs behind latest
- Monitoring port: 8080

#### Zeus - Bungie API Reverse Proxy

**Purpose**: Provides rate limiting and optional load balancing for Bungie API requests.

**Key Features**:

- **Optional IPv6 Load Balancing**: When `IPV6` environment variable is set, distributes requests across sequential IPv6 addresses (round robin)
- **Differentiated Rate Limiting**: Separate limits for stats.bungie.net vs www.bungie.net (always enabled)
- **Health Monitoring**: BetterUptime probe support
- **Development Mode**: Use `--dev` flag to disable round robin (single transport) while keeping rate limiting enabled

**Configuration**:

- Default port: 7777
- IPv6 configuration: Optional via `IPV6` environment variable (base address)
- Configurable IPv6 interface and address count via flags (`--interface`, `--v6_n`)
- Stats API: 40 requests/second per IP, 90 burst
- WWW API: 12 requests/second per IP, 25 burst
- Development mode: `--dev` flag disables round robin but keeps rate limiting enabled (used by Tilt)

#### Hermes - Queue Worker Manager

**Purpose**: Manages all queue workers with self-scaling topic managers for different processing types.

**Key Features**:

- **Topic Management**: Coordinates multiple queue types with independent scaling
- **Contest Mode Support**: Higher worker counts during contest weekends
- **Dynamic Scaling**: Scales workers up/down based on queue depth and processing metrics
- **Graceful Shutdown**: Proper cleanup and resource management

**Managed Topics**:

- `player_crawl`: Player profile data processing
- `activity_history_crawl`: Player activity history updates
- `character_fill`: Missing character data completion
- `clan_crawl`: Clan information processing
- `pgcr_blocked_retry`: Retry mechanism for failed PGCRs
- `instance_store`: Primary PGCR data storage
- `instance_cheat_check`: Post-storage cheat detection

### Scheduled Services (Cron Jobs)

These services run on a schedule via system crontab:

#### Hera - Top Player Crawler

**Purpose**: Keeps player data fresh by crawling top players from various leaderboards.

**Key Features**:

- **Leaderboard Analysis**: Processes top N players from individual and raid leaderboards
- **Clan Discovery**: Discovers and processes clans from top players
- **Bulk Operations**: Efficient batch processing with configurable concurrency
- **Member Validation**: Ensures discovered players exist in the system

**Usage**: `./bin/hera -top 1500 -reqs 14`

#### Hades - Missed PGCR Recovery

**Purpose**: Processes PGCRs that were missed during normal crawling operations.

**Key Features**:

- **Gap Processing**: Handles missing PGCR sequences with configurable range limits
- **Retry Logic**: Intelligent retry with exponential backoff
- **Progress Tracking**: Detailed reporting of recovery success/failure rates
- **Safety Limits**: Prevents processing of overly large gaps

**Usage**:

- `./bin/hades`: Process missed PGCRs
- `./bin/hades -gap`: Process gaps in sequences

#### Nemesis - Cheat Detection & Player Maintenance

**Purpose**: Runs comprehensive cheat detection analysis and player account maintenance.

**Key Features**:

- **Player Cheat Level Analysis**: Calculates and updates player cheat levels
- **Instance Re-checking**: Re-processes instances for high-risk players
- **Blacklist Management**: Automatically blacklists flagged instances and player instances
- **Statistical Reporting**: Provides detailed cheat detection statistics

#### Athena - Manifest Downloader

**Purpose**: Downloads and processes Destiny 2 manifest data for weapon and feature definitions.

**Key Features**:

- **Manifest Fetching**: Downloads latest manifest from Bungie API
- **Definition Processing**: Extracts weapon and feature definitions
- **Database Updates**: Updates PostgreSQL with latest definitions
- **Version Management**: Only processes new manifests

**Processing**:

- Weapon definitions (hash, name, icon, element, ammo type, slot, type, rarity)
- Activity feat definitions (skulls/modifiers for raids)

## Queue Worker System

### Message Queue Architecture

The system uses RabbitMQ with a topic-based architecture where each queue type is managed as a "topic" with:

- **Self-Scaling Workers**: Automatically scales based on queue depth and processing metrics
- **Contest Mode Support**: Higher worker counts during contest periods
- **Configurable Parameters**: Min/max workers, scale thresholds, prefetch counts
- **Failure Handling**: Built-in retry mechanisms and error handling

### Queue Types

#### Primary Data Flow Queues

1. **`instance_store`** - Primary PGCR storage pipeline

   - **Purpose**: Stores processed PGCRs to PostgreSQL and ClickHouse
   - **Triggers**: Character fill, player crawl, cheat check side effects
   - **Workers**: 5-50 (15 desired, 30 contest)

2. **`instance_cheat_check`** - Post-storage cheat detection
   - **Purpose**: Runs cheat detection algorithms on stored instances
   - **Workers**: 5-100 (25 desired, 75 contest)

#### Support Queues

3. **`player_crawl`** - Player data updates

   - **Purpose**: Fetches and updates player profile data
   - **Workers**: 5-100 (20 desired, 50 contest)

4. **`activity_history_crawl`** - Activity history processing

   - **Purpose**: Updates player activity history from Bungie API
   - **Workers**: 1-20 (3 desired, 10 contest)

5. **`character_fill`** - Character data completion

   - **Purpose**: Fills missing character information
   - **Workers**: 1-50 (5 desired, 20 contest)

6. **`clan_crawl`** - Clan information updates

   - **Purpose**: Processes clan data and membership
   - **Workers**: 1-25 (3 desired, 10 contest)

7. **`pgcr_blocked_retry`** - Failed PGCR retry mechanism
   - **Purpose**: Retries PGCRs that failed due to permissions or rate limiting
   - **Workers**: 1-10 (2 desired, 5 contest)

### Scaling Parameters

Each topic has configurable scaling parameters:

- **MinWorkers/MaxWorkers**: Hard limits on worker count
- **DesiredWorkers**: Target worker count under normal conditions
- **ContestWeekendWorkers**: Higher target during contest periods
- **ScaleUpThreshold**: Queue depth that triggers scaling up
- **ScaleDownThreshold**: Queue depth that triggers scaling down
- **ScaleUpPercent/ScaleDownPercent**: Rate of scaling changes

## Data Flow Architecture

### Primary PGCR Processing Flow

1. **Atlas** crawls PGCR IDs sequentially from Bungie API
2. **Zeus** proxies API requests with load balancing and rate limiting
3. **PGCR Processing** validates and transforms raw API responses
4. **Instance Store Queue** receives successful PGCRs for storage
5. **Orchestrated Storage** saves to both PostgreSQL and ClickHouse atomically
6. **Side Effects** trigger downstream processing:
   - Character fill for missing character data
   - Player crawl for new or stale players
   - Cheat check for completed instances

### Recovery and Retry Mechanisms

1. **Offload Workers** (Atlas): Handle slow or problematic PGCRs
2. **Missed Log Processing** (Hades): Recovers PGCRs that failed completely
3. **Blocked Retry Queue**: Handles permission-based failures
4. **Gap Detection**: Identifies and fills missing PGCR sequences

### Cheat Detection Pipeline

1. **Instance Analysis**: Examines completed instances for suspicious patterns
2. **Heuristic Application**: Applies multiple cheat detection algorithms
3. **Player Flag Management**: Updates player cheat levels and flags
4. **Blacklist Management**: Automatically promotes high-confidence flags
5. **Webhook Notifications**: Sends Discord alerts for flagged content

## Domain Services Architecture

### Service Organization

Services in `lib/services/` are organized by business domain with clear boundaries:

#### `pgcr_processing/` - PGCR Domain

- **FetchAndProcessPGCR()**: Coordinates API fetch and data transformation
- **ParsePGCRToInstance()**: Converts Bungie API format to internal structure
- **CalculateDateCompleted()**: Determines instance completion timestamp
- **Result Types**: Success, NotFound, NonRaid, SystemDisabled, etc.

#### `instance_storage/` - Storage Orchestration

- **StorePGCR()**: Orchestrates multi-database storage with atomicity
- **StoreRawJSON()**: Compressed JSON storage in PostgreSQL
- **Store()**: Structured instance data storage
- **StoreToClickHouse()**: Analytics database storage
- **Side Effect Management**: Triggers downstream queue processing

#### `cheat_detection/` - Anti-Cheat System

- **CheckForCheats()**: Main cheat detection entry point
- **Heuristic Algorithms**: Lowman, speedrun, kill analysis, time dilation
- **Player Management**: Cheat level calculation and blacklist management
- **Webhook Integration**: Discord notifications for flagged content

#### `player/` - Player Domain

- **Crawl()**: Fetches player data from Bungie API
- **UpdateActivityHistory()**: Processes player activity timeline
- **Data Management**: Player profiles, characters, statistics

#### `character/` - Character Domain

- **Fill()**: Completes missing character information

#### `clan/` - Clan Domain

- **Crawl()**: Fetches clan data and membership
- **ParseClanDetails()**: Processes clan banner and metadata

### Database Architecture

#### PostgreSQL - Primary Data Store

- **Multi-schema structure**: core, definitions, clan, extended, raw, flagging, leaderboard
- **ACID Compliance**: Ensures data consistency for critical operations
- **Relationship Management**: Complex queries across normalized tables
- **JSON Storage**: Compressed raw PGCR data for replay capability

#### ClickHouse - Analytics Database

- **Time-series Optimization**: Optimized for analytical queries
- **Materialized Views**: Pre-computed aggregations
- **Column Storage**: Efficient compression and query performance
- **Real-time Ingestion**: Receives data from PostgreSQL storage operations

## Infrastructure Components

### Docker Services

- **PostgreSQL**: Primary relational database with persistent volumes
- **RabbitMQ**: Message queue with management interface
- **ClickHouse**: Analytics database with custom configuration
- **Prometheus**: Metrics collection and storage

### Monitoring & Alerting

- **Prometheus Metrics**: Comprehensive application and infrastructure metrics
- **Discord Webhooks**: Real-time alerts for critical events and cheat detection
- **Health Checks**: BetterUptime integration for service monitoring
- **Performance Tracking**: Request latency, queue depths, processing rates

### Development Workflow

#### Setup

```bash
# Clone and setup environment
./bootstrap.sh
# Copy and configure environment
cp example.env .env
# Start infrastructure services
make up
```

#### Building & Running

```bash
make bin        # Build all binaries
make <service>  # Build specific service
make dev        # Build and start all services with hot reload (Tilt)
```

#### Database Management

```bash
make migrate    # Run database migrations
make seed       # Populate seed data
```

#### Service Management

```bash
make up         # Start core services (hermes, atlas, zeus)
make services   # Start all services
make logs       # View aggregated logs
make down       # Stop all services
```

## Environment Configuration

See `example.env` for all configuration options.

### Critical Variables

- `BUNGIE_API_KEY`: Primary Bungie API authentication
- `ZEUS_API_KEYS`: Comma-separated list of API keys for Zeus rotation
- `IPV6`: Base IPv6 address for Zeus load balancing
- Database credentials: `POSTGRES_*`, `CLICKHOUSE_*`, `RABBITMQ_*`
- Webhook URLs: `ATLAS_WEBHOOK_URL`, `HADES_WEBHOOK_URL`

## Deployment Architecture

### Service Types

#### Long-Running Services (Docker Compose)

- **Atlas**: PGCR crawler
- **Zeus**: API proxy
- **Hermes**: Queue manager

#### Scheduled Tasks (Cron)

- **Hera**: Daily/weekly player updates
- **Hades**: Missed PGCR recovery
- **Nemesis**: Cheat detection maintenance
- **Athena**: Manifest updates

#### On-Demand Tools

- **tools/**: Various maintenance and data correction utilities

### Performance Characteristics

#### Throughput

- **Atlas**: Processes ~1000-5000 PGCRs per minute
- **Queue Workers**: Self-scaling based on load
- **API Rate Limits**: Managed through Zeus proxy with multiple IPs/keys

#### Scalability

- **Horizontal**: Multiple Zeus instances for higher API throughput
- **Vertical**: Worker count scaling based on queue depth
- **Database**: PostgreSQL primary with ClickHouse for analytics

## Contributing

1. Follow the architectural principles and service boundaries
2. Keep infrastructure configuration separate from application code
3. Use domain-driven design for new services
4. Implement proper error handling and monitoring
5. Update documentation for significant architectural changes

## Future Improvements

1. **Enhanced Monitoring**: Real-time dashboards and advanced alerting
2. **API Rate Optimization**: Intelligent request batching and caching
3. **Horizontal Scaling**: Kubernetes deployment for better resource utilization
4. **Data Pipeline Optimization**: Streaming analytics and real-time processing
5. **Advanced Cheat Detection**: Machine learning integration for pattern recognition
