package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var commands = map[string]func(){
	"update-skull-hashes": updateSkullHashes,
	"flag-restricted-pgcrs": flagRestrictedPGCRs,
	"process-single-pgcr": processSinglePGCR,
	// Add more commands here
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
	fmt.Println("Usage: go run ./scripts <command>")
	fmt.Println("Available commands:")
	for name := range commands {
		fmt.Printf("  - %s\n", name)
	}
}
