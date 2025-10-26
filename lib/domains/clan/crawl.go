package clan

import (
	"raidhub/lib/utils"
	"raidhub/lib/web/bungie"
)

var ClanLogger utils.Logger

func init() {
	ClanLogger = utils.NewLogger("clan")
}

// Crawl fetches and processes clan data
func Crawl(groupId int64) error {
	ClanLogger.Info("Crawling clan", "groupId", groupId)

	// Get clan from Bungie API
	_, _, err := bungie.Client.GetGroup(groupId)
	if err != nil {
		ClanLogger.Error("Error fetching clan", "groupId", groupId, "error", err)
		return err
	}

	ClanLogger.Info("Successfully fetched clan", "groupId", groupId)

	// Note: Clan storage typically includes members and requires additional API calls
	// This worker exists for basic clan information storage
	// Full implementation would include member crawling and storage
	// See apps/hera/main.go for a complete implementation example

	return nil
}
