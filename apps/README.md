# Applications

This directory contains all RaidHub Services applications. Applications are categorized by how they run:

## Long-Running Services (Start with `make up`)

These services run continuously and should be started when the system starts:

### `hermes` - Queue Worker Manager

- **Purpose**: Manages all queue workers for processing game data
- **Queues**: Activity history, player crawls, character data, PGCRs, clan data, cheat detection
- **Monitoring**: Port 8083
- **Start**: `make hermes` or part of `make up`

### `atlas` - PGCR Crawler

- **Purpose**: Crawls PostGame Carnage Reports from Bungie API
- **Workers**: Configurable (default 25)
- **Monitoring**: Port 8080
- **Start**: `make atlas` or part of `make up`

### `zeus` - Bungie API Proxy

- **Purpose**: Reverse proxy for Bungie API with rate limiting and IPv6 support
- **Port**: 7777
- **Features**: Load balancing across multiple IPv6 addresses, rate limiting
- **Start**: `make zeus` or part of `make up`

## Cron Jobs (Scheduled Tasks)

These applications run on a schedule via system crontab:

### `hera` - Top Player Crawl

- **Purpose**: Crawls top players from leaderboards to keep player data fresh
- **Schedule**: Should run periodically (e.g., daily or weekly)
- **Usage**: `./bin/hera -top 1500`
- **Setup**: Add to crontab for scheduled execution

### `hades` - Missed Log Processor

- **Purpose**: Processes PGCRs that were missed during normal crawling
- **Usage**: `./bin/hades` or `./bin/hades -gap`
- **Schedule**: Should run periodically to catch missed PGCRs

### `nemesis` - Player Account Tasks

- **Purpose**: Runs scheduled player account maintenance tasks
- **Schedule**: Configured via crontab
- **Usage**: Run automatically via cron

### `athena` - Manifest Downloader

- **Purpose**: Fetches and updates Destiny 2 manifest data
- **Schedule**: Configured via crontab
- **Usage**: Run automatically via cron

**Note**: See `docs/NAMING_CONVENTIONS.md` for naming conventions.

**Manual scripts** are located in the `tools/` directory.

## Startup Configuration

The `make up` command starts the essential long-running services:

```makefile
# Long-running services started with make up
up: hermes atlas zeus
```

## Cron Configuration

Cron jobs are configured in `infrastructure/cron/crontab/` directory:

- Development: `infrastructure/cron/crontab/dev.crontab`
- Production: `infrastructure/cron/crontab/prod.crontab`

See `infrastructure/cron/README.md` for more details on cron configuration.

## Monitoring

- **Hermes**: http://localhost:8083/metrics
- **Atlas**: http://localhost:8080/metrics
- **Prometheus**: http://localhost:9090
