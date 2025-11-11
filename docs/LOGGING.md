# Logging Standards and Guidelines

## Overview

RaidHub Services uses a centralized logging system with structured log levels and consistent naming conventions. Each service has its own logger to ensure proper service identification and organized output.

## Logger Setup

### Package-Level Loggers

Each package should declare a logger at the package level using descriptive names. **Do not create separate `logger.go` files** - declare loggers directly in your main package files.

```go
package myservice

import (
    "raidhub/lib/utils/logging"
    // other imports...
)

var logger = logging.NewLogger("SERVICE_NAME")

// Your service code here...
```

### Real Example

See `lib/services/pgcr_processing/process-pgcr.go`:

```go
package pgcr_processing

import (
    "errors"
    "fmt"
    "raidhub/lib/dto"
    "raidhub/lib/utils/logging"
    // other imports...
)

var logger = logging.NewLogger("PGCR_PROCESSING_SERVICE")

func ProcessPGCR(pgcr *bungie.DestinyPostGameCarnageReport) (*dto.ProcessedInstance, PGCRResult) {
    logger.Debug("STARTING_PGCR_PROCESSING", map[string]any{
        "instanceId": pgcr.ActivityDetails.InstanceId,
    })
    // ... processing logic
}
```

### Naming Conventions

- **Services** (`lib/services/`): Use `*_SERVICE` suffix

  - `INSTANCE_STORAGE_SERVICE`
  - `CHEAT_DETECTION_SERVICE`
  - `PGCR_PROCESSING_SERVICE`
  - `PLAYER_SERVICE`
  - `CHARACTER_SERVICE`
  - `INSTANCE_SERVICE`
  - `CLAN_SERVICE`

- **Infrastructure** (`lib/database/`, `lib/messaging/`, etc.): Use component name

  - `POSTGRES`
  - `MONITORING`
  - `MIGRATIONS` (in lib/migrations) or `Migrations` (in lib/database/migrations)

- **Applications** (`apps/`): Use Greek mythology names (matching app names)

  - `atlas` (main logger)
  - `atlas::metricsService` (sub-logger)
  - `atlas::offloadWorker` (sub-logger)
  - `hermes`
  - `zeus`

- **Tools** (`tools/`): Use tool-specific names (SCREAMING_SNAKE_CASE)

  - `MISSED_PGCR`
  - `MANIFEST_DOWNLOADER`
  - `LEADERBOARD_CLAN_CRAWL`
  - `CHEAT_DETECTION`
  - `REFRESH_VIEW_TOOL`
  - `TOOLS` (for general tool logging)
  - `FLAG_RESTRICTED_TOOL`
  - `PROCESS_PGCR_TOOL`
  - `FIX_SHERPA_TOOL`
  - `UPDATE_SKULL_TOOL`
  - `SEED`
  - `MIGRATIONS` or `Migrations` (depending on package)

- **Web Clients** (`lib/web/`): Use service name with `_CLIENT`
  - `BUNGIE_CLIENT`
  - `PROMETHEUS_API_CLIENT`

## Configuration

### Log Level

The logging system supports configurable log levels to control verbosity. Log levels can be set via:

1. **Environment Variable**: `LOG_LEVEL` (recommended for production)

   ```bash
   export LOG_LEVEL=warn  # Only show warnings and errors
   ```

2. **CLI Flags**: `-log-level` or `-log` (for tools and services)

   ```bash
   ./bin/hermes --log-level=debug
   ./bin/atlas --log debug
   ```

3. **Verbose Flag**: `-v` or `-verbose` (equivalent to `debug` level)
   ```bash
   ./bin/hermes --verbose
   ```

**Available Log Levels** (in order of severity):

- `debug` - Most verbose, includes all DEBUG logs
- `info` - Default level, shows operational information
- `warn` - Only warnings and errors
- `error` - Only errors (includes FATAL logs)

When a log level is set, only logs at that level or higher will be output. For example, setting `LOG_LEVEL=warn` will show WARN and ERROR logs (including FATAL), but hide INFO and DEBUG logs.

**Note**: `Fatal` is not a separate configurable log level - fatal logs are always shown when `error` level is enabled. The `Fatal()` method will always exit the application after logging.

### Output Redirection

