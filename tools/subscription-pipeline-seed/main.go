// subscription-pipeline-seed is a dev-only helper: it truncates/seeds Postgres (synthetic clan + two
// subscription destinations: player + clan), then publishes one InstanceParticipantRefresh for a
// ClickHouse instance_id. Use tools/replay-subscription-instance for normal “replay this id” runs.
//
// Clan resolution uses Postgres clan.clan / clan.clan_members only — Redis is not part of this pipeline.
//
//	go run ./tools/subscription-pipeline-seed/ -webhook-url='https://discord.com/api/webhooks/...'
//	(default -instance-id is the dev Vow E2E instance; use -instance-id=0 for latest ClickHouse row)
//
// Flags: -skip-seed to only publish (subscriptions DB must already match the instance).
// When seeding, -webhook-url is required (pass each run; do not commit webhook URLs to .env).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"strings"

	"raidhub/lib/database/clickhouse"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("subscription-pipeline-seed")

// TestClanGroupID is a dedicated synthetic Bungie group id for local E2E (avoid colliding with real clans).
const testClanGroupID int64 = 9000000000001

// defaultSubscriptionSeedInstanceID matches tools/replay-subscription-instance: dev Vow E2E instance (2 players, subscriptions wired).
const defaultSubscriptionSeedInstanceID int64 = 16818312483

var (
	instanceID = flag.Int64("instance-id", defaultSubscriptionSeedInstanceID, "ClickHouse instance_id to replay (0 = most recent by date_completed)")
	skipSeed   = flag.Bool("skip-seed", false, "Skip TRUNCATE/seed — only publish replay")
	webhookURL = flag.String("webhook-url", "", "Discord webhook URL for seeded destinations (required when not using -skip-seed)")
)

func main() {
	logging.ParseFlags()
	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	postgres.Wait()
	clickhouse.Wait()
	publishing.Wait()

	ctx := context.Background()

	inst, err := subscriptions.LoadDTOInstanceFromClickHouse(ctx, *instanceID)
	if err != nil {
		logger.Fatal("LOAD_INSTANCE_FAILED", err, nil)
		return
	}
	if len(inst.Players) < 2 {
		logger.Fatal("INSTANCE_TOO_SMALL", fmt.Errorf("need at least 2 players for player+clan e2e"), map[string]any{
			logging.INSTANCE_ID: inst.InstanceId,
			logging.COUNT:       len(inst.Players),
		})
		return
	}

	clanPlayerMID := inst.Players[0].Player.MembershipId
	playerRuleMID := inst.Players[1].Player.MembershipId

	if !*skipSeed {
		wh := strings.TrimSpace(*webhookURL)
		if wh == "" {
			logger.Fatal("MISSING_WEBHOOK_URL", fmt.Errorf("pass -webhook-url for seeded destinations (or use -skip-seed)"), nil)
			return
		}
		if err := seedE2E(ctx, wh, clanPlayerMID, playerRuleMID); err != nil {
			logger.Fatal("SEED_FAILED", err, nil)
			return
		}
		logger.Info("E2E_SEED_OK", map[string]any{
			"clan_group_id":   testClanGroupID,
			"clan_player_mid": clanPlayerMID,
			"player_rule_mid": playerRuleMID,
			"webhook_url_set": true,
		})
	}

	ev := subscriptions.NewSubscriptionEvent(inst)
	if err := publishing.PublishJSONMessage(ctx, routing.InstanceParticipantRefresh, ev); err != nil {
		logger.Fatal("PUBLISH_FAILED", err, map[string]any{logging.INSTANCE_ID: *instanceID})
		return
	}

	logger.Info("E2E_REPLAY_SENT", map[string]any{
		logging.INSTANCE_ID: inst.InstanceId,
		"expect":            "Hermes PROCESSING_SUBSCRIPTION_DELIVERY with channel_id 1 and 2 (player dest + clan dest)",
	})
}

func seedE2E(ctx context.Context, webhookURL string, clanPlayerMID, playerRuleMID int64) error {
	_, err := postgres.DB.ExecContext(ctx, `
		INSERT INTO clan.clan (group_id, name, motto, call_sign, clan_banner_data)
		VALUES ($1, 'E2E Subscription Clan', '-', '-', '{}'::jsonb)
		ON CONFLICT (group_id) DO NOTHING`,
		testClanGroupID)
	if err != nil {
		return fmt.Errorf("insert clan: %w", err)
	}

	_, err = postgres.DB.ExecContext(ctx, `
		INSERT INTO clan.clan_members (group_id, membership_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		testClanGroupID, clanPlayerMID)
	if err != nil {
		return fmt.Errorf("insert clan member: %w", err)
	}

	_, err = postgres.DB.ExecContext(ctx, `TRUNCATE subscriptions.rule, subscriptions.destination RESTART IDENTITY CASCADE`)
	if err != nil {
		return fmt.Errorf("truncate subscriptions: %w", err)
	}

	var destPlayer, destClan int64
	err = postgres.DB.QueryRowContext(ctx, `
		INSERT INTO subscriptions.destination (channel_type, webhook_url)
		VALUES ('discord_webhook', $1)
		RETURNING id`, webhookURL).Scan(&destPlayer)
	if err != nil {
		return fmt.Errorf("insert destination player: %w", err)
	}
	err = postgres.DB.QueryRowContext(ctx, `
		INSERT INTO subscriptions.destination (channel_type, webhook_url)
		VALUES ('discord_webhook', $1)
		RETURNING id`, webhookURL).Scan(&destClan)
	if err != nil {
		return fmt.Errorf("insert destination clan: %w", err)
	}

	_, err = postgres.DB.ExecContext(ctx, `
		INSERT INTO subscriptions.rule (destination_id, scope, membership_id)
		VALUES ($1, 'player', $2)`,
		destPlayer, playerRuleMID)
	if err != nil {
		return fmt.Errorf("insert player rule: %w", err)
	}

	_, err = postgres.DB.ExecContext(ctx, `
		INSERT INTO subscriptions.rule (destination_id, scope, group_id)
		VALUES ($1, 'clan', $2)`,
		destClan, testClanGroupID)
	if err != nil {
		return fmt.Errorf("insert clan rule: %w", err)
	}

	var isPrivate bool
	err = postgres.DB.QueryRowContext(ctx, `SELECT is_private FROM core.player WHERE membership_id = $1`, playerRuleMID).Scan(&isPrivate)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("player %d not in core.player (required for player-scope match)", playerRuleMID)
		}
		return fmt.Errorf("lookup player for rule: %w", err)
	}
	if isPrivate {
		return fmt.Errorf("player %d is private; player-scope subscription will not match", playerRuleMID)
	}

	return nil
}
