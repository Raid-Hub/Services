package instance_storage

import (
	"context"
	"database/sql"
	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"time"
)

// ReplacePGCR deletes existing instance data and stores the new PGCR
// This is used to fix malformed PGCrs by re-fetching and replacing
// All operations happen in a single transaction to ensure atomicity
func ReplacePGCR(inst *dto.Instance, raw *bungie.DestinyPostGameCarnageReport) (*time.Duration, bool, error) {
	// Start transaction for atomic delete + store
	tx, err := postgres.DB.Begin()
	if err != nil {
		logger.Warn(FAILED_TO_INITIATE_TRANSACTION, err, nil)
		return nil, false, err
	}
	defer tx.Rollback()

	// 1. Clear existing instance data (to replace with new data)
	err = clearInstance(tx, inst.InstanceId)
	if err != nil {
		logger.Warn("FAILED_TO_REPLACE_EXISTING_INSTANCE", err, map[string]any{logging.INSTANCE_ID: inst.InstanceId})
		return nil, false, err
	}

	// 2. Store raw JSON (pgcr domain) - within same transaction
	rawIsNew, err := StoreRawJSON(tx, raw)
	if err != nil {
		logger.Warn(ERROR_STORING_RAW_PGCR, err, nil)
		return nil, false, err
	}

	// 3. Store instance data (instance domain) - within same transaction
	sideEffects, instanceIsNew, err := Store(tx, inst)
	if err != nil {
		logger.Warn(ERROR_STORING_INSTANCE_DATA, err, nil)
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

	// 4. Store to ClickHouse BEFORE committing Postgres transaction
	// This ensures best-effort atomicity: if ClickHouse fails, we roll back everything
	err = StoreToClickHouse(inst)
	if err != nil {
		logger.Warn(FAILED_TO_STORE_TO_CLICKHOUSE, err, nil)
		return nil, false, err // Roll back the entire transaction
	}

	// 5. Commit transaction (only if everything succeeded)
	err = tx.Commit()
	if err != nil {
		logger.Warn(FAILED_TO_COMMIT_TRANSACTION, err, nil)
		return nil, false, err
	}

	// Calculate lag using current time (after storage is complete)
	lag := time.Since(inst.DateCompleted)

	// 6. Get activity info for metrics and logging
	activityInfo, err := getActivityInfo(inst.Hash)

	// 7. Publish side effects (only if instance was new)
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

	// Log successful replacement
	logFields := map[string]any{
		logging.INSTANCE_ID: inst.InstanceId,
		logging.LAG:         lag,
	}
	if activityInfo.activityName != "" {
		logFields["activity"] = activityInfo.activityName
		logFields["version"] = activityInfo.versionName
	}
	logger.Info("REPLACED_PGCR_SUCCESSFULLY", logFields)

	return &lag, isNew, nil
}


// ClearInstance clears an instance and all related data from the database
// Clears in order: instance_character_weapon -> instance_character -> instance_player -> instance -> pgcr
// This is used to clear existing data before replacing with new data
// Returns error if clearing fails
func clearInstance(tx *sql.Tx, instanceID int64) error {
	// Delete in order to respect foreign key constraints
	// 1. Delete instance_character_weapon
	_, err := tx.Exec(`DELETE FROM extended.instance_character_weapon WHERE instance_id = $1`, instanceID)
	if err != nil {
		logger.Warn("FAILED_TO_DELETE_INSTANCE_CHARACTER_WEAPON", err, map[string]any{logging.INSTANCE_ID: instanceID})
		return err
	}

	// 2. Delete instance_character
	_, err = tx.Exec(`DELETE FROM extended.instance_character WHERE instance_id = $1`, instanceID)
	if err != nil {
		logger.Warn("FAILED_TO_DELETE_INSTANCE_CHARACTER", err, map[string]any{logging.INSTANCE_ID: instanceID})
		return err
	}

	// 3. Delete instance_player
	_, err = tx.Exec(`DELETE FROM core.instance_player WHERE instance_id = $1`, instanceID)
	if err != nil {
		logger.Warn("FAILED_TO_DELETE_INSTANCE_PLAYER", err, map[string]any{logging.INSTANCE_ID: instanceID})
		return err
	}

	// 4. Delete instance
	_, err = tx.Exec(`DELETE FROM core.instance WHERE instance_id = $1`, instanceID)
	if err != nil {
		logger.Warn("FAILED_TO_DELETE_INSTANCE", err, map[string]any{logging.INSTANCE_ID: instanceID})
		return err
	}

	// 5. Delete raw pgcr
	_, err = tx.Exec(`DELETE FROM raw.pgcr WHERE instance_id = $1`, instanceID)
	if err != nil {
		logger.Warn("FAILED_TO_DELETE_RAW_PGCR", err, map[string]any{logging.INSTANCE_ID: instanceID})
		return err
	}

	return nil
}