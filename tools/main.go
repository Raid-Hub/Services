package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	activityhistory "raidhub/tools/activity-history-update"
	bob "raidhub/tools/bob"
	fixsherpa "raidhub/tools/fix-sherpa-clears"
	flagrestricted "raidhub/tools/flag-restricted-pgcrs"
	pan "raidhub/tools/pan"
	processpgcr "raidhub/tools/process-single-pgcr"
	updateskull "raidhub/tools/update-skull-hashes"

	"github.com/joho/godotenv"
)

var commands = map[string]func(){
	"update-skull-hashes":     updateskull.UpdateSkullHashes,
	"flag-restricted-pgcrs":   flagrestricted.FlagRestrictedPGCRs,
	"process-single-pgcr":     processpgcr.ProcessSinglePGCR,
	"activity-history-update": activityhistory.ActivityHistoryUpdate,
	"fix-sherpa-clears":       fixsherpa.FixSherpaClears,
	"pan":                     pan.Main,
	"bob":                     bob.Main,
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

	// load .env
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
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
