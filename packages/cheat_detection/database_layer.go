package cheat_detection

import (
	"database/sql"
	"encoding/json"
	"time"
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
			'seasonId', i."season_id", 
			'cheatOverride', i."cheat_override",
			'players', (
				SELECT 
					ARRAY_AGG(
						DISTINCT JSONB_BUILD_OBJECT(
							'membershipId', ip."membership_id",
							'completed', ip."completed",
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

func getRecentFlags(since time.Time, db *sql.DB) ([]FlagInstance, []FlagInstancePlayer, error) {
	instanceRows, err := db.Query(`SELECT "instance_id", "cheat_check_bitmask", "cheat_probability"
		FROM "flag_instance"
		WHERE "flagged_at" > $1;`, since)
	if err != nil {
		return nil, nil, err
	}
	defer instanceRows.Close()

	playerRows, err := db.Query(`SELECT "instance_id", "membership_id", "cheat_check_bitmask", "cheat_probability"
		FROM "flag_instance_player"
		WHERE "flagged_at" > $1;`, since)
	if err != nil {
		return nil, nil, err
	}
	defer playerRows.Close()

	var instances []FlagInstance
	for instanceRows.Next() {
		var instance FlagInstance
		err = instanceRows.Scan(&instance.InstanceId, &instance.CheatCheckBitmask, &instance.CheatProbability)
		if err != nil {
			return nil, nil, err
		}
		instances = append(instances, instance)
	}

	var players []FlagInstancePlayer
	for playerRows.Next() {
		var player FlagInstancePlayer
		err = playerRows.Scan(&player.InstanceId, &player.MembershipId, &player.CheatCheckBitmask, &player.CheatProbability)
		if err != nil {
			return nil, nil, err
		}
		players = append(players, player)
	}

	return instances, players, nil
}

func getPlayerFlags(membershipId int64, db *sql.DB) ([]FlagInstancePlayer, error) {
	rows, err := db.Query(`WITH ranked_flags AS (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY "instance_id" ORDER BY "is_override" DESC, "flagged_at" DESC) AS rank
			FROM "flag_instance_player"
			WHERE "membership_id" = $1
		)
		SELECT
			"instance_id",
			"cheat_check_bitmask",
			"cheat_probability",
		FROM ranked_flags
		WHERE rank = 1
			AND "cheat_probability" > 0
		ORDER BY "is_override" DESC, "flagged_at" DESC;`, membershipId)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var flags []FlagInstancePlayer
	for rows.Next() {
		var flag FlagInstancePlayer
		err = rows.Scan(&flag.InstanceId, &flag.CheatCheckBitmask, &flag.CheatProbability)
		if err != nil {
			return nil, err
		}
		flags = append(flags, flag)
	}

	return flags, nil
}
