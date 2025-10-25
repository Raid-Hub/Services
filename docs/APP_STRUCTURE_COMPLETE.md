# Application Structure - Complete

## Summary

Apps have been categorized and documented according to their execution model.

## Categories

### 1. Long-Running Services

These start automatically with `make up`:

- `hermes` - Queue worker manager
- `atlas` - PGCR crawler
- `zeus` - Bungie API proxy

### 2. Cron Jobs

These run on schedule via crontab:

- `hera` - Top player crawl (daily at 6 AM)
- `hades` - Missed log processor (every 6 hours)
- `nemesis` - Player account maintenance (daily at 3 AM)
- `athena` - Manifest downloader (daily at 2 AM)

### 3. Manual Tools

These run manually via the consolidated tools binary:

- `activity-history-update` - Updates player activity history
- `fix-sherpa-clears` - Fixes sherpa and first clear data
- `flag-restricted-pgcrs` - Flags restricted PGCRs
- `process-single-pgcr` - Processes a single PGCR
- `update-skull-hashes` - Updates skull hashes

## Changes Made

### Documentation

- ✅ `apps/README.md` - Documented all apps by category
- ✅ `docs/APP_CATEGORIZATION.md` - Detailed categorization guide
- ✅ Updated `README.md` with app execution model

### Makefile

- ✅ Updated `make up` to start long-running services
- ✅ Created `make services` for Docker containers
- ✅ Clarified separation between apps and services

### Cron Configuration

- ✅ Added hera and hades to production crontab
- ✅ Documented in dev crontab (commented out for optional use)

## Folder Structure

All apps remain in `apps/` directory. Manual tools have been moved to `tools/` directory:

```
apps/
├── hermes/      # Long-running service
├── atlas/       # Long-running service
├── zeus/        # Long-running service
├── hera/        # Cron job
├── hades/       # Cron job
├── nemesis/     # Cron job
└── athena/      # Cron job

tools/           # Consolidated tools binary
├── main.go      # Command dispatcher
├── activity-history-update/
├── fix-sherpa-clears/
├── flag-restricted-pgcrs/
├── process-single-pgcr/
└── update-skull-hashes/
```

## Usage

### Starting Long-Running Services

```bash
make up          # Start hermes, atlas, zeus
```

### Running Cron Jobs

Configured in crontab:

```bash
crontab infrastructure/cron/crontab/prod.crontab
```

### Running Manual Tools

These are commands for the consolidated tools binary:

```bash
./bin/tools activity-history-update
./bin/tools fix-sherpa-clears
./bin/tools flag-restricted-pgcrs
./bin/tools process-single-pgcr
./bin/tools update-skull-hashes
```

## Benefits

This structure makes it clear:

1. What runs automatically → Long-running services
2. What runs on schedule → Cron jobs
3. What runs manually → Tools

This helps with:

- System administration
- Troubleshooting
- Understanding system behavior
- Onboarding new developers
