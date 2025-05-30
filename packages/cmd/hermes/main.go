package main

import (
	"log"
	"raidhub/packages/async/activity_history"
	"raidhub/packages/async/character_fill"
	"raidhub/packages/async/clan_crawl"
	"raidhub/packages/async/pgcr_blocked"
	"raidhub/packages/async/pgcr_cheat_check"
	"raidhub/packages/async/pgcr_clickhouse"
	"raidhub/packages/async/pgcr_exists"
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
	destiny2ApiWg.Add(1)
	readonlyDestiny2ApiWg := util.NewReadOnlyWaitGroup(&destiny2ApiWg)

	activityHistoryQueue := activity_history.Create()
	activityHistoryQueue.Conn = conn
	activityHistoryQueue.Db = db
	activityHistoryQueue.Wg = &readonlyDestiny2ApiWg
	go activityHistoryQueue.Register(3, true)

	player_crawl.CreateOutboundChannel(conn)
	playersQueue := player_crawl.Create()
	playersQueue.Db = db
	playersQueue.Conn = conn
	playersQueue.Wg = &readonlyDestiny2ApiWg
	go playersQueue.Register(30, true)

	activityCharactersQueue := character_fill.Create()
	activityCharactersQueue.Db = db
	activityCharactersQueue.Conn = conn
	activityCharactersQueue.Wg = &readonlyDestiny2ApiWg
	go activityCharactersQueue.Register(5, true)

	pgcrsClickhouseQueue := pgcr_clickhouse.CreateClickhouseQueue()
	pgcrsClickhouseQueue.Db = db
	pgcrsClickhouseQueue.Conn = conn
	go pgcrsClickhouseQueue.Register(1, false)

	pgcr_exists.CreateOutboundChannel(conn)
	bonusPgcrsFetchQueue := pgcr_exists.CreateFetchWorker()
	bonusPgcrsFetchQueue.Db = db
	bonusPgcrsFetchQueue.Conn = conn
	bonusPgcrsFetchQueue.Wg = &readonlyDestiny2ApiWg
	go bonusPgcrsFetchQueue.Register(5, true)

	bonusPgcrsStoreQueue := pgcr_exists.CreateStoreWorker()
	bonusPgcrsStoreQueue.Db = db
	bonusPgcrsStoreQueue.Conn = conn
	// 1 worker because it's a write operation with often related records which would cause deadlocks
	go bonusPgcrsStoreQueue.Register(1, false)

	var groupsApiWg sync.WaitGroup
	groupsApiWg.Add(1)
	readonlyGroupsApiWg := util.NewReadOnlyWaitGroup(&groupsApiWg)

	clanQueue := clan_crawl.Create()
	clanQueue.Db = db
	clanQueue.Conn = conn
	clanQueue.Wg = &readonlyGroupsApiWg
	go clanQueue.Register(1, true)

	cheatDetectionQueue := pgcr_cheat_check.Create()
	cheatDetectionQueue.Db = db
	cheatDetectionQueue.Conn = conn
	go cheatDetectionQueue.Register(1, true)

	pgcr_blocked.CreateOutboundChannel(conn)
	pgcrBlockedQueue := pgcr_blocked.Create()
	pgcrBlockedQueue.Db = db
	pgcrBlockedQueue.Conn = conn
	pgcrBlockedQueue.Wg = &readonlyDestiny2ApiWg
	// Before a Raid Race, set this number to be 200
	go pgcrBlockedQueue.Register(20, true)

	monitoring.RegisterHermes(8083)

	// Set up Bungie API monitoring
	go func() {
		destiny2Enabled := false
		groupsEnabled := false
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
					time.Sleep(30 * time.Second)
					continue
				}
			}

			if !res.Systems["Destiny2"].Enabled && destiny2Enabled {
				destiny2ApiWg.Add(1)
				destiny2Enabled = false
				log.Printf("Destiny 2 API is now disabled")
			} else if res.Systems["Destiny2"].Enabled && !destiny2Enabled {
				destiny2ApiWg.Add(-1)
				destiny2Enabled = true
				log.Printf("Destiny 2 API is now enabled")
			}

			if !res.Systems["Groups"].Enabled && groupsEnabled {
				groupsApiWg.Add(1)
				groupsEnabled = false
				log.Printf("Groups API is now disabled")
			} else if res.Systems["Groups"].Enabled && !groupsEnabled {
				groupsApiWg.Add(-1)
				groupsEnabled = true
				log.Printf("Groups API is now enabled")
			}

			time.Sleep(30 * time.Second)
		}
	}()

	// Keep the main thread running
	forever := make(chan bool)
	<-forever
}
