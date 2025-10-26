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
│   │   ├── migrations/          # Migration files (multi-schema)
│   │   ├── seeds/               # Seed data (JSON format)
│   │   ├── init/                # Database initialization
│   │   └── tools/               # Migration and seeding tools
│   │       ├── migrate/         # Migration tool
│   │       └── seed/            # Seeding tool
│   ├── clickhouse/              # ClickHouse infrastructure
│   │   ├── migrations/          # ClickHouse migrations
│   │   ├── seeds/               # Seed data
│   │   ├── views/               # ClickHouse views
│   │   └── tools/               # Migration tools
│   ├── cron/                    # Cron job infrastructure
│   │   ├── crontab/             # Crontab definitions
│   │   └── scripts/             # Cron job scripts
│   ├── prometheus/              # Prometheus config
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

- **PostgreSQL**: Multi-schema migrations, seeds, initialization in `infrastructure/postgres/`
- **ClickHouse**: Migrations, seeds, views in `infrastructure/clickhouse/`
- **Schema Structure**: `core`, `definitions`, `clan`, `extended`, `raw`, `flagging`, `leaderboard`
- Connection logic in `shared/database/` for use by apps
- Separate migrate/seed tools for each database

### 4. Cron Job Management

- Crontab definitions in `infrastructure/cron/`
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

### Database Management

