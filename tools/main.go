package main

import (
	"flag"
	"fmt"
	"os"

	activityhistory "raidhub/tools/activity-history-update"
	fixsherpa "raidhub/tools/fix-sherpa-clears"
	flagrestricted "raidhub/tools/flag-restricted-pgcrs"
	processpgcr "raidhub/tools/process-single-pgcr"
	updateskull "raidhub/tools/update-skull-hashes"
)

var commands = map[string]func(){
	"update-skull-hashes":     updateskull.UpdateSkullHashes,
	"flag-restricted-pgcrs":   flagrestricted.FlagRestrictedPGCRs,
	"process-single-pgcr":     processpgcr.ProcessSinglePGCR,
	"activity-history-update": activityhistory.ActivityHistoryUpdate,
	"fix-sherpa-clears":       fixsherpa.FixSherpaClears,
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		printUsage(commands)
		os.Exit(1)
	}

	cmd := flag.Arg(0)
	fn, exists := commands[cmd]
	if !exists {
		fmt.Printf("Unknown command: %s\n", cmd)
		printUsage(commands)
		os.Exit(1)
	}

	fn()
}

func printUsage(commands map[string]func()) {
	fmt.Println("Usage: ./bin/tools <command>")
	fmt.Println("\nAvailable commands:")
	for name := range commands {
		fmt.Printf("  - %s\n", name)
	}
}
