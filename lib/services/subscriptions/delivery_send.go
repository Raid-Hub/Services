package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// HTTPDestinationHeader carries the partner webhook URL when posting to SUBSCRIPTION_WEBHOOK_RELAY_URL (outbound relay).
const HTTPDestinationHeader = "X-RaidHub-Destination"

// SendSubscriptionDelivery POSTs the destination URL. Discord uses Components V2 webhook payloads
// (always direct to Discord — never SUBSCRIPTION_WEBHOOK_RELAY_URL). http_callback sends
// application/json dto.Instance (same shape as api.raidhub.io/instance/:id); relay applies only there.
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
		if err := discord.SendWebhook(ctx, webhookURL, wh); err != nil {
			if discord.IsPermanentDeliveryError(err) {
				return processing.NewUnretryableError(err)
			}
			return err
		}
		return nil
	case messages.DeliveryChannelHttpCallback:
		if message.Instance == nil {
			return processing.NewUnretryableError(fmt.Errorf(
				"subscription delivery: http_callback missing instance payload"))
		}
		return postSubscriptionInstanceJSON(ctx, webhookURL, message.Instance)
	default:
		return processing.NewUnretryableError(fmt.Errorf("unsupported channel type %q", message.ChannelType))
	}
}

func httpCallbackPermanentStatus(code int) bool {
	if code < 400 || code >= 500 {
		return false
	}
	switch code {
	case 408, 425, 429:
		return false
	default:
		return true
	}
}

func truncateHTTPCallbackErrBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func redactCallbackURLForLog(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "(invalid url)"
	}
	u.RawQuery = ""
	u.Fragment = ""
	out := u.String()
	if out == "" {
		return raw
	}
	return out
}

// postSubscriptionInstanceJSON POSTs JSON to the partner URL with X-RaidHub-Key from env.
// If SUBSCRIPTION_WEBHOOK_RELAY_URL is set, POSTs to that URL instead with the same body and headers,
// plus Authorization: Bearer (same value as X-RaidHub-Key) and X-RaidHub-Destination (true partner URL).
func postSubscriptionInstanceJSON(ctx context.Context, partnerURL string, inst any) error {
	payload, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	partnerKey := strings.TrimSpace(env.SubscriptionHTTPWebhookSecret)
	if partnerKey == "" {
		return processing.NewUnretryableError(fmt.Errorf(
			"SUBSCRIPTION_HTTP_WEBHOOK_SECRET is not set (required for http_callback X-RaidHub-Key)"))
	}

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
		req.Header.Set("Authorization", "Bearer "+partnerKey)
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
	detail := truncateHTTPCallbackErrBody(body)
	safeURL := redactCallbackURLForLog(logURL)
	if httpCallbackPermanentStatus(resp.StatusCode) {
		return processing.NewUnretryableError(fmt.Errorf(
			"http_callback POST %s: status %d: %s", safeURL, resp.StatusCode, detail))
	}
	return fmt.Errorf("http_callback POST %s: status %d: %s", safeURL, resp.StatusCode, detail)
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
	return assembleRaidDiscordEmbed(msg.InstanceId, pre, pre.ActivityName, pre.VersionName, pre.PathSegment, pre.Feats,
		profiles, statsMap, pre.StatsUnavailable)
}
