package pgcr

import (
	"database/sql"
	"fmt"
	"log"
	"raidhub/queue-workers/character_fill"
	"raidhub/queue-workers/pgcr_cheat_check"
	"raidhub/queue-workers/pgcr_clickhouse"
	"raidhub/queue-workers/player_crawl"
	"raidhub/shared/bungie"
	"raidhub/shared/monitoring"
	"raidhub/shared/pgcr_types"
	"raidhub/shared/database/postgres"
	"raidhub/shared/stats"
	"sync"
	"time"

	"github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Returns lag, is_new, err
func StorePGCR(pgcr *pgcr_types.ProcessedActivity, raw *bungie.DestinyPostGameCarnageReport, db *sql.DB, channel *amqp.Channel) (*time.Duration, bool, error) {
	// Identify the raid which this PGCR belongs to
	var activityId int
	var isRaid bool
	var activityName string
	var versionName string
	err := db.QueryRow(`SELECT activity_id, is_raid, activity_definition.name, version_definition.name
			FROM activity_version 
			JOIN activity_definition ON activity_version.activity_id = activity_definition.id 
			JOIN version_definition ON activity_version.version_id = version_definition.id
			WHERE hash = $1`,
		pgcr.Hash).Scan(&activityId, &isRaid, &activityName, &versionName)
	if err != nil {
		log.Printf("Error finding activity_id for %d", pgcr.Hash)
		return nil, false, err
	}

	lag := time.Since(pgcr.DateCompleted)

	// Store the raw JSON
	err = StoreJSON(raw, db)
	if err != nil {
		log.Println("Failed to store raw JSON")
		return nil, false, err
	}

	err = pgcr_clickhouse.SendToClickhouse(channel, pgcr)
	if err != nil {
		log.Println("Failed to send to clickhouse")
		return nil, false, err

	}

	tx, err := db.Begin()
	if err != nil {
		log.Println("Failed to initiate transaction")
		return nil, false, err
	}

	defer tx.Rollback()

	// Nothing should happen if this fails
	_, err = tx.Exec(`INSERT INTO "instance" (
		"instance_id",
		"hash",
		"flawless",
		"completed",
		"fresh",
		"player_count",
		"date_started",
		"date_completed",
		"platform_type",
		"duration",
		"score",
		"skull_hashes"
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, pgcr.InstanceId, pgcr.Hash,
		pgcr.Flawless, pgcr.Completed, pgcr.Fresh, pgcr.PlayerCount,
		pgcr.DateStarted, pgcr.DateCompleted, pgcr.MembershipType, pgcr.DurationSeconds, pgcr.Score, pq.Array(pgcr.SkullHashes))

	if err != nil {
		pqErr, ok := err.(*pq.Error)
		if ok && (pqErr.Code == "23505") {
			log.Printf("Duplicate instanceId: %d", pgcr.InstanceId)
			return &lag, false, nil
		} else {
			log.Printf("Error inserting activity into DB for instanceId %d", pgcr.InstanceId)
			return nil, false, err
		}
	}

	var characterRequests = make([]character_fill.CharacterFillRequest, 0)

	completedDictionary := map[int64]bool{}
	fastestClearSoFar := map[int64]int{}
	for _, playerActivity := range pgcr.Players {
		var playerRaidClearCount int
		var duration int
		// the sum is a null hack, but it finds distinct rows anyways
		err = tx.QueryRow(`
			SELECT COALESCE(SUM(ps.clears), 0) AS count, COALESCE(SUM(a.duration), 100000000)
			FROM player_stats ps
			LEFT JOIN instance a ON ps.fastest_instance_id = a.instance_id
			WHERE ps.membership_id = $1 AND ps.activity_id = $2`, playerActivity.Player.MembershipId, activityId).
			Scan(&playerRaidClearCount, &duration)
		fastestClearSoFar[playerActivity.Player.MembershipId] = duration

		if err != nil {
			log.Printf("Error querying clears in DB for instance_id, membership_id, activity_id: %d, %d, %d", pgcr.InstanceId, playerActivity.Player.MembershipId, activityId)
			return nil, false, err
		}

		if playerActivity.Finished {
			completedDictionary[playerActivity.Player.MembershipId] = playerRaidClearCount > 0
		}

		if _, err := postgres.UpsertPlayer(tx, &playerActivity.Player); err != nil {
			log.Printf("Error inserting player %d into DB for instanceId %d: %s",
				playerActivity.Player.MembershipId, pgcr.InstanceId, err)
			return nil, false, err
		}

		_, err = tx.Exec(`
			INSERT INTO "instance_player" (
				"instance_id",
				"membership_id",
				"completed",
				"time_played_seconds"
			) 
			VALUES ($1, $2, $3, $4);`,
			pgcr.InstanceId, playerActivity.Player.MembershipId,
			playerActivity.Finished, playerActivity.TimePlayedSeconds)
		if err != nil {
			log.Printf("Error inserting instance_player into DB for instanceId, membershipId %d, %d: %s", pgcr.InstanceId,
				playerActivity.Player.MembershipId, err)
			return nil, false, err
		}

		// update the player_stats table
		_, err = tx.Exec(`INSERT INTO player_stats ("membership_id", "activity_id")
			VALUES ($1, $2)
			ON CONFLICT (membership_id, activity_id) DO NOTHING`,
			playerActivity.Player.MembershipId, activityId)

		if err != nil {
			log.Printf("Error inserting player_stats into DB for membershipId, activity_id: %d, %d", playerActivity.Player.MembershipId, activityId)
			return nil, false, err
		}

		// Send a crawl request if needed
		if playerActivity.Player.MembershipType == nil || *playerActivity.Player.MembershipType == 0 {
			err = player_crawl.SendMessage(channel, playerActivity.Player.MembershipId)
			if err != nil {
				log.Fatalf("Failed to send player crawl request: %s", err)
			}
		}
		for _, character := range playerActivity.Characters {
			_, err = tx.Exec(`
				INSERT INTO "instance_character" (
					"instance_id",
					"membership_id",
					"character_id",
					"class_hash",
					"emblem_hash",
					"completed",
					"score",
					"kills",
					"assists",
					"deaths",
					"precision_kills",
					"super_kills",
					"grenade_kills",
					"melee_kills",
					"time_played_seconds",
					"start_seconds"
				) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16);`,
				pgcr.InstanceId, playerActivity.Player.MembershipId,
				character.CharacterId, character.ClassHash, character.EmblemHash, character.Completed, character.Score,
				character.Kills, character.Assists, character.Deaths, character.PrecisionKills, character.SuperKills,
				character.GrenadeKills, character.MeleeKills, character.TimePlayedSeconds, character.StartSeconds)
			if err != nil {
				log.Printf("Error inserting instance_character into DB for instanceId, membershipId, characterId %d, %d, %d: %s",
					pgcr.InstanceId, playerActivity.Player.MembershipId, character.CharacterId, err)
				return nil, false, err
			}

			var wg sync.WaitGroup
			errs := make(chan error, len(character.Weapons))
			for _, w := range character.Weapons {
				wg.Add(1)
				go func(weapon pgcr_types.ProcessedCharacterActivityWeapon) {
					defer wg.Done()
					_, err = tx.Exec(`
						INSERT INTO "instance_character_weapon" (
							"instance_id",
							"membership_id",
							"character_id",
							"weapon_hash",
							"kills",
							"precision_kills"
						) 
						VALUES ($1, $2, $3, $4, $5, $6);`,
						pgcr.InstanceId, playerActivity.Player.MembershipId,
						character.CharacterId, weapon.WeaponHash, weapon.Kills, weapon.PrecisionKills)
					if err != nil {
						errs <- err
						log.Printf("Error inserting instance_character_weapon into DB for instanceId, character_id, weapon_hash %d, %d, %d: %s",
							pgcr.InstanceId, character.CharacterId, weapon.WeaponHash, err)
					}
				}(w)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				return nil, false, err
			}

			if character.ClassHash == nil {
				characterRequests = append(characterRequests, character_fill.CharacterFillRequest{
					MembershipId: playerActivity.Player.MembershipId,
					CharacterId:  character.CharacterId,
					InstanceId:   pgcr.InstanceId,
				})
			}
		}

	}

	// determine if a sherpa took place
	noobsCount := 0
	anyPro := false
	for _, hasClears := range completedDictionary {
		if hasClears {
			anyPro = true
		} else {
			noobsCount++
		}
	}

	// update total_time_played_seconds for all players
	for _, playerActivity := range pgcr.Players {
		_, err = tx.Exec(`UPDATE player_stats 
			SET total_time_played_seconds = total_time_played_seconds + $1 
			WHERE membership_id = $2 AND activity_id = $3`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId, activityId)
		if err != nil {
			log.Printf("Error updating total_time_played_seconds for membershipId %d", playerActivity.Player.MembershipId)
			return nil, false, err
		}

		_, err = tx.Exec(`UPDATE player 
			SET total_time_played_seconds = total_time_played_seconds + $1
			WHERE membership_id = $2`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId)
		if err != nil {
			log.Printf("Error updating total_time_played_seconds for membershipId %d", playerActivity.Player.MembershipId)
			return nil, false, err
		}

	}

	sherpasHappened := anyPro && noobsCount > 0
	if sherpasHappened {
		log.Printf("Found %d sherpas for instance %d", noobsCount, pgcr.InstanceId)
	}

	for membershipId, hasClears := range completedDictionary {
		var playerActivity *pgcr_types.ProcessedActivityPlayer
		for _, pa := range pgcr.Players {
			if pa.Player.MembershipId == membershipId {
				playerActivity = &pa
				break
			}
		}
		if playerActivity == nil {
			log.Fatalf("Player %d not found in pgcr.Players", membershipId)
		}

		if hasClears && sherpasHappened {
			playerActivity.Sherpas = noobsCount
			// set sherpas for p_activity
			_, err = tx.Exec(`UPDATE 
				instance_player
			SET 
				sherpas = $1
			WHERE 
				membership_id = $2 AND
				instance_id = $3`, playerActivity.Sherpas, membershipId, pgcr.InstanceId)

			if err != nil {
				log.Printf("Error updating sherpa count for instance_player with instanceId, membershipId %d, %d", pgcr.InstanceId, membershipId)
				return nil, false, err
			}

		} else if !hasClears {
			playerActivity.IsFirstClear = true
			// first clear, update p_activity
			_, err = tx.Exec(`UPDATE 
				instance_player
			SET 
				is_first_clear = true
			WHERE 
				membership_id = $1 AND
				instance_id = $2`, membershipId, pgcr.InstanceId)

			if err != nil {
				log.Printf("Error updating first clear for instanceId, membershipId %d, %d", pgcr.InstanceId, membershipId)
				return nil, false, err
			}

			// crawl the player on first clear
			log.Printf("Crawling player %d on first clear in instance %d", membershipId, pgcr.InstanceId)
			player_crawl.SendMessage(channel, playerActivity.Player.MembershipId)
		}

		// raid specific stats
		_, err = tx.Exec(`UPDATE player_stats 
			SET 
				sherpas = player_stats.sherpas + $3,
				clears = player_stats.clears + 1,
				fresh_clears = CASE
						WHEN $4 = true THEN player_stats.fresh_clears + 1
						ELSE player_stats.fresh_clears
					END,
				fastest_instance_id = CASE
						WHEN $4 = true AND $5::int < $6::int THEN $7::bigint
						ELSE player_stats.fastest_instance_id
					END
			WHERE
				membership_id = $1 AND
				activity_id = $2;
			`, membershipId, activityId, playerActivity.Sherpas, pgcr.Fresh, pgcr.DurationSeconds, fastestClearSoFar[membershipId], pgcr.InstanceId)

		if err != nil {
			log.Printf("Error updating player_stats for membershipId %d", membershipId)
			return nil, false, err
		}

		// global stats
		_, err = tx.Exec(`UPDATE player 
			SET 
				clears = player.clears + 1,
				sherpas = player.sherpas + $2,
				fresh_clears = CASE 
						WHEN $3 = true THEN player.fresh_clears + 1
						ELSE player.fresh_clears
					END
			WHERE membership_id = $1`, membershipId, playerActivity.Sherpas, pgcr.Fresh)

		if err != nil {
			log.Printf("Error updating global stats for membershipId %d", membershipId)
			return nil, false, err
		}

		if pgcr.Fresh != nil && *pgcr.Fresh && pgcr.DurationSeconds < fastestClearSoFar[membershipId] {
			_, err := stats.UpdatePlayerSumOfBest(membershipId, tx)
			if err != nil {
				log.Printf("Error updating sum of best for membershipId %d", membershipId)
				return nil, false, err
			}
		}

	}

	if err != nil {
		log.Fatal(err)
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
		return nil, false, err
	}

	for _, req := range characterRequests {
		character_fill.SendMessage(channel, &req)
	}

	if pgcr.DateCompleted.After(time.Now().Add(-5 * time.Hour)) {
		// If the PGCR is newer than 5 hours, we consider it a new PGCR
		monitoring.PGCRStoreActivity.WithLabelValues(activityName, versionName, fmt.Sprintf("%v", pgcr.Completed)).Inc()
	}

	err = pgcr_cheat_check.SendMessage(channel, pgcr.InstanceId)

	return &lag, true, nil
}