- Migrations are managed through custom Go tools in `infrastructure/postgres/tools/`
- Multi-schema structure with numbered migration files
- Seeds populate initial data using JSON format
- Views provide analytical queries
- Database initialization handled via templates

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
make migrate    # Run database migrations (multi-schema structure)
make seed       # Seed initial data (definitions, seasons, activities)
```

## Environment Variables

See `example.env` for all required environment variables.

Key variables:

- `BUNGIE_API_KEY`: Bungie API key for data access
- Database credentials (POSTGRES*\*, RABBITMQ*\_, CLICKHOUSE\_\_)
- Monitoring credentials (PROMETHEUS*\*, GRAFANA*\*)

## Future Improvements

The architecture is designed to support future enhancements:

1. Enhanced monitoring and alerting
2. Additional service integrations
3. Performance optimizations
4. Extended queue worker capabilities

## Domain Package Organization (`lib/domains/`)

The `lib/domains/` directory contains domain logic organized by entity type. Each domain package encapsulates all operations, data access, and business logic related to a specific entity.

### Domain Package Purposes

#### **`player/`** - Player Domain

**Purpose**: Manages player data, profiles, and statistics.

**Responsibilities**:

- Fetch player profiles from Bungie API
- Store and update player information in database
- Track player activity history
- Query player statistics and metadata

**Key Functions**:

- `Crawl()` - Fetch player data from Bungie API
- `GetPlayer()` - Retrieve player by membership ID
- `CreateOrUpdatePlayer()` - Insert or update player record
- `UpdateHistoryLastCrawled()` - Update activity history timestamp
- `GetPlayersNeedingHistoryUpdate()` - Find players needing refresh
- `GetPlayerCharacters()` - Get all characters for a player
- `UpdateActivityHistory()` - Fetch and store activity history

#### **`pgcr/`** - PGCR Domain

**Purpose**: Manages Post-Game Carnage Reports (raid completion data).

**Responsibilities**:

- Fetch PGCRs from Bungie API via Zeus proxy
- Process raw PGCR data into structured format
- Store PGCR instances in PostgreSQL
- Store raw JSON in database
- Determine if instance is "fresh" or checkpoint
- Handle PGCR processing workflow

**Key Functions**:

- `FetchAndProcessPGCR()` - Fetch and parse PGCR from API
- `ProcessDestinyReport()` - Convert Bungie API response to internal format
- `StorePGCR()` - Store processed PGCR to database
- `StoreJSON()` - Store compressed raw JSON
- `RetrieveJSON()` - Retrieve raw JSON by instance ID
- `ProcessBlocked()` - Handle blocked PGCRs
- `CheckExists()` - Check if PGCR exists in database
- `CheckCheat()` - Trigger cheat detection on PGCR
- `StoreClickhouse()` - Queue PGCR for ClickHouse
- `CalculateDateCompleted()` - Calculate completion timestamp
- `CalculateDurationSeconds()` - Calculate activity duration
- `WriteMissedLog()` - Log missed PGCR IDs

#### **`cheat_detection/`** - Cheat Detection Domain

**Purpose**: Detects and flags cheating behavior in raid completions.

**Responsibilities**:

- Analyze instance data for cheating patterns
- Apply heuristic algorithms to detect impossible/low-probability runs
- Flag instances and players with cheat probability scores
- Update player cheat levels
- Generate webhooks for flagged content
- Manage blacklisted players and instances

**Key Functions**:

- `CheckForCheats()` - Main entry point for cheat detection
- `getInstance()` - Fetch full instance data with players/characters/weapons
- `flagInstance()` - Flag an instance as cheated
- `flagPlayerInstance()` - Flag a player within an instance
- `GetAllInstanceFlagsByPlayer()` - Get all flags for a player
- `GetRecentlyPlayedBlacklistedPlayers()` - Get recently active blacklisted players
- `BlacklistRecentInstances()` - Auto-blacklist instances for blacklisted players
- `BlacklistFlaggedInstances()` - Upgrade high-probability flagged instances to blacklist
- `GetCheaterAccountChance()` - Calculate account-level cheat probability
- `UpdatePlayerCheatLevel()` - Update player cheat level based on flags
- `GetMinimumCheatLevel()` - Determine minimum cheat level from flags
- `SendFlaggedInstanceWebhook()` - Send Discord webhook for flagged instance
- `SendFlaggedPlayerWebhooks()` - Send Discord webhook for flagged players

**Heuristics** (in `methods.go` and `profile_heuristics.go`):

- Lowman detection (too few players)
- Speedrun time analysis
- Total kills analysis
- Time dilation detection
- Player-specific heuristics (kills per second, weapon diversity, etc.)

#### **`character/`** - Character Domain

**Purpose**: Manages character data for players.

**Responsibilities**:

- Fill missing character information
- Fetch character details from Bungie API

**Key Functions**:

- `Fill()` - Fetch and store character data

#### **`clan/`** - Clan Domain

**Purpose**: Manages clan (group) data.

**Responsibilities**:

- Fetch clan information from Bungie API
- Parse clan details
- Store clan data

**Key Functions**:

- `Crawl()` - Fetch clan data from API
- `ParseClanDetails()` - Parse clan banner/name/motto

#### **`stats/`** - Statistics Domain

**Purpose**: Calculates statistical aggregations.

**Responsibilities**:

- Update player sum-of-best times
- Aggregate raid completion statistics

**Key Functions**:

- `UpdatePlayerSumOfBest()` - Calculate and update sum of best clear times

#### **`instance/`** - Instance Domain

**Purpose**: Manages raid instance data and storage.

Contains types and operations for raid instances:

- `Instance` - Processed raid instance
- `InstancePlayer` - Player within an instance
- `InstanceCharacter` - Character within an instance
- `InstanceCharacterWeapon` - Weapon used by character
- Storage and retrieval operations for instances

### Package Boundary Rules

#### **`lib/database/postgres/`** - Database Infrastructure Only

**Should contain**:

- Database connection singleton (`singleton.go`)
- Connection initialization (`init()` functions)
- Database monitoring utilities (`watch.go`)
- **ONLY low-level database utilities**

**Should NOT contain**:

- Domain-specific query logic
- Business rules
- Entity operations
- Transaction management for specific domains

#### **`lib/domains/*/`** - Domain Logic Only

**Should contain**:

- All queries specific to that domain
- Business logic for that domain
- Entity operations
- Domain-specific types

**Should NOT contain**:

- Database connection setup
- Generic utilities (use `lib/utils/`)
- Cross-domain logic (create shared package or import between domains as needed)

## Contributing

1. Follow the architecture principles
2. Keep infrastructure and app code separate
3. Keep database utilities separate from domain logic
4. Each domain should be self-contained
5. Document new services and workers
6. Update this document with significant changes
