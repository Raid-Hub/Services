package instance

import (
	"log"
	"raidhub/lib/database/postgres"
)

// Block marks an instance as blocked in the blacklist_instance table
func Block(instanceId int64) error {
	_, err := postgres.DB.Exec(`
		INSERT INTO blacklist_instance (instance_id, report_source, reason)
		VALUES ($1, 'BungieAPI', 'Insufficient privileges')
		ON CONFLICT (instance_id) DO NOTHING
	`, instanceId)
	if err != nil {
		log.Printf("Error blocking instance %d: %v", instanceId, err)
		return err
	}

	log.Printf("Successfully blocked instance %d", instanceId)
	return nil
}
