package clan

import (
	"context"
	"fmt"
	"raidhub/lib/services/clans"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/bungie"
)

// Clan service logging constants
const (
	CLAN_CRAWL_ERROR = "CLAN_CRAWL_ERROR"
)

var logger = logging.NewLogger("CLAN_SERVICE")

// Crawl fetches and processes clan data
func Crawl(ctx context.Context, groupId int64) error {
	logger.Info("CLAN_CRAWLING", map[string]any{
		logging.GROUP_ID: groupId,
	})

	result, err := bungie.Client.GetGroup(ctx, groupId)
	if err != nil {
		logger.Warn(CLAN_CRAWL_ERROR, err, map[string]any{
			logging.GROUP_ID:  groupId,
			logging.OPERATION: "fetch_clan",
		})
		return err
	}
	if result.Data == nil {
		err := fmt.Errorf("GetGroup(%d): nil response data", groupId)
		logger.Warn(CLAN_CRAWL_ERROR, err, map[string]any{
			logging.GROUP_ID:  groupId,
			logging.OPERATION: "fetch_clan",
		})
		return err
	}

	if err := clans.WarmPlayerClanCacheFromGroupMembers(ctx, groupId); err != nil {
		logger.Warn(CLAN_CRAWL_ERROR, err, map[string]any{
			logging.GROUP_ID:  groupId,
			logging.OPERATION: "warm_clan_player_cache",
		})
		return err
	}

	logger.Info("CLAN_FETCHED", map[string]any{
		logging.GROUP_ID: groupId,
		logging.STATUS:   "success",
	})

	return nil
}
