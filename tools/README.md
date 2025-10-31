# Tools

This directory contains utility scripts and tools for manual execution. Each tool is built as a separate binary.

## Available Tools

Each tool can be run directly:

```bash
./bin/<tool-name> [arguments]
```

### Available Tools

- `process-missed-pgcrs` - Processes missed PGCRs (runs every 15 minutes via cron)
- `manifest-downloader` - Downloads Destiny 2 manifest (runs multiple times daily via cron)
- `leaderboard-clan-crawl` - Crawls clans for top leaderboard players (runs weekly via cron)
- `cheat-detection` - Cheat detection and account maintenance (runs 4 times daily via cron)
- `refresh-view` - Refreshes materialized views (runs daily via cron)
- `activity-history-update` - Updates activity history for players who haven't been crawled recently
- `fix-sherpa-clears` - Rebuilds sherpa and first clear columns to fix race conditions
- `flag-restricted-pgcrs` - Flags PGCRs as restricted based on various criteria
- `process-single-pgcr` - Processes a single PGCR by instance ID
- `update-skull-hashes` - Updates skull hashes in the database

## Building

Build all tools:

```bash
make tools
```

This will build each tool as a separate binary in `./bin/<tool-name>`.

Or build a specific tool manually:

```bash
go build -o ./bin/<tool-name> ./tools/<tool-name>
```

## Running

Run any tool directly:

```bash
./bin/process-missed-pgcrs [--gap] [--force] [--workers=<number>] [--retries=<number>]
./bin/manifest-downloader [--out=<dir>] [--force] [--disk]
./bin/leaderboard-clan-crawl [--top=<number>] [--reqs=<number>]
./bin/cheat-detection
./bin/refresh-view <view_name>
./bin/activity-history-update
./bin/fix-sherpa-clears
./bin/flag-restricted-pgcrs
./bin/process-single-pgcr <instance_id>
./bin/update-skull-hashes
```

## Structure

Each tool is in its own directory under `tools/`:
- `tools/<tool-name>/main.go` - The tool implementation

Each tool is a standalone binary with its own `main()` function.
