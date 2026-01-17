package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("clear-flags")

func parseBitmap(bitmapStr string) (uint64, error) {
	parts := strings.Split(bitmapStr, ",")
	var bitmap uint64

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid bitmap value: %s", part)
		}

		bitmap |= value
	}

	if bitmap == 0 {
		return 0, fmt.Errorf("no valid bitmap values specified")
	}

	return bitmap, nil
}

func main() {
	bitmapStr := flag.String("bitmap", "", "Comma-separated bitmap values to match (e.g., 1024,2048) - values will be combined with OR")
	dateStr := flag.String("date", "", "Earliest date (RFC3339 format, e.g., 2024-01-01T00:00:00Z) - only flags on or after this date will be cleared")
	dryRun := flag.Bool("dry-run", false, "Dry run mode - show what would be cleared without making changes")

	logging.ParseFlags()

	if *bitmapStr == "" {
		logger.Fatal("MISSING_BITMAP", fmt.Errorf("bitmap is required"), map[string]any{})
	}

	if *dateStr == "" {
		logger.Fatal("MISSING_DATE", fmt.Errorf("date is required"), map[string]any{})
	}

	// Parse bitmap values
	bitmap, err := parseBitmap(*bitmapStr)
	if err != nil {
		logger.Fatal("INVALID_BITMAP", err, map[string]any{"bitmap": *bitmapStr})
	}

	// Parse date
	earliestDate, err := time.Parse(time.RFC3339, *dateStr)
	if err != nil {
		logger.Fatal("INVALID_DATE", err, map[string]any{"date": *dateStr})
	}

	currentVersion := cheat_detection.CheatCheckVersion

	logger.Info("VALIDATION_PASSED", map[string]any{
		"bitmap":          bitmap,
		"earliest_date":   earliestDate,
		"current_version": currentVersion,
		"dry_run":         *dryRun,
	})

	// Wait for PostgreSQL connection
	postgres.Wait()

	if *dryRun {
		// For dry run, query to see what would be affected
		tx, err := postgres.DB.Begin()
		if err != nil {
			logger.Fatal("TRANSACTION_START_ERROR", err, map[string]any{})
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
			logger.Fatal("QUERY_INSTANCE_FLAGS_ERROR", err, map[string]any{})
		}
		defer rows.Close()

		for rows.Next() {
			var instanceId int64
			if err := rows.Scan(&instanceId); err != nil {
				logger.Fatal("SCAN_INSTANCE_ID_ERROR", err, map[string]any{})
			}
			instanceIds = append(instanceIds, instanceId)
		}
		rows.Close()

		// Find affected instance IDs from flag_instance_player
		rows, err = tx.Query(`
			SELECT DISTINCT instance_id
			FROM flag_instance_player
			WHERE (cheat_check_bitmask & $1) = $1
				AND flagged_at >= $2
				AND cheat_check_version != $3
		`, bitmap, earliestDate, currentVersion)
		if err != nil {
			logger.Fatal("QUERY_PLAYER_FLAGS_ERROR", err, map[string]any{})
		}
		defer rows.Close()

		instanceIdMap := make(map[int64]bool)
		for _, id := range instanceIds {
			instanceIdMap[id] = true
		}

		for rows.Next() {
			var instanceId int64
			if err := rows.Scan(&instanceId); err != nil {
				logger.Fatal("SCAN_INSTANCE_ID_ERROR", err, map[string]any{})
			}
			if !instanceIdMap[instanceId] {
				instanceIds = append(instanceIds, instanceId)
				instanceIdMap[instanceId] = true
			}
		}
		rows.Close()

		logger.Info("FOUND_AFFECTED_INSTANCES", map[string]any{
			"instance_count": len(instanceIds),
		})

		if len(instanceIds) == 0 {
			logger.Info("NO_FLAGS_TO_CLEAR", map[string]any{})
			return
		}

		// Get affected membership IDs
		membershipIds, err := cheat_detection.GetAffectedMembershipIds(instanceIds)
		if err != nil {
			logger.Fatal("QUERY_MEMBERSHIP_IDS_ERROR", err, map[string]any{})
		}

		logger.Info("FOUND_AFFECTED_PLAYERS", map[string]any{
			"player_count": len(membershipIds),
		})

		// Get player names for dry-run output
		playerNames, err := cheat_detection.GetPlayerNames(membershipIds)
		if err != nil {
			logger.Fatal("QUERY_PLAYER_NAMES_ERROR", err, map[string]any{})
		}

		// Get sample instance IDs (first 10)
		sampleInstanceIds := instanceIds
		if len(sampleInstanceIds) > 10 {
			sampleInstanceIds = sampleInstanceIds[:10]
		}

		logger.Info("DRY_RUN_COMPLETE", map[string]any{
			"instances_to_clear":   len(instanceIds),
			"players_to_reset":     len(membershipIds),
			"player_names":         playerNames,
			"sample_instance_ids":  sampleInstanceIds,
			"would_clear_flags":    true,
			"would_remove_bl":      true,
			"would_reset_cheat_lv": true,
		})
		return
	}

	// Use service function to clear flags
	instanceFlagsDeleted, playerFlagsDeleted, instanceIds, err := cheat_detection.ClearFlagsByBitmap(bitmap, earliestDate, currentVersion)
	if err != nil {
		logger.Fatal("CLEAR_FLAGS_ERROR", err, map[string]any{})
	}

	logger.Info("FOUND_AFFECTED_INSTANCES", map[string]any{
		"instance_count": len(instanceIds),
	})

	if len(instanceIds) == 0 {
		logger.Info("NO_FLAGS_TO_CLEAR", map[string]any{})
		return
	}

	// Get affected membership IDs
	membershipIds, err := cheat_detection.GetAffectedMembershipIds(instanceIds)
	if err != nil {
		logger.Fatal("QUERY_MEMBERSHIP_IDS_ERROR", err, map[string]any{})
	}

	logger.Info("FOUND_AFFECTED_PLAYERS", map[string]any{
		"player_count": len(membershipIds),
	})

	// Reset cheat_level for affected players
	playersReset, err := cheat_detection.ResetPlayerCheatLevel(membershipIds)
	if err != nil {
		logger.Fatal("RESET_CHEAT_LEVEL_ERROR", err, map[string]any{})
	}

	// Get player names for final output
	playerNames, err := cheat_detection.GetPlayerNames(membershipIds)
	if err != nil {
		logger.Fatal("QUERY_PLAYER_NAMES_ERROR", err, map[string]any{})
	}

	// Get sample instance IDs (first 10)
	sampleInstanceIds := instanceIds
	if len(sampleInstanceIds) > 10 {
		sampleInstanceIds = sampleInstanceIds[:10]
	}

	// Note: blacklist deletions are handled inside ClearFlagsByBitmap
	logger.Info("CLEAR_FLAGS_COMPLETE", map[string]any{
		"instance_flags_deleted": instanceFlagsDeleted,
		"player_flags_deleted":    playerFlagsDeleted,
		"players_reset":           playersReset,
		"instances_affected":      len(instanceIds),
		"players_affected":         len(membershipIds),
		"player_names":            playerNames,
		"sample_instance_ids":     sampleInstanceIds,
	})
}
