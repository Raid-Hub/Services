package main

import (
	"log"
	"raidhub/queue-workers/activity_history"
	"raidhub/queue-workers/character_fill"
	"raidhub/queue-workers/clan_crawl"
	"raidhub/queue-workers/pgcr_blocked"
	"raidhub/queue-workers/pgcr_cheat_check"
	"raidhub/queue-workers/pgcr_clickhouse"
	"raidhub/queue-workers/pgcr_exists"
	"raidhub/queue-workers/player_crawl"
	"raidhub/shared/bungie"
	"raidhub/shared/database/postgres"
	"raidhub/shared/messaging/rabbit"
	"raidhub/shared/monitoring"
	"raidhub/shared/utils"
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
	readonlyDestiny2ApiWg := utils.NewReadOnlyWaitGroup(&destiny2ApiWg)

	activityHistoryQueue := activity_history.Create()
	activityHistoryQueue.Conn = conn
	activityHistoryQueue.Db = db
	activityHistoryQueue.Wg = &readonlyDestiny2ApiWg
	go activityHistoryQueue.Register(1, true)

	player_crawl.CreateOutboundChannel(conn)
	playersQueue := player_crawl.Create()
	playersQueue.Db = db
	playersQueue.Conn = conn
	playersQueue.Wg = &readonlyDestiny2ApiWg
	go playersQueue.Register(50, true)

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
	// undo change to 0 later
	go bonusPgcrsFetchQueue.Register(1, true)

	bonusPgcrsStoreQueue := pgcr_exists.CreateStoreWorker()
	bonusPgcrsStoreQueue.Db = db
	bonusPgcrsStoreQueue.Conn = conn
	// 1 worker because it's a write operation with often related records which would cause deadlocks
	go bonusPgcrsStoreQueue.Register(1, false)

	var groupsApiWg sync.WaitGroup
	groupsApiWg.Add(1)
	readonlyGroupsApiWg := utils.NewReadOnlyWaitGroup(&groupsApiWg)

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
	go pgcrBlockedQueue.Register(200, true)

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