Logs can be redirected to files while still maintaining console output. This is useful for:

- Log aggregation and analysis
- Long-term log storage
- Separating logs from application output

**Environment Variables**:

- `STDOUT` - Redirects INFO and DEBUG logs to a file (in addition to console)
- `STDERR` - Redirects WARN, ERROR, and FATAL logs to a file (in addition to console)

**Behavior**:
When `STDOUT` or `STDERR` environment variables are set, logs are written to **both** the file and the original console output. This ensures logs are always visible in the console while also being persisted to files.

```bash
# Write logs to files while keeping console output
export STDOUT=./logs/app.log
export STDERR=./logs/errors.log
./bin/hermes

# Logs appear both in console and in files
```

**File Handling**:

- Files are created automatically if they don't exist
- Logs are appended to existing files (no truncation)
- If file creation fails, the application will panic on startup

## Log Levels

### DEBUG

- **Purpose**: Detailed information for debugging and troubleshooting
- **Usage**: Only logged when log level is set to `debug` (via `LOG_LEVEL=debug`, `--verbose`, `-v`, `--log-level=debug`, or `--log debug`)
- **Persistence**: Only shown when debugging - hidden by default
- **Examples**:
  - Variable values during processing
  - Detailed API request/response data
  - Step-by-step algorithm execution
  - Internal state information

```go
logger.Debug("REQUEST_PROCESSING", map[string]any{
    "userId": userId,
    "stage": "validation",
    "requestData": data,
})
logger.Debug("CACHE_OPERATION", map[string]any{
    "type": "hit",
    "key": key,
    "value": value,
})
```

**Note**: DEBUG logs should be implemented to respect verbose flags in applications.

### INFO

- **Purpose**: Important operational information for monitoring and tracking
- **Usage**: Important events, successful operations, system state changes
- **Persistence**: **PERSISTED** - expect these logs to be searchable
- **Examples**:
  - Service startup/shutdown
  - Successful database connections
  - Important business logic milestones
  - Performance metrics

```go
logger.Info("SERVICE_STARTED", map[string]any{
    "port": 8080,
    "status": "ready",
})
logger.Info("BATCH_PROCESSED", map[string]any{
    "type": "pgcr",
    "count": count,
    "duration": duration,
})
```

### WARN

- **Purpose**: Issues that should be monitored but don't require alerts
- **Usage**: Problems that don't crash the app but need attention
- **Persistence**: **PERSISTED** - logged for monitoring and analysis
- **Examples**:
  - API failures that are retried
  - Data inconsistencies
  - Performance degradation
  - External service errors
  - Business logic violations

```go
logger.Warn("API_CONNECTION_FAILED", err, map[string]any{
    "service": "bungie",
    "attempt": attemptCount,
    "action": "retrying",
})
logger.Warn("INVALID_DATA_DETECTED", err, map[string]any{
    "entity": "player",
    "playerId": playerId,
    "issue": "completion_data",
})
// Can pass nil if there's no error
logger.Warn("PERFORMANCE_DEGRADATION", nil, map[string]any{
    "response_time": "2s",
    "threshold": "500ms",
})
```

### ERROR

- **Purpose**: Errors that should be monitored and alerted on (Sentriable errors)
- **Usage**: Problems that need immediate attention but don't crash the app
- **Persistence**: **PERSISTED** and **ALERTED** - expect these to trigger Sentry alerts
- **Examples**:
  - Critical business logic failures
  - Data corruption issues
  - Authentication/authorization failures
  - External service dependencies failing
  - Operations that must succeed but failed

```go
logger.Error("AUTHENTICATION_FAILED", err, map[string]any{
    "user_id": userId,
    "action": "access_denied",
})
logger.Error("DATA_CORRUPTION_DETECTED", err, map[string]any{
    "entity": "instance",
    "instance_id": instanceId,
    "issue": "invalid_completion_data",
})
// Error is automatically added to fields with key "error" if provided
```

### FATAL

- **Purpose**: Unrecoverable errors that require the application to crash
- **Usage**: Critical system failures where the app cannot continue safely
- **Log Level**: Treated as `error` level for filtering purposes
- **Persistence**: **PERSISTED** and **ALERTED** - logs then **CRASHES** with `os.Exit(1)`
- **Examples**:
  - Database connection failures during startup
  - Critical configuration missing
  - System resource exhaustion
  - Programming errors that violate invariants

