// subscription-onboard is an admin CLI to attach a Discord webhook destination to clan and/or player
// subscription rules (beta onboarding). Uses explicit Postgres ids only (no name search).
//
// Clan: -clan-group-id from https://raidhub.io/clan/<id> (repeat for multiple).
// Player: -player-membership-id from https://raidhub.io/profile/<id> (repeat for multiple).
//
// Example:
//
//	go run ./tools/subscription-onboard -webhook-url 'https://discord.com/api/webhooks/...' -clan-group-id 5243173
//	go run ./tools/subscription-onboard -webhook-url '...' -clan-group-id 4927161 -player-membership-id 4611686018488107374
//	go run ./tools/subscription-onboard -webhook-url '...' -clan-group-id 4927161 -require-completed
//	go run ./tools/subscription-onboard -webhook-url '...' -clan-group-id 5411410 -activity-raid-bitmap 400
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"raidhub/lib/database/postgres"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("subscription-onboard")

func main() {
	webhookURL := flag.String("webhook-url", "", "Discord Incoming Webhook URL (required)")
	dryRun := flag.Bool("dry-run", false, "Print actions only; do not write to the database")

	var clanGroupIDs int64Slice
	flag.Var(&clanGroupIDs, "clan-group-id", "Clan group_id (repeat for multiple). Same number as raidhub.io/clan/<id>.")

	var playerMembershipIDs int64Slice
	flag.Var(&playerMembershipIDs, "player-membership-id", "Player membership_id (repeat for multiple). Same number as raidhub.io/profile/<id>.")

	requireFresh := flag.Bool("require-fresh", false, "Only notify for fresh raid starts (not checkpoint)")
	requireCompleted := flag.Bool("require-completed", false, "Only notify for full clears (completed instance)")
	activityRaidBitmapStr := flag.String("activity-raid-bitmap", "", "Raid filter: uint64 bitmask, decimal or 0x hex (same layout as cheat_detection raid bits). Empty or 0 = all raids.")

	flag.Parse()
	logging.ParseFlags()

	if strings.TrimSpace(*webhookURL) == "" {
		fmt.Fprintln(os.Stderr, "error: -webhook-url is required")
		flag.Usage()
		os.Exit(2)
	}
	if len(clanGroupIDs) == 0 && len(playerMembershipIDs) == 0 {
		fmt.Fprintln(os.Stderr, "error: pass at least one of -clan-group-id or -player-membership-id")
		os.Exit(2)
	}

	ctx := context.Background()
	postgres.Wait()

	if err := subscriptions.ValidateDiscordWebhookURL(*webhookURL); err != nil {
		logger.Fatal("WEBHOOK_URL_INVALID", err, nil)
	}

	var raidBitmap uint64
	if strings.TrimSpace(*activityRaidBitmapStr) != "" {
		v, err := strconv.ParseUint(strings.TrimSpace(*activityRaidBitmapStr), 0, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: invalid -activity-raid-bitmap:", err)
			os.Exit(2)
		}
		raidBitmap = v
	}

	criteria := subscriptions.RuleInstanceCriteria{
		RequireFresh:       *requireFresh,
		RequireCompleted:   *requireCompleted,
		ActivityRaidBitmap: raidBitmap,
	}

	if *dryRun {
		runDryRun(ctx, *webhookURL, clanGroupIDs, playerMembershipIDs, criteria)
		os.Exit(0)
	}

	destID, created, err := subscriptions.FindOrCreateDestinationByWebhook(ctx, *webhookURL)
	if err != nil {
		logger.Fatal("DESTINATION_FAILED", err, nil)
	}
	logger.Info("DESTINATION", map[string]any{"destination_id": destID, "created": created})

	for _, gid := range clanGroupIDs {
		clanName, clanInDB, err := subscriptions.LookupClanNameOptional(ctx, gid)
		if err != nil {
			logger.Fatal("CLAN_LOOKUP_FAILED", err, map[string]any{"group_id": gid})
		}
		inserted, err := subscriptions.UpsertClanRuleWithInstanceCriteria(ctx, destID, gid, criteria)
		if err != nil {
			logger.Fatal("CLAN_RULE_FAILED", err, map[string]any{"group_id": gid})
		}
		fields := map[string]any{
			"group_id":          gid,
			"inserted":          inserted,
			"require_fresh":        criteria.RequireFresh,
			"require_completed":    criteria.RequireCompleted,
			"activity_raid_bitmap": criteria.ActivityRaidBitmap,
			"clan_in_database":     clanInDB,
		}
		if clanInDB {
			fields["clan_name"] = clanName
		} else {
			fields["clan_name"] = "(not in clan.clan yet — rule still uses this group_id)"
		}
		logger.Info("CLAN_RULE", fields)
	}

	if len(playerMembershipIDs) > 0 {
		ins, upd, err := subscriptions.UpsertPlayerRulesWithInstanceCriteria(ctx, destID, playerMembershipIDs, criteria)
		if err != nil {
			logger.Fatal("PLAYER_RULES_FAILED", err, nil)
		}
		logger.Info("PLAYER_RULES", map[string]any{"rules_inserted": ins, "rules_updated": upd, "players": len(playerMembershipIDs), "require_fresh": criteria.RequireFresh, "require_completed": criteria.RequireCompleted, "activity_raid_bitmap": criteria.ActivityRaidBitmap})
	}

	logger.Info("DONE", map[string]any{"destination_id": destID})
}

func runDryRun(ctx context.Context, webhookURL string, clanGroupIDs, playerMembershipIDs []int64, criteria subscriptions.RuleInstanceCriteria) {
	logger.Info("DRY_RUN", map[string]any{
		"webhook_url":       "(valid)",
		"note":              "no database writes",
		"require_fresh":        criteria.RequireFresh,
		"require_completed":    criteria.RequireCompleted,
		"activity_raid_bitmap": criteria.ActivityRaidBitmap,
	})

	for _, gid := range clanGroupIDs {
		clanName, clanInDB, err := subscriptions.LookupClanNameOptional(ctx, gid)
		if err != nil {
			logger.Fatal("CLAN_LOOKUP_FAILED", err, map[string]any{"group_id": gid})
		}
		fields := map[string]any{"group_id": gid, "clan_in_database": clanInDB}
		if clanInDB {
			fields["clan_name"] = clanName
		} else {
			fields["clan_name"] = "(not in clan.clan yet — rule would still use this group_id)"
		}
		logger.Info("WOULD_ADD_CLAN_RULE", fields)
	}

	for _, mid := range playerMembershipIDs {
		logger.Info("WOULD_ADD_PLAYER_RULE", map[string]any{"membership_id": mid})
	}
}

type int64Slice []int64

func (s *int64Slice) String() string {
	return fmt.Sprint(*s)
}

func (s *int64Slice) Set(v string) error {
	x, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || x <= 0 {
		return fmt.Errorf("invalid id %q (expect positive integer)", v)
	}
	*s = append(*s, x)
	return nil
}
