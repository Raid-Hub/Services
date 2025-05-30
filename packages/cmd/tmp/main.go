package main

import (
	"log"
	"raidhub/packages/bungie"
	"raidhub/packages/cheat_detection"
	"raidhub/packages/postgres"
)

func main() {
	db, err := postgres.Connect()
	if err != nil {
		log.Fatal("Error connecting to postgres:", err)
	}
	defer db.Close()

	// rows, err := db.Query(`SELECT instance_id FROM instance_player
	// 	WHERE membership_id = 4611686018446136355
	// 	ORDER BY instance_id ASC`)
	// if err != nil {
	// 	log.Fatalf("Error getting instance_ids: %s", err)
	// }
	// defer rows.Close()

	// log.Println("Starting cheat check")

	// var instanceID int64
	// for rows.Next() {
	// 	if err := rows.Scan(&instanceID); err != nil {
	// 		log.Fatalf("error %s", err)
	// 	} else {
	// 		i, t, ps, _, err := cheat_detection.CheckForCheats(instanceID, db)
	// 		if err != nil {
	// 			log.Fatalln(err)
	// 		}
	// 		if t.Probability > cheat_detection.Threshold {
	// 			log.Printf("processed instance_id %d with p %0.3f", i.InstanceId, t.Probability)
	// 		} else {
	// 			// log.Printf("processed instance_id %d", i.InstanceId)
	// 		}

	// 		for _, p := range ps {
	// 			if p.Probability > cheat_detection.Threshold {
	// 				log.Printf("player %d flagged at %0.3f", p.MembershipId, p.Probability)
	// 			}
	// 		}
	// 	}
	// }

	// if err := rows.Err(); err != nil {
	// 	log.Fatalf("Row iteration error: %s", err)
	// }

	// countBlacklisted, err := cheat_detection.BlacklistFlaggedInstances(db)
	// if err != nil {
	// 	log.Fatalf("Error blacklisting flagged instances: %s", err)
	// }
	// log.Printf("Blacklisted %d flagged instances", countBlacklisted)

	// log.Println("Starting blacklist recent instances...")

	// count, _, err := cheat_detection.BlacklistRecentInstances(db,4611686018540679768, time.Now())
	// if err != nil {
	// 	log.Fatalf("Error blacklisting recent instances: %s", err)
	// }
	// log.Printf("Blacklisted %d recent instances", count)

	var membershipId int64 = 4611686018488107374

	var ageInDays float64
	var clears int
	var membershipType int
	var iconPath string
	var bungieName string
	var currentCheatLevel int
	var isPrivate bool
	// get the age of the account and # of clears
	err = db.QueryRow(`
		SELECT 
			EXTRACT(EPOCH FROM age(NOW(), first_seen)) / 86400 AS age_in_days,
			clears,
		    membership_type,
			icon_path,
			bungie_name,
			cheat_level,
			is_private
		FROM player
		WHERE membership_id = $1
	`, membershipId).Scan(&ageInDays, &clears, &membershipType, &iconPath, &bungieName, &currentCheatLevel, &isPrivate)
	if err != nil {
		log.Fatalf("Error getting player info for %d: %s", membershipId, err)
	}

	var flawlessRatio float64
	var lowmanRatio float64
	var soloRatio float64
	err = db.QueryRow(`
		SELECT 
			COUNT(CASE WHEN i.completed AND flawless THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS flawless_ratio,
			COUNT(CASE WHEN i.completed AND player_count <= 3 THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS lowman_ratio,
			COUNT(CASE WHEN i.completed AND player_count = 1 THEN 1 END) * 1.0 / GREATEST(COUNT(CASE WHEN i.completed = true THEN 1 END), 1) AS solo_ratio
		FROM instance_player
		JOIN instance i USING (instance_id)
		WHERE i.date_started >= NOW() - INTERVAL '60 days'
			AND membership_id = $1
	`, membershipId).Scan(&flawlessRatio, &lowmanRatio, &soloRatio)
	if err != nil {
		log.Fatalf("Error getting ratios for player %d: %s", membershipId, err)
	}

	res, err := bungie.GetProfile(membershipType, membershipId, []int{100})
	if err != nil {
		log.Fatalf("Error getting profile for player %d: %s", membershipId, err)
	}

	cheaterAccountChance, bitFlags := cheat_detection.GetCheaterAccountChance(res.Profile.Data, clears, ageInDays, flawlessRatio, lowmanRatio, soloRatio, isPrivate)

	log.Printf("Cheater account chance for %s (%d): %0.3f, flags: %s", bungieName, membershipId, cheaterAccountChance, cheat_detection.GetCheaterAccountFlagsStrings(bitFlags))
}
