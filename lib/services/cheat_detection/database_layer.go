package cheat_detection

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"raidhub/lib/utils/logging"
	"time"

	"raidhub/lib/database/postgres"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Cheat detection logging constants
const (
	CHEAT_CHECK_ERROR        = "CHEAT_CHECK_ERROR"
	CHEAT_DETECTED           = "CHEAT_DETECTED"
	PLAYER_INFO_ERROR        = "PLAYER_INFO_ERROR"
	PLAYER_PROFILE_ERROR     = "PLAYER_PROFILE_ERROR"
	PLAYER_NO_DATA           = "PLAYER_NO_DATA"
	CHEAT_LEVEL_UPDATED      = "CHEAT_LEVEL_UPDATED"
	CHEAT_LEVEL_UPDATE_ERROR = "CHEAT_LEVEL_UPDATE_ERROR"
	WEBHOOK_ERROR            = "WEBHOOK_ERROR"
	RATE_LIMITER_ERROR       = "RATE_LIMITER_ERROR"
	PLAYER_BLACKLISTED       = "PLAYER_BLACKLISTED"
)

var logger = logging.NewLogger("CHEAT_DETECTION_SERVICE")

func getInstance(instanceId int64) (*Instance, error) {
	row := postgres.DB.QueryRow(`SELECT 
		JSONB_BUILD_OBJECT(
			'instanceId', i."instance_id", 
			'activity', av."activity_id",
			'version', av."version_id",
			'score', i."score", 
			'flawless', i."flawless", 
			'completed', i."completed", 
			'fresh', i."fresh", 
			'playerCount', i."player_count", 
			'dateStarted', TO_CHAR(i."date_started" AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			'dateCompleted', TO_CHAR(i."date_completed" AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), 
			'daysAfterRelease', EXTRACT(EPOCH FROM (i."date_completed" - ad."release_date")) / 86400.0,
			'duration', i."duration", 
			'platformType', i."platform_type", 
			'season', i."season_id",
			'raidPath', ad."splash_path",
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

func GetAllInstanceFlagsByPlayer(out chan PlayerInstanceFlagStats, versionLike string) *sql.Rows {
	// Get all players who have been flagged excessively in the last 30 days
	rows, err := postgres.DB.Query(`
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
				AND NOT i.is_whitelisted
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
		logger.Warn(CHEAT_CHECK_ERROR, err, map[string]any{
			logging.OPERATION: "database_query",
		})
		panic(err)
	}

	return rows
}

type BlacklistedPlayerDTO struct {
	MembershipId int64     `json:"membership_id"`
	LastSeen     time.Time `json:"last_seen"`
}

