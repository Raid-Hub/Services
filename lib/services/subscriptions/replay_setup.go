package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
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
