package main

import (
	"log"
	"raidhub/packages/async/activity_history"
	"raidhub/packages/async/bonus_pgcr"
	"raidhub/packages/async/character_fill"
	"raidhub/packages/async/clan_crawl"
	"raidhub/packages/async/pgcr_cheat_check"
	"raidhub/packages/async/pgcr_clickhouse"
	"raidhub/packages/async/player_crawl"
	"raidhub/packages/bungie"
	"raidhub/packages/monitoring"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"
	"raidhub/packages/util"
	"sync"
	"time"
)

func main() {
	log.SetFlags(0) // Disable timestamps
	db, err := postgres.Connect()
	if err != nil {
		log.Fatal("Error connecting to postgres", err)
	}
	defer db.Close()

	conn, err := rabbit.Init()
	if err != nil {
		log.Fatal("Error connecting to rabbit", err)
	}
	defer rabbit.Cleanup()

	var destiny2ApiWg sync.WaitGroup
	readonlyDestiny2ApiWg := util.NewReadOnlyWaitGroup(&destiny2ApiWg)

	activityHistoryQueue := activity_history.Create()
	activityHistoryQueue.Conn = conn
	activityHistoryQueue.Db = db
	activityHistoryQueue.Wg = &readonlyDestiny2ApiWg
	go activityHistoryQueue.Register(3)

	playersQueue := player_crawl.Create()
	playersQueue.Db = db
	playersQueue.Conn = conn
	playersQueue.Wg = &readonlyDestiny2ApiWg
	go playersQueue.Register(10)

	activityCharactersQueue := character_fill.Create()
	activityCharactersQueue.Db = db
	activityCharactersQueue.Conn = conn
	activityCharactersQueue.Wg = &readonlyDestiny2ApiWg
	go activityCharactersQueue.Register(5)

	pgcrsClickhouseQueue := pgcr_clickhouse.CreateClickhouseQueue()
	pgcrsClickhouseQueue.Db = db
	pgcrsClickhouseQueue.Conn = conn
	go pgcrsClickhouseQueue.Register(1)

	bonus_pgcr.CreateOutboundChannel(conn)
	bonusPgcrsFetchQueue := bonus_pgcr.CreateFetchWorker()
	bonusPgcrsFetchQueue.Db = db
	bonusPgcrsFetchQueue.Conn = conn
	bonusPgcrsFetchQueue.Wg = &readonlyDestiny2ApiWg
	go bonusPgcrsFetchQueue.Register(25)

	bonusPgcrsStoreQueue := bonus_pgcr.CreateStoreWorker()
	bonusPgcrsStoreQueue.Db = db
	bonusPgcrsStoreQueue.Conn = conn
	// 1 worker because it's a write operation with often related records which would cause deadlocks
	go bonusPgcrsStoreQueue.Register(1)

	var groupsApiWg sync.WaitGroup
	readonlyGroupsApiWg := util.NewReadOnlyWaitGroup(&groupsApiWg)

	clanQueue := clan_crawl.Create()
	clanQueue.Db = db
	clanQueue.Conn = conn
	clanQueue.Wg = &readonlyGroupsApiWg
	go clanQueue.Register(1)

	cheatDetectionQueue := pgcr_cheat_check.Create()
	cheatDetectionQueue.Db = db
	cheatDetectionQueue.Conn = conn
	go cheatDetectionQueue.Register(1)

	monitoring.RegisterPrometheus(8083)

	// Set up Bungie API monitoring
	go func() {
		destiny2Enabled := true
		groupsEnabled := true
		for {
			log.Printf("Checking Bungie API status")
			res, err := bungie.GetCommonSettings()
			if err != nil {
				log.Printf("Failed to get common settings: %s", err)
				time.Sleep(5 * time.Second)
				res, err = bungie.GetCommonSettings()
				if err != nil {
					destiny2ApiWg.Add(1)
					destiny2Enabled = false
					log.Printf("Destiny 2 API is down or erroring")
					time.Sleep(90 * time.Second)
					continue
				}
			}

			if !res.Systems["Destiny2"].Enabled && destiny2Enabled {
				destiny2ApiWg.Add(1)
				destiny2Enabled = false
				log.Printf("Destiny 2 API is disabled")
			} else if res.Systems["Destiny2"].Enabled && !destiny2Enabled {
				destiny2ApiWg.Add(-1)
				destiny2Enabled = true
				log.Printf("Destiny 2 API is enabled")
			}

			if !res.Systems["Groups"].Enabled && groupsEnabled {
				groupsApiWg.Add(1)
				groupsEnabled = false
				log.Printf("Groups API is disabled")
			} else if res.Systems["Groups"].Enabled && !groupsEnabled {
				groupsApiWg.Add(-1)
				groupsEnabled = true
				log.Printf("Groups API is enabled")
			}

			time.Sleep(60 * time.Second)
		}
	}()

	// Keep the main thread running
	forever := make(chan bool)
	<-forever
}
