package processpgcr

import (
	"flag"
	"fmt"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils"
	"strconv"
)

var processPGCRLogger = utils.NewLogger("PROCESS_PGCR_TOOL")

func ProcessSinglePGCR() {
	// 1. Parse the instance ID from command line args
	// Since main.go uses flag.Parse(), the actual arguments start from flag.Arg(1)
	if flag.NArg() < 2 {
		processPGCRLogger.ErrorF("Usage: scripts process-single-pgcr <instance_id>")
	}

	instanceId, err := strconv.ParseInt(flag.Arg(1), 10, 64)
	if err != nil {
		processPGCRLogger.ErrorFf("Invalid instance ID: %v", err)
	}

	processPGCRLogger.InfoF("Processing PGCR with instance ID: %d", instanceId)

	// 2. Fetch and process the PGCR
	result, processedActivity, rawPGCR, err := pgcr_processing.FetchAndProcessPGCR(instanceId)
	if err != nil {
		processPGCRLogger.ErrorFf("Failed to fetch and process PGCR: %v", err)
	}

	if result != pgcr_processing.Success {
		processPGCRLogger.ErrorFf("PGCR fetch failed with result: %v", result)
	}

	if processedActivity == nil {
		processPGCRLogger.ErrorF("Processed activity is nil")
	}

	if rawPGCR == nil {
		processPGCRLogger.ErrorF("Raw PGCR is nil")
	}

	processPGCRLogger.InfoF("Successfully fetched and processed PGCR")

	// 3. Store the PGCR
	lag, isNew, err := instance_storage.StorePGCR(processedActivity, rawPGCR)
	if err != nil {
		processPGCRLogger.ErrorFf("Failed to store PGCR: %v", err)
	}

	if isNew {
		processPGCRLogger.InfoF("✓ Stored NEW PGCR with lag: %v", lag)
	} else {
		processPGCRLogger.InfoF("✓ PGCR already exists (lag: %v)", lag)
	}

	fmt.Printf("\n=== PGCR Processing Complete ===\n")
	fmt.Printf("Instance ID: %d\n", processedActivity.InstanceId)
	fmt.Printf("Activity Hash: %d\n", processedActivity.Hash)
	fmt.Printf("Date Started: %s\n", processedActivity.DateStarted)
	fmt.Printf("Duration: %d seconds\n", processedActivity.DurationSeconds)
	fmt.Printf("Players: %d\n", len(processedActivity.Players))
	fmt.Printf("Fresh: %t\n", *processedActivity.Fresh)
	fmt.Printf("Flawless: %t\n", *processedActivity.Flawless)
	fmt.Printf("Completed: %t\n", processedActivity.Completed)
	if lag != nil {
		fmt.Printf("Processing Lag: %v\n", *lag)
	}
}
