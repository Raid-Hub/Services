package subscriptions

import (
	"context"
	"fmt"
	"strings"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"
)

// attachDestinationWebhooks loads webhook URLs for all destinations in one batch so the delivery
// worker does not query Postgres.
func attachDestinationWebhooks(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	seen := make(map[int64]struct{})
	ids := make([]int64, 0, len(deliveries))
	for _, d := range deliveries {
		if _, ok := seen[d.DestinationChannelId]; ok {
			continue
		}
		seen[d.DestinationChannelId] = struct{}{}
		ids = append(ids, d.DestinationChannelId)
	}
	byID, err := loadActiveDestinationsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for i := range deliveries {
		id := deliveries[i].DestinationChannelId
		row, ok := byID[id]
		if !ok {
			return fmt.Errorf("subscription destination %d not found or inactive", id)
		}
		if strings.TrimSpace(row.WebhookURL) == "" {
			return fmt.Errorf("subscription destination %d has no webhook URL", id)
		}
		deliveries[i].WebhookURL = row.WebhookURL
	}
	return nil
}

// preloadDiscordEmbedData loads activity metadata, fireteam profiles, instance stats, and instance feats
// once per match batch (embed body only; URL is attached in attachDestinationWebhooks).
func preloadDiscordEmbedData(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	d0 := deliveries[0]

	meta, err := loadActivityRaidMeta(ctx, d0.ActivityHash)
	if err != nil {
		return err
	}
	var actName, verName, pathSeg string
	if meta != nil {
		actName = meta.ActivityName
		verName = meta.VersionName
		pathSeg = meta.PathSegment
	}

	profiles, err := player.PlayerProfilesForDelivery(ctx, d0.FireteamMembershipIds)
	if err != nil {
		return err
	}
	classByMembership, err := loadInstancePlayerClasses(ctx, d0.InstanceId)
	if err != nil {
		return err
	}
	ftProf := make([]messages.DiscordFireteamProfile, 0, len(profiles))
	for _, p := range profiles {
		ftProf = append(ftProf, messages.DiscordFireteamProfile{
			MembershipID: p.MembershipID,
			DisplayName:  p.DisplayName,
			IconURL:      p.IconURL,
			ClassHash:    classByMembership[p.MembershipID],
		})
	}

	statsMap, statsErr := loadInstancePlayerStats(ctx, d0.InstanceId)
	statsUnavailable := statsErr != nil
	if statsErr != nil {
		logger.Warn("SUBSCRIPTIONS_INSTANCE_STATS_UNAVAILABLE", statsErr, map[string]any{
			logging.INSTANCE_ID: d0.InstanceId,
		})
	}
	statsSlice := make([]messages.DiscordInstanceStat, 0, len(d0.FireteamMembershipIds))
	for _, mid := range d0.FireteamMembershipIds {
		s := InstancePlayerStats{}
		if statsMap != nil {
			s = statsMap[mid]
		}
		statsSlice = append(statsSlice, messages.DiscordInstanceStat{
			MembershipID:      mid,
			Kills:             s.Kills,
			Deaths:            s.Deaths,
			Assists:           s.Assists,
			TimePlayedSeconds: s.TimePlayedSeconds,
		})
	}

	feats, err := loadInstanceFeatsForDiscord(ctx, d0.InstanceId)
	if err != nil {
		return err
	}

	for i := range deliveries {
		deliveries[i].EmbedPreload = &messages.DiscordEmbedPreload{
			ActivityName:     actName,
			VersionName:      verName,
			PathSegment:      pathSeg,
			FireteamProfiles: ftProf,
			InstanceStats:    statsSlice,
			StatsUnavailable: statsUnavailable,
			Feats:            feats,
		}
	}
	return nil
}
