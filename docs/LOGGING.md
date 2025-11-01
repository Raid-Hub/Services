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

## Log Levels

### DEBUG

- **Purpose**: Detailed information for debugging and troubleshooting
- **Usage**: Only logged when verbose flag is explicitly passed (`--verbose`, `-v`, etc.)
- **Persistence**: **NOT** persisted by default - only shown when debugging
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
logger.Warn("API_CONNECTION_FAILED", map[string]any{
    "service": "bungie",
    "error": err,
    "attempt": attemptCount,
    "action": "retrying",
})
logger.Warn("INVALID_DATA_DETECTED", map[string]any{
    "entity": "player",
    "playerId": playerId,
    "issue": "completion_data",
    "error": err,
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
logger.Error("AUTHENTICATION_FAILED", map[string]any{
    "user_id": userId,
    "error": err,
    "action": "access_denied",
})
logger.Error("DATA_CORRUPTION_DETECTED", map[string]any{
    "entity": "instance",
    "instance_id": instanceId,
    "issue": "invalid_completion_data",
    "error": err,
})
```

### FATAL

- **Purpose**: Unrecoverable errors that require the application to crash
- **Usage**: Critical system failures where the app cannot continue safely
- **Persistence**: **PERSISTED** and **ALERTED** - logs then **CRASHES** with panic
- **Examples**:
  - Database connection failures during startup
  - Critical configuration missing
  - System resource exhaustion
  - Programming errors that violate invariants

```go
logger.Fatal("DATABASE_CONNECTION_FAILED", map[string]any{
    "phase": "startup",
    "type": "postgresql",
    "error": err,
})
logger.Fatal("CONFIGURATION_MISSING", map[string]any{
    "key": configKey,
    "phase": "startup",
    "severity": "critical",
})
```

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
    logger.Warn("EXTERNAL_API_CALL_FAILED", map[string]any{
        "error": err,
        "action": "retrying",
    })
    // Continue with retry logic
}

// Example: Critical authentication failure (requires alert)
if err := validateUserPermissions(userId); err != nil {
    logger.Error("USER_PERMISSION_VALIDATION_FAILED", map[string]any{
        "userId": userId,
        "error": err,
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
    logger.Fatal("DATABASE_CONNECTION_FAILED", map[string]any{
        "phase": "startup",
        "error": err,
    })
    // Application crashes here with panic
}
```

### When NOT to Use FATAL

- **User input errors** - return error instead
- **Individual request failures** - use ERROR (if critical) or WARN and continue
- **Data processing errors** - use ERROR (if serious) or WARN and skip item
- **Expected business logic failures** - use ERROR (if needs alerts) or WARN/INFO

## Interface Reference

| Method    | Signature                                  | Usage                    | Output |
| --------- | ------------------------------------------ | ------------------------ | ------ |
| `Info()`  | `Info(key string, fields map[string]any)`  | Operational information  | stdout |
| `Warn()`  | `Warn(key string, fields map[string]any)`  | Issues needing attention | stderr |
| `Error()` | `Error(key string, fields map[string]any)` | Sentry alerts            | stderr |
| `Debug()` | `Debug(key string, fields map[string]any)` | Verbose flag only        | stdout |
| `Fatal()` | `Fatal(key string, fields map[string]any)` | Logs then crashes        | stderr |

- **Message**: SCREAMING_UPPER_CASE string for the event
- **Fields**: Structured key-value pairs using `map[string]any{}`

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

Always include relevant context with errors:

```go
logger.Warn("DATABASE_QUERY_FAILED", map[string]any{
    "query": "SELECT * FROM users",
    "params": params,
    "error": err,
})
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
log.Fatalf("err: %v", err) → logger.Fatal("ERROR", map[string]any{"error": err})

// Setup
import "raidhub/lib/utils/logging"
var logger = logging.NewLogger("SERVICE_NAME")
```

**Benefits**: Service identification, Sentry integration, proper log levels, structured logging

## Monitoring & Alerting

- **DEBUG**: Only shown with verbose flag, not persisted
- **INFO**: Persisted for operational visibility
- **WARN**: Persisted for monitoring and analysis
- **ERROR**: Persisted and triggers Sentry alerts
- **FATAL**: Persisted, triggers alerts, then crashes app

All logs are structured JSON for easy querying and analysis.

## Loki / Promtail / Grafana (Local Dev)

### Startup

1. Generate configs:

```
make config
```

2. Start via Tilt (recommended):

```
tilt up
```

3. Or start via Docker Compose only:

```
docker-compose up -d
```

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
logger.Warn(SUSPICIOUS_ACTIVITY_DETECTED, map[string]any{
    logging.MEMBERSHIP_ID: playerId,
    logging.TYPE: "stat_anomaly",
    logging.ACTION: "flagged_for_review",
})
logger.Fatal(DATABASE_CONNECTION_FAILED, map[string]any{
    logging.DB_TYPE: "postgresql",
    logging.OPERATION: "query_player_stats",
    logging.ERROR: err.Error(),
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
logger.Warn("POSTGRES_CONNECTION_POOL_APPROACHING_LIMIT", map[string]any{
    logging.COUNT: count,
    logging.TYPE: "active_connections",
    logging.ACTION: "monitor_pool_usage",
})
```

---

This logging system provides consistent, searchable, and monitorable logs across all RaidHub Services while maintaining clear service boundaries and appropriate log levels for different operational needs.
