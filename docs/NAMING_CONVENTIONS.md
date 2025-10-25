# Naming Conventions

This document explains the naming conventions used throughout RaidHub Services.

## Application Naming

RaidHub Services uses two distinct naming patterns based on execution model:

### Greek Mythology Names

**Used for**: Long-running services and cron jobs

These applications run automatically (either continuously or on schedule), so they use mythological names:

- `hermes` - Queue worker manager (messenger god)
- `atlas` - PGCR crawler (titan who carried the world)
- `zeus` - Bungie API proxy (king of gods)
- `hera` - Top player crawl (queen of gods)
- `hades` - Missed log processor (god of underworld)
- `nemesis` - Player account maintenance (goddess of retribution)
- `athena` - Manifest downloader (goddess of wisdom)

### Descriptive Names

**Used for**: Manual scripts

These applications are run manually by developers/operators, so they use descriptive names that explain what they do:

- `activity-history-update` - Updates player activity history
- `fix-sherpa-clears` - Fixes sherpa and first clear data

## Rationale

### Why Mythological Names for Services?

- Services run automatically and are part of the infrastructure
- Mythological names add character and are memorable
- Differentiates from manual tools

### Why Descriptive Names for Scripts?

- Scripts are used interactively by developers
- Descriptive names make it immediately clear what the script does
- No need to guess or remember obscure names
- Self-documenting names reduce need to look up documentation

## Other Naming Patterns

### Directories

- `apps/` - Application code
- `queue-workers/` - Background workers
- `infrastructure/` - Infrastructure configuration
- `shared/` - Shared libraries
- `tools/` - Utility scripts

### Binaries

Binaries in `bin/` directory use the same name as their source:

- `bin/hermes` from `apps/hermes/`
- `bin/pan` from `apps/pan/`

## Examples

### Correct Usage

**Services** (automatic execution):

```bash
make up          # Starts hermes, atlas, zeus
crontab -e       # Configure hera, hades, nemesis, athena
```

**Tools** (manual execution via consolidated binary):

```bash
./bin/tools activity-history-update    # Update player activity history
./bin/tools fix-sherpa-clears         # Fix sherpa/first-clear data
```

### Incorrect Usage

Don't mix naming patterns:

- ❌ `bin/hercules` for a tool (use descriptive name)
- ❌ `bin/player-lookup` for a service (use mythological name)
- ❌ `bin/manifest` for a cron job (use mythological name)
- ✅ `./bin/tools activity-history-update` for a tool command
- ✅ `bin/athena` for a manifest downloader service

## Summary

| Type                  | Naming Pattern    | Examples                                                           |
| --------------------- | ----------------- | ------------------------------------------------------------------ |
| Long-running services | Greek mythology   | hermes, atlas, zeus                                                |
| Cron jobs             | Greek mythology   | hera, hades, nemesis, athena                                       |
| Manual tools          | Descriptive names | activity-history-update, fix-sherpa-clears (via ./bin/tools <cmd>) |
