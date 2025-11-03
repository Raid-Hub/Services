package logging

import (
	"flag"
	"os"
	"raidhub/lib/env"
	"strings"
)

var (
	verbose      bool
	logLevel     string
	loggingFlags *flag.FlagSet
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
	loggingFlags = flag.NewFlagSet("logging", flag.ContinueOnError)
	verboseFlag := loggingFlags.Bool("v", false, "enable verbose (debug) logging")
	verboseLongFlag := loggingFlags.Bool("verbose", false, "enable verbose (debug) logging")
	logLevelFlag := loggingFlags.String("log", "", "set log level (debug, info, warn, error)")
	logLevelLongFlag := loggingFlags.String("log-level", "", "set log level (debug, info, warn, error)")

	// Parse logging flags from os.Args (skip program name with os.Args[1:])
	// Ignoring errors (flags might not be present)
	// This allows logging flags to work independently of when application flags are parsed
	if len(os.Args) > 1 {
		loggingFlags.Parse(os.Args[1:])
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