func GetRecentlyPlayedBlacklistedPlayers(since time.Time) ([]BlacklistedPlayerDTO, error) {
	rows, err := postgres.DB.Query(`
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

func BlacklistRecentInstances(blacklistedPlayer BlacklistedPlayerDTO) (int64, int64, error) {
	tx, err := postgres.DB.Begin()
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

func BlacklistFlaggedInstances() (int64, error) {
	// blacklist all instances that have been flagged with cheat_probability >= 0.9
	result, err := postgres.DB.Exec(`
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

// ClearPlayerFlagsAndBlacklists clears all flags and blacklists for a specific player
// Returns the number of flags and blacklists cleared
func ClearPlayerFlagsAndBlacklists(membershipId int64, currentVersion string) (int64, int64, int64, int64, []int64, error) {
	tx, err := postgres.DB.Begin()
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	defer tx.Rollback()

	// Find all instance IDs where this player is blacklisted
	var blacklistedInstanceIds []int64
	instanceIdMap := make(map[int64]bool)

	// First, get instances from blacklist_instance_player
	rows, err := tx.Query(`
		SELECT DISTINCT instance_id
		FROM blacklist_instance_player
		WHERE membership_id = $1
	`, membershipId)
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			return 0, 0, 0, 0, nil, err
		}
		if !instanceIdMap[instanceId] {
			blacklistedInstanceIds = append(blacklistedInstanceIds, instanceId)
			instanceIdMap[instanceId] = true
		}
	}
	rows.Close()

	// Also get instances from blacklist_instance where the player participated
	rows, err = tx.Query(`
		SELECT DISTINCT bi.instance_id
		FROM blacklist_instance bi
		JOIN instance_player ip ON bi.instance_id = ip.instance_id
		WHERE ip.membership_id = $1
	`, membershipId)
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			return 0, 0, 0, 0, nil, err
		}
		if !instanceIdMap[instanceId] {
			blacklistedInstanceIds = append(blacklistedInstanceIds, instanceId)
			instanceIdMap[instanceId] = true
		}
	}
	rows.Close()

	// Remove blacklist_instance_player entries for this player
	var blacklistPlayerDeleted int64
	if len(blacklistedInstanceIds) > 0 {
		result, err := tx.Exec(`
			DELETE FROM blacklist_instance_player
			WHERE membership_id = $1
		`, membershipId)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}
		blacklistPlayerDeleted, _ = result.RowsAffected()
	}

	// Remove blacklist_instance entries for those instances
	var blacklistInstanceDeleted int64
	if len(blacklistedInstanceIds) > 0 {
		result, err := tx.Exec(`
			DELETE FROM blacklist_instance
			WHERE instance_id = ANY($1)
		`, pq.Array(blacklistedInstanceIds))
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}
		blacklistInstanceDeleted, _ = result.RowsAffected()
	}

	// Clear flag_instance_player for all players in those instances (only if version differs from current)
	var playerFlagsDeleted int64
	if len(blacklistedInstanceIds) > 0 {
		result, err := tx.Exec(`
			DELETE FROM flag_instance_player
			WHERE instance_id = ANY($1)
			AND cheat_check_version != $2
		`, pq.Array(blacklistedInstanceIds), currentVersion)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}
		playerFlagsDeleted, _ = result.RowsAffected()
	}

	// Clear flag_instance for those instances (only if version differs from current)
	var instanceFlagsDeleted int64
	if len(blacklistedInstanceIds) > 0 {
		result, err := tx.Exec(`
			DELETE FROM flag_instance
			WHERE instance_id = ANY($1)
			AND cheat_check_version != $2
		`, pq.Array(blacklistedInstanceIds), currentVersion)
		if err != nil {
			return 0, 0, 0, 0, nil, err
		}
		instanceFlagsDeleted, _ = result.RowsAffected()
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, 0, nil, err
	}

	return instanceFlagsDeleted, playerFlagsDeleted, blacklistPlayerDeleted, blacklistInstanceDeleted, blacklistedInstanceIds, nil
}

// ClearFlagsByBitmap clears flags matching a specific bitmap pattern
// Returns the number of flags cleared and affected instance IDs
func ClearFlagsByBitmap(bitmap uint64, earliestDate time.Time, currentVersion string) (int64, int64, []int64, error) {
	tx, err := postgres.DB.Begin()
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback()

	// Find affected instance IDs from flag_instance (only those NOT from current version)
	var instanceIds []int64
	rows, err := tx.Query(`
		SELECT DISTINCT instance_id
		FROM flag_instance
		WHERE (cheat_check_bitmask & $1) = $1
			AND flagged_at >= $2
			AND cheat_check_version != $3
	`, bitmap, earliestDate, currentVersion)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			return 0, 0, nil, err
		}
		instanceIds = append(instanceIds, instanceId)
	}
	rows.Close()

	// Find affected instance IDs from flag_instance_player (only those NOT from current version)
	rows, err = tx.Query(`
		SELECT DISTINCT instance_id
		FROM flag_instance_player
		WHERE (cheat_check_bitmask & $1) = $1
			AND flagged_at >= $2
			AND cheat_check_version != $3
	`, bitmap, earliestDate, currentVersion)
	if err != nil {
		return 0, 0, nil, err
	}
	defer rows.Close()

	instanceIdMap := make(map[int64]bool)
	for _, id := range instanceIds {
		instanceIdMap[id] = true
	}

	for rows.Next() {
		var instanceId int64
		if err := rows.Scan(&instanceId); err != nil {
			return 0, 0, nil, err
		}
		if !instanceIdMap[instanceId] {
			instanceIds = append(instanceIds, instanceId)
			instanceIdMap[instanceId] = true
		}
	}
	rows.Close()

	// Clear flag_instance (only those NOT from current version)
	result, err := tx.Exec(`
		DELETE FROM flag_instance
		WHERE (cheat_check_bitmask & $1) = $1
			AND flagged_at >= $2
			AND cheat_check_version != $3
	`, bitmap, earliestDate, currentVersion)
	if err != nil {
		return 0, 0, nil, err
	}
	instanceFlagsDeleted, _ := result.RowsAffected()

	// Clear flag_instance_player (only those NOT from current version)
	result, err = tx.Exec(`
		DELETE FROM flag_instance_player
		WHERE (cheat_check_bitmask & $1) = $1
			AND flagged_at >= $2
			AND cheat_check_version != $3
	`, bitmap, earliestDate, currentVersion)
	if err != nil {
		return 0, 0, nil, err
	}
	playerFlagsDeleted, _ := result.RowsAffected()

	// Remove blacklist_instance_player for affected instances
	result, err = tx.Exec(`
		DELETE FROM blacklist_instance_player
		WHERE instance_id = ANY($1)
	`, pq.Array(instanceIds))
	if err != nil {
		return 0, 0, nil, err
	}

	// Remove blacklist_instance for affected instances
	result, err = tx.Exec(`
		DELETE FROM blacklist_instance
		WHERE instance_id = ANY($1)
	`, pq.Array(instanceIds))
	if err != nil {
		return 0, 0, nil, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, nil, err
	}

	return instanceFlagsDeleted, playerFlagsDeleted, instanceIds, nil
}