```go
logger.Fatal("DATABASE_CONNECTION_FAILED", err, map[string]any{
    "phase": "startup",
    "type": "postgresql",
})
logger.Fatal("CONFIGURATION_MISSING", nil, map[string]any{
    "key": configKey,
    "phase": "startup",
    "severity": "critical",
})
// Error is automatically added to fields with key "error" if provided
```

**Note**: `Fatal` is not a separate configurable log level. Fatal logs are always shown when the log level is set to `error` or lower. The `Fatal()` method will always exit the application after logging, regardless of log level configuration.

## Error Handling Philosophy

### Use WARN for Monitoring Issues

Use `logger.Warn()` for problems that:

- Don't crash the application
- Should be monitored but don't require immediate alerts
- Can be handled gracefully (retries, fallbacks, etc.)
- Indicate potential issues that need tracking

### Use ERROR for Critical Issues

Use `logger.Error()` for problems that:

- Don't crash the application
- Require immediate attention via Sentry alerts
- Indicate serious operational problems
- Need prompt investigation and resolution

```go
// Example: API failure with retry (monitoring)
if err := externalAPI.Call(); err != nil {
    logger.Warn("EXTERNAL_API_CALL_FAILED", err, map[string]any{
        "action": "retrying",
    })
    // Continue with retry logic
}

// Example: Critical authentication failure (requires alert)
if err := validateUserPermissions(userId); err != nil {
    logger.Error("USER_PERMISSION_VALIDATION_FAILED", err, map[string]any{
        "userId": userId,
        "action": "access_denied",
    })
    return fmt.Errorf("access denied: %w", err)
}
```

### Use FATAL for Unrecoverable Issues

Use `logger.Fatal()` for problems that:

- Make the application unable to continue safely
- Require immediate restart/intervention
- Indicate critical system failures

```go
// Example: Critical startup failure
if err := database.Connect(); err != nil {
    logger.Fatal("DATABASE_CONNECTION_FAILED", err, map[string]any{
        "phase": "startup",
    })
    // Application crashes here with os.Exit(1)
}
```

### When NOT to Use FATAL

- **User input errors** - return error instead
- **Individual request failures** - use ERROR (if critical) or WARN and continue
- **Data processing errors** - use ERROR (if serious) or WARN and skip item
- **Expected business logic failures** - use ERROR (if needs alerts) or WARN/INFO

## Interface Reference

| Method    | Signature                                             | Usage                    | Output | Respects Log Level           |
| --------- | ----------------------------------------------------- | ------------------------ | ------ | ---------------------------- |
| `Info()`  | `Info(key string, fields map[string]any)`             | Operational information  | stdout | Yes                          |
| `Warn()`  | `Warn(key string, err error, fields map[string]any)`  | Issues needing attention | stderr | Yes                          |
| `Error()` | `Error(key string, err error, fields map[string]any)` | Sentry alerts            | stderr | Yes                          |
| `Debug()` | `Debug(key string, fields map[string]any)`            | Verbose flag only        | stdout | Yes                          |
| `Fatal()` | `Fatal(key string, err error, fields map[string]any)` | Logs then crashes        | stderr | Yes (treated as error level) |

**Output Behavior**:

- INFO and DEBUG logs are written to stdout (or both stdout and file if `STDOUT` is set)
- WARN, ERROR, and FATAL logs are written to stderr (or both stderr and file if `STDERR` is set)
- All logging methods respect the configured log level - logs below the current level are not output
- `Fatal` logs are shown when log level is `error` or lower (fatal is not a separate configurable level)

**Error Parameter**:

- `Warn()`, `Error()`, and `Fatal()` methods accept an `error` as the second parameter
- If the error is not `nil`, it is automatically added to the fields map with the key `"error"` (using `err.Error()`)
- You can pass `nil` if there's no error to log
- The `fields` parameter can be `nil` if you only want to log the error

**Parameters**:

- **Message**: SCREAMING_UPPER_CASE string for the event
- **Error**: Optional error to include in the log (automatically added to fields with key "error")
- **Fields**: Structured key-value pairs using `map[string]any{}` (can be `nil`)

