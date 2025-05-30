package cheat_detection

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"raidhub/packages/bungie"
	"time"

	"github.com/google/uuid"
)

func getInstance(instanceId int64, db *sql.DB) (*Instance, error) {
	row := db.QueryRow(`SELECT 
		JSONB_BUILD_OBJECT(
			'instanceId', i."instance_id", 
			'activity', av."activity_id",
			'version', av."version_id",
			'score', i."score", 
			'flawless', i."flawless", 
			'completed', i."completed", 
			'fresh', i."fresh", 
			'playerCount', i."player_count", 
			'dateStarted', i."date_started" AT TIME ZONE 'UTC', 
			'dateCompleted', i."date_completed" AT TIME ZONE 'UTC', 
			'daysAfterRelease', EXTRACT(EPOCH FROM (i."date_completed" - ad."release_date")) / 86400.0,
			'duration', i."duration", 
			'platformType', i."platform_type", 
			'season', i."season_id",
			'raidPath', ad."r2_path",
			'players', (
				SELECT 
					ARRAY_AGG(
						DISTINCT JSONB_BUILD_OBJECT(
							'membershipId', ip."membership_id",
							'finished', ip."completed",
							'timePlayedSeconds', ip."time_played_seconds",
							'sherpas', ip."sherpas",
							'isFirstClear', ip."is_first_clear",
							'characters', (
								SELECT 
									ARRAY_AGG(
										DISTINCT JSONB_BUILD_OBJECT(
											'characterId', ic."character_id",
											'classHash', ic."class_hash",
											'emblemHash', ic."emblem_hash",
											'completed', ic."completed",
											'score', ic."score",
											'kills', ic."kills",
											'assists', ic."assists",
											'deaths', ic."deaths",
											'precisionKills', ic."precision_kills",
											'superKills', ic."super_kills",
											'grenadeKills', ic."grenade_kills",
											'meleeKills', ic."melee_kills",
											'timePlayedSeconds', ic."time_played_seconds",
											'startSeconds', ic."start_seconds",
											'weapons', (
												SELECT 
													ARRAY_AGG(
														DISTINCT JSONB_BUILD_OBJECT(
															'kills', icw."kills",
															'precisionKills', icw."precision_kills",
															'name', wd."name",
															'weaponType', wd."weapon_type",
															'ammoType', wd."ammo_type",
															'slot', wd."slot",
															'element', wd."element"
														)
													)
												FROM "instance_character_weapon" icw
												JOIN "weapon_definition" wd ON icw."weapon_hash" = wd."hash"
												WHERE icw."instance_id" = ic."instance_id"
													AND icw."membership_id" = ic."membership_id"
													AND icw."character_id" = ic."character_id"
											)
										)
									)
								FROM "instance_character" ic
								WHERE ic."instance_id" = ip."instance_id"
									AND ic."membership_id" = ip."membership_id"
							)
						)
					)
				FROM "instance_player" ip
				WHERE ip."instance_id" = i."instance_id"
			)
		) AS instance_json
		FROM "instance" i
		JOIN "activity_version" av ON i."hash" = av."hash"
		JOIN "activity_definition" ad ON av."activity_id" = ad."id"
		WHERE i."instance_id" = $1;
		`, instanceId)

	var bytes []byte
	err := row.Scan(&bytes)
	if err != nil {
		return nil, err
	}

	var instance Instance
	err = json.Unmarshal(bytes, &instance)

	return &instance, err
}

type FlagInstance struct {
	InstanceId        int64
	CheatCheckVersion string
	CheatCheckBitmask uint64
	CheatProbability  float64
	Explanation       string
}

type FlagInstancePlayer struct {
	InstanceId        int64
	MembershipId      int64
	CheatCheckVersion string
	CheatCheckBitmask uint64
	CheatProbability  float64
	Explanation       string
}

