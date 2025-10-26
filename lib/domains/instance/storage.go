package instance

import (
	"database/sql"
	"fmt"
	"log"
	"raidhub/lib/database/postgres"
	"raidhub/lib/domains/player"
	"raidhub/lib/domains/stats"
	"raidhub/lib/dto"
	"raidhub/lib/messaging/messages"
	"sync"

	"github.com/lib/pq"
)

// StoreSideEffects contains routing operations to be executed after transaction commits
type StoreSideEffects struct {
	CharacterFillRequests []messages.CharacterFillMessage
	PlayerCrawlRequests   []int64
	CheatCheckRequest     bool
}

// Store stores instance data to the database within a transaction
// Returns side effects that should be handled after commit
func Store(tx *sql.Tx, inst *dto.Instance) (*StoreSideEffects, error) {
	sideEffects := &StoreSideEffects{}
	// Get activity information
	activityId, _, _, err := getActivityInfo(inst.Hash)
	if err != nil {
		return nil, err
	}

	// Insert into instance table
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
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, inst.InstanceId, inst.Hash,
		inst.Flawless, inst.Completed, inst.Fresh, inst.PlayerCount,
		inst.DateStarted, inst.DateCompleted, inst.MembershipType, inst.DurationSeconds, inst.Score, pq.Array(inst.SkullHashes))

	if err != nil {
		pqErr, ok := err.(*pq.Error)
		if ok && (pqErr.Code == "23505") {
			// Duplicate instance - not an error
			log.Printf("Duplicate instanceId: %d", inst.InstanceId)
			return nil, nil
		} else {
			log.Printf("Error inserting instance into DB for instanceId %d", inst.InstanceId)
			return nil, err
		}
	}

	// Store players and collect data for stats updates
	var characterRequests []messages.CharacterFillMessage
	completedDictionary := map[int64]bool{}
	fastestClearSoFar := map[int64]int{}

	for _, playerActivity := range inst.Players {
		// Get existing player stats
		var playerRaidClearCount int
		var duration int
		err = tx.QueryRow(`
			SELECT COALESCE(SUM(ps.clears), 0) AS count, COALESCE(SUM(a.duration), 100000000)
			FROM player_stats ps
			LEFT JOIN instance a ON ps.fastest_instance_id = a.instance_id
			WHERE ps.membership_id = $1 AND ps.activity_id = $2`, playerActivity.Player.MembershipId, activityId).
			Scan(&playerRaidClearCount, &duration)
		fastestClearSoFar[playerActivity.Player.MembershipId] = duration

		if err != nil {
			log.Printf("Error querying clears in DB for instance_id, membership_id, activity_id: %d, %d, %d", inst.InstanceId, playerActivity.Player.MembershipId, activityId)
			return nil, err
		}

		if playerActivity.Finished {
			completedDictionary[playerActivity.Player.MembershipId] = playerRaidClearCount > 0
		}

		// Upsert player
		if _, err := player.UpsertPlayer(tx, &playerActivity.Player); err != nil {
			log.Printf("Error inserting player %d into DB for instanceId %d: %s",
				playerActivity.Player.MembershipId, inst.InstanceId, err)
			return nil, err
		}

		// Insert into instance_player
		_, err = tx.Exec(`
			INSERT INTO "instance_player" (
				"instance_id",
				"membership_id",
				"completed",
				"time_played_seconds"
			) 
			VALUES ($1, $2, $3, $4);`,
			inst.InstanceId, playerActivity.Player.MembershipId,
			playerActivity.Finished, playerActivity.TimePlayedSeconds)
		if err != nil {
			log.Printf("Error inserting instance_player into DB for instanceId, membershipId %d, %d: %s", inst.InstanceId,
				playerActivity.Player.MembershipId, err)
			return nil, err
		}

		// Update player_stats table
		_, err = tx.Exec(`INSERT INTO player_stats ("membership_id", "activity_id")
			VALUES ($1, $2)
			ON CONFLICT (membership_id, activity_id) DO NOTHING`,
			playerActivity.Player.MembershipId, activityId)

		if err != nil {
			log.Printf("Error inserting player_stats into DB for membershipId, activity_id: %d, %d", playerActivity.Player.MembershipId, activityId)
			return nil, err
		}

		// Collect player crawl request if needed (will be sent after transaction)
		if playerActivity.Player.MembershipType == nil || *playerActivity.Player.MembershipType == 0 {
			sideEffects.PlayerCrawlRequests = append(sideEffects.PlayerCrawlRequests, playerActivity.Player.MembershipId)
		}

		// Store characters and weapons
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
				inst.InstanceId, playerActivity.Player.MembershipId,
				character.CharacterId, character.ClassHash, character.EmblemHash, character.Completed, character.Score,
				character.Kills, character.Assists, character.Deaths, character.PrecisionKills, character.SuperKills,
				character.GrenadeKills, character.MeleeKills, character.TimePlayedSeconds, character.StartSeconds)
			if err != nil {
				log.Printf("Error inserting instance_character into DB for instanceId, membershipId, characterId %d, %d, %d: %s",
					inst.InstanceId, playerActivity.Player.MembershipId, character.CharacterId, err)
				return nil, err
			}

			// Store weapons concurrently
			var wg sync.WaitGroup
			errs := make(chan error, len(character.Weapons))
			for _, weapon := range character.Weapons {
				wg.Add(1)
				go func(weapon dto.InstanceCharacterWeapon) {
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
						inst.InstanceId, playerActivity.Player.MembershipId,
						character.CharacterId, weapon.WeaponHash, weapon.Kills, weapon.PrecisionKills)
					if err != nil {
						errs <- err
						log.Printf("Error inserting instance_character_weapon into DB for instanceId, character_id, weapon_hash %d, %d, %d: %s",
							inst.InstanceId, character.CharacterId, weapon.WeaponHash, err)
					}
				}(weapon)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				return nil, err
			}

			if character.ClassHash == nil {
				// Create character fill request
				sideEffects.CharacterFillRequests = append(sideEffects.CharacterFillRequests, messages.NewCharacterFillMessage(
					playerActivity.Player.MembershipId,
					character.CharacterId,
					inst.InstanceId,
				))
			}
		}
	}

	// Update total_time_played_seconds for all players
	for _, playerActivity := range inst.Players {
		_, err = tx.Exec(`UPDATE player_stats 
			SET total_time_played_seconds = total_time_played_seconds + $1 
			WHERE membership_id = $2 AND activity_id = $3`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId, activityId)
		if err != nil {
			log.Printf("Error updating total_time_played_seconds for membershipId %d", playerActivity.Player.MembershipId)
			return nil, err
		}

		_, err = tx.Exec(`UPDATE player 
			SET total_time_played_seconds = total_time_played_seconds + $1
			WHERE membership_id = $2`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId)
		if err != nil {
			log.Printf("Error updating total_time_played_seconds for membershipId %d", playerActivity.Player.MembershipId)
			return nil, err
		}
	}

	// Determine sherpa logic
	noobsCount := 0
	anyPro := false
	for _, hasClears := range completedDictionary {
		if hasClears {
			anyPro = true
		} else {
			noobsCount++
		}
	}

	sherpasHappened := anyPro && noobsCount > 0
	if sherpasHappened {
		log.Printf("Found %d sherpas for instance %d", noobsCount, inst.InstanceId)
	}

	// Update stats for each player
	for membershipId, hasClears := range completedDictionary {
		var playerActivity *dto.InstancePlayer
		for _, pa := range inst.Players {
			if pa.Player.MembershipId == membershipId {
				playerActivity = &pa
				break
			}
		}
		if playerActivity == nil {
			log.Fatalf("Player %d not found in inst.Players", membershipId)
		}

		if hasClears && sherpasHappened {
			playerActivity.Sherpas = noobsCount
			_, err = tx.Exec(`UPDATE 
				instance_player
			SET 
				sherpas = $1
			WHERE 
				membership_id = $2 AND
				instance_id = $3`, playerActivity.Sherpas, membershipId, inst.InstanceId)

			if err != nil {
				log.Printf("Error updating sherpa count for instance_player with instanceId, membershipId %d, %d", inst.InstanceId, membershipId)
				return nil, err
			}
		} else if !hasClears {
			playerActivity.IsFirstClear = true
			_, err = tx.Exec(`UPDATE 
				instance_player
			SET 
				is_first_clear = true
			WHERE 
				membership_id = $1 AND
				instance_id = $2`, membershipId, inst.InstanceId)

			if err != nil {
				log.Printf("Error updating first clear for instanceId, membershipId %d, %d", inst.InstanceId, membershipId)
				return nil, err
			}

			// Crawl the player on first clear (will be sent after transaction)
			log.Printf("Crawling player %d on first clear in instance %d", membershipId, inst.InstanceId)
			sideEffects.PlayerCrawlRequests = append(sideEffects.PlayerCrawlRequests, membershipId)
		}

		// Update raid-specific stats
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
			`, membershipId, activityId, playerActivity.Sherpas, inst.Fresh, inst.DurationSeconds, fastestClearSoFar[membershipId], inst.InstanceId)

		if err != nil {
			log.Printf("Error updating player_stats for membershipId %d", membershipId)
			return nil, err
		}

		// Update global stats
		_, err = tx.Exec(`UPDATE player 
			SET 
				clears = player.clears + 1,
				sherpas = player.sherpas + $2,
				fresh_clears = CASE 
						WHEN $3 = true THEN player.fresh_clears + 1
						ELSE player.fresh_clears
					END
			WHERE membership_id = $1`, membershipId, playerActivity.Sherpas, inst.Fresh)

		if err != nil {
			log.Printf("Error updating global stats for membershipId %d", membershipId)
			return nil, err
		}

		if inst.Fresh != nil && *inst.Fresh && inst.DurationSeconds < fastestClearSoFar[membershipId] {
			_, err := stats.UpdatePlayerSumOfBest(membershipId, tx)
			if err != nil {
				log.Printf("Error updating sum of best for membershipId %d", membershipId)
				return nil, err
			}
		}
	}

	// Collect character fill requests (will be sent after transaction)
	sideEffects.CharacterFillRequests = characterRequests

	// Mark cheat check request needed (will be sent after transaction)
	sideEffects.CheatCheckRequest = true

	return sideEffects, nil
}

// getActivityInfo retrieves activity information from the database
func getActivityInfo(hash uint32) (int, string, string, error) {
	var activityId int
	var activityName string
	var versionName string
	err := postgres.DB.QueryRow(`SELECT activity_id, activity_definition.name, version_definition.name
			FROM activity_version 
			JOIN activity_definition ON activity_version.activity_id = activity_definition.id 
			JOIN version_definition ON activity_version.version_id = version_definition.id
			WHERE hash = $1`,
		hash).Scan(&activityId, &activityName, &versionName)
	if err != nil {
		return 0, "", "", fmt.Errorf("error finding activity_id for hash %d: %w", hash, err)
	}
	return activityId, activityName, versionName, nil
}
