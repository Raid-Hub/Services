package logging

import (
	"flag"
	"raidhub/lib/env"
	"strings"
	"sync"
)

var (
	verbose      bool
	logLevel     string
	once         sync.Once
	verboseFlag *bool
	verboseLongFlag *bool
	logLevelFlag *string
	logLevelLongFlag *string
)

// logLevelPriority maps log levels to their numeric priority (higher = more important)
var logLevelPriority = map[string]int{
	Debug: 0,
	Info:  1,
	Warn:  2,
	Error: 3,
	// fatal is not a configurable level - it's always shown when error level is enabled
}

func init() {
	// Default log level is "info"
	logLevel = Info

	// Check environment variable first
	if envLevel := env.LogLevel; isValidLogLevel(envLevel) {
		logLevel = strings.ToLower(envLevel)
	}

	// Create a separate FlagSet for logging flags to avoid conflicts with application flags
	verboseFlag = flag.Bool("v", false, "enable verbose (debug) logging")
	verboseLongFlag = flag.Bool("verbose", false, "enable verbose (debug) logging")
	logLevelFlag = flag.String("log", "", "set log level (debug, info, warn, error)")
	logLevelLongFlag = flag.String("log-level", "", "set log level (debug, info, warn, error)")

}

// ParseFlags parses command-line flags using the standard flag package and processes
// logging-specific flags (-v, -verbose, -log, -log-level).
//
// This function should be used instead of flag.Parse() throughout the codebase to ensure:
// 1. Logging flags are properly processed and applied
// 2. Flag parsing happens only once (via sync.Once)
// 3. Consistent flag parsing behavior across all applications and tools
//
// Usage: Call logging.ParseFlags() in your main() function after defining all flags
// and before accessing flag values or flag.Args()/flag.NArg().
func ParseFlags() {
	once.Do(func() {
		if (!flag.Parsed()) {
			flag.Parse()
		}
		
		// Set verbose if either flag was provided (overrides log level for backwards compatibility)
		if *verboseFlag || *verboseLongFlag {
			verbose = true
			logLevel = Debug
		}

		// If log-level flag is provided, use it (overrides env var and verbose flags)
		if *logLevelFlag != "" && isValidLogLevel(*logLevelFlag) || *logLevelLongFlag != "" && isValidLogLevel(*logLevelLongFlag) {
			if *logLevelFlag != "" && isValidLogLevel(*logLevelFlag) {
				logLevel = strings.ToLower(*logLevelFlag)
			} else if *logLevelLongFlag != "" && isValidLogLevel(*logLevelLongFlag) {
				logLevel = strings.ToLower(*logLevelLongFlag)
			}
			verbose = (logLevel == Debug)
		}
	})
}

// isValidLogLevel checks if the provided log level is valid
func isValidLogLevel(level string) bool {
	if level == "" {
		return false
	}
	_, ok := logLevelPriority[strings.ToLower(level)]
	return ok
}

// IsVerbose returns true if verbose logging is enabled via either flag or log level is debug
func IsVerbose() bool {
	return verbose || logLevel == Debug
}

// GetLogLevel returns the current log level
func GetLogLevel() string {
	return logLevel
}

// SetLogLevel programmatically sets the log level
func SetLogLevel(level string) {
	if isValidLogLevel(level) {
		logLevel = strings.ToLower(level)
		verbose = (logLevel == Debug)
	}
}

// SetVerbose programmatically enables/disables verbose logging
func SetVerbose(enabled bool) {
	verbose = enabled
	if enabled {
		logLevel = Debug
	}
}

// ShouldLog checks if a given log level should be logged based on current log level
func ShouldLog(level string) bool {
	currentPriority := logLevelPriority[logLevel]
	logPriority := logLevelPriority[level]
	return logPriority >= currentPriority
}
