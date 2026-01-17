package main

import (
	"flag"
	"fmt"
	"strconv"

	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"

	"github.com/lib/pq"
)

var logger = logging.NewLogger("clear-player")

func main() {
	membershipIdStr := flag.String("player", "", "Membership ID of the player to clear")
	dryRun := flag.Bool("dry-run", false, "Dry run mode - show what would be cleared without making changes")

	logging.ParseFlags()

	if *membershipIdStr == "" {
		logger.Fatal("MISSING_PLAYER", fmt.Errorf("player membership ID is required"), map[string]any{})
	}

	// Parse membership ID
	membershipId, err := strconv.ParseInt(*membershipIdStr, 10, 64)
	if err != nil {
		logger.Fatal("INVALID_PLAYER_ID", err, map[string]any{"player": *membershipIdStr})
	}

	currentVersion := cheat_detection.CheatCheckVersion

	logger.Info("VALIDATION_PASSED", map[string]any{
		"membership_id":   membershipId,
		"current_version": currentVersion,
		"dry_run":         *dryRun,
	})

	// Wait for PostgreSQL connection
	postgres.Wait()

	// Get player name for logging
	bungieName, err := cheat_detection.GetPlayerName(membershipId)
	if err != nil {
		logger.Fatal("PLAYER_NOT_FOUND", err, map[string]any{"membership_id": membershipId})
	}

	logger.Info("PLAYER_FOUND", map[string]any{
		"membership_id": membershipId,
		"bungie_name":   bungieName,
	})

	if *dryRun {
		// For dry run, we need to check what would be cleared
		// We'll use the service function but in a transaction that we rollback
		tx, err := postgres.DB.Begin()
		if err != nil {
			logger.Fatal("TRANSACTION_START_ERROR", err, map[string]any{})
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
			logger.Fatal("QUERY_BLACKLISTED_INSTANCES_ERROR", err, map[string]any{})
		}
		defer rows.Close()

		for rows.Next() {
			var instanceId int64
			if err := rows.Scan(&instanceId); err != nil {
				logger.Fatal("SCAN_INSTANCE_ID_ERROR", err, map[string]any{})
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
			logger.Fatal("QUERY_BLACKLISTED_INSTANCES_BY_PLAYER_ERROR", err, map[string]any{})
		}
		defer rows.Close()

		for rows.Next() {
			var instanceId int64
			if err := rows.Scan(&instanceId); err != nil {
				logger.Fatal("SCAN_INSTANCE_ID_ERROR", err, map[string]any{})
			}
			if !instanceIdMap[instanceId] {
				blacklistedInstanceIds = append(blacklistedInstanceIds, instanceId)
				instanceIdMap[instanceId] = true
			}
		}
		rows.Close()

		logger.Info("FOUND_BLACKLISTED_INSTANCES", map[string]any{
			"instance_count": len(blacklistedInstanceIds),
		})

		// Get sample instance IDs (first 10)
		sampleInstanceIds := blacklistedInstanceIds
		if len(sampleInstanceIds) > 10 {
			sampleInstanceIds = sampleInstanceIds[:10]
		}

		// Check cheat level
		var cheatLevel int
		err = tx.QueryRow(`SELECT cheat_level FROM player WHERE membership_id = $1`, membershipId).Scan(&cheatLevel)
		if err != nil {
			logger.Fatal("QUERY_CHEAT_LEVEL_ERROR", err, map[string]any{})
		}

		// Count what would be deleted
		var wouldDeleteFlags int
		var wouldDeleteFpis int
		var wouldDeleteBls int
		var wouldDeleteBlis int

		if len(blacklistedInstanceIds) > 0 {
			// Count flag_instance_player that would be deleted (all players for those instances)
			err = tx.QueryRow(`
				SELECT COUNT(*)
				FROM flag_instance_player
				WHERE instance_id = ANY($1)
				AND cheat_check_version != $2
			`, pq.Array(blacklistedInstanceIds), currentVersion).Scan(&wouldDeleteFpis)
			if err != nil {
				logger.Fatal("COUNT_FPIS_ERROR", err, map[string]any{})
			}

			// Count flag_instance that would be deleted
			err = tx.QueryRow(`
				SELECT COUNT(*)
				FROM flag_instance
				WHERE instance_id = ANY($1)
				AND cheat_check_version != $2
			`, pq.Array(blacklistedInstanceIds), currentVersion).Scan(&wouldDeleteFlags)
			if err != nil {
				logger.Fatal("COUNT_FLAGS_ERROR", err, map[string]any{})
			}

			// Count blacklist_instance_player that would be deleted
			err = tx.QueryRow(`
				SELECT COUNT(*)
				FROM blacklist_instance_player
				WHERE membership_id = $1
			`, membershipId).Scan(&wouldDeleteBls)
			if err != nil {
				logger.Fatal("COUNT_BLS_ERROR", err, map[string]any{})
			}

			// Count blacklist_instance that would be deleted
			err = tx.QueryRow(`
				SELECT COUNT(*)
				FROM blacklist_instance
				WHERE instance_id = ANY($1)
			`, pq.Array(blacklistedInstanceIds)).Scan(&wouldDeleteBlis)
			if err != nil {
				logger.Fatal("COUNT_BLIS_ERROR", err, map[string]any{})
			}
		}

		logger.Info("DRY_RUN_COMPLETE", map[string]any{
			"membership_id":                     membershipId,
			"bungie_name":                       bungieName,
			"blacklisted_instances":             len(blacklistedInstanceIds),
			"sample_instance_ids":               sampleInstanceIds,
			"current_cheat_level":               cheatLevel,
			"flag_instance_removed":             wouldDeleteFlags,
			"flag_instance_player_removed":      wouldDeleteFpis,
			"blacklist_instance_player_removed": wouldDeleteBls,
			"blacklist_instance_removed":        wouldDeleteBlis,
			"would_remove_blacklist":            true,
			"would_clear_flags":                 true,
			"would_reset_cheat_level":           true,
		})
		return
	}

	// Use service function to clear flags and blacklists
	instanceFlagsDeleted, playerFlagsDeleted, blacklistPlayerDeleted, blacklistInstanceDeleted, blacklistedInstanceIds, err := cheat_detection.ClearPlayerFlagsAndBlacklists(membershipId, currentVersion)
	if err != nil {
		logger.Fatal("CLEAR_PLAYER_FLAGS_ERROR", err, map[string]any{})
	}

	// Reset cheat_level for the player
	cheatLevelReset, err := cheat_detection.ResetPlayerCheatLevel([]int64{membershipId})
	if err != nil {
		logger.Fatal("RESET_CHEAT_LEVEL_ERROR", err, map[string]any{})
	}

	// Get sample instance IDs (first 10)
	sampleInstanceIds := blacklistedInstanceIds
	if len(sampleInstanceIds) > 10 {
		sampleInstanceIds = sampleInstanceIds[:10]
	}

	logger.Info("CLEAR_PLAYER_COMPLETE", map[string]any{
		"membership_id":                     membershipId,
		"bungie_name":                       bungieName,
		"flag_instance_removed":             instanceFlagsDeleted,
		"flag_instance_player_removed":      playerFlagsDeleted,
		"blacklist_instance_player_removed": blacklistPlayerDeleted,
		"blacklist_instance_removed":        blacklistInstanceDeleted,
		"cheat_level_reset":                 cheatLevelReset,
		"instances_affected":                len(blacklistedInstanceIds),
		"sample_instance_ids":               sampleInstanceIds,
	})
}
