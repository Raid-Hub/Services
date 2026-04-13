package clans

import (
	"context"
	"fmt"
	"strconv"
	"time"

	rdb "raidhub/lib/database/redis"
	"raidhub/lib/web/bungie"

	"github.com/redis/go-redis/v9"
)

const playerClanCacheTTL = 6 * time.Hour

// playerClanCacheNone is stored when a player has no active clan so we don't re-crawl on every instance.
const playerClanCacheNone = "none"

func playerClanCacheKey(membershipId int64) string {
	return fmt.Sprintf("clan:player:%d", membershipId)
}

func setPlayerClanCache(ctx context.Context, membershipId int64, groupId *int64) {
	cacheVal := playerClanCacheNone
	if groupId != nil {
		cacheVal = strconv.FormatInt(*groupId, 10)
	}
	key := playerClanCacheKey(membershipId)
	if err := rdb.Client.Set(ctx, key, cacheVal, playerClanCacheTTL).Err(); err != nil {
		logger.Warn("CLAN_CACHE_REDIS_SET_FAILED", err, map[string]any{"key": key})
	}
}

// WarmPlayerClanCacheFromGroupMembers paginates GetMembersOfGroup and writes clan:player:{membershipId}
// -> groupId for every Destiny member. Intended for clan_crawl after GetGroup succeeds.
func WarmPlayerClanCacheFromGroupMembers(ctx context.Context, groupId int64) error {
	page := 1
	for {
		result, err := bungie.Client.GetMembersOfGroup(ctx, groupId, page)
		if err != nil {
			return fmt.Errorf("GetMembersOfGroup(%d, page %d): %w", groupId, page, err)
		}
		if result.Data == nil {
			return fmt.Errorf("GetMembersOfGroup(%d, page %d): nil response", groupId, page)
		}
		gid := groupId
		for _, m := range result.Data.Results {
			if m.DestinyUserInfo.MembershipId == 0 {
				continue
			}
			setPlayerClanCache(ctx, m.DestinyUserInfo.MembershipId, &gid)
		}
		if !result.Data.HasMore || len(result.Data.Results) == 0 {
			break
		}
		page++
	}
	return nil
}

// ResolveClan returns the player's active clan group_id (or nil if no clan).
// It checks Redis first; on miss it calls Bungie GetGroupsForMember and writes through.
func ResolveClan(ctx context.Context, membershipType int, membershipId int64) (groupId *int64, fromCache bool, err error) {
	key := playerClanCacheKey(membershipId)

	val, err := rdb.Client.Get(ctx, key).Result()
	if err == nil {
		if val == playerClanCacheNone {
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

	// One row per group this user has joined; Member is that user in that group. Warm Redis for
	// each distinct membershipId with the first active group for that id (same ordering as before
	// for the queried player).
	var activeGroupId *int64
	seenMid := make(map[int64]bool)
	for _, gm := range result.Data.Results {
		gid := gm.Group.GroupId
		if result.Data.AreAllMembershipsInactive[gid] {
			continue
		}
		mid := gm.Member.DestinyUserInfo.MembershipId
		if mid == 0 || seenMid[mid] {
			continue
		}
		seenMid[mid] = true
		g := gid
		setPlayerClanCache(ctx, mid, &g)
		if mid == membershipId {
			activeGroupId = &g
		}
	}
	if !seenMid[membershipId] {
		setPlayerClanCache(ctx, membershipId, nil)
	}

	return activeGroupId, false, nil
}
