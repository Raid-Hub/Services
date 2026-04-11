package subscriptions

import (
	"context"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/clan"
	"raidhub/lib/services/player"
)

// MatchEvent is stage 2 of the subscription pipeline (see README.md). Order of operations:
//  1. applySubscriptionRules — privacy, clan lookup, rules → one row per matched destination
//  2. enrichDeliveryRaidContext — instance-wide raid fields on each row
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

// applySubscriptionRules resolves privacy, clan membership, active rules, and produces one
// SubscriptionDeliveryMessage per destination that matched (no webhook or embed preload yet).
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
	clansByMember, err := clan.GroupIDsByMembershipIDs(ctx, membershipIDs)
	if err != nil {
		return nil, err
	}

	clanGroupIDs := uniqueClanGroupIDs(clansByMember, membershipIDs)
	rules, err := loadSubscriptionRulesForMatch(ctx, membershipIDs, clanGroupIDs, message.ActivityHash)
	if err != nil {
		return nil, err
	}

	return matchRulesToDeliveries(message.InstanceId, message.ParticipantData, rules, privacy, clansByMember)
}

func enrichDeliveryRaidContext(d *messages.SubscriptionDeliveryMessage, msg messages.SubscriptionMatchMessage) {
	d.ActivityHash = msg.ActivityHash
	d.DateCompleted = msg.DateCompleted
	d.DurationSeconds = msg.DurationSeconds
	d.Completed = msg.Completed
	d.PlayerCount = msg.PlayerCount
	d.FireteamMembershipIds = fireteamMembershipIDs(msg.ParticipantData)
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
