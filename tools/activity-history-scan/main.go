package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("activity-history-scan")

const (
	numProfiles          = 2_500
	topXProfiles         = 500_000
	lastCrawledThreshold = "180 days"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	logger.Info("STARTING_ACTIVITY_HISTORY_SCAN", map[string]any{})

	logger.Debug("QUERYING_PLAYERS_TO_SCAN", map[string]any{})

	query := fmt.Sprintf(`SELECT * FROM (
		SELECT membership_id FROM player
		WHERE history_last_crawled IS NULL OR (history_last_crawled < NOW() - INTERVAL '%s')
		ORDER BY _search_score DESC
		LIMIT $1
	) foo
	ORDER BY RANDOM() LIMIT $2`, lastCrawledThreshold)

	rows, err := postgres.DB.QueryContext(ctx, query, topXProfiles, numProfiles)
	if err != nil {
		logger.Fatal("ERROR_QUERYING_PLAYERS_TO_SCAN", err, nil)
	}
	defer rows.Close()

	var id int64
	logger.Info("SCANNING", map[string]any{})
	count := 0
	for rows.Next() {
		rows.Scan(&id)
		err = publishing.PublishInt64Message(ctx, routing.ActivityCrawl, id)
		if err != nil {
			logger.Error("ERROR_PUBLISHING_INSTANCE_ID", err, map[string]any{
				"instance_id": id,
			})
		}
		count++
	}

	logger.Info("ACTIVITY_HISTORY_SCAN_COMPLETE", map[string]any{
		"scanned_players":            numProfiles,
		"top_x_profiles":             topXProfiles,
		"last_crawled_threshold":     lastCrawledThreshold,
		"num_instance_ids_published": count,
	})
}
