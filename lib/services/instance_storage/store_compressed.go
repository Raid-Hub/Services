package instance_storage

import (
	"database/sql"
	"encoding/json"
	"log"
	"raidhub/lib/web/bungie"
)

// StoreRawJSON stores raw PGCR JSON to the pgcr table within a transaction
// Returns (isNew, error) - true if this is a new PGCR (not duplicate)
func StoreRawJSON(tx *sql.Tx, report *bungie.DestinyPostGameCarnageReport) (bool, error) {
	stmt, err := tx.Prepare(`INSERT INTO pgcr (instance_id, data)
		VALUES ($1, $2)
		ON CONFLICT (instance_id) DO NOTHING;`)
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	// Marshal the struct to JSON
	jsonData, err := json.Marshal(report)
	if err != nil {
		return false, err
	}

	result, err := stmt.Exec(report.ActivityDetails.InstanceId, jsonData)
	if err != nil {
		return false, err
	}

	rowsAdded, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAdded == 0 {
		log.Printf("Duplicate raw PGCR: %d", report.ActivityDetails.InstanceId)
		return false, nil
	}

	return true, nil
}
