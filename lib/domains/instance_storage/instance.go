package instance_storage

import (
	"database/sql"
	"fmt"
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
}

// Store stores instance data to the database within a transaction
// Returns side effects that should be handled after commit
func Store(tx *sql.Tx, inst *dto.Instance) (*StoreSideEffects, error) {
	sideEffects := &StoreSideEffects{}

	activityId, _, _, err := getActivityInfo(inst.Hash)
	if err != nil {
		return nil, err
	}

	isDuplicate, err := insertInstance(tx, inst)
	if err != nil {
		return nil, err
	}
	if isDuplicate {
		return nil, nil
	}

	completedDictionary := map[int64]bool{}
	fastestClearSoFar := map[int64]int{}
	var characterRequests []messages.CharacterFillMessage

	for _, playerActivity := range inst.Players {
		duration, err := getFastestClearDuration(tx, playerActivity.Player.MembershipId, activityId)
		if err != nil {
			return nil, err
		}
		fastestClearSoFar[playerActivity.Player.MembershipId] = duration

		playerRaidClearCount, err := getPlayerClearCount(tx, playerActivity.Player.MembershipId, activityId)
		if err != nil {
			return nil, err
		}

		if playerActivity.Finished {
			completedDictionary[playerActivity.Player.MembershipId] = playerRaidClearCount > 0
		}

		err = storePlayerData(tx, inst, playerActivity, activityId)
		if err != nil {
			return nil, err
		}

		if playerActivity.Player.MembershipType == nil || *playerActivity.Player.MembershipType == 0 {
			sideEffects.PlayerCrawlRequests = append(sideEffects.PlayerCrawlRequests, playerActivity.Player.MembershipId)
		}

		charRequests, err := storeCharacterData(tx, inst, playerActivity, sideEffects.CharacterFillRequests)
		if err != nil {
			return nil, err
		}
		characterRequests = charRequests
	}

	err = updateTimePlayedSeconds(tx, inst, activityId)
	if err != nil {
		return nil, err
	}

	noobsCount, sherpasHappened := determineSherpas(completedDictionary, inst.InstanceId)

	err = updatePlayerStats(tx, inst, completedDictionary, activityId, noobsCount, sherpasHappened, fastestClearSoFar, &sideEffects.PlayerCrawlRequests)
	if err != nil {
		return nil, err
	}

	sideEffects.CharacterFillRequests = characterRequests

	return sideEffects, nil
}

// insertInstance inserts instance data into the database
func insertInstance(tx *sql.Tx, inst *dto.Instance) (bool, error) {
	_, err := tx.Exec(`INSERT INTO "instance" (
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
			return true, nil // isDuplicate = true
		} else {
			return false, fmt.Errorf("inserting instance %d: %w", inst.InstanceId, err)
		}
	}
	return false, nil // not duplicate
}

// getFastestClearDuration retrieves the current fastest clear duration for a player
func getFastestClearDuration(tx *sql.Tx, membershipId int64, activityId int) (int, error) {
	var duration int
	err := tx.QueryRow(`
		SELECT COALESCE(SUM(a.duration), 100000000)
		FROM player_stats ps
		LEFT JOIN instance a ON ps.fastest_instance_id = a.instance_id
		WHERE ps.membership_id = $1 AND ps.activity_id = $2`, membershipId, activityId).
		Scan(&duration)
	if err != nil {
		return 0, fmt.Errorf("querying fastest clear duration for membership_id %d, activity_id %d: %w", membershipId, activityId, err)
	}
	return duration, nil
}

// getPlayerClearCount retrieves the number of clears for a player
func getPlayerClearCount(tx *sql.Tx, membershipId int64, activityId int) (int, error) {
	var playerRaidClearCount int
	err := tx.QueryRow(`
		SELECT COALESCE(SUM(ps.clears), 0) AS count
		FROM player_stats ps
		WHERE ps.membership_id = $1 AND ps.activity_id = $2`, membershipId, activityId).
		Scan(&playerRaidClearCount)
	if err != nil {
		return 0, fmt.Errorf("querying clears for membership_id %d, activity_id %d: %w", membershipId, activityId, err)
	}
	return playerRaidClearCount, nil
}

// storePlayerData stores player-related data for an instance
func storePlayerData(tx *sql.Tx, inst *dto.Instance, playerActivity dto.InstancePlayer, activityId int) error {
	if _, err := player.UpsertPlayer(tx, &playerActivity.Player); err != nil {
		return fmt.Errorf("inserting player %d for instance %d: %w", playerActivity.Player.MembershipId, inst.InstanceId, err)
	}

	_, err := tx.Exec(`
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
		return fmt.Errorf("inserting instance_player for instance %d, membership %d: %w", inst.InstanceId, playerActivity.Player.MembershipId, err)
	}

	_, err = tx.Exec(`INSERT INTO player_stats ("membership_id", "activity_id")
		VALUES ($1, $2)
		ON CONFLICT (membership_id, activity_id) DO NOTHING`,
		playerActivity.Player.MembershipId, activityId)
	if err != nil {
		return fmt.Errorf("inserting player_stats for membership %d, activity %d: %w", playerActivity.Player.MembershipId, activityId, err)
	}

	return nil
}