## Best Practices

### 1. Structured Logging ⭐ CRITICAL

**ALWAYS use key-value pairs** - never plain strings:

```go
// ✅ GOOD - Structured with context
logger.Info("DATABASE_CONNECTED", map[string]any{
    "type": "postgresql",
})
logger.Info("USER_LOGIN", map[string]any{
    "userId": 12345,
    "ip": "192.168.1.1",
    "status": "success",
})
logger.Info("REQUEST_PROCESSED", map[string]any{
    "method": "POST",
    "endpoint": "/api/users",
    "duration": "150ms",
})

// ❌ BAD - Plain strings (hard to search/filter)
logger.Info("DATABASE_CONNECTION_ESTABLISHED", nil)
logger.Info("USER_LOGIN_SUCCESSFUL", nil)
logger.Info("USER_LOGIN", nil) // Still not structured - missing fields!
```

**Why structured logging matters:**

- **Searchable**: `grep 'type.*postgresql'` finds all postgres connections
- **Filterable**: Log aggregators can filter by key-value pairs
- **Contextual**: Always includes relevant metadata for debugging

### 2. Consistent Error Context

Always include relevant context with errors. The error parameter is automatically added to fields:

```go
logger.Warn("DATABASE_QUERY_FAILED", err, map[string]any{
    "query": "SELECT * FROM users",
    "params": params,
})
// Error is automatically added to fields with key "error"
```

### 3. Performance Considerations

- DEBUG logs should not impact production performance (only shown with verbose flag)
- Use structured logging over string formatting when possible
- Avoid logging large objects at INFO level

### 4. Security

- **Never log sensitive data**: passwords, API keys, tokens, PII
- Redact or hash sensitive fields when logging is necessary
- Be careful with user-generated content

```go
// ✅ GOOD - Structured with safe data
logger.Info("API_REQUEST_COMPLETED", map[string]any{
    "method": "GET",
    "endpoint": "/api/users",
    "userId": userId,
    "status": 200,
})

// ❌ BAD - Contains sensitive data
logger.Info("API_REQUEST", map[string]any{
    "headers": headers,
    "body": body,
})

// ❌ BAD - Plain string (not searchable)
logger.Info("API_REQUEST_COMPLETED_SUCCESSFULLY", nil)
```

## Quick Migration Guide

### Standard log → Domain logger:

```go
// Before
import "log"
log.Println("message")     → logger.Info("MESSAGE", nil)
log.Printf("msg %s", var)  → logger.Info("MESSAGE", map[string]any{"var": var})
log.Fatalf("err: %v", err) → logger.Fatal("ERROR", err, nil)

// Setup
import "raidhub/lib/utils/logging"
var logger = logging.NewLogger("SERVICE_NAME")
```

**Benefits**: Service identification, Sentry integration, proper log levels, structured logging

## Monitoring & Alerting

- **DEBUG**: Only shown when log level is `debug` (via `LOG_LEVEL` or `--verbose` flag)
- **INFO**: Shown at default `info` level or higher, persisted for operational visibility
- **WARN**: Shown at `warn` level or higher, persisted for monitoring and analysis
- **ERROR**: Shown at `error` level, persisted and triggers Sentry alerts
- **FATAL**: Shown at `error` level (not a separate configurable level), persisted, triggers alerts, then crashes app

All logs are structured using logfmt format for easy querying and analysis. Logs respect the configured log level and can be redirected to files via `STDOUT` and `STDERR` environment variables.

## Loki / Promtail / Grafana

### Local Development (Docker)

By default, Promtail is configured to scrape logs from Docker containers via Docker service discovery.

Start via Tilt (recommended):

```bash
tilt up
```

Or start via Docker Compose only:

```bash
docker-compose up -d
```

### Production (Undockerized)

When running services as native processes (not in Docker), Promtail won't collect logs by default because it's configured for Docker service discovery.

**To enable log collection in production:**

1. **Configure services to write logs to files** using `STDOUT` and `STDERR` environment variables:

   ```bash
   export STDOUT=/var/log/raidhub/app.log
   export STDERR=/var/log/raidhub/errors.log
   ```

