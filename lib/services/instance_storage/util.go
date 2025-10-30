package instance_storage

import (
	"database/sql"
	"fmt"
	"raidhub/lib/database/postgres"
	"sync"
)

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
		if err == sql.ErrNoRows {
			return 0, "", "", fmt.Errorf("activity not found for hash %d: %w", hash, err)
		}
		return 0, "", "", fmt.Errorf("error finding activity_id for hash %d: %w", hash, err)
	}

	// Store in cache
	cacheMu.Lock()
	activityInfoCache[hash] = cacheEntry
	cacheMu.Unlock()

	return cacheEntry.activityId, cacheEntry.activityName, cacheEntry.versionName, nil
}
