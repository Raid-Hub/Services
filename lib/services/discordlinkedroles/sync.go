package discordlinkedroles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"raidhub/lib/database/authdb"
	"raidhub/lib/database/postgres"
	"raidhub/lib/env"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"

	"github.com/lib/pq"
)

var logger = logging.NewLogger("DISCORD_LINKED_ROLES")

var discordHTTPClient = &http.Client{Timeout: 18 * time.Second}

// HandleMessage performs Turso lookup, Postgres clears (sum across all Destiny profiles), Discord PUT, and Turso sync columns.
func HandleMessage(ctx context.Context, msg *messages.DiscordRoleMetadataSyncMessage) error {
	start := time.Now()
	outcome := "unknown"
	defer func() {
		global_metrics.DiscordLRWorkDuration.WithLabelValues(outcome).Observe(time.Since(start).Seconds())
	}()

	if !env.DiscordLinkedRolesEnabled {
		outcome = "disabled"
		return nil
	}
	if env.DiscordApplicationID == "" {
		logger.Warn("DISCORD_APP_ID_MISSING", nil, nil)
		outcome = "missing_env"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return nil
	}

	ids := msg.DestinyMembershipIds
	if len(ids) == 0 {
		outcome = "bad_message"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return nil
	}

	if msg.Trigger == messages.TriggerAccountLinkedRolesSync {
		for _, id := range ids {
			if err := authdb.PurgeTursoAuthLookupCacheForDestiny(ctx, id); err != nil {
				logger.Warn("DISCORD_ROLE_METADATA_PURGE_D2B_FAILED", err, map[string]any{"destiny_membership_id": id})
			}
		}
	}

	bungie, err := authdb.LookupBungieByDestinyMembershipID(ctx, ids[0])
	if err != nil {
		outcome = "turso_error"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return err
	}
	if bungie == "" {
		outcome = "unlinked"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return nil
	}
	for _, id := range ids[1:] {
		b2, err := authdb.LookupBungieByDestinyMembershipID(ctx, id)
		if err != nil {
			outcome = "turso_error"
			global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
			return err
		}
		if b2 != bungie {
			logger.Warn("DISCORD_ROLE_METADATA_MIXED_BUNGIE", nil, map[string]any{
				"expected_bungie": bungie,
				"destiny_id":      id,
				"got_bungie":      b2,
			})
			outcome = "bad_message"
			global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
			return nil
		}
	}

	if msg.Trigger == messages.TriggerAccountLinkedRolesSync {
		if err := authdb.PurgeTursoAuthLookupCacheForDiscordByBungie(ctx, bungie); err != nil {
			logger.Warn("DISCORD_ROLE_METADATA_PURGE_DISCORD_CACHE_FAILED", err, map[string]any{"bungie_membership_id": bungie})
		}
	}

	acc, err := authdb.LookupDiscordByBungieMembershipID(ctx, bungie)
	if err != nil {
		outcome = "turso_error"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return err
	}
	if acc == nil {
		outcome = "unlinked"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		return nil
	}

	membershipIDs := make([]int64, 0, len(ids))
	for _, s := range ids {
		mid, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			outcome = "bad_message"
			global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
			return nil
		}
		membershipIDs = append(membershipIDs, mid)
	}

	var totalClears int64
	err = postgres.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(clears::bigint), 0) FROM core.player WHERE membership_id = ANY($1)`,
		pq.Array(membershipIDs),
	).Scan(&totalClears)
	if err != nil {
		logger.Warn("PLAYER_CLEARS_SUM_FAILED", err, map[string]any{"destiny_membership_ids": ids})
		outcome = "postgres_error"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), strPtr("postgres_player"))
		return err
	}

	metaKey := env.DiscordLinkedRolesMetadataKey
	bodyMap := map[string]any{
		"platform_name": "RaidHub",
		"metadata": map[string]string{
			metaKey: strconv.FormatInt(totalClears, 10),
		},
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		outcome = "internal"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), strPtr("internal_marshal"))
		return fmt.Errorf("marshal role connection body: %w", err)
	}

	url := fmt.Sprintf("https://discord.com/api/v10/users/@me/applications/%s/role-connection", env.DiscordApplicationID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		outcome = "internal"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), strPtr("internal_request"))
		return err
	}
	req.Header.Set("Authorization", "Bearer "+acc.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := discordHTTPClient.Do(req)
	if err != nil {
		outcome = "discord_transport_error"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), strPtr("discord_transport"))
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		outcome = "discord_auth"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		code := fmt.Sprintf("http_%d", resp.StatusCode)
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), &code)
		_ = authdb.PurgeTursoAuthLookupCacheForDiscordByBungie(ctx, acc.BungieMembershipID)
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		outcome = "discord_http_error"
		global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
		code := fmt.Sprintf("http_%d", resp.StatusCode)
		_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), &code)
		return fmt.Errorf("discord role connection: status %d", resp.StatusCode)
	}

	outcome = "ok"
	global_metrics.DiscordLRWork.WithLabelValues(outcome).Inc()
	_ = authdb.UpdateSyncOutcome(ctx, acc.BungieMembershipID, time.Now(), nil)
	return nil
}

func strPtr(s string) *string {
	return &s
}
