package cheat_detection

import (
	"log"
)

// CheckCheat runs cheat detection on a PGCR
func CheckCheat(instanceId int64) error {
	log.Printf("Checking for cheat on instance %d", instanceId)

	// Run cheat detection
	_, instanceResult, playerResults, _, err := CheckForCheats(instanceId)
	if err != nil {
		log.Printf("Error running cheat detection on instance %d: %v", instanceId, err)
		return err
	}

	// Log results
	if instanceResult.Probability > 0 {
		log.Printf("Cheat detected on instance %d - probability: %f, playerFlags: %d", instanceId, instanceResult.Probability, len(playerResults))
	} else {
		log.Printf("No cheat detected on instance %d", instanceId)
	}

	return nil
}
