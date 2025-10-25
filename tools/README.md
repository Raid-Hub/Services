# Tools

This directory contains utility scripts and tools for manual execution. All tools are consolidated into a single `tools` binary that dispatches commands.

## Available Tools

Run any tool with:

```bash
./bin/tools <command>
```

### Available Commands

- `activity-history-update` - Updates activity history for players who haven't been crawled recently
- `fix-sherpa-clears` - Rebuilds sherpa and first clear columns to fix race conditions
- `flag-restricted-pgcrs` - Flags PGCRs as restricted based on various criteria
- `process-single-pgcr` - Processes a single PGCR by instance ID
- `update-skull-hashes` - Updates skull hashes in the database

## Building

Build the tools binary:

```bash
make tools
```

Or manually:

```bash
go build -o ./bin/tools ./tools
```

## Running

Run any tool command:

```bash
./bin/tools activity-history-update
./bin/tools fix-sherpa-clears
./bin/tools flag-restricted-pgcrs
./bin/tools process-single-pgcr
./bin/tools update-skull-hashes
```

To see all available commands:

```bash
./bin/tools
```

## Structure

The tools directory contains:

- `main.go` - Command dispatcher and entry point
- Individual tool subdirectories, each with its own package:
  - `activity-history-update/` - Activity history updater
  - `fix-sherpa-clears/` - Sherpa clears fixer
  - `flag-restricted-pgcrs/` - PGCR flagger
  - `process-single-pgcr/` - Single PGCR processor
  - `update-skull-hashes/` - Skull hash updater

Each tool is a separate package with an exported function that implements the tool logic.
