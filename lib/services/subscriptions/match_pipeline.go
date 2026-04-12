package subscriptions

import (
	"context"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/player"
)

// MatchEvent is stage 2 of the subscription pipeline (see README.md). Order of operations:
//  1. applySubscriptionRules — privacy, clan from message, rules → one row per matched destination
//  2. enrichDeliveryRaidContext — DiscordEmbedPreload raid context for discord_webhook rows
//  3. attachDestinationWebhooks — batch-load webhook URLs (Postgres; not repeated in stage 3)
//  4. preloadDiscordEmbedData — batch-load Discord embed body (activity, players, stats, feats)
//  5. preloadHttpCallbackInstance — dto.Instance for http_callback (API-shaped JSON)
func MatchEvent(ctx context.Context, message messages.SubscriptionMatchMessage) ([]messages.SubscriptionDeliveryMessage, error) {
	deliveries, err := applySubscriptionRules(ctx, message)
	if err != nil {
		return nil, err
	}
	if len(deliveries) == 0 {
		return deliveries, nil
	}
	for i := range deliveries {
		enrichDeliveryRaidContext(&deliveries[i], message)
	}
	if err := attachDestinationWebhooks(ctx, deliveries); err != nil {
		return nil, err
	}
	if err := preloadDiscordEmbedData(ctx, deliveries); err != nil {
		return nil, err
	}
	if err := preloadHttpCallbackInstance(ctx, deliveries); err != nil {
		return nil, err
	}
	return deliveries, nil
}

// applySubscriptionRules resolves privacy, reads clan membership from the message
// (resolved by stage 1 via Redis/Bungie), loads active rules, and produces one
// SubscriptionDeliveryMessage per destination that matched.
func applySubscriptionRules(ctx context.Context, message messages.SubscriptionMatchMessage) ([]messages.SubscriptionDeliveryMessage, error) {
	membershipIDs := make([]int64, 0, len(message.ParticipantData))
	for _, p := range message.ParticipantData {
		if p.Status != messages.ParticipantPlayerUnresolved {
			membershipIDs = append(membershipIDs, p.MembershipId)
		}
	}

	privacy, err := player.PrivateFlagsByMembershipIDs(ctx, membershipIDs)
	if err != nil {
		return nil, err
	}

	clansByMember := make(map[int64][]int64, len(message.ParticipantData))
	for _, p := range message.ParticipantData {
		if p.GroupId != nil {
			clansByMember[p.MembershipId] = []int64{*p.GroupId}
		}
	}

	clanGroupIDs := uniqueClanGroupIDs(clansByMember, membershipIDs)
	rules, err := loadSubscriptionRulesForMatch(ctx, membershipIDs, clanGroupIDs)
	if err != nil {
		return nil, err
	}

	return matchRulesToDeliveries(message.InstanceId, message.ParticipantData, rules, privacy, clansByMember)
}

func enrichDeliveryRaidContext(d *messages.SubscriptionDeliveryMessage, msg messages.SubscriptionMatchMessage) {
	if d.ChannelType != messages.DeliveryChannelDiscordWebhook {
		return
	}
	d.EmbedPreload = &messages.DiscordEmbedPreload{
		ActivityHash:          msg.ActivityHash,
		DateCompleted:         msg.DateCompleted,
		DurationSeconds:       msg.DurationSeconds,
		Completed:             msg.Completed,
		PlayerCount:           msg.PlayerCount,
		FireteamMembershipIds: fireteamMembershipIDs(msg.ParticipantData),
	}
}

func fireteamMembershipIDs(participants []messages.ParticipantResult) []int64 {
	if len(participants) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(participants))
	for _, p := range participants {
		ids = append(ids, p.MembershipId)
	}
	return ids
}
