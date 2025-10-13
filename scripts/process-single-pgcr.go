package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"raidhub/packages/bungie"
	"raidhub/packages/pgcr"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"
	"strconv"
	"time"
)

func processSinglePGCR() {
	// 1. Parse the instance ID from command line args
	// Since main.go uses flag.Parse(), the actual arguments start from flag.Arg(1)
	if flag.NArg() < 2 {
		log.Fatal("Usage: scripts process-single-pgcr <instance_id>")
	}

	instanceId, err := strconv.ParseInt(flag.Arg(1), 10, 64)
	if err != nil {
		log.Fatalf("Invalid instance ID: %v", err)
	}

	log.Printf("Processing PGCR with instance ID: %d", instanceId)

	// 2. Fetch the PGCR from Bungie
	apiKey := os.Getenv("BUNGIE_API_KEY")
	if apiKey == "" {
		log.Fatal("BUNGIE_API_KEY environment variable not set")
	}

	baseURL := os.Getenv("BUNGIE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://www.bungie.net"
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	decoder, statusCode, cleanup, err := bungie.GetPGCR(client, baseURL, instanceId, apiKey)
	if err != nil {
		log.Fatalf("Failed to fetch PGCR: %v", err)
	}
	defer cleanup()

	if statusCode != 200 {
		log.Fatalf("Bungie API returned non-200 status code: %d", statusCode)
	}

	// Decode the response
	var response bungie.DestinyPostGameCarnageReportResponse
	if err := decoder.Decode(&response); err != nil {
		log.Fatalf("Failed to decode PGCR response: %v", err)
	}

	if response.ErrorCode != 1 {
		log.Fatalf("Bungie API returned error: %s (code: %d)", response.ErrorStatus, response.ErrorCode)
	}

	log.Printf("Successfully fetched PGCR from Bungie")

	// 3. Process the PGCR
	// Connect to database
	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Connect to RabbitMQ
	conn, err := rabbit.Init()
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbit.Cleanup()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to create channel: %v", err)
	}
	defer ch.Close()

	// Process the PGCR
	processedActivity, err := pgcr.ProcessDestinyReport(&response.Response)
	if err != nil {
		log.Fatalf("Failed to process PGCR: %v", err)
	}

	log.Printf("Successfully processed PGCR for activity hash: %d", processedActivity.Hash)

	// Store the PGCR
	lag, isNew, err := pgcr.StorePGCR(processedActivity, &response.Response, db, ch)
	if err != nil {
		log.Fatalf("Failed to store PGCR: %v", err)
	}

	if isNew {
		log.Printf("✓ Stored NEW PGCR with lag: %v", lag)
	} else {
		log.Printf("✓ PGCR already exists (lag: %v)", lag)
	}

	fmt.Printf("\n=== PGCR Processing Complete ===\n")
	fmt.Printf("Instance ID: %d\n", instanceId)
	fmt.Printf("Activity Hash: %d\n", processedActivity.Hash)
	fmt.Printf("Date Started: %s\n", processedActivity.DateStarted)
	fmt.Printf("Duration: %d seconds\n", processedActivity.DurationSeconds)
	fmt.Printf("Players: %d\n", len(processedActivity.Players))
	fmt.Printf("Fresh: %t\n", *processedActivity.Fresh)
	fmt.Printf("Flawless: %t\n", *processedActivity.Flawless)
	fmt.Printf("Completed: %t\n", processedActivity.Completed)
	fmt.Printf("Processing Lag: %v\n", lag)
}