package updateskull

import (
	"encoding/json"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"

	"github.com/lib/pq"
)

var logger = logging.NewLogger("UPDATE_SKULL_TOOL")

func UpdateSkullHashes() {
	db := postgres.DB

	rows, err := db.Query(`
		SELECT pgcr.data FROM instance 
		JOIN pgcr USING (instance_id)
		WHERE hash = 1044919065 AND instance_id < 16377060020`)
	if err != nil {
		logger.Error("ERROR_QUERYING_INSTANCE_TABLE", map[string]any{logging.ERROR: err.Error()})
	}
	defer rows.Close()

	stmt, err := db.Prepare(`UPDATE instance SET skull_hashes = $1 WHERE instance_id = $2`)
	if err != nil {
		logger.Error("ERROR_PREPARING_UPDATE_STATEMENT", map[string]any{logging.ERROR: err.Error()})
	}
	defer stmt.Close()

	count := 0
	total := 225000.0
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			logger.Error("ERROR_SCANNING_INSTANCE_ID", map[string]any{logging.ERROR: err.Error()})
		}

		var pgcr bungie.DestinyPostGameCarnageReport
		err := json.Unmarshal(data, &pgcr)
		if err != nil {
			logger.Error("ERROR_UNMARSHALLING_PGCR_DATA", map[string]any{logging.ERROR: err.Error()})
		}

		if pgcr.SelectedSkullHashes != nil {
			seen := make(map[uint32]struct{})
			for _, hash := range *pgcr.SelectedSkullHashes {
				seen[hash] = struct{}{}
			}
			skullHashes := make([]uint32, 0, len(seen))
			for hash := range seen {
				skullHashes = append(skullHashes, hash)
			}

			_, err := stmt.Exec(pq.Array(skullHashes), pgcr.ActivityDetails.InstanceId)
			if err != nil {
				logger.Error("ERROR_UPDATING_INSTANCE_WITH_SKULL_HASHES", map[string]any{"instance_id": pgcr.ActivityDetails.InstanceId, logging.ERROR: err.Error()})
			}
		}

		count++
		if count%1000 == 0 {
			ratio := float64(count) / total
			logger.Info("PROCESSED_INSTANCES", map[string]any{"percent": ratio * 100})
		}
	}
}
