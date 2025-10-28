package cheat_detection

import "raidhub/lib/utils"

var logger = utils.NewLogger("CHEAT_DETECTION")

// CheckCheat runs cheat detection on a PGCR
func CheckCheat(instanceId int64) error {
	// Run cheat detection
	_, instanceResult, playerResults, _, err := CheckForCheats(instanceId)
	if err != nil {
		logger.ErrorF("Failed to check for cheat instanceId=%d: %v", instanceId, err)
		return err
	}

	// Log results (only if cheat detected with meaningful probability)
	if instanceResult.Probability > 0.01 {
		logger.InfoF("Cheat detected instanceId=%d probability=%.3f playerFlags=%d", instanceId, instanceResult.Probability, len(playerResults))
	}

	return nil
}
