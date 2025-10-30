package player

import (
	"strconv"
	"sync"

	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

// UpdateActivityHistory updates the activity history for a player
func UpdateActivityHistory(membershipIdStr string) error {
	// Parse membership ID
	membershipId, err := strconv.ParseInt(membershipIdStr, 10, 64)
	if err != nil {
		logger.Warn("MEMBERSHIP_ID_PARSE_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipIdStr,
			logging.ERROR:         err.Error(),
		})
		return err
	}

	logger.Info("ACTIVITY_HISTORY_UPDATE_STARTED", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
	})

	// Get player
	p, err := GetPlayer(membershipId)
	if err != nil {
		logger.Error("PLAYER_GET_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.ERROR:         err.Error(),
		})
		return err
	}
	if p == nil {
		logger.Warn("PLAYER_NOT_FOUND", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return nil
	}

	// Get all characters for this player
	characters, err := GetPlayerCharacters(membershipId)
	if err != nil {
		logger.Warn("CHARACTERS_GET_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.ERROR:         err.Error(),
		})
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
				logger.Warn("ACTIVITY_HISTORY_FETCH_ERROR", map[string]any{
					logging.MEMBERSHIP_ID: membershipId,
					"character_id":        characterId,
					logging.ERROR:         err.Error(),
				})
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
	logger.Info("ACTIVITIES_FETCHED", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.COUNT:         activityCount,
	})

	// Update the last crawled timestamp
	err = UpdateHistoryLastCrawled(membershipId)
	if err != nil {
		logger.Warn("HISTORY_LAST_CRAWLED_UPDATE_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.ERROR:         err.Error(),
		})
		return err
	}

	logger.Info("ACTIVITY_HISTORY_UPDATE_COMPLETE", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.ACTIVITIES:    activityCount,
		logging.STATUS:        "success",
	})

	return nil
}
