package player

import (
	"context"
	"sync"

	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

// UpdateActivityHistory updates the activity history for a player
func UpdateActivityHistory(ctx context.Context, membershipId int64) error {
	// Parse membership ID
	logger.Debug("ACTIVITY_HISTORY_UPDATE_STARTED", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
	})

	// Get player to check current privacy status
	p, err := GetPlayer(membershipId)
	if err != nil {
		logger.Error("PLAYER_GET_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}
	if p == nil {
		logger.Warn("PLAYER_NOT_FOUND", nil, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return nil
	}

	// Get all characters for this player
	characters, err := GetPlayerCharacters(ctx, membershipId)
	if err != nil {
		logger.Warn("CHARACTERS_GET_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}

	// Fetch activity history for each character concurrently
	var wg sync.WaitGroup
	instanceIds := make(chan int64, 100)
	hasPrivacyError := false
	var privacyErrorMu sync.Mutex

	wg.Add(len(characters))
	for _, char := range characters {
		go func(characterId int64) {
			defer wg.Done()
			// Fetch activity history for this character
			result := bungie.Client.GetActivityHistoryInChannel(ctx, 2, membershipId, characterId, 5, instanceIds)

			if result.Error != nil {
				logger.Warn("ACTIVITY_HISTORY_FETCH_ERROR", result.Error, map[string]any{
					logging.MEMBERSHIP_ID: membershipId,
					logging.CHARACTER_ID:  characterId,
				})
			}

			// Check for privacy restriction
			if result.PrivacyErrorCode == bungie.DestinyPrivacyRestriction {
				privacyErrorMu.Lock()
				hasPrivacyError = true
				privacyErrorMu.Unlock()
			}
		}(char.CharacterID)
	}

	// Close instanceIds channel when all fetches are done
	go func() {
		wg.Wait()
		close(instanceIds)
	}()

	// Collect instance IDs
	activityCount := 0
	for range instanceIds {
		activityCount++
	}
	logger.Info("ACTIVITIES_FETCHED", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.COUNT:         activityCount,
	})

	// Update privacy status if it changed
	checkAndUpdatePrivacy(membershipId, hasPrivacyError)

	// Update the last crawled timestamp
	err = UpdateHistoryLastCrawled(membershipId)
	if err != nil {
		logger.Warn("HISTORY_LAST_CRAWLED_UPDATE_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
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
