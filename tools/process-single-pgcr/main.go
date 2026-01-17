package main

import (
	"context"
	"flag"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
	"strconv"
)

var logger = logging.NewLogger("process-single-pgcr")

func main() {
	logging.ParseFlags()

	// Parse the instance ID from command line args
	// Since logging.ParseFlags() is used, the actual arguments start from flag.Arg(0)
	if flag.NArg() < 1 {
		logger.Error("USAGE_ERROR", nil, map[string]any{"message": "Usage: scripts process-single-pgcr <instance_id>"})
		return
	}

	instanceId, err := strconv.ParseInt(flag.Arg(0), 10, 64)
	if err != nil {
		logger.Error("INVALID_INSTANCE_ID", err, map[string]any{})
		return
	}

	logger.Info("PROCESSING_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// Fetch and process the PGCR
	result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(context.Background(), instanceId, 0)

	if result != pgcr_processing.Success {
		logger.Error("PGCR_FETCH_FAILED", nil, map[string]any{logging.INSTANCE_ID: instanceId, "result": result})
		return
	}

	logger.Info("SUCCESSFULLY_FETCHED_AND_PROCESSED_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	// Store the PGCR
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
