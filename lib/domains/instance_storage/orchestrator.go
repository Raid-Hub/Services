package instance_storage

import (
	"fmt"
	"log"
	"raidhub/lib/database/postgres"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring"
	"raidhub/lib/web/bungie"
	"time"
)

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
		log.Println("Failed to initiate transaction")
		return nil, false, err
	}
	defer tx.Rollback()

	// 1. Store raw JSON (pgcr domain) - within transaction
	_, err = StoreRawJSON(tx, raw)
	if err != nil {
		log.Printf("Error storing raw PGCR: %s", err)
		return nil, false, err
	}

	// 2. Store instance data (instance domain) - within same transaction
	sideEffects, err := Store(tx, inst)
	if err != nil {
		log.Printf("Error storing instance data: %s", err)
		return nil, false, err
	}

	// 3. Store to ClickHouse BEFORE committing Postgres transaction
	// This ensures best-effort atomicity: if ClickHouse fails, we roll back everything
	err = StoreToClickHouse(inst)
	if err != nil {
		log.Printf("Failed to store to ClickHouse: %v", err)
		return nil, false, err // Roll back the entire transaction
	}

	// 4. Commit transaction (only if ClickHouse succeeded)
	err = tx.Commit()
	if err != nil {
		log.Println("Failed to commit transaction")
		return nil, false, err
	}

	// 4. Monitoring
	_, activityName, versionName, err := getActivityInfo(inst.Hash)
	if err == nil && inst.DateCompleted.After(time.Now().Add(-5*time.Hour)) {
		monitoring.PGCRStoreActivity.WithLabelValues(activityName, versionName, fmt.Sprintf("%v", inst.Completed)).Inc()
	}

	// Publish side effects
	if sideEffects.CharacterFillRequests != nil {
		for _, characterFillRequest := range sideEffects.CharacterFillRequests {
			routing.Publisher.PublishJSONMessage(routing.CharacterFill, characterFillRequest)
		}
	}
	if sideEffects.PlayerCrawlRequests != nil {
		for _, playerCrawlRequest := range sideEffects.PlayerCrawlRequests {
			routing.Publisher.PublishJSONMessage(routing.PlayerCrawl, playerCrawlRequest)
		}
	}
	routing.Publisher.PublishTextMessage(routing.PGCRCheatCheck, fmt.Sprintf("%d", inst.InstanceId))

	log.Printf("Successfully stored instance %d", inst.InstanceId)
	return &lag, true, nil
}
