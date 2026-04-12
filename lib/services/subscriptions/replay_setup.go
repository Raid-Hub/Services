package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"

	"raidhub/lib/database/postgres"
)

const discordWebhookPrefix = "https://discord.com/api/webhooks/"
const discordWebhookPrefixLegacy = "https://discordapp.com/api/webhooks/"

// ErrNotDiscordWebhook indicates the URL is not a Discord Incoming Webhook URL (e.g. it may be a channel link).
var ErrNotDiscordWebhook = fmt.Errorf("not a Discord Incoming Webhook URL (expected https://discord.com/api/webhooks/...)")

// ValidateDiscordWebhookURL returns nil if the string looks like a Discord webhook URL suitable for subscriptions.destination.
func ValidateDiscordWebhookURL(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	if strings.Contains(u, "/channels/") && !strings.HasPrefix(u, discordWebhookPrefix) && !strings.HasPrefix(u, discordWebhookPrefixLegacy) {
		return fmt.Errorf("%w: got a channel link; create an Incoming Webhook in that channel (Server Settings → Integrations → Webhooks) and use its URL", ErrNotDiscordWebhook)
	}
	if !strings.HasPrefix(u, discordWebhookPrefix) && !strings.HasPrefix(u, discordWebhookPrefixLegacy) {
		return fmt.Errorf("%w", ErrNotDiscordWebhook)
	}
	return nil
}

// ValidateHTTPSCallbackURL returns nil if the URL is suitable for subscriptions.destination http_callback
// (HTTPS POST, JSON body). Blocks obvious SSRF targets (localhost, RFC1918, link-local, metadata IP).
// Production egress may apply additional controls (e.g. Cloudflare WAF / URL filters on partner endpoints).
func ValidateHTTPSCallbackURL(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return fmt.Errorf("callback URL is empty")
	}
	if !strings.HasPrefix(u, "https://") {
		return fmt.Errorf("callback URL must use https://")
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("callback URL parse: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("callback URL has no host")
	}
	if err := validateCallbackHost(host); err != nil {
		return err
	}
	return nil
}

func validateCallbackHost(host string) error {
	h := strings.ToLower(strings.TrimSpace(host))
	switch h {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return fmt.Errorf("callback host %q is not allowed", host)
	}
	if strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") {
		return fmt.Errorf("callback host %q is not allowed", host)
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("callback host IP %q is not allowed", host)
		}
	}
	return nil
}

// FindOrCreateDestinationByHTTPSCallback returns the subscriptions.destination id for this URL, inserting http_callback if none exists.
func FindOrCreateDestinationByHTTPSCallback(ctx context.Context, callbackURL string) (id int64, created bool, err error) {
	if err := ValidateHTTPSCallbackURL(callbackURL); err != nil {
		return 0, false, err
	}
	u := strings.TrimSpace(callbackURL)
	err = postgres.DB.QueryRowContext(ctx,
		`SELECT id FROM subscriptions.destination WHERE webhook_url = $1`, u).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	err = postgres.DB.QueryRowContext(ctx, `
		INSERT INTO subscriptions.destination (channel_type, webhook_url)
		VALUES ('http_callback', $1)
		RETURNING id`, u).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// FindOrCreateDestinationByWebhook returns the subscriptions.destination id for this webhook_url, inserting a row if none exists.
func FindOrCreateDestinationByWebhook(ctx context.Context, webhookURL string) (id int64, created bool, err error) {
	if err := ValidateDiscordWebhookURL(webhookURL); err != nil {
		return 0, false, err
	}
	u := strings.TrimSpace(webhookURL)
	err = postgres.DB.QueryRowContext(ctx,
		`SELECT id FROM subscriptions.destination WHERE webhook_url = $1`, u).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return 0, false, err
	}
	err = postgres.DB.QueryRowContext(ctx, `
		INSERT INTO subscriptions.destination (channel_type, webhook_url)
		VALUES ('discord_webhook', $1)
		RETURNING id`, u).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// DestinationExists returns true if id refers to an active destination.
func DestinationExists(ctx context.Context, destinationID int64) (bool, error) {
	var ok bool
	err := postgres.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM subscriptions.destination WHERE id = $1 AND is_active)`, destinationID).Scan(&ok)
	return ok, err
}

// EnsurePlayerRulesForReplay inserts player-scope rules so each membership_id receives deliveries for this destination when they appear in an instance.
// Idempotent: skips rows that already exist (active player rule for that destination + membership_id).
func EnsurePlayerRulesForReplay(ctx context.Context, destinationID int64, membershipIDs []int64) (inserted int, err error) {
	for _, mid := range membershipIDs {
		res, err := postgres.DB.ExecContext(ctx, `
			INSERT INTO subscriptions.rule (destination_id, scope, membership_id)
			SELECT $1, 'player', $2
			WHERE NOT EXISTS (
				SELECT 1 FROM subscriptions.rule r
				WHERE r.destination_id = $1
				  AND r.scope = 'player'
				  AND r.membership_id = $2
				  AND r.is_active
			)`, destinationID, mid)
		if err != nil {
			return inserted, fmt.Errorf("rule for membership_id %d: %w", mid, err)
		}
		n, _ := res.RowsAffected()
		inserted += int(n)
	}
	return inserted, nil
}
