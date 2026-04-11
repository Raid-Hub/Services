package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"raidhub/lib/env"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/services/player"
	"raidhub/lib/web/discord"
)

var subscriptionHTTPDeliveryClient = &http.Client{Timeout: 60 * time.Second}

// HTTPCallbackSecretHeader is the header name for the shared secret on http_callback POSTs.
const HTTPCallbackSecretHeader = "X-RaidHub-Key"

// SendSubscriptionDelivery POSTs the destination URL. Discord uses Components V2 webhook payloads;
// http_callback sends application/json dto.Instance (same shape as api.raidhub.io/instance/:id).
func SendSubscriptionDelivery(ctx context.Context, message messages.SubscriptionDeliveryMessage) error {
	webhookURL := strings.TrimSpace(message.WebhookURL)
	if webhookURL == "" {
		return processing.NewUnretryableError(fmt.Errorf(
			"subscription delivery: missing webhookUrl (expected subscription_match output)"))
	}

	switch message.ChannelType {
	case messages.DeliveryChannelDiscordWebhook:
		if message.EmbedPreload == nil {
			return processing.NewUnretryableError(fmt.Errorf(
				"subscription delivery: discord_webhook missing embedPreload"))
		}
		wh := buildRaidWebhookFromEmbedPreload(message)
		return discord.SendWebhook(webhookURL, wh)
	case messages.DeliveryChannelHttpCallback:
		if message.Instance == nil {
			return processing.NewUnretryableError(fmt.Errorf(
				"subscription delivery: http_callback missing instance payload"))
		}
		return postSubscriptionInstanceJSON(ctx, webhookURL, message.Instance)
	default:
		return fmt.Errorf("unsupported channel type %q", message.ChannelType)
	}
}

// postSubscriptionInstanceJSON POSTs JSON to the partner URL with X-RaidHub-Key from env.
func postSubscriptionInstanceJSON(ctx context.Context, targetURL string, inst any) error {
	payload, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HTTPCallbackSecretHeader, strings.TrimSpace(env.SubscriptionHTTPWebhookSecret))
	resp, err := subscriptionHTTPDeliveryClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http_callback POST %s: status %d: %s", targetURL, resp.StatusCode, strings.TrimSpace(string(body)))
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