func flagInstance(flag FlagInstance, tx *sql.Tx) error {
	_, err := tx.Exec(`INSERT INTO "flag_instance"
		("instance_id", "cheat_check_version", "cheat_check_bitmask", "cheat_probability")
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, flag.InstanceId, flag.CheatCheckVersion, flag.CheatCheckBitmask, flag.CheatProbability)
	return err
}

func flagPlayerInstance(flag FlagInstancePlayer, tx *sql.Tx) error {
	_, err := tx.Exec(`INSERT INTO "flag_instance_player"
		("instance_id", "membership_id", "cheat_check_version", "cheat_check_bitmask", "cheat_probability")
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`, flag.InstanceId, flag.MembershipId, flag.CheatCheckVersion, flag.CheatCheckBitmask, flag.CheatProbability)
	return err
}

func GetAllInstanceFlagsByPlayer(db *sql.DB, out chan PlayerInstanceFlagStats, versionLike string) *sql.Rows {
	// Get all players who have been flagged excessively in the last 30 days
	rows, err := db.Query(`
		WITH flags AS (
			SELECT DISTINCT ON (membership_id, instance_id)
				instance_id,
				membership_id,
				CASE
					WHEN fp.cheat_probability >= 0.85 THEN 'a'
					WHEN fp.cheat_probability >= 0.50 OR f.cheat_probability >= 0.99 THEN 'b'
					WHEN fp.cheat_probability >= 0.25 OR f.cheat_probability >= 0.50 THEN 'c'
					ELSE 'd'
				END AS flag_class
			FROM flag_instance_player fp
			LEFT JOIN flag_instance f USING (instance_id, cheat_check_version)
			JOIN instance i USING (instance_id)
			WHERE fp.flagged_at >= NOW() - INTERVAL '60 days'
				AND cheat_check_version LIKE $1
			ORDER BY membership_id, instance_id, fp.flagged_at DESC
		)
		SELECT 
			membership_id,
			COUNT(*) AS flagged_count,
			COUNT(CASE WHEN flag_class = 'a' THEN 1 END) AS flags_type_a,
			COUNT(CASE WHEN flag_class = 'b' THEN 1 END) AS flags_type_b,
			COUNT(CASE WHEN flag_class = 'c' THEN 1 END) AS flags_type_c,
			COUNT(CASE WHEN flag_class = 'd' THEN 1 END) AS flags_type_d
		FROM (
			SELECT membership_id, flag_class
			FROM flags
			JOIN player USING (membership_id)
			WHERE cheat_level < 4
				AND NOT is_whitelisted
		) AS flags
		GROUP BY membership_id
		HAVING COUNT(*) >= 3
	`, versionLike)

	if err != nil {
		log.Fatalf("Error querying the database: %s", err)
	}

	return rows
}

func UpdatePlayerCheatLevel(db *sql.DB, flag PlayerInstanceFlagStats) (int, float64, uint64) {
	var ageInDays float64
	var clears int
	var membershipType int
	var iconPath string
	var bungieName string
	var currentCheatLevel int
	var isPrivate bool
	// get the age of the account and # of clears
	err := db.QueryRow(`
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
	`, flag.MembershipId).Scan(&ageInDays, &clears, &membershipType, &iconPath, &bungieName, &currentCheatLevel, &isPrivate)
	if err != nil {
		log.Fatalf("Error getting player info for %d: %s", flag.MembershipId, err)
		return -1, 0, 0
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
	`, flag.MembershipId).Scan(&flawlessRatio, &lowmanRatio, &soloRatio)
	if err != nil {
		log.Fatalf("Error getting ratios for player %d: %s", flag.MembershipId, err)
		return -1, 0, 0
	}

	res, err := bungie.GetProfile(membershipType, flag.MembershipId, []int{100})
	if err != nil {
		log.Printf("Error getting profile for player %d: %s", flag.MembershipId, err)
		return -1, 0, 0
	}

	cheaterAccountChance, bitFlags := GetCheaterAccountChance(res.Profile.Data, clears, ageInDays, flawlessRatio, lowmanRatio, soloRatio, isPrivate)

	minCheatLevel := GetMinimumCheatLevel(flag, cheaterAccountChance)

	if minCheatLevel > currentCheatLevel {
		log.Printf("Upgrading cheat level for player %d %s from %d to %d", flag.MembershipId, bungieName, currentCheatLevel, minCheatLevel)
		// Update the player's cheat level in the database
		_, err = db.Exec(`
			UPDATE player
			SET cheat_level = GREATEST(cheat_level, $1)
			WHERE membership_id = $2;
		`, minCheatLevel, flag.MembershipId)

		if err != nil {
			log.Fatalf("Error updating cheat level for player %d: %s", flag.MembershipId, err)
		}

		if minCheatLevel == 4 {
			flag.SendBlacklistedPlayerWebhook(res.Profile.Data, clears, ageInDays, bungieName, iconPath, cheaterAccountChance, bitFlags)
		}

		if minCheatLevel >= 2 {
			err := SendGmReportWebhook(flag.MembershipId, GmReportWebhookMetadata{
				CheaterAccountProbability:  cheaterAccountChance,
				CheaterAccountHeuristics:   GetCheaterAccountFlagsStrings(bitFlags),
				RaidHubCheatLevel:          minCheatLevel,
				EstimatedAccountAgeDays:    ageInDays,
				LookBackDays:               60,
				RaidClears:                 clears,
				FractionRaidClearsSolo:     soloRatio,
				FractionRaidClearsLowman:   lowmanRatio,
				FractionRaidClearsFlawless: flawlessRatio,
				Flags: GmReportWebhookFlags{
					Total:  flag.FlaggedCount,
					ClassA: flag.FlagsA,
					ClassB: flag.FlagsB,
					ClassC: flag.FlagsC,
					ClassD: flag.FlagsD,
				},
			})

			if err != nil {
				log.Printf("Error sending GM Report webhook for player %d: %s", flag.MembershipId, err)
			}
		}
	}

	return minCheatLevel, cheaterAccountChance, bitFlags
}

