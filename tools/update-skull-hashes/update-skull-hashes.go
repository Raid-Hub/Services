package updateskull

import (
	"encoding/json"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils"
	"raidhub/lib/web/bungie"

	"github.com/lib/pq"
)

var updateSkullLogger = utils.NewLogger("UPDATE_SKULL_TOOL")

func UpdateSkullHashes() {
	db := postgres.DB

	rows, err := db.Query(`
		SELECT pgcr.data FROM instance 
		JOIN pgcr USING (instance_id)
		WHERE hash = 1044919065 AND instance_id < 16377060020`)
	if err != nil {
		updateSkullLogger.ErrorF("Error querying instance table:", err)
	}
	defer rows.Close()

	stmt, err := db.Prepare(`UPDATE instance SET skull_hashes = $1 WHERE instance_id = $2`)
	if err != nil {
		updateSkullLogger.ErrorF("Error preparing update statement:", err)
	}
	defer stmt.Close()

	count := 0
	total := 225000.0
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			updateSkullLogger.ErrorFln("Error scanning instance_id:", err)
		}

		var pgcr bungie.DestinyPostGameCarnageReport
		err := json.Unmarshal(data, &pgcr)
		if err != nil {
			updateSkullLogger.ErrorFln("Error unmarshalling pgcr data:", err)
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
				updateSkullLogger.ErrorFf("Error updating instance %d with skull hashes: %v", pgcr.ActivityDetails.InstanceId, err)
			}
		}

		count++
		if count%1000 == 0 {
			ratio := float64(count) / total
			updateSkullLogger.InfoF("Processed %.2f%% of instances", ratio*100)
		}
	}
}
