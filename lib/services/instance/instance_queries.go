package instance

import (
	"context"
	"errors"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
	"time"
)

var logger = logging.NewLogger("INSTANCE_SERVICE")

func CheckExists(instanceId int64) (bool, error) {
	var exists bool
	err := postgres.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM instance WHERE instance_id = $1)
	`, instanceId).Scan(&exists)
	if err != nil {
		logger.Error("INSTANCE_EXISTS_CHECK_ERROR", err, map[string]any{
			logging.INSTANCE_ID: instanceId,
		})
		return false, err
	}

	return exists, nil
}

// GetLatestInstanceId returns the latest instance ID minus the provided buffer
func GetLatestInstanceId(buffer int64) (int64, error) {
	var latestID int64
	err := postgres.DB.QueryRow(`SELECT instance_id FROM instance WHERE instance_id < 100000000000 ORDER BY instance_id DESC LIMIT 1`).Scan(&latestID)
	if err != nil {
		logger.Warn("LATEST_INSTANCE_ID_ERROR", err, nil)
		return 0, err
	} else {
		return latestID - buffer, nil
	}
}

// GetLatestInstance returns the latest instance ID and completion date
func GetLatestInstance() (int64, time.Time, error) {
	var latestID int64
	var dateCompleted time.Time
	err := postgres.DB.QueryRow(`SELECT instance_id, date_completed FROM instance WHERE instance_id < 100000000000 ORDER BY instance_id DESC LIMIT 1`).Scan(&latestID, &dateCompleted)
	if err != nil {
		logger.Warn("LATEST_INSTANCE_ERROR", err, nil)
		return 0, time.Time{}, err
	} else {
		return latestID, dateCompleted, nil
	}
}

// GetLatestInstanceIdFromWeb uses binary search to find the latest valid PGCR instance ID from the Bungie API
// Instance IDs are incrementing numbers. This is useful when the database is empty (e.g., in dev mode)
func GetLatestInstanceIdFromWeb(buffer int64) (int64, error) {
	logger.Info("STARTING_WEB_SEARCH_FOR_LATEST_INSTANCE", map[string]any{
		"method":      "binary_search",
		"upper_bound": 1_000_000_000_000,
		"buffer":      buffer,
	})

	// Instance IDs increment over time. Start from a high estimate and search backwards
	upperBound := int64(1_000_000_000_000) // 1 trillion
	lowerBound := int64(1)

	// Binary search to find the highest valid instance ID
	left := lowerBound
	right := upperBound
	latestFound := int64(0)

	maxIterations := 50
	iterations := 0

	for left <= right && iterations < maxIterations {
		iterations++
		mid := (left + right) / 2

		logger.Debug("BINARY_SEARCH_ITERATION", map[string]any{
			"iteration": iterations,
			"left":      left,
			"right":     right,
			"mid":       mid,
		})

		// Try to fetch PGCR at mid point
		result, err := bungie.Client.GetPGCR(context.Background(), mid, nil) // GetPGCR still uses netUrl.Values

		if err == nil {
			// PGCR exists, this is a valid instance ID
			latestFound = mid
			// Search in the upper half (higher instance IDs)
			left = mid + 1
		} else if result.BungieErrorCode == bungie.PGCRNotFound {
			// PGCR not found, search in lower half (lower instance IDs)
			right = mid - 1
		} else {
			// Other error (rate limit, system disabled, etc.), skip this and try slightly lower
			right = mid - 1
			time.Sleep(500 * time.Millisecond) // Brief delay to avoid rate limits
		}
	}

	if latestFound == 0 {
		err := errors.New("no valid PGCR found in search range")
		logger.Warn("NO_LATEST_INSTANCE_FOUND_FROM_WEB", err, map[string]any{
			"iterations": iterations,
		})
		return 0, err
	}

	logger.Info("LATEST_INSTANCE_FOUND_FROM_WEB", map[string]any{
		"instance_id": latestFound,
		"iterations":  iterations,
		"buffer":      buffer,
	})

	return latestFound - buffer, nil
}

// GetInstanceIdsByHash returns instance IDs for instances matching the given hash that are completed
func GetInstanceIdsByHash(hash int64) ([]int64, error) {
	rows, err := postgres.DB.Query(`SELECT instance_id FROM instance WHERE hash = $1 AND completed`, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instanceIds []int64
	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			return nil, err
		}
		instanceIds = append(instanceIds, instanceId)
	}
	return instanceIds, rows.Err()
}
