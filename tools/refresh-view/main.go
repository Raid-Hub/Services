package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"raidhub/lib/database/postgres"
	"raidhub/lib/utils/logging"
	"time"
)

var logger = logging.NewLogger("refresh-view")

func refreshCache(ctx context.Context, cacheView string) bool {
	logger.Info("REFRESHING_CACHE", map[string]any{logging.CACHE: cacheView})
	start := time.Now()

	_, err := postgres.DB.ExecContext(ctx, fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s WITH DATA", cacheView))
	if err != nil {
		logger.Error("ERROR_REFRESHING_CACHE", err, map[string]any{logging.CACHE: cacheView})
		return false
	}

	logger.Info("CACHE_REFRESHED", map[string]any{logging.CACHE: cacheView, "duration": time.Since(start).String()})
	return true
}

func refreshView(ctx context.Context, viewName string) bool {
	logger.Info("REFRESHING_VIEW", map[string]any{logging.VIEW: viewName})
	start := time.Now()

	_, err := postgres.DB.ExecContext(ctx, fmt.Sprintf("REFRESH MATERIALIZED VIEW CONCURRENTLY %s WITH DATA", viewName))
	if err != nil {
		logger.Error("ERROR_REFRESHING_VIEW", err, map[string]any{logging.VIEW: viewName})
		return false
	}

	logger.Info("VIEW_REFRESHED", map[string]any{logging.VIEW: viewName, "duration": time.Since(start).String()})
	return true
}

func main() {
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	if flag.NArg() < 1 {
		logger.Error("NO_VIEW_NAME", nil, map[string]any{"message": "No view name provided. Usage: ./bin/refresh-view <view_name>"})
		os.Exit(1)
		return
	}

	viewName := flag.Arg(0)
	postgres.Wait()

	ctx := context.Background()
	var hasError bool

	// Determine cache views if this view has any
	var cacheViews []string
	switch viewName {
	case "individual_global_leaderboard":
		cacheViews = []string{"_global_leaderboard_cache"}
	case "individual_raid_leaderboard":
		cacheViews = []string{"_individual_activity_leaderboard_cache"}
	}

	// Step 1: Refresh caches first if they exist
	for _, cacheView := range cacheViews {
		if !refreshCache(ctx, cacheView) {
			hasError = true
		}
	}

	// Step 2: Refresh the main view (normal refresh for all views)
	if !refreshView(ctx, viewName) {
		hasError = true
	}

	if hasError {
		os.Exit(1)
	}
}
