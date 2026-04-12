package subscriptions

import (
	"context"
	"fmt"
	"strconv"
	"time"

	rdb "raidhub/lib/database/redis"
	"raidhub/lib/web/bungie"

	"github.com/redis/go-redis/v9"
)

const clanCacheTTL = 6 * time.Hour

// "none" is stored when a player has no active clan so we don't re-crawl on every instance.
const clanCacheNone = "none"

func clanCacheKey(membershipId int64) string {
	return fmt.Sprintf("clan:player:%d", membershipId)
}

// ResolveClan returns the player's active clan group_id (or nil if no clan).
// It checks Redis first; on miss it calls Bungie GetGroupsForMember and writes through.
func ResolveClan(ctx context.Context, membershipType int, membershipId int64) (groupId *int64, fromCache bool, err error) {
	key := clanCacheKey(membershipId)

	val, err := rdb.Client.Get(ctx, key).Result()
	if err == nil {
		if val == clanCacheNone {
			return nil, true, nil
		}
		gid, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			return &gid, true, nil
		}
	} else if err != redis.Nil {
		return nil, false, fmt.Errorf("redis GET %s: %w", key, err)
	}

	result, err := bungie.Client.GetGroupsForMember(ctx, membershipType, membershipId)
	if err != nil {
		return nil, false, fmt.Errorf("GetGroupsForMember(%d, %d): %w", membershipType, membershipId, err)
	}
	if result.Data == nil {
		return nil, false, fmt.Errorf("GetGroupsForMember(%d, %d): nil response", membershipType, membershipId)
	}

	var activeGroupId *int64
	for _, gm := range result.Data.Results {
		gid := gm.Group.GroupId
		if !result.Data.AreAllMembershipsInactive[gid] {
			activeGroupId = &gid
			break
		}
	}

	cacheVal := clanCacheNone
	if activeGroupId != nil {
		cacheVal = strconv.FormatInt(*activeGroupId, 10)
	}
	if err := rdb.Client.Set(ctx, key, cacheVal, clanCacheTTL).Err(); err != nil {
		logger.Warn("CLAN_CACHE_REDIS_SET_FAILED", err, map[string]any{"key": key})
	}

	return activeGroupId, false, nil
}