// storeCharacterData stores character and weapon data for a player in an instance
func storeCharacterData(tx *sql.Tx, inst *dto.Instance, playerActivity dto.InstancePlayer, characterRequests []messages.CharacterFillMessage) ([]messages.CharacterFillMessage, error) {
	for _, character := range playerActivity.Characters {
		_, err := tx.Exec(`
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
			return nil, fmt.Errorf("inserting instance_character for instance %d, membership %d, character %d: %w",
				inst.InstanceId, playerActivity.Player.MembershipId, character.CharacterId, err)
		}

		var wg sync.WaitGroup
		errs := make(chan error, len(character.Weapons))
		for _, weapon := range character.Weapons {
			wg.Add(1)
			go func(weapon dto.InstanceCharacterWeapon) {
				defer wg.Done()
				_, err := tx.Exec(`
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
					errs <- fmt.Errorf("inserting instance_character_weapon for instance %d, character %d, weapon %d: %w",
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
			characterRequests = append(characterRequests, messages.NewCharacterFillMessage(
				playerActivity.Player.MembershipId,
				character.CharacterId,
				inst.InstanceId,
			))
		}
	}
	return characterRequests, nil
}

// updateTimePlayedSeconds updates the total time played for all players in the instance
func updateTimePlayedSeconds(tx *sql.Tx, inst *dto.Instance, activityId int) error {
	for _, playerActivity := range inst.Players {
		_, err := tx.Exec(`UPDATE player_stats 
			SET total_time_played_seconds = total_time_played_seconds + $1 
			WHERE membership_id = $2 AND activity_id = $3`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId, activityId)
		if err != nil {
			return fmt.Errorf("updating total_time_played_seconds for membership %d: %w", playerActivity.Player.MembershipId, err)
		}

		_, err = tx.Exec(`UPDATE player 
			SET total_time_played_seconds = total_time_played_seconds + $1
			WHERE membership_id = $2`,
			playerActivity.TimePlayedSeconds, playerActivity.Player.MembershipId)
		if err != nil {
			return fmt.Errorf("updating total_time_played_seconds for membership %d: %w", playerActivity.Player.MembershipId, err)
		}
	}
	return nil
}

// determineSherpas determines if sherpas occurred in this instance
func determineSherpas(completedDictionary map[int64]bool, instanceId int64) (int, bool) {
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

	return noobsCount, sherpasHappened
}

// updatePlayerStats updates statistics for players who completed the instance
func updatePlayerStats(tx *sql.Tx, inst *dto.Instance, completedDictionary map[int64]bool, activityId int, noobsCount int, sherpasHappened bool, fastestClearSoFar map[int64]int, playerCrawlRequests *[]int64) error {
	for membershipId, hasClears := range completedDictionary {
		var playerActivity *dto.InstancePlayer
		for _, pa := range inst.Players {
			if pa.Player.MembershipId == membershipId {
				playerActivity = &pa
				break
			}
		}
		if playerActivity == nil {
			return fmt.Errorf("player %d not found in inst.Players", membershipId)
		}

		if hasClears && sherpasHappened {
			playerActivity.Sherpas = noobsCount
			_, err := tx.Exec(`UPDATE 
				instance_player
			SET 
				sherpas = $1
			WHERE 
				membership_id = $2 AND
				instance_id = $3`, playerActivity.Sherpas, membershipId, inst.InstanceId)

			if err != nil {
				return fmt.Errorf("updating sherpa count for instance %d, membership %d: %w", inst.InstanceId, membershipId, err)
			}
		} else if !hasClears {
			playerActivity.IsFirstClear = true
			_, err := tx.Exec(`UPDATE 
				instance_player
			SET 
				is_first_clear = true
			WHERE 
				membership_id = $1 AND
				instance_id = $2`, membershipId, inst.InstanceId)

			if err != nil {
				return fmt.Errorf("updating first clear for instance %d, membership %d: %w", inst.InstanceId, membershipId, err)
			}

			*playerCrawlRequests = append(*playerCrawlRequests, membershipId)
		}

		_, err := tx.Exec(`UPDATE player_stats 
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
			return fmt.Errorf("updating player_stats for membership %d: %w", membershipId, err)
		}

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
			return fmt.Errorf("updating global stats for membership %d: %w", membershipId, err)
		}

		if inst.Fresh != nil && *inst.Fresh && inst.DurationSeconds < fastestClearSoFar[membershipId] {
			_, err := stats.UpdatePlayerSumOfBest(membershipId, tx)
			if err != nil {
				return fmt.Errorf("updating sum of best for membership %d: %w", membershipId, err)
			}
		}
	}
	return nil
}
