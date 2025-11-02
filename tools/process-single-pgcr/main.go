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
		logger.Error("USAGE_ERROR", nil, map[string]any{"message": "Usage: scripts process-single-pgcr <instance_id>"})
		return
	}

	instanceId, err := strconv.ParseInt(flag.Arg(1), 10, 64)
	if err != nil {
		logger.Error("INVALID_INSTANCE_ID", err, map[string]any{})
		return
	}

	logger.Info("PROCESSING_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// 2. Fetch and process the PGCR
	result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(instanceId)

	if result != pgcr_processing.Success {
		logger.Error("PGCR_FETCH_FAILED", nil, map[string]any{logging.INSTANCE_ID: instanceId, "result": result})
		return
	}

	logger.Info("SUCCESSFULLY_FETCHED_AND_PROCESSED_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// 3. Store the PGCR
	lag, committed, err := instance_storage.StorePGCR(instance, pgcr)
	if err != nil {
		logger.Error("FAILED_TO_STORE_PGCR", err, map[string]any{logging.INSTANCE_ID: instanceId})
		return
	}

	// Prepare summary fields
	summaryFields := map[string]any{
		logging.INSTANCE_ID: instance.InstanceId,
		logging.HASH:        instance.Hash,
		"date_started":      instance.DateStarted,
		"date_completed":    instance.DateCompleted,
		logging.DURATION:    instance.DurationSeconds,
		logging.COUNT:       len(instance.Players),
		"completed":         instance.Completed,
	}
	if instance.Fresh != nil {
		summaryFields["fresh"] = *instance.Fresh
	}
	if instance.Flawless != nil {
		summaryFields["flawless"] = *instance.Flawless
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
