package activityhistory

import (
	"fmt"
	"log"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/routing"
)

const (
	numProfiles = 2500
	clearsRange = 200_000
)

// ActivityHistoryUpdate is the command function
func ActivityHistoryUpdate() {
	log.Println("starting")

	log.Println("querying")
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
	log.Println("scanning")
	for rows.Next() {
		rows.Scan(&id)
		err = routing.Publisher.PublishTextMessage(routing.ActivityCrawl, fmt.Sprintf("%d", id))
		if err != nil {
			panic(err)
		}
	}

	log.Println("done")
}
