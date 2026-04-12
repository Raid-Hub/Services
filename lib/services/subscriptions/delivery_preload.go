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
			return fmt.Errorf("subscription destination %d has no URL", id)
		}
		switch row.ChannelType {
		case string(messages.DeliveryChannelDiscordWebhook):
			if err := ValidateDiscordWebhookURL(row.WebhookURL); err != nil {
				return fmt.Errorf("subscription destination %d: %w", id, err)
			}
		case string(messages.DeliveryChannelHttpCallback):
			if err := ValidateHTTPSCallbackURL(row.WebhookURL); err != nil {
				return fmt.Errorf("subscription destination %d: %w", id, err)
			}
		default:
			return fmt.Errorf("subscription destination %d: unsupported channel_type %q", id, row.ChannelType)
		}
		deliveries[i].ChannelType = messages.DeliveryChannelType(row.ChannelType)
		deliveries[i].WebhookURL = row.WebhookURL
	}
	return nil
}

// preloadDiscordEmbedData loads activity metadata, fireteam profiles, instance stats, and instance feats
// once per match batch for discord_webhook destinations (URL + channel type are set in attachDestinationWebhooks).
func preloadDiscordEmbedData(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	var needDiscord bool
	for i := range deliveries {
		if deliveries[i].ChannelType == messages.DeliveryChannelDiscordWebhook {
			needDiscord = true
			break
		}
	}
	if !needDiscord {
		return nil
	}
	var d0 *messages.SubscriptionDeliveryMessage
	for i := range deliveries {
		if deliveries[i].ChannelType == messages.DeliveryChannelDiscordWebhook {
			d0 = &deliveries[i]
			break
		}
	}
	if d0 == nil {
		return nil
	}
	ep0 := d0.EmbedPreload
	if ep0 == nil {
		return fmt.Errorf("internal: discord delivery missing embed preload raid context")
	}

	meta, err := loadActivityRaidMeta(ctx, ep0.ActivityHash)
	if err != nil {
		return err
	}
	var actName, verName, pathSeg string
	if meta != nil {
		actName = meta.ActivityName
		verName = meta.VersionName
		pathSeg = meta.PathSegment
	}

	profiles, err := player.PlayerProfilesForDelivery(ctx, ep0.FireteamMembershipIds)
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
	statsSlice := make([]messages.DiscordInstanceStat, 0, len(ep0.FireteamMembershipIds))
	for _, mid := range ep0.FireteamMembershipIds {
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
		if deliveries[i].ChannelType != messages.DeliveryChannelDiscordWebhook {
			continue
		}
		ep := deliveries[i].EmbedPreload
		if ep == nil {
			return fmt.Errorf("internal: discord delivery missing embed preload")
		}
		ep.ActivityName = actName
		ep.VersionName = verName
		ep.PathSegment = pathSeg
		ep.FireteamProfiles = ftProf
		ep.InstanceStats = statsSlice
		ep.StatsUnavailable = statsUnavailable
		ep.Feats = feats
	}
	return nil
}

// preloadHttpCallbackInstance loads dto.Instance once per batch for http_callback destinations (same JSON shape as the public instance API).
func preloadHttpCallbackInstance(ctx context.Context, deliveries []messages.SubscriptionDeliveryMessage) error {
	if len(deliveries) == 0 {
		return nil
	}
	var need bool
	for i := range deliveries {
		if deliveries[i].ChannelType == messages.DeliveryChannelHttpCallback {
			need = true
			break
		}
	}
	if !need {
		return nil
	}
	inst, err := LoadDTOInstanceFromPostgres(ctx, deliveries[0].InstanceId)
	if err != nil {
		return err
	}
	for i := range deliveries {
		if deliveries[i].ChannelType == messages.DeliveryChannelHttpCallback {
			deliveries[i].Instance = inst
		}
	}
	return nil
}
