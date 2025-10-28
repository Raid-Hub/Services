package player

import (
	"context"
	"time"

	"raidhub/lib/web/bungie"
)

// Crawl fetches and processes player data
func Crawl(ctx context.Context, membershipId int64) error {
	PlayerLogger.Info("Starting player crawl", "membershipId", membershipId)

	// Get player from database
	p, err := GetPlayer(membershipId)
	if err != nil {
		PlayerLogger.Error("Error getting player", "membershipId", membershipId, "error", err)
		return err
	}

	// If player doesn't exist or needs update, fetch from Bungie API
	if p == nil || needsUpdate(*p) {
		// Fetch profile from Bungie API (membership type 2 is Xbox)
		// Component 100 is for basic profile info
		result, _, err := bungie.Client.GetProfile(2, membershipId, []int{100})
		if err != nil {
			PlayerLogger.Error("Error fetching profile from Bungie", "membershipId", membershipId, "error", err)
			return err
		}
		if result == nil || !result.Success || result.Data == nil {
			PlayerLogger.Error("No profile data returned from Bungie", "membershipId", membershipId)
			return err
		}
		profile := result.Data

		// Extract user info from profile
		if profile.Profile.Data == nil {
			PlayerLogger.Warn("No profile data returned", "membershipId", membershipId)
			return nil
		}

		userInfo := profile.Profile.Data.UserInfo

		// Create or update player,
		newPlayer := &Player{
			MembershipId: userInfo.MembershipId,
			DisplayName:  userInfo.DisplayName,
		}

		if err := CreateOrUpdatePlayer(newPlayer); err != nil {
			PlayerLogger.Error("Error creating/updating player", "membershipId", membershipId, "error", err)
			return err
		}
	}

	PlayerLogger.Info("Successfully crawled player", "membershipId", membershipId)
	return nil
}

// needsUpdate checks if player data needs to be refreshed
func needsUpdate(p Player) bool {
	// Update if never crawled or last crawled more than a day ago
	return p.HistoryLastCrawled.IsZero() ||
		time.Since(p.HistoryLastCrawled) > 24*time.Hour
}
