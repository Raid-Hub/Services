package instance

import (
	"log"
	"raidhub/lib/database/postgres"
)

// CheckExists checks if an instance exists in the database
func CheckExists(instanceId int64) (bool, error) {
	var exists bool
	err := postgres.DB.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM instance WHERE instance_id = $1)
	`, instanceId).Scan(&exists)
	if err != nil {
		log.Printf("Error checking instance existence: %v", err)
		return false, err
	}

	return exists, nil
}
