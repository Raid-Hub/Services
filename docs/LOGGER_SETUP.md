# Logger Setup Guide

Each domain package should have its own logger that uses the shared `utils.Logger` interface.

## Quick Setup

For any new domain package (e.g., `lib/services/clan`, `lib/services/pgcr`), create a `logger.go` file:

## File: `lib/services/{domain}/logger.go`

```go
package {domain}

import (
	"raidhub/lib/utils"
)

var {domain}Logger utils.Logger

func init() {
	{domain}Logger = utils.NewLogger("{DOMAIN_SERVICE}")
}
```

## Example: Real Implementation

See `lib/services/pgcr_processing/logger.go`:

```go
package pgcr_processing

import (
	"raidhub/lib/utils"
)

var PGCRLogger = utils.NewLogger("PGCR_PROCESSING_SERVICE")
```

Note: The `init()` function is optional - you can initialize the logger at package level as shown above.

## Usage

Once set up, you can use the logger anywhere in your domain package:

```go
package {domain}

func doSomething() {
	// Simple logging with variadic arguments
	{domain}Logger.Info("Starting process", "user_id", userId)
	{domain}Logger.Error("Something went wrong:", err)
	{domain}Logger.Warn("Warning message", "field_name", value)
	{domain}Logger.Debug("Debug info", "detail", info)

	// Formatted logging
	{domain}Logger.InfoF("Processing user %s with ID %d", username, userId)
	{domain}Logger.ErrorF("Failed to process: %v", err)
	{domain}Logger.WarnF("Warning for %s: %s", component, message)
	{domain}Logger.DebugF("Debug: %+v", debugObject)
}
```

## How It Works

1. **Shared Interface**: All domain loggers implement `utils.Logger` from `lib/utils/logger.go`
2. **Package-Level Variable**: The logger is declared at package level
3. **Auto-Initialization**: The `init()` function runs when the package is imported, ensuring the logger is always initialized before use
4. **Thread-Safe**: Go's `init()` functions are guaranteed to run exactly once before any other code, making additional synchronization unnecessary

## Interface Methods

### Basic Logging Methods

- `Info(v ...any)` - Informational messages
- `Warn(v ...any)` - Warning messages
- `Error(v ...any)` - Error messages
- `Debug(v ...any)` - Debug messages

### Formatted Logging Methods

- `InfoF(format string, args ...any)` - Formatted informational messages
- `WarnF(format string, args ...any)` - Formatted warning messages
- `ErrorF(format string, args ...any)` - Formatted error messages
- `DebugF(format string, args ...any)` - Formatted debug messages

**Basic methods** accept any number of arguments that will be printed space-separated.
**Formatted methods** use printf-style formatting with format strings and arguments.

## Migration from Standard Log Package

Many applications currently use the standard Go `log` package. To migrate:

### Before (using standard log):

```go
import "log"

log.Println("Starting process...")
log.Printf("Processing user %s", username)
log.Fatalf("Fatal error: %v", err)
```

### After (using domain logger):

```go
// In logger.go
var AppLogger = utils.NewLogger("APP_NAME")

// In your code
AppLogger.Info("Starting process...")
AppLogger.InfoF("Processing user %s", username)
AppLogger.Error("Fatal error:", err) // Note: no automatic exit like log.Fatal
```

**Benefits of migration:**

- Consistent log formatting across all services
- Service identification in log prefixes
- Better error separation (stderr vs stdout)
- Future extensibility (can easily add structured logging, file output, etc.)
