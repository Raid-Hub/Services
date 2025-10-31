package main

import (
	"flag"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
	"strconv"
)

var logger = logging.NewLogger("PROCESS_PGCR_TOOL")

func ProcessSinglePGCR() {
	// 1. Parse the instance ID from command line args
	// Since main.go uses flag.Parse(), the actual arguments start from flag.Arg(1)
	if flag.NArg() < 2 {
		logger.Error("USAGE_ERROR", map[string]any{"message": "Usage: scripts process-single-pgcr <instance_id>"})
		return
	}

	instanceId, err := strconv.ParseInt(flag.Arg(1), 10, 64)
	if err != nil {
		logger.Error("INVALID_INSTANCE_ID", map[string]any{logging.ERROR: err.Error()})
		return
	}

	logger.Info("PROCESSING_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// 2. Fetch and process the PGCR
	result, processedActivity, rawPGCR, err := pgcr_processing.FetchAndProcessPGCR(instanceId)
	if err != nil {
		logger.Error("FAILED_TO_FETCH_AND_PROCESS_PGCR", map[string]any{logging.INSTANCE_ID: instanceId, logging.ERROR: err.Error()})
		return
	}

	if result != pgcr_processing.Success {
		logger.Error("PGCR_FETCH_FAILED", map[string]any{logging.INSTANCE_ID: instanceId, "result": result})
		return
	}

	if processedActivity == nil {
		logger.Error("PROCESSED_ACTIVITY_IS_NIL", map[string]any{logging.INSTANCE_ID: instanceId})
		return
	}

	if rawPGCR == nil {
		logger.Error("RAW_PGCR_IS_NIL", map[string]any{logging.INSTANCE_ID: instanceId})
		return
	}

	logger.Info("SUCCESSFULLY_FETCHED_AND_PROCESSED_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// 3. Store the PGCR
	lag, committed, err := instance_storage.StorePGCR(processedActivity, rawPGCR)
	if err != nil {
		logger.Error("FAILED_TO_STORE_PGCR", map[string]any{logging.INSTANCE_ID: instanceId, logging.ERROR: err.Error()})
		return
	}

	// Prepare summary fields
	summaryFields := map[string]any{
		logging.INSTANCE_ID: processedActivity.InstanceId,
		logging.HASH:        processedActivity.Hash,
		"date_started":      processedActivity.DateStarted,
		"date_completed":    processedActivity.DateCompleted,
		logging.DURATION:    processedActivity.DurationSeconds,
		logging.COUNT:       len(processedActivity.Players),
		"completed":         processedActivity.Completed,
	}
	if processedActivity.Fresh != nil {
		summaryFields["fresh"] = *processedActivity.Fresh
	}
	if processedActivity.Flawless != nil {
		summaryFields["flawless"] = *processedActivity.Flawless
	}
	if lag != nil {
		summaryFields[logging.LAG] = lag
	}

	if committed {
		logger.Info("STORED_NEW_PGCR", summaryFields)
	} else {
		logger.Info("PGCR_ALREADY_EXISTS", summaryFields)
	}
}

func main() {
	flag.Parse()
	ProcessSinglePGCR()
}