type BlacklistedPlayerDTO struct {
	MembershipId int64     `json:"membership_id"`
	LastSeen     time.Time `json:"last_seen"`
}

func GetRecentlyPlayedBlacklistedPlayers(db *sql.DB, since time.Time) ([]BlacklistedPlayerDTO, error) {
	rows, err := db.Query(`
		SELECT membership_id, last_seen
		FROM player
		WHERE cheat_level = 4
			AND last_seen >= $1
	`, since)
	if err != nil {
		return nil, fmt.Errorf("error querying recently blacklisted players: %w", err)
	}
	defer rows.Close()

	var players []BlacklistedPlayerDTO
	for rows.Next() {
		var player BlacklistedPlayerDTO
		if err := rows.Scan(
			&player.MembershipId,
			&player.LastSeen,
		); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		players = append(players, player)
	}

	return players, nil
}

func BlacklistRecentInstances(db *sql.DB, blacklistedPlayer BlacklistedPlayerDTO) (int64, int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	tempTableName := fmt.Sprintf("temp_tainted_instances_%s", uuid.New().String())

	_, err = tx.Exec(fmt.Sprintf(`
        CREATE TEMP TABLE "%s" ON COMMIT DROP AS
        SELECT DISTINCT ON (instance_id)
            instance_id,
            COALESCE(fi.cheat_check_version, fip.cheat_check_version) AS cheat_check_version
        FROM instance_player ip
        JOIN instance i USING (instance_id)
        LEFT JOIN flag_instance fi USING (instance_id)
        LEFT JOIN flag_instance_player fip USING (instance_id, membership_id)
		LEFT JOIN team_activity_version_leaderboard avl USING (instance_id)
		LEFT JOIN world_first_contest_leaderboard wfc USING (instance_id)
        WHERE ip.membership_id = $1
            AND (
				fi.cheat_probability >= 0.75 
				OR fip.cheat_probability >= 0.5
				OR (
					(
						i.player_count = 1
						OR ip.time_played_seconds >= 300
						OR (ip.time_played_seconds::DOUBLE PRECISION / i.duration) >= 0.30
						OR ip.completed = true
						OR fi.cheat_probability >= 0.25
						OR fip.cheat_probability >= 0.10
					)
					AND (
						i.date_completed >= ($2::timestamp - INTERVAL '60 days')
						OR (
							i.date_completed >= ($2::timestamp - INTERVAL '1 year')
							AND (avl.instance_id IS NOT NULL OR wfc.instance_id IS NOT NULL)
						)
					)
				)
			)
        ORDER BY instance_id, COALESCE(fi.flagged_at, fip.flagged_at) DESC`, tempTableName),
		blacklistedPlayer.MembershipId, blacklistedPlayer.LastSeen)

	if err != nil {
		return 0, 0, err
	}

	r, err := tx.Exec(fmt.Sprintf(`
        INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
        SELECT instance_id, 'BlacklistedPlayerCascade', cheat_check_version, 
               'Blacklisted player ' || $1::text || ' has played in this instance'
        FROM "%s"
        ON CONFLICT DO NOTHING`, tempTableName),
		blacklistedPlayer.MembershipId)

	if err != nil {
		return 0, 0, err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("error getting rows affected: %w", err)
	}

	_, err = tx.Exec(fmt.Sprintf(`
        INSERT INTO blacklist_instance_player (instance_id, membership_id, reason)
        SELECT instance_id, $1, 'Automatic blacklist due to player standing'
        FROM "%s"
        ON CONFLICT DO NOTHING`, tempTableName),
		blacklistedPlayer.MembershipId)

	if err != nil {
		return 0, 0, err
	}

	var instancesElligible int64
	err = tx.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tempTableName)).Scan(&instancesElligible)
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return rowsAffected, instancesElligible, nil
}

func BlacklistFlaggedInstances(db *sql.DB) (int64, error) {
	// blacklist all instances that have been flagged with cheat_probability >= 0.9
	result, err := db.Exec(`
		INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
		SELECT DISTINCT ON (instance_id)
		 	instance_id, 'CheatCheck', fi.cheat_check_version,
			'Flagged >= 0.95'
		FROM flag_instance fi
		JOIN instance USING (instance_id)
		WHERE fi.cheat_probability >= 0.95
			AND fi.flagged_at >= NOW() - INTERVAL '60 days'
		ORDER BY instance_id, fi.flagged_at DESC
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		return 0, fmt.Errorf("error blacklisting flagged instances: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error getting rows affected: %w", err)
	}

	return rowsAffected, nil
}
