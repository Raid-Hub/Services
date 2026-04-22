package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/messages"

	"github.com/lib/pq"
)

// destinationCacheTTL bounds staleness for webhook URLs / channel type (admin updates).
const destinationCacheTTL = 3 * time.Minute

type destinationCacheEntry struct {
	row destinationRow
	exp time.Time
}

var (
	destinationCacheMu sync.Mutex
	destinationCache   = make(map[int64]destinationCacheEntry) // id -> row

	activityRaidMetaMu    sync.RWMutex
	activityRaidMetaCache = make(map[uint32]activityRaidMetaCacheEntry)
)

// activityRaidMetaCacheEntry caches definition-backed display metadata per activity hash (same process lifetime as raid_bitmap).
type activityRaidMetaCacheEntry struct {
	meta   *ActivityRaidMeta
	noRows bool
}

type subscriptionRule struct {
	ID                 int64
	DestinationID      int64
	Scope              string
	MembershipID       sql.NullInt64
	GroupID            sql.NullInt64
	ChannelType        string
	RequireFresh       bool
	RequireCompleted   bool
	ActivityRaidBitmap uint64
}

// loadSubscriptionRulesForMatch loads only rules that could apply to this instance:
// player-scope rows for these membership ids and clan-scope rows for these group ids.
// Uses partial unique indexes (membership_id, destination_id) / (group_id, destination_id) for player/clan lookups.
func loadSubscriptionRulesForMatch(ctx context.Context, playerMembershipIDs, clanGroupIDs []int64) ([]subscriptionRule, error) {
	if len(playerMembershipIDs) == 0 && len(clanGroupIDs) == 0 {
		return nil, nil
	}
	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT r.id, r.destination_id, r.scope, r.membership_id, r.group_id,
		       d.channel_type, r.require_fresh, r.require_completed, r.activity_raid_bitmap
		FROM subscriptions.rule r
		INNER JOIN subscriptions.destination d ON d.id = r.destination_id AND d.is_active
		WHERE r.is_active
		  AND (
		    (r.scope = 'player' AND r.membership_id = ANY($1))
		    OR (r.scope = 'clan' AND r.group_id = ANY($2))
		  )`,
		pq.Array(playerMembershipIDs),
		pq.Array(clanGroupIDs),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []subscriptionRule
	for rows.Next() {
		var r subscriptionRule
		if err := rows.Scan(&r.ID, &r.DestinationID, &r.Scope, &r.MembershipID, &r.GroupID,
			&r.ChannelType, &r.RequireFresh, &r.RequireCompleted, &r.ActivityRaidBitmap); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ruleMatchesInstanceCriteria enforces subscriptions.rule require_* and activity_raid_bitmap (AND semantics).
func ruleMatchesInstanceCriteria(ctx context.Context, msg messages.SubscriptionMatchMessage, rule subscriptionRule) (bool, error) {
	if rule.RequireCompleted && !msg.Completed {
		return false, nil
	}
	if rule.RequireFresh {
		if msg.Fresh == nil || !*msg.Fresh {
			return false, nil
		}
	}
	return ruleMatchesRaidBitmap(ctx, msg.ActivityHash, rule.ActivityRaidBitmap)
}

// uniqueClanGroupIDs returns distinct group ids among the given participants' clan memberships.
func uniqueClanGroupIDs(clansByMember map[int64][]int64, participantMembershipIDs []int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	for _, mid := range participantMembershipIDs {
		for _, gid := range clansByMember[mid] {
			if _, ok := seen[gid]; !ok {
				seen[gid] = struct{}{}
				out = append(out, gid)
			}
		}
	}
	return out
}

type destinationRow struct {
	WebhookURL  string
	ChannelType string
}

// loadActiveDestinationsByIDs returns active destinations keyed by id. Missing ids are omitted.
func loadActiveDestinationsByIDs(ctx context.Context, destinationIDs []int64) (map[int64]destinationRow, error) {
	if len(destinationIDs) == 0 {
		return map[int64]destinationRow{}, nil
	}
	now := time.Now()
	out := make(map[int64]destinationRow, len(destinationIDs))
	var missing []int64
	destinationCacheMu.Lock()
	for _, id := range destinationIDs {
		e, ok := destinationCache[id]
		if ok && now.Before(e.exp) {
			out[id] = e.row
			continue
		}
		missing = append(missing, id)
	}
	destinationCacheMu.Unlock()

	if len(missing) == 0 {
		return out, nil
	}

	rows, err := postgres.DB.QueryContext(ctx, `
		SELECT
			d.id,
			CASE
				WHEN d.channel_type = 'discord_webhook' THEN
					'https://discord.com/api/webhooks/' || c.webhook_id || '/' || c.webhook_token
				WHEN d.channel_type = 'http_callback' THEN h.callback_url
				ELSE NULL
			END AS webhook_url,
			d.channel_type
		FROM subscriptions.destination d
		LEFT JOIN subscriptions.discord_destination_config c ON c.destination_id = d.id
		LEFT JOIN subscriptions.http_callback_destination_config h ON h.destination_id = d.id
		WHERE d.id = ANY($1) AND d.is_active`,
		pq.Array(missing),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exp := now.Add(destinationCacheTTL)
	destinationCacheMu.Lock()
	defer destinationCacheMu.Unlock()
	for rows.Next() {
		var id int64
		var webhookURL sql.NullString
		var d destinationRow
		if err := rows.Scan(&id, &webhookURL, &d.ChannelType); err != nil {
			return nil, err
		}
		d.WebhookURL = webhookURL.String
		out[id] = d
		destinationCache[id] = destinationCacheEntry{row: d, exp: exp}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func matchRulesToDeliveries(
	ctx context.Context,
	msg messages.SubscriptionMatchMessage,
	participants []messages.ParticipantResult,
	rules []subscriptionRule,
	privacy map[int64]bool,
	clansByMember map[int64][]int64,
) ([]messages.SubscriptionDeliveryMessage, error) {
	instanceID := msg.InstanceId
	eligible := make([]messages.ParticipantResult, 0, len(participants))
	for _, p := range participants {
		if p.Status == messages.ParticipantPlayerUnresolved {
			continue
		}
		eligible = append(eligible, p)
	}

	memberSet := make(map[int64]struct{}, len(eligible))
	for _, p := range eligible {
		memberSet[p.MembershipId] = struct{}{}
	}

	groupSet := make(map[int64]struct{})
	for _, p := range eligible {
		for _, g := range clansByMember[p.MembershipId] {
			groupSet[g] = struct{}{}
		}
	}

	type agg struct {
		channelType string
		players     map[int64]struct{}
		clans       map[int64]struct{}
	}
	ensureAgg := func(byDest map[int64]*agg, destID int64, channelType string) *agg {
		a := byDest[destID]
		if a != nil {
			return a
		}
		a = &agg{
			channelType: channelType,
			players:     make(map[int64]struct{}),
			clans:       make(map[int64]struct{}),
		}
		byDest[destID] = a
		return a
	}

	byDest := make(map[int64]*agg)

	for _, rule := range rules {
		ok, err := ruleMatchesInstanceCriteria(ctx, msg, rule)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		switch rule.Scope {
		case "player":
			if !rule.MembershipID.Valid {
				continue
			}
			mid := rule.MembershipID.Int64
			if _, ok := memberSet[mid]; !ok {
				continue
			}
			if privacy[mid] {
				continue
			}
			ensureAgg(byDest, rule.DestinationID, rule.ChannelType).players[mid] = struct{}{}
		case "clan":
			if !rule.GroupID.Valid {
				continue
			}
			gid := rule.GroupID.Int64
			if _, ok := groupSet[gid]; !ok {
				continue
			}
			ensureAgg(byDest, rule.DestinationID, rule.ChannelType).clans[gid] = struct{}{}
		default:
			continue
		}
	}

	out := make([]messages.SubscriptionDeliveryMessage, 0, len(byDest))
	for destID, a := range byDest {
		sum := messages.DeliveryScope{
			PlayerMembershipIds: mapKeysSorted(a.players),
			ClanGroupIds:        mapKeysSorted(a.clans),
		}
		out = append(out, messages.SubscriptionDeliveryMessage{
			InstanceId:           instanceID,
			DestinationChannelId: destID,
			ChannelType:          messages.DeliveryChannelType(a.channelType),
			DedupeKey:            fmt.Sprintf("sub:%d:%d", destID, instanceID),
			Scope:                sum,
		})
	}
	return out, nil
}

func mapKeysSorted(m map[int64]struct{}) []int64 {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// ActivityRaidMeta is display metadata for a raid / activity hash.
type ActivityRaidMeta struct {
	ActivityName string
	VersionName  string
	PathSegment  string // definitions.activity_definition.splash_path — cdn.raidhub.io/content/splash/{slug}/…
}

func loadActivityRaidMeta(ctx context.Context, activityHash uint32) (*ActivityRaidMeta, error) {
	if activityHash == 0 {
		return nil, nil
	}
	activityRaidMetaMu.RLock()
	if e, ok := activityRaidMetaCache[activityHash]; ok {
		activityRaidMetaMu.RUnlock()
		if e.noRows {
			return nil, nil
		}
		c := *e.meta
		return &c, nil
	}
	activityRaidMetaMu.RUnlock()

	activityRaidMetaMu.Lock()
	defer activityRaidMetaMu.Unlock()
	if e, ok := activityRaidMetaCache[activityHash]; ok {
		if e.noRows {
			return nil, nil
		}
		c := *e.meta
		return &c, nil
	}

	var meta ActivityRaidMeta
	err := postgres.DB.QueryRowContext(ctx, `
		SELECT ad.name, vd.name, ad.splash_path
		FROM definitions.activity_version av
		JOIN definitions.activity_definition ad ON ad.id = av.activity_id
		JOIN definitions.version_definition vd ON vd.id = av.version_id
		WHERE av.hash = $1`,
		int64(activityHash),
	).Scan(&meta.ActivityName, &meta.VersionName, &meta.PathSegment)
	if err != nil {
		if err == sql.ErrNoRows {
			activityRaidMetaCache[activityHash] = activityRaidMetaCacheEntry{noRows: true}
			return nil, nil
		}
		return nil, err
	}
	meta.PathSegment = strings.Trim(meta.PathSegment, "/")
	stored := new(ActivityRaidMeta)
	*stored = meta
	activityRaidMetaCache[activityHash] = activityRaidMetaCacheEntry{meta: stored}
	out := *stored
	return &out, nil
}
