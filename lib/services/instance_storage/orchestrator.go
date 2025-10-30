package instance_storage

import (
	"context"
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring"
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
	lag := time.Since(inst.DateCompleted)

	// Start transaction for atomic storage of pgcr + instance data
	tx, err := postgres.DB.Begin()
	if err != nil {
		logger.Warn(FAILED_TO_INITIATE_TRANSACTION, map[string]any{logging.ERROR: err.Error()})
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
		return nil, false, nil
	}

	// At least one was new, so we need to commit
	// (sideEffects == nil means instance was duplicate, but raw might be new)
	// (rawIsNew == false means raw was duplicate, but instance might be new)

	// 3. Store to ClickHouse BEFORE committing Postgres transaction
	// This ensures best-effort atomicity: if ClickHouse fails, we roll back everything
	err = StoreToClickHouse(inst)
	if err != nil {
		logger.Warn(FAILED_TO_STORE_TO_CLICKHOUSE, map[string]any{logging.ERROR: err.Error()})
		return nil, false, err // Roll back the entire transaction
	}

	// 4. Commit transaction (only if ClickHouse succeeded)
	err = tx.Commit()
	if err != nil {
		logger.Warn(FAILED_TO_COMMIT_TRANSACTION, map[string]any{logging.ERROR: err.Error()})
		return nil, false, err
	}

	// 5. Monitoring
	_, activityName, versionName, err := getActivityInfo(inst.Hash)
	if err == nil && inst.DateCompleted.After(time.Now().Add(-5*time.Hour)) {
		monitoring.PGCRStoreActivity.WithLabelValues(activityName, versionName, fmt.Sprintf("%v", inst.Completed)).Inc()
	}

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
		publishing.PublishTextMessage(context.TODO(), routing.InstanceCheatCheck, fmt.Sprintf("%d", inst.InstanceId))
	}

	return &lag, isNew, nil
}
