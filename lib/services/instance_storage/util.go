package instance_storage

import (
	"database/sql"
	"fmt"
	"raidhub/lib/database/postgres"
	"sync"
)

var (
	activityInfoCache = make(map[uint32]ActivityInfo)
	cacheMu           = sync.RWMutex{}
)

type ActivityInfo struct {
	activityId   int
	activityName string
	versionName  string
}

// getActivityInfo retrieves activity information from the database (with caching)
func getActivityInfo(hash uint32) (ActivityInfo, error) {
	// Try cache first
	cacheMu.RLock()
	cached, found := activityInfoCache[hash]
	cacheMu.RUnlock()

	if found {
		return cached, nil
	}

	// Query database
	cacheEntry := ActivityInfo{}
	err := postgres.DB.QueryRow(`SELECT activity_id, activity_definition.name, version_definition.name
			FROM activity_version 
			JOIN activity_definition ON activity_version.activity_id = activity_definition.id 
			JOIN version_definition ON activity_version.version_id = version_definition.id
			WHERE hash = $1`,
		hash).Scan(&cacheEntry.activityId, &cacheEntry.activityName, &cacheEntry.versionName)
	if err != nil {
		if err == sql.ErrNoRows {
			return ActivityInfo{}, fmt.Errorf("activity not found for hash %d: %w", hash, err)
		}
		return ActivityInfo{}, fmt.Errorf("error finding activity_id for hash %d: %w", hash, err)
	}

	// Store in cache
	cacheMu.Lock()
	activityInfoCache[hash] = cacheEntry
	cacheMu.Unlock()

	return cacheEntry, nil
}
