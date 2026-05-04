package authdb

import (
	"context"
	"fmt"
	"time"

	rdb "raidhub/lib/database/redis"

	"github.com/redis/go-redis/v9"
)

// Redis key layout (shared across Hermes replicas). Bump authTursoCacheVersion if value shape changes.
// We intentionally do NOT cache Discord OAuth rows: access_token must not live in Redis (NFR-03).
const (
	authTursoCacheVersion = "v1"
	// destiny_profile mapping changes rarely once linked.
	authTursoD2BHitTTL  = 24 * time.Hour
	authTursoD2BMissTTL = 5 * time.Minute
)

func authTursoRedisPrefix() string {
	return "authdb:" + authTursoCacheVersion
}

func redisD2BKey(destinyMembershipID string) string {
	return fmt.Sprintf("%s:d2b:%s", authTursoRedisPrefix(), destinyMembershipID)
}

func redisDiscordByBungieKey(bungieMembershipID string) string {
	return fmt.Sprintf("%s:discord:%s", authTursoRedisPrefix(), bungieMembershipID)
}

func redisBungieDestiniesKey(bungieMembershipID string) string {
	return fmt.Sprintf("%s:bungie_destinies:%s", authTursoRedisPrefix(), bungieMembershipID)
}

func redisAuthCacheClient() *redis.Client {
	if rdb.Client == nil {
		return nil
	}
	return rdb.Client
}

// LookupBungieByDestinyMembershipID returns the Bungie membership id for a Destiny profile row, or "" if unknown.
// Uses Redis read-through when Redis is available.
func LookupBungieByDestinyMembershipID(ctx context.Context, destinyMembershipID string) (string, error) {
	if destinyMembershipID == "" {
		return "", nil
	}
	if c := redisAuthCacheClient(); c != nil {
		if v, ok, err := redisGetD2B(ctx, c, destinyMembershipID); err == nil && ok {
			return v, nil
		} else if err != nil {
			// Treat Redis errors as cache miss.
		}
	}
	out, err := lookupBungieByDestinyMembershipIDFromDB(ctx, destinyMembershipID)
	if err != nil {
		return "", err
	}
	if c := redisAuthCacheClient(); c != nil {
		_ = redisSetD2B(ctx, c, destinyMembershipID, out)
	}
	return out, nil
}

// LookupDiscordByBungieMembershipID returns the Discord OAuth row for a Bungie user, if linked.
// Always reads Turso for tokens (never Redis) so bearer tokens are not stored in Redis.
func LookupDiscordByBungieMembershipID(ctx context.Context, bungieMembershipID string) (*DiscordAccountRow, error) {
	if bungieMembershipID == "" {
		return nil, nil
	}
	return lookupDiscordByBungieMembershipIDFromDB(ctx, bungieMembershipID)
}

func redisGetD2B(ctx context.Context, c *redis.Client, destiny string) (value string, hit bool, err error) {
	s, e := c.Get(ctx, redisD2BKey(destiny)).Result()
	if e == redis.Nil {
		return "", false, nil
	}
	if e != nil {
		return "", false, e
	}
	return s, true, nil
}

func redisSetD2B(ctx context.Context, c *redis.Client, destiny, bungie string) error {
	k := redisD2BKey(destiny)
	ttl := authTursoD2BHitTTL
	if bungie == "" {
		ttl = authTursoD2BMissTTL
	}
	if err := c.Set(ctx, k, bungie, ttl).Err(); err != nil {
		return err
	}
	if bungie != "" {
		if err := c.SAdd(ctx, redisBungieDestiniesKey(bungie), destiny).Err(); err != nil {
			return err
		}
		_ = c.Expire(ctx, redisBungieDestiniesKey(bungie), authTursoD2BHitTTL).Err()
	}
	return nil
}

// PurgeTursoAuthLookupCacheForDestiny drops the cached destiny→Bungie mapping for one profile (and index entry when known).
func PurgeTursoAuthLookupCacheForDestiny(ctx context.Context, destinyMembershipID string) error {
	c := redisAuthCacheClient()
	if c == nil || destinyMembershipID == "" {
		return nil
	}
	k := redisD2BKey(destinyMembershipID)
	bungie, err := c.Get(ctx, k).Result()
	if err != nil && err != redis.Nil {
		_ = c.Del(ctx, k).Err()
		return nil
	}
	pipe := c.Pipeline()
	pipe.Del(ctx, k)
	if err == nil && bungie != "" {
		pipe.SRem(ctx, redisBungieDestiniesKey(bungie), destinyMembershipID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// PurgeTursoAuthLookupCacheForDiscordByBungie drops any legacy cached Discord OAuth key for a Bungie user (tokens are no longer cached).
func PurgeTursoAuthLookupCacheForDiscordByBungie(ctx context.Context, bungieMembershipID string) error {
	c := redisAuthCacheClient()
	if c == nil || bungieMembershipID == "" {
		return nil
	}
	return c.Del(ctx, redisDiscordByBungieKey(bungieMembershipID)).Err()
}

// PurgeTursoAuthLookupCacheForBungie drops all cached Turso auth lookups tied to a Bungie user (destiny→bungie rows indexed for them plus legacy Discord key).
func PurgeTursoAuthLookupCacheForBungie(ctx context.Context, bungieMembershipID string) error {
	c := redisAuthCacheClient()
	if c == nil || bungieMembershipID == "" {
		return nil
	}
	setKey := redisBungieDestiniesKey(bungieMembershipID)
	dests, err := c.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}
	pipe := c.Pipeline()
	for _, d := range dests {
		if d != "" {
			pipe.Del(ctx, redisD2BKey(d))
		}
	}
	pipe.Del(ctx, setKey)
	pipe.Del(ctx, redisDiscordByBungieKey(bungieMembershipID))
	_, err = pipe.Exec(ctx)
	return err
}
