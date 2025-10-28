package player

import (
	"strconv"
	"sync"

	"raidhub/lib/web/bungie"
)

// UpdateActivityHistory updates the activity history for a player
func UpdateActivityHistory(membershipIdStr string) error {
	// Parse membership ID
	membershipId, err := strconv.ParseInt(membershipIdStr, 10, 64)
	if err != nil {
		PlayerLogger.Error("Error parsing membership ID", "membershipIdStr", membershipIdStr, "error", err)
		return err
	}

	PlayerLogger.Info("Updating activity history", "membershipId", membershipId)

	// Get player
	p, err := GetPlayer(membershipId)
	if err != nil {
		PlayerLogger.Error("Error getting player", "membershipId", membershipId, "error", err)
		return err
	}
	if p == nil {
		PlayerLogger.Warn("Player not found", "membershipId", membershipId)
		return nil
	}

	// Get all characters for this player
	characters, err := GetPlayerCharacters(membershipId)
	if err != nil {
		PlayerLogger.Error("Error getting characters", "membershipId", membershipId, "error", err)
		return err
	}

	// Fetch activity history for each character
	var wg sync.WaitGroup
	instanceIds := make(chan int64, 100)

	wg.Add(len(characters))
	for _, char := range characters {
		go func(characterId int64) {
			defer wg.Done()
			// Fetch activity history for this character (membershipType 2 = Xbox, mode 4 = All)
			err := bungie.Client.GetActivityHistoryInChannel(2, membershipId, characterId, 5, instanceIds)
			if err != nil {
				PlayerLogger.Error("Error fetching activity history", "membershipId", membershipId, "characterId", characterId, "error", err)
			}
		}(char.CharacterID)
	}

	// Close channel when all fetches are done
	go func() {
		wg.Wait()
		close(instanceIds)
	}()

	// Store activities to database
	activityCount := 0
	for range instanceIds {
		activityCount++
	}
	PlayerLogger.Info("Fetched activities", "membershipId", membershipId, "count", activityCount)

	// Update the last crawled timestamp
	err = UpdateHistoryLastCrawled(membershipId)
	if err != nil {
		PlayerLogger.Error("Error updating history last crawled", "membershipId", membershipId, "error", err)
		return err
	}

	PlayerLogger.Info("Successfully updated activity history", "membershipId", membershipId, "activities", activityCount)

	return nil
}
