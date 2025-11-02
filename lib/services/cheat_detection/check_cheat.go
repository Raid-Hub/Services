package cheat_detection

import (
	"database/sql"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
)

// Logger is declared in database_layer.go

// CheckCheat runs cheat detection on a PGCR
func CheckCheat(instanceId int64) error {
	// Check if this instance was already flagged with an old cheat check version
	var existingVersion sql.NullString
	err := postgres.DB.QueryRow(`SELECT cheat_check_version FROM flag_instance WHERE instance_id = $1 LIMIT 1`, instanceId).Scan(&existingVersion)
	if err == nil && existingVersion.Valid && existingVersion.String != CheatCheckVersion {
		logger.Debug("OLD_CHEAT_CHECK_VERSION", map[string]any{
			logging.INSTANCE_ID: instanceId,
			"existing_version":  existingVersion.String,
			"current_version":   CheatCheckVersion,
		})
	}

	// Run cheat detection
	_, instanceResult, playerResults, _, err := CheckForCheats(instanceId)
	if err != nil {
		logger.Error(CHEAT_CHECK_ERROR, err, map[string]any{
			logging.INSTANCE_ID: instanceId,
			logging.OPERATION:   "CheckForCheats",
		})
		return err
	}

	// Log results (only if cheat detected above threshold)
	if instanceResult.Probability > Threshold {
		logger.Info("CHEAT_DETECTED", map[string]any{
			logging.INSTANCE_ID:   instanceId,
			"probability":         instanceResult.Probability,
			"player_flags":        len(playerResults),
			"cheat_check_version": CheatCheckVersion,
		})
	}

	for _, playerResult := range playerResults {
		logger.Info("PLAYER_CHEAT_DETECTED", map[string]any{
			logging.INSTANCE_ID:   instanceId,
			logging.MEMBERSHIP_ID: playerResult.MembershipId,
			"probability":         playerResult.Probability,
			"cheat_check_version": CheatCheckVersion,
		})
	}

	return nil
}
