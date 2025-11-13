package player

import (
	"context"
	"fmt"
	"time"

	"raidhub/lib/messaging/processing"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

// Crawl fetches and processes player data
func Crawl(ctx context.Context, membershipId int64) error {
	// Get player from database
	p, err := GetPlayer(membershipId)
	if err != nil {
		logger.Warn("PLAYER_GET_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}

	// If player doesn't exist or needs update, fetch from Bungie API
	if p != nil && !(needsUpdate(*p)) {
		return nil
	}
	logger.Debug("PLAYER_CRAWL_START", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		"is_new_player":       p == nil,
	})
	var result bungie.BungieHttpResult[bungie.DestinyProfileResponse]

	var knownType *int
	if p != nil && p.MembershipType != nil {
		knownType = p.MembershipType
	}
	_, result, err = bungie.ResolveProfile(membershipId, knownType)

	if err != nil {
		logger.Warn("BUNGIE_PROFILE_FETCH_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}
	if !result.Success || result.Data == nil {
		logger.Warn("NO_PROFILE_DATA", nil, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.REASON:        "bungie_api_empty_response",
		})
		if result.BungieErrorCode != bungie.Success {
			return fmt.Errorf("bungie error: %s [%d]", result.BungieErrorStatus, result.BungieErrorCode)
		}
		return fmt.Errorf("bungie api returned unsuccessful response")
	}
	profile := result.Data

	// Extract user info from profile
	if profile.Profile.Data == nil {
		logger.Warn("NO_PROFILE_DATA", nil, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.REASON:        "empty_profile_data",
		})
		return nil
	}

	userInfo := profile.Profile.Data.UserInfo
	now := time.Now()

	// Find icon path from most recently played character
	var iconPath *string = nil
	var mostRecentDate time.Time = time.Time{}

	if profile.Characters.Data != nil {
		for _, character := range *profile.Characters.Data {
			if iconPath == nil || character.DateLastPlayed.After(mostRecentDate) {
				icon := character.EmblemPath
				iconPath = &icon
				mostRecentDate = character.DateLastPlayed
			}
		}
	}

	// If no character icon found, fall back to user icon
	if iconPath == nil && userInfo.IconPath != nil {
		iconPath = userInfo.IconPath
	}

	// Determine first_seen from oldest activity and check privacy (default to now for new players)
	firstSeen, isPrivate, err := getFirstSeenAndPrivacy(userInfo.MembershipType, membershipId, profile.Characters.Data, now)
	if err != nil {
		// Return error so worker can retry (transient) or mark as unretryable
		return err
	}

	// Create or update player
	newPlayer := &Player{
		MembershipId:                userInfo.MembershipId,
		MembershipType:              &userInfo.MembershipType,
		DisplayName:                 userInfo.DisplayName,
		IconPath:                    iconPath,
		BungieGlobalDisplayName:     userInfo.BungieGlobalDisplayName,
		BungieGlobalDisplayNameCode: bungie.FixBungieGlobalDisplayNameCode(userInfo.BungieGlobalDisplayNameCode),
		LastSeen:                    now,
		FirstSeen:                   firstSeen, // SQL will preserve existing first_seen via LEAST() for updates
	}

	savedPlayer, wasUpdated, err := CreateOrUpdatePlayer(newPlayer)
	if err != nil {
		logger.Warn("PLAYER_UPSERT_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}
	fields := map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		"membership_type":     *savedPlayer.MembershipType,
		"last_seen":           savedPlayer.LastSeen.Format(time.RFC3339),
		"first_seen":          savedPlayer.FirstSeen.Format(time.RFC3339),
	}
	if savedPlayer.BungieGlobalDisplayName != nil && savedPlayer.BungieGlobalDisplayNameCode != nil {
		fields["bungie_name"] = fmt.Sprintf("%s#%s", *savedPlayer.BungieGlobalDisplayName, *savedPlayer.BungieGlobalDisplayNameCode)
	}
	if savedPlayer.DisplayName != nil {
		fields["display_name"] = *savedPlayer.DisplayName
	}

	if wasUpdated {
		logger.Info("PLAYER_UPDATED", fields)
	} else {
		logger.Info("PLAYER_CREATED", fields)
	}

	// Update privacy status if it changed
	if profile.Characters.Data != nil && len(*profile.Characters.Data) > 0 {
		checkAndUpdatePrivacy(membershipId, isPrivate)
	}

	return nil
}

// getFirstSeenAndPrivacy fetches activity history once and determines both:
// - first_seen time from the oldest activity
// - privacy status from the API response
// Returns error if the request failed and should be retried (transient) or failed permanently (unretryable)
func getFirstSeenAndPrivacy(membershipType int, membershipId int64, charactersData *map[int64]bungie.DestinyCharacterComponent, defaultTime time.Time) (time.Time, bool, error) {
	if charactersData == nil || len(*charactersData) == 0 {
		return defaultTime, false, nil
	}

	// Get first character ID
	var firstCharacterId int64
	for charId := range *charactersData {
		firstCharacterId = charId
		break
	}

	// Fetch activities to determine first_seen and check privacy (single API call)
	historyResult, err := bungie.Client.GetActivityHistoryPage(membershipType, membershipId, firstCharacterId, 250, 0, bungie.ModeStory)
	// Check privacy from API response
	if historyResult.BungieErrorCode == bungie.DestinyPrivacyRestriction {
		// Privacy restriction error - history is private (not an error, just private)
		return defaultTime, true, nil
	} else if historyResult.Success {
		// Determine first_seen from oldest activity
		if historyResult.Data != nil && len(historyResult.Data.Activities) > 0 {
			activities := historyResult.Data.Activities
			// Activities are ordered newest first, so the last one is the oldest
			oldestActivity := activities[len(activities)-1]
			firstSeen, parseErr := time.Parse(time.RFC3339, oldestActivity.Period)
			if parseErr != nil {
				return defaultTime, false, parseErr
			}
			return firstSeen, false, nil
		}
		// No activities found, but call was successful
		return defaultTime, false, nil
	} else if err != nil {
		logFields := map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		}

		// Check if this is a BungieError
		if !bungie.IsTransientError(historyResult.BungieErrorCode, historyResult.HttpStatusCode) {
			logger.Error("ACTIVITY_HISTORY_FETCH_ERROR", err, logFields)
			return defaultTime, false, processing.NewUnretryableError(err)
		}

		// All other errors are transient by default - log as warning
		logger.Warn("ACTIVITY_HISTORY_FETCH_FAILED", err, logFields)
		return defaultTime, false, err
	} else {
		// Unsuccessful request, but err is nil: treat as unretryable error and log
		logFields := map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			"bungie_error_code":   historyResult.BungieErrorCode,
			"http_status_code":    historyResult.HttpStatusCode,
		}
		errMsg := fmt.Errorf("activity history fetch failed: BungieErrorCode=%d, HttpStatusCode=%d", historyResult.BungieErrorCode, historyResult.HttpStatusCode)
		logger.Error("ACTIVITY_HISTORY_FETCH_ERROR", errMsg, logFields)
		return defaultTime, false, processing.NewUnretryableError(errMsg)
	}
}

// needsUpdate checks if player data needs to be refreshed
func needsUpdate(p Player) bool {
	// Update if never crawled or last crawled more than a day ago
	needs := p.HistoryLastCrawled.IsZero() ||
		time.Since(p.HistoryLastCrawled) > 24*time.Hour
	if needs {
		logger.Debug("PLAYER_NEEDS_UPDATE", map[string]any{
			logging.MEMBERSHIP_ID:  p.MembershipId,
			"history_last_crawled": p.HistoryLastCrawled,
		})
	}
	return needs
}
