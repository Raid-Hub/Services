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
// HTTPDestinationHeader carries the partner webhook URL when posting to SUBSCRIPTION_WEBHOOK_RELAY_URL (Cloudflare Worker).
const HTTPDestinationHeader = "X-RaidHub-Destination"

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
// If SUBSCRIPTION_WEBHOOK_RELAY_URL is set, POSTs to that URL instead with the same body and headers,
// plus Authorization: Bearer (relay secret) and X-RaidHub-Destination (true partner URL).
func postSubscriptionInstanceJSON(ctx context.Context, partnerURL string, inst any) error {
	payload, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	partnerKey := strings.TrimSpace(env.SubscriptionHTTPWebhookSecret)

	// Relay is off unless SUBSCRIPTION_WEBHOOK_RELAY_URL is set; then we POST to the Worker and pass
	// the real partner URL in X-RaidHub-Destination. Empty/unset env means direct POST to partnerURL only.
	relay := strings.TrimSpace(env.SubscriptionWebhookRelayURL)
	if relay != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, relay, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(HTTPCallbackSecretHeader, partnerKey)
		req.Header.Set(HTTPDestinationHeader, partnerURL)
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(env.SubscriptionWebhookRelaySecret))
		return doHTTPCallbackResponse(req, relay)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, partnerURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HTTPCallbackSecretHeader, partnerKey)
	return doHTTPCallbackResponse(req, partnerURL)
}

func doHTTPCallbackResponse(req *http.Request, logURL string) error {
	resp, err := subscriptionHTTPDeliveryClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http_callback POST %s: status %d: %s", logURL, resp.StatusCode, strings.TrimSpace(string(body)))
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