2. **Update Promtail configuration** (`infrastructure/promtail/promtail.yml`) to read from filesystem instead of Docker:

   - Comment out the `docker_sd_configs` section
   - Uncomment and configure the `file_logs` job to point to your log file paths
   - Ensure Promtail has read access to the log files

3. **Deploy Loki and Promtail** separately (as systemd services, Kubernetes DaemonSet, or standalone processes)

### Access

- Grafana: http://localhost:${GRAFANA_PORT}
- Loki: http://localhost:${LOKI_PORT}

### LogQL Examples

- Errors by source (last 5m):

```
sum by (source) (count_over_time({level="ERROR"}[5m]))
```

- Errors by logger (last 5m):

```
sum by (logger) (count_over_time({level="ERROR"}[5m]))
```

- Recent logs from hermes container:

```
{source="hermes"} | line_format "{{.line}}" | limit 100
```

- Logs from HERMES logger (regardless of source):

```
{logger="HERMES"} | line_format "{{.line}}" | limit 100
```

- Warnings or errors from atlas container:

```
{source="atlas", level=~"WARN|ERROR"}
```

### Label Meanings

- **`source`**: Docker Compose service name (hermes, atlas, zeus, postgres, rabbitmq, clickhouse, etc.)
- **`logger`**: Logger name from log prefix `[LOGGER]` (HERMES, ATLAS, ZEUS, POSTGRES, etc.)
- **`level`**: Log level (INFO, WARN, ERROR, FATAL, DEBUG)
- **`container`**: Docker container name

### Notes

- Log format is preserved (structured text) and parsed by Promtail.
- Tilt now runs `hermes`, `atlas`, `zeus` as Docker services, so Promtail collects logs via Docker service discovery.
- Use `source` to filter by which container/app emitted the log.
- Use `logger` to filter by the logger name used in the code.

## Examples by Service Type

### Services

```go
// lib/services/cheat_detection/
import (
    "raidhub/lib/utils/logging"
)

// Constants or strings work, but they must be SCREAMING_SNAKE_CASE
const (
    SUSPICIOUS_ACTIVITY_DETECTED = "SUSPICIOUS_ACTIVITY_DETECTED"
    DATABASE_CONNECTION_FAILED = "DATABASE_CONNECTION_FAILED"
)

var logger = logging.NewLogger("CHEAT_DETECTION_SERVICE")

logger.Debug("STARTING_CHEAT_DETECTION_ANALYSIS", map[string]any{
    logging.MEMBERSHIP_ID: membershipId,
    logging.TYPE: "behavioral_analysis",
})
logger.Warn(SUSPICIOUS_ACTIVITY_DETECTED, nil, map[string]any{
    logging.MEMBERSHIP_ID: playerId,
    logging.TYPE: "stat_anomaly",
    logging.ACTION: "flagged_for_review",
})
logger.Fatal(DATABASE_CONNECTION_FAILED, err, map[string]any{
    logging.TYPE: "postgresql",
    logging.OPERATION: "query_player_stats",
})
```

### Applications

```go
// apps/hermes/main.go
import (
    "raidhub/lib/utils/logging"
)

// Constants for major lifecycle events
const (
    STARTING_TOPIC = "STARTING_TOPIC"
    STARTED_TOPIC = "STARTED_TOPIC"
)

var HermesLogger = logging.NewLogger("hermes")

HermesLogger.Info(STARTED_TOPIC, map[string]any{
    "topic": "instance_store",
    "mode": "all",
})
HermesLogger.Info("MISSED_PGCRS_FOUND", map[string]any{
    logging.COUNT: count,
    logging.TYPE: "pgcr",
    logging.ACTION: "queued_for_processing",
})
```

### Infrastructure

```go
// lib/database/postgres/
import (
    "raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("POSTGRES")

logger.Info("POSTGRES_CONNECTED", map[string]any{
    logging.STATUS: "ready",
})
logger.Warn("POSTGRES_CONNECTION_POOL_APPROACHING_LIMIT", nil, map[string]any{
    logging.COUNT: count,
    logging.TYPE: "active_connections",
    logging.ACTION: "monitor_pool_usage",
})
```

---

This logging system provides consistent, searchable, and monitorable logs across all RaidHub Services while maintaining clear service boundaries and appropriate log levels for different operational needs.
