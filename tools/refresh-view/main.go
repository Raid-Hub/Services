package refreshview

import (
	"context"
	"flag"
	"fmt"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"time"
)

var logger = logging.NewLogger("REFRESH_VIEW_TOOL")

// RefreshView is the command function for refreshing materialized views
// Handles both regular views and leaderboard views (with cache management)
// Usage: ./bin/tools refresh-view <view_name>
func RefreshView() {
	if flag.NArg() < 1 {
		logger.Error("NO_VIEW_NAME", map[string]any{"message": "No view name provided. Usage: ./bin/tools refresh-view <view_name>"})
		return
	}

	viewName := flag.Arg(0)
	postgres.Wait()

	ctx := context.Background()

	// Determine cache views if this view has any
	var cacheViews []string
	switch viewName {
	case "individual_global_leaderboard":
		cacheViews = []string{"_global_leaderboard_cache"}
	case "individual_raid_leaderboard":
		cacheViews = []string{"_raid_leaderboard_cache"}
	}

	// Step 1: Refresh caches first if they exist
	for _, cacheView := range cacheViews {
		refreshCache(ctx, cacheView)
	}

	// Step 2: Refresh the main view (normal refresh for all views)
	refreshView(ctx, viewName)

	// Step 3: Clear caches after refresh if they exist
	for _, cacheView := range cacheViews {
		clearCache(ctx, cacheView)
	}
}

func refreshCache(ctx context.Context, cacheView string) {
	logger.Info("REFRESHING_CACHE", map[string]any{"cache": cacheView})
	start := time.Now()

	_, err := postgres.DB.ExecContext(ctx, fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY leaderboard.%s WITH DATA", cacheView))
	if err != nil {
		logger.Error("ERROR_REFRESHING_CACHE", map[string]any{"cache": cacheView, logging.ERROR: err.Error()})
		return
	}

	logger.Info("CACHE_REFRESHED", map[string]any{"cache": cacheView, "duration": time.Since(start).String()})
}

func refreshView(ctx context.Context, viewName string) {
	logger.Info("REFRESHING_VIEW", map[string]any{"view": viewName})
	start := time.Now()

	_, err := postgres.DB.ExecContext(ctx, fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s WITH DATA", viewName))
	if err != nil {
		logger.Error("ERROR_REFRESHING_VIEW", map[string]any{"view": viewName, logging.ERROR: err.Error()})
		return
	}

	logger.Info("VIEW_REFRESHED", map[string]any{"view": viewName, "duration": time.Since(start).String()})
}

func clearCache(ctx context.Context, cacheView string) {
	logger.Info("CLEARING_CACHE", map[string]any{"cache": cacheView})
	start := time.Now()

	// Refresh without CONCURRENTLY first (required before dropping)
	_, err := postgres.DB.ExecContext(ctx, fmt.Sprintf("REFRESH MATERIALIZED VIEW leaderboard.%s WITH DATA", cacheView))
	if err != nil {
		logger.Error("ERROR_REFRESHING_CACHE_BEFORE_DROP", map[string]any{"cache": cacheView, logging.ERROR: err.Error()})
		return
	}

	// Drop the cache view
	_, err = postgres.DB.ExecContext(ctx, fmt.Sprintf("DROP MATERIALIZED VIEW leaderboard.%s CASCADE", cacheView))
	if err != nil {
		logger.Error("ERROR_DROPPING_CACHE", map[string]any{"cache": cacheView, logging.ERROR: err.Error()})
		return
	}

	// Recreate with original query structure
	var recreateSQL string
	switch cacheView {
	case "_global_leaderboard_cache":
		recreateSQL = `CREATE MATERIALIZED VIEW leaderboard._global_leaderboard_cache AS
SELECT
  membership_id,
  clears,
  fresh_clears,
  sherpas,
  sum_of_best,
  total_time_played_seconds,
  wfr_score,
  COUNT(*) OVER ()::numeric AS total_count
FROM "core"."player"
WHERE clears > 0 AND NOT is_private AND cheat_level < 2;
CREATE UNIQUE INDEX idx_global_leaderboard_cache_membership_id ON leaderboard._global_leaderboard_cache (membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_clears ON leaderboard._global_leaderboard_cache (clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_fresh_clears ON leaderboard._global_leaderboard_cache (fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_sherpas ON leaderboard._global_leaderboard_cache (sherpas DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_speed ON leaderboard._global_leaderboard_cache (sum_of_best ASC NULLS LAST, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_time ON leaderboard._global_leaderboard_cache (total_time_played_seconds DESC, membership_id ASC);
CREATE INDEX idx_global_leaderboard_cache_wfr ON leaderboard._global_leaderboard_cache (wfr_score DESC, membership_id ASC);`
	case "_raid_leaderboard_cache":
		recreateSQL = `CREATE MATERIALIZED VIEW leaderboard._raid_leaderboard_cache AS
SELECT
  ps.activity_id,
  ps.membership_id,
  ps.clears,
  ps.fresh_clears,
  ps.sherpas,
  ps.total_time_played_seconds,
  COUNT(*) OVER (PARTITION BY ps.activity_id) AS total_count
FROM "core"."player_stats" ps
JOIN "core"."player" p ON p.membership_id = ps.membership_id
WHERE ps.clears > 0
  AND NOT p.is_private
  AND p.cheat_level < 2;
CREATE UNIQUE INDEX idx_raid_leaderboard_cache_activity_membership ON leaderboard._raid_leaderboard_cache (activity_id ASC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_clears ON leaderboard._raid_leaderboard_cache (activity_id ASC, clears DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_fresh_clears ON leaderboard._raid_leaderboard_cache (activity_id ASC, fresh_clears DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_sherpas ON leaderboard._raid_leaderboard_cache (activity_id ASC, sherpas DESC, membership_id ASC);
CREATE INDEX idx_raid_leaderboard_cache_time ON leaderboard._raid_leaderboard_cache (activity_id ASC, total_time_played_seconds DESC, membership_id ASC);`
	default:
		logger.Error("UNKNOWN_CACHE_VIEW", map[string]any{"cache": cacheView})
		return
	}

	_, err = postgres.DB.ExecContext(ctx, recreateSQL)
	if err != nil {
		logger.Error("ERROR_RECREATING_CACHE", map[string]any{"cache": cacheView, logging.ERROR: err.Error()})
		return
	}

	logger.Info("CACHE_CLEARED", map[string]any{"cache": cacheView, "duration": time.Since(start).String()})
}
