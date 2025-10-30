package player

import (
	"context"
	"fmt"
	"time"

	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

// Crawl fetches and processes player data
func Crawl(ctx context.Context, membershipId int64) error {
	// Get player from database
	p, err := GetPlayer(membershipId)
	if err != nil {
		logger.Warn("PLAYER_GET_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.ERROR:         err.Error(),
		})
		return err
	}

	// If player doesn't exist or needs update, fetch from Bungie API
	if p == nil || needsUpdate(*p) {
		var result bungie.BungieHttpResult[bungie.DestinyProfileResponse]

		var knownType *int
		if p != nil && p.MembershipType != nil {
			knownType = p.MembershipType
		}
		_, result, err = bungie.ResolveMembershipType(membershipId, knownType)

		if err != nil {
			logger.Warn("BUNGIE_PROFILE_FETCH_ERROR", map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.ERROR:         err.Error(),
			})
			return err
		}
		if !result.Success || result.Data == nil {
			logger.Warn("NO_PROFILE_DATA", map[string]any{
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
			logger.Warn("NO_PROFILE_DATA", map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.REASON:        "empty_profile_data",
			})
			return nil
		}

		userInfo := profile.Profile.Data.UserInfo
		now := time.Now()

		// Create or update player
		newPlayer := &Player{
			MembershipId:                userInfo.MembershipId,
			MembershipType:              &userInfo.MembershipType,
			DisplayName:                 userInfo.DisplayName,
			IconPath:                    userInfo.IconPath,
			BungieGlobalDisplayName:     userInfo.BungieGlobalDisplayName,
			BungieGlobalDisplayNameCode: bungie.FixBungieGlobalDisplayNameCode(userInfo.BungieGlobalDisplayNameCode),
			LastSeen:                    now,
			FirstSeen:                   now, // SQL will preserve existing first_seen via LEAST() for updates
		}

		if err := CreateOrUpdatePlayer(newPlayer); err != nil {
			logger.Warn("PLAYER_UPSERT_ERROR", map[string]any{
				logging.MEMBERSHIP_ID: membershipId,
				logging.ERROR:         err.Error(),
			})
			return err
		}
	}

	return nil
}

// needsUpdate checks if player data needs to be refreshed
func needsUpdate(p Player) bool {
	// Update if never crawled or last crawled more than a day ago
	return p.HistoryLastCrawled.IsZero() ||
		time.Since(p.HistoryLastCrawled) > 24*time.Hour
}
