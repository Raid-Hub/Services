# Logger Setup Guide

Each domain package should have its own logger that uses the shared `utils.Logger` interface.

## Quick Setup

For any new domain package (e.g., `lib/domains/clan`, `lib/domains/pgcr`), create a `logger.go` file:

## File: `lib/domains/{domain}/logger.go`

```go
package {domain}

import (
	"raidhub/lib/utils"
)

var {domain}Logger utils.Logger

func init() {
	{domain}Logger = utils.NewLogger("{domain}")
}
```

## Example: Player Logger

See `lib/domains/player/logger.go`:

```go
package player

import (
	"raidhub/lib/utils"
)

var PlayerLogger utils.Logger

func init() {
	PlayerLogger = utils.NewLogger("player")
}
```

## Usage

Once set up, you can use the logger anywhere in your domain package:

```go
package {domain}

import "raidhub/lib/domains/{domain}"

func doSomething() {
	{domain}.{Domain}Logger.Info("Starting process", "key", "value")
	{domain}.{Domain}Logger.Error("Something went wrong", "error", err)
	{domain}.{Domain}Logger.Warn("Warning message", "field", value)
	{domain}.{Domain}Logger.Debug("Debug info", "detail", info)
}
```

## How It Works

1. **Shared Interface**: All domain loggers implement `utils.Logger` from `lib/utils/logger.go`
2. **Package-Level Variable**: The logger is declared at package level
3. **Auto-Initialization**: The `init()` function runs when the package is imported, ensuring the logger is always initialized before use
4. **Thread-Safe**: Go's `init()` functions are guaranteed to run exactly once before any other code, making additional synchronization unnecessary

## Interface Methods

- `Info(msg string, fields ...any)` - Informational messages
- `Warn(msg string, fields ...any)` - Warning messages
- `Error(msg string, fields ...any)` - Error messages
- `Debug(msg string, fields ...any)` - Debug messages

All methods accept a message string followed by key-value pairs for structured logging.
