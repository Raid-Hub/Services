package subscriptions

import (
	"context"
	"fmt"
	"strings"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/services/player"
	"raidhub/lib/web/discord"
)

// SendSubscriptionDelivery POSTs the Discord webhook. Payloads must come from the subscription_match
// stage (webhook URL + embed preload set). Any other shape is rejected without retry.
func SendSubscriptionDelivery(ctx context.Context, message messages.SubscriptionDeliveryMessage) error {
	webhookURL := strings.TrimSpace(message.WebhookURL)
	if webhookURL == "" || message.EmbedPreload == nil {
		return processing.NewUnretryableError(fmt.Errorf(
			"subscription delivery: missing webhookUrl or embedPreload (expected subscription_match output)"))
	}

	if message.ChannelType != messages.DeliveryChannelDiscordWebhook {
		return fmt.Errorf("unsupported channel type %q", message.ChannelType)
	}

	wh := buildRaidWebhookFromEmbedPreload(message)
	return discord.SendWebhook(webhookURL, wh)
}

func buildRaidWebhookFromEmbedPreload(msg messages.SubscriptionDeliveryMessage) *discord.Webhook {
	pre := msg.EmbedPreload
	profiles := make([]player.PlayerProfileForDelivery, 0, len(pre.FireteamProfiles))
	for _, p := range pre.FireteamProfiles {
		profiles = append(profiles, player.PlayerProfileForDelivery{
			MembershipID: p.MembershipID,
			DisplayName:  p.DisplayName,
			IconURL:      p.IconURL,
			ClassHash:    p.ClassHash,
		})
	}
	statsMap := make(map[int64]InstancePlayerStats, len(pre.InstanceStats))
	for _, s := range pre.InstanceStats {
		statsMap[s.MembershipID] = InstancePlayerStats{
			Kills:             s.Kills,
			Deaths:            s.Deaths,
			Assists:           s.Assists,
			TimePlayedSeconds: s.TimePlayedSeconds,
		}
	}
	return assembleRaidDiscordEmbed(msg, pre.ActivityName, pre.VersionName, pre.PathSegment, pre.Feats,
		profiles, statsMap, pre.StatsUnavailable)
}
