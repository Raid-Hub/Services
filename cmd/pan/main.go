package main

import (
	"log"
	"raidhub/packages/async/activity_history"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"
)

const (
	numProfiles = 2500
	clearsRange = 200_000
)

func main() {
	log.Println("starting")
	db, err := postgres.Connect()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	log.Println("connecting")
	conn, err := rabbit.Init()
	if err != nil {
		panic(err)
	}
	defer rabbit.Cleanup()

	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Println("querying")
	rows, err := db.Query(`SELECT * FROM (
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
		err = activity_history.SendMessage(ch, id)
		if err != nil {
			panic(err)
		}
	}

	log.Println("done")
}
