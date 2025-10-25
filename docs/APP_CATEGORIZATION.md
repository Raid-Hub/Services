# Application Categorization

This document defines how RaidHub Services applications are categorized by their execution model.

## Categories

### 1. Long-Running Services

**Definition**: Services that run continuously and are started at system startup.

**Apps**:

- `hermes` - Queue worker manager
- `atlas` - PGCR crawler
- `zeus` - Bungie API proxy

**Start with**: `make up`

### 2. Cron Jobs

**Definition**: Applications that run on a schedule via system crontab.

**Apps**:

- `hera` - Top player crawl (runs daily)
- `hades` - Missed log processor (runs every 6 hours)
- `nemesis` - Player account maintenance (runs daily)
- `athena` - Manifest downloader (runs daily)

**Configure in**: `infrastructure/cron/crontab/`

### 3. Manual Tools

**Definition**: Utilities that are executed manually as needed for specific tasks.

**Location**: `tools/` directory

**Tools** (in `tools/` directory, consolidated into single binary):

- `activity-history-update` - Updates player activity history
- `fix-sherpa-clears` - Fixes sherpa and first clear data
- `flag-restricted-pgcrs` - Flags restricted PGCRs
- `process-single-pgcr` - Processes a single PGCR
- `update-skull-hashes` - Updates skull hashes
- `pan` - Activity history crawler (moved from cmd/)
- `bob` - Sherpa/first clear rebuild script (moved from cmd/)

**Execute**: `./bin/tools <command>` when needed

## Folder Organization

```
apps/                     # All applications (builds all)
├── hermes/              # Long-running service
├── atlas/               # Long-running service
├── zeus/                # Long-running service
├── hera/                # Cron job
├── hades/               # Cron job
├── nemesis/             # Cron job
└── athena/              # Cron job
```

## Makefile Targets

### Building

```bash
make bin          # Build all apps
make <app>        # Build specific app
```

### Running Long-Running Services

```bash
make up           # Start hermes, atlas, zeus
make hermes       # Start specific service
make atlas        # Start specific service
make zeus         # Start specific service
```

### Running Manual Tools

These are commands for the consolidated tools binary:

```bash
./bin/tools activity-history-update
./bin/tools fix-sherpa-clears
./bin/tools flag-restricted-pgcrs
./bin/tools process-single-pgcr
./bin/tools update-skull-hashes
./bin/tools pan
./bin/tools bob
```

## Naming Conventions

Note the naming differences:

- **Services and Cron Jobs**: Use Greek mythology names (hermes, atlas, zeus, hera, hades, nemesis, athena)
- **Manual Tools**: Use descriptive names that explain what they do (activity-history-update, fix-sherpa-clears)

See `docs/NAMING_CONVENTIONS.md` for details.

## Rationale

This categorization makes it clear:

1. **What runs automatically** (long-running services)
2. **What runs on schedule** (cron jobs)
3. **What you run manually** (scripts)

This helps with:

- System administration
- Troubleshooting
- Understanding system behavior
- Onboarding new developers