// FlagInstanceManually flags an instance manually with the given parameters
func FlagInstanceManually(instanceId int64, cheatCheckVersion string, cheatCheckBitmask uint64, cheatProbability float64) error {
	_, err := postgres.DB.Exec(`
		INSERT INTO flag_instance (instance_id, cheat_check_version, cheat_check_bitmask, flagged_at, cheat_probability)
		VALUES ($1, $2, $3, NOW(), $4)
		ON CONFLICT DO NOTHING
	`, instanceId, cheatCheckVersion, cheatCheckBitmask, cheatProbability)
	return err
}

// BlacklistInstanceManually blacklists an instance manually with the given parameters
func BlacklistInstanceManually(instanceId int64, cheatCheckVersion string, reason string) error {
	_, err := postgres.DB.Exec(`
		INSERT INTO blacklist_instance (instance_id, report_source, cheat_check_version, reason)
		VALUES ($1, 'Manual', $2, $3)
		ON CONFLICT (instance_id)
		DO UPDATE SET report_source = 'Manual', cheat_check_version = $2, reason = $3, created_at = NOW()
	`, instanceId, cheatCheckVersion, reason)
	return err
}

// ResetPlayerCheatLevel resets the cheat level for one or more players
func ResetPlayerCheatLevel(membershipIds []int64) (int64, error) {
	if len(membershipIds) == 0 {
		return 0, nil
	}
	result, err := postgres.DB.Exec(`
		UPDATE player
		SET cheat_level = 0
		WHERE membership_id = ANY($1)
	`, pq.Array(membershipIds))
	if err != nil {
		return 0, err
	}
	rowsAffected, err := result.RowsAffected()
	return rowsAffected, err
}

// GetPlayerName retrieves the bungie_name for a player
func GetPlayerName(membershipId int64) (string, error) {
	var bungieName string
	err := postgres.DB.QueryRow(`SELECT bungie_name FROM player WHERE membership_id = $1`, membershipId).Scan(&bungieName)
	return bungieName, err
}

// GetPlayerNames retrieves bungie_names for multiple players
func GetPlayerNames(membershipIds []int64) ([]string, error) {
	if len(membershipIds) == 0 {
		return []string{}, nil
	}
	rows, err := postgres.DB.Query(`
		SELECT bungie_name
		FROM player
		WHERE membership_id = ANY($1)
		ORDER BY bungie_name
	`, pq.Array(membershipIds))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playerNames []string
	for rows.Next() {
		var bungieName string
		if err := rows.Scan(&bungieName); err != nil {
			return nil, err
		}
		playerNames = append(playerNames, bungieName)
	}
	return playerNames, rows.Err()
}

// GetAffectedMembershipIds gets membership IDs for players in the given instances
func GetAffectedMembershipIds(instanceIds []int64) ([]int64, error) {
	if len(instanceIds) == 0 {
		return []int64{}, nil
	}
	rows, err := postgres.DB.Query(`
		SELECT DISTINCT membership_id
		FROM instance_player
		WHERE instance_id = ANY($1)
	`, pq.Array(instanceIds))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var membershipIds []int64
	for rows.Next() {
		var membershipId int64
		if err := rows.Scan(&membershipId); err != nil {
			return nil, err
		}
		membershipIds = append(membershipIds, membershipId)
	}
	return membershipIds, rows.Err()
}
