package main

import (
	"context"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("TOOLS")

const (
	numProfiles = 2500
	clearsRange = 200_000
)

// ActivityHistoryUpdate is the command function
func ActivityHistoryUpdate() {
	logger.Info("STARTING", map[string]any{})

	logger.Info("QUERYING", map[string]any{})
	rows, err := postgres.DB.Query(`SELECT * FROM (
		SELECT membership_id FROM player
		WHERE history_last_crawled IS NULL OR (history_last_crawled < NOW() - INTERVAL '25 weeks')
		ORDER BY clears DESC
		LIMIT $1
	) foo
	ORDER BY RANDOM() LIMIT $2`, clearsRange, numProfiles)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var id int64
	logger.Info("SCANNING", map[string]any{})
	for rows.Next() {
		rows.Scan(&id)
		err = publishing.PublishInt64Message(context.TODO(), routing.ActivityCrawl, id)
		if err != nil {
			panic(err)
		}
	}

	logger.Info("DONE", map[string]any{})
}

func main() {
	ActivityHistoryUpdate()
}
