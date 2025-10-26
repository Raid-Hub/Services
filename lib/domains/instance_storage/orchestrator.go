package instance_storage

import (
	"fmt"
	"log"
	"raidhub/lib/database/postgres"
	"raidhub/lib/domains/instance"
	"raidhub/lib/domains/pgcr"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring"
	"raidhub/lib/web/bungie"
	"sync"
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
	_, err = pgcr.StoreRawJSON(tx, raw)
	if err != nil {
		log.Printf("Error storing raw PGCR: %s", err)
		return nil, false, err
	}

	// 2. Store instance data (instance domain) - within same transaction
	sideEffects, err := instance.Store(tx, inst)
	if err != nil {
		log.Printf("Error storing instance data: %s", err)
		return nil, false, err
	}

	// 3. Store to ClickHouse BEFORE committing Postgres transaction
	// This ensures best-effort atomicity: if ClickHouse fails, we roll back everything
	err = instance.StoreToClickHouse(inst)
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
	if sideEffects.CheatCheckRequest {
		routing.Publisher.PublishTextMessage(routing.PGCRCheatCheck, fmt.Sprintf("%d", inst.InstanceId))
	}

	log.Printf("Successfully stored instance %d", inst.InstanceId)
	return &lag, true, nil
}

var (
	activityInfoCache = make(map[uint32]activityInfoCacheEntry)
	cacheMu           = sync.RWMutex{}
)

type activityInfoCacheEntry struct {
	activityId   int
	activityName string
	versionName  string
}

// getActivityInfo retrieves activity information from the database (with caching)
func getActivityInfo(hash uint32) (int, string, string, error) {
	// Try cache first
	cacheMu.RLock()
	cached, found := activityInfoCache[hash]
	cacheMu.RUnlock()

	if found {
		return cached.activityId, cached.activityName, cached.versionName, nil
	}

	// Query database
	cacheEntry := activityInfoCacheEntry{}
	err := postgres.DB.QueryRow(`SELECT activity_id, activity_definition.name, version_definition.name
			FROM activity_version 
			JOIN activity_definition ON activity_version.activity_id = activity_definition.id 
			JOIN version_definition ON activity_version.version_id = version_definition.id
			WHERE hash = $1`,
		hash).Scan(&cacheEntry.activityId, &cacheEntry.activityName, &cacheEntry.versionName)
	if err != nil {
		return 0, "", "", fmt.Errorf("error finding activity_id for hash %d: %w", hash, err)
	}

	// Store in cache
	cacheMu.Lock()
	activityInfoCache[hash] = cacheEntry
	cacheMu.Unlock()

	return cacheEntry.activityId, cacheEntry.activityName, cacheEntry.versionName, nil
}
