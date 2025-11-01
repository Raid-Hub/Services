package instance_storage

import (
	"context"
	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"time"
)

var logger = logging.NewLogger("INSTANCE_STORAGE_SERVICE")

// StorePGCR orchestrates the complete PGCR storage workflow
// It coordinates storage across:
// 1. pgcr domain (raw JSON storage)
// 2. instance domain (structured data storage)
// 3. ClickHouse publishing (external, non-transactional)
func StorePGCR(inst *dto.Instance, raw *bungie.DestinyPostGameCarnageReport) (*time.Duration, bool, error) {
	startTime := time.Now()
	
	// Start transaction for atomic storage of pgcr + instance data
	tx, err := postgres.DB.Begin()
	if err != nil {
		logger.Warn(FAILED_TO_INITIATE_TRANSACTION, map[string]any{logging.ERROR: err.Error()})
		global_metrics.InstanceStorageOperations.WithLabelValues("begin_transaction", "error").Inc()
		return nil, false, err
	}
	defer tx.Rollback()

	// 1. Store raw JSON (pgcr domain) - within transaction
	rawIsNew, err := StoreRawJSON(tx, raw)
	if err != nil {
		logger.Warn(ERROR_STORING_RAW_PGCR, map[string]any{logging.ERROR: err.Error()})
		return nil, false, err
	}

	// 2. Store instance data (instance domain) - within same transaction
	sideEffects, instanceIsNew, err := Store(tx, inst)
	if err != nil {
		logger.Warn(ERROR_STORING_INSTANCE_DATA, map[string]any{logging.ERROR: err.Error()})
		return nil, false, err
	}
	// Determine if this was a new PGCR (either raw or instance was new)
	isNew := rawIsNew || instanceIsNew

	// If neither was new, return early - no need to process further
	if !isNew {
		tx.Rollback()
		logger.Debug(DUPLICATE_INSTANCE, map[string]any{
			logging.INSTANCE_ID: inst.InstanceId,
		})
		return nil, false, nil
	}

	// At least one was new, so we need to commit
	// (sideEffects == nil means instance was duplicate, but raw might be new)
	// (rawIsNew == false means raw was duplicate, but instance might be new)

	// 3. Store to ClickHouse BEFORE committing Postgres transaction
	// This ensures best-effort atomicity: if ClickHouse fails, we roll back everything
	clickhouseStart := time.Now()
	err = StoreToClickHouse(inst)
	clickhouseDuration := time.Since(clickhouseStart)
	if err != nil {
		logger.Warn(FAILED_TO_STORE_TO_CLICKHOUSE, map[string]any{logging.ERROR: err.Error()})
		global_metrics.InstanceStorageOperations.WithLabelValues("store_to_clickhouse", "error").Inc()
		global_metrics.InstanceStorageOperationDuration.WithLabelValues("store_to_clickhouse", "error").Observe(clickhouseDuration.Seconds())
		return nil, false, err // Roll back the entire transaction
	}
	global_metrics.InstanceStorageOperations.WithLabelValues("store_to_clickhouse", "success").Inc()
	global_metrics.InstanceStorageOperationDuration.WithLabelValues("store_to_clickhouse", "success").Observe(clickhouseDuration.Seconds())

	// 4. Commit transaction (only if ClickHouse succeeded)
	err = tx.Commit()
	if err != nil {
		logger.Warn(FAILED_TO_COMMIT_TRANSACTION, map[string]any{logging.ERROR: err.Error()})
		global_metrics.InstanceStorageOperations.WithLabelValues("commit_transaction", "error").Inc()
		return nil, false, err
	}
	global_metrics.InstanceStorageOperations.WithLabelValues("commit_transaction", "success").Inc()

	// Calculate lag using current time (after storage is complete)
	lag := time.Since(inst.DateCompleted)

	// 5. Get activity info for metrics and logging
	activityInfo, err := getActivityInfo(inst.Hash)

	// Publish side effects (only if instance was new)
	if instanceIsNew && sideEffects != nil {
		if sideEffects.CharacterFillRequests != nil {
			for _, characterFillRequest := range sideEffects.CharacterFillRequests {
				publishing.PublishJSONMessage(context.TODO(), routing.CharacterFill, characterFillRequest)
			}
		}
		if sideEffects.PlayerCrawlRequests != nil {
			for _, playerCrawlRequest := range sideEffects.PlayerCrawlRequests {
				publishing.PublishJSONMessage(context.TODO(), routing.PlayerCrawl, playerCrawlRequest)
			}
		}
		publishing.PublishInt64Message(context.TODO(), routing.InstanceCheatCheck, inst.InstanceId)
	}

	// Track overall storage duration and success
	totalDuration := time.Since(startTime)
	status := "duplicate"
	if isNew {
		status = "success"
	}
	global_metrics.InstanceStorageOperations.WithLabelValues("store_pgcr", status).Inc()
	global_metrics.InstanceStorageOperationDuration.WithLabelValues("store_pgcr", status).Observe(totalDuration.Seconds())

	// Log successful storage
	if isNew {
		logFields := map[string]any{
			logging.INSTANCE_ID: inst.InstanceId,
			logging.LAG:         lag,
			"activity":          activityInfo.activityName,
			"version":           activityInfo.versionName,
		}
		logger.Info(STORED_NEW_INSTANCE, logFields)
	} else {
		logger.Debug(FOUND_DUPLICATE_INSTANCE, map[string]any{
			logging.INSTANCE_ID: inst.InstanceId,
			logging.LAG:         lag,
		})
	}

	return &lag, isNew, nil
}
