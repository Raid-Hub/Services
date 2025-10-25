package updateskull

import (
	"encoding/json"
	"log"
	"raidhub/packages/bungie"
	"raidhub/packages/pgcr"
	"raidhub/packages/postgres"

	"github.com/lib/pq"
)

func UpdateSkullHashes() {
	db, err := postgres.Connect()
	if err != nil {
		log.Fatal("Error connecting to postgres:", err)
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT pgcr.data FROM instance 
		JOIN pgcr USING (instance_id)
		WHERE hash = 1044919065 AND instance_id < 16377060020`)
	if err != nil {
		log.Fatal("Error querying instance table:", err)
	}
	defer rows.Close()

	stmt, err := db.Prepare(`UPDATE instance SET skull_hashes = $1 WHERE instance_id = $2`)
	if err != nil {
		log.Fatal("Error preparing update statement:", err)
	}
	defer stmt.Close()

	count := 0
	total := 225000.0
	for rows.Next() {
		var compressedData []byte
		if err := rows.Scan(&compressedData); err != nil {
			log.Fatalln("Error scanning instance_id:", err)
		}

		data, err := pgcr.GzipDecompress(compressedData)
		if err != nil {
			log.Fatalln("Error decompressing JSON data:", err)
		}

		var pgcr bungie.DestinyPostGameCarnageReport
		err = json.Unmarshal(data, &pgcr)
		if err != nil {
			log.Fatalln("Error unmarshalling pgcr data:", err)
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
				log.Fatalf("Error updating instance %d with skull hashes: %v", pgcr.ActivityDetails.InstanceId, err)
			}
		}

		count++
		if count%1000 == 0 {
			ratio := float64(count) / total
			log.Printf("Processed %.2f%% of instances", ratio*100)
		}
	}
}
