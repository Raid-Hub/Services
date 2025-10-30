package logging

import (
	"flag"
	"os"
)

var verbose bool

func init() {
	flag.Bool("verbose", false, "--verbose")
	flag.Bool("v", false, "-v")
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			verbose = true
			break
		}
	}
}

// IsVerbose returns true if verbose logging is enabled via either flag
func IsVerbose() bool {
	return verbose
}

// SetVerbose programmatically enables/disables verbose logging
func SetVerbose(enabled bool) {
	verbose = enabled
}
