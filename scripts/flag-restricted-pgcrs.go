package main

import (
	"log"
	"net/http"
	"os"
	"raidhub/packages/cheat_detection"
	"raidhub/packages/pgcr"
	"raidhub/packages/postgres"
)

const cheatCheckVersion = ""

func flagRestrictedPGCRs() {
	db, err := postgres.Connect()
	if err != nil {
		log.Fatal("Error connecting to postgres:", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT instance_id FROM instance WHERE hash = $1 and completed`, 3896382790)
	if err != nil {
		log.Fatal("Error querying instance table:", err)
	}
	defer rows.Close()

	// for each pgcr, query bungie API to check if it is restricted
	client := &http.Client{}
	securityKey := os.Getenv("BUNGIE_API_KEY")

	stmnt, err := db.Prepare(
		`INSERT INTO flag_instance (instance_id, cheat_check_version, cheat_check_bitmask, flagged_at, cheat_probability)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT DO NOTHING`,
	)
	if err != nil {
		log.Fatal("Error preparing insert statement:", err)
	}
	defer stmnt.Close()

	stmnt2, err := db.Prepare(`INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
		VALUES ($1, 'Manual', $2, $3)
        ON CONFLICT (instance_id)
		DO UPDATE SET report_source = 'Manual', cheat_check_version = $2, reason = $3, created_at = NOW()`)
	if err != nil {
		log.Fatal("Error preparing blacklist insert statement:", err)
	}
	defer stmnt2.Close()

	total := 0
	badApples := 0

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			log.Fatalln("Error scanning instance_id:", err)
		}
	
		result, _, _, _ := pgcr.FetchAndProcessPGCR(client, instanceId, securityKey)
		total++
		
		switch result {
			case pgcr.InsufficientPrivileges:
				log.Printf("Instance %d is restricted", instanceId)
			case pgcr.Success:
				log.Printf("Instance %d is not restricted", instanceId)
			default:
				log.Printf("Instance %d returned unexpected result: %d", instanceId, result)
				result, _, _, _ = pgcr.FetchAndProcessPGCR(client, instanceId, securityKey)
		}

		if result == pgcr.InsufficientPrivileges {
			badApples++
			_, err = stmnt.Exec(instanceId, cheatCheckVersion, cheat_detection.RestrictedPGCR|cheat_detection.DesertPerpetual, 1.0)
			if err != nil {
				log.Printf("Error flagging instance %d: %s", instanceId, err)
			} else {
				log.Printf("Flagged instance %d as restricted", instanceId)
			}
			_, err = stmnt2.Exec(instanceId, cheatCheckVersion, "Restricted PGCR")
			if err != nil {
				log.Printf("Error blacklisting instance %d: %s", instanceId, err)
			} else {
				log.Printf("Blacklisted instance %d as restricted PGCR", instanceId)
			}
		}
	}

	log.Printf("Total instances checked: %d, Restricted instances flagged: %d", total, badApples)
}