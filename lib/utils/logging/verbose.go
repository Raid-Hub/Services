package logging

import (
	"flag"
	"os"
)

var (
	verbose      bool
	loggingFlags *flag.FlagSet
)

func init() {
	// Create a separate FlagSet for logging flags to avoid conflicts with application flags
	loggingFlags = flag.NewFlagSet("logging", flag.ContinueOnError)
	verboseFlag := loggingFlags.Bool("v", false, "enable verbose (debug) logging")
	verboseLongFlag := loggingFlags.Bool("verbose", false, "enable verbose (debug) logging")
	
	// Parse logging flags from os.Args, ignoring errors (flags might not be present)
	// This allows logging flags to work independently of when application flags are parsed
	loggingFlags.Parse(os.Args)
	
	// Set verbose if either flag was provided
	verbose = *verboseFlag || *verboseLongFlag
}

// IsVerbose returns true if verbose logging is enabled via either flag
func IsVerbose() bool {
	return verbose
}

// SetVerbose programmatically enables/disables verbose logging
func SetVerbose(enabled bool) {
	verbose = enabled
}

// SyncLoggingFlags should be called by applications after flag.Parse() if they want
// logging flags to work even when other flags are present. This is optional - the
// init() function will handle most cases, but this ensures logging flags work
// correctly even when applications have their own flag definitions.
func SyncLoggingFlags() {
	verboseFlag := flag.Lookup("v")
	verboseLongFlag := flag.Lookup("verbose")
	
	if verboseFlag != nil && verboseFlag.Value.String() == "true" {
		verbose = true
	} else if verboseLongFlag != nil && verboseLongFlag.Value.String() == "true" {
		verbose = true
	}
}
