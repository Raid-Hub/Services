package instance

import (
	"raidhub/lib/database/postgres"
	"time"
)

func CheckExists(instanceId int64) (bool, error) {
	var exists bool
	err := postgres.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM instance WHERE instance_id = $1)
	`, instanceId).Scan(&exists)
	if err != nil {
		InstanceLogger.Error("Error checking instance existence", "instanceId", instanceId, "error", err)
		return false, err
	}

	return exists, nil
}

// GetLatestInstanceId returns the latest instance ID minus the provided buffer
func GetLatestInstanceId(buffer int64) (int64, error) {
	var latestID int64
	err := postgres.DB.QueryRow(`SELECT instance_id FROM instance WHERE instance_id < 100000000000 ORDER BY instance_id DESC LIMIT 1`).Scan(&latestID)
	if err != nil {
		InstanceLogger.Error("Error getting latest instance id", "error", err)
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
		InstanceLogger.Error("Error getting latest instance", "error", err)
		return 0, time.Time{}, err
	} else {
		return latestID, dateCompleted, nil
	}
}
