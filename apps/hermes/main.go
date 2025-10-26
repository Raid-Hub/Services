package main

import (
	"context"
	"flag"
	"log"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/rabbit"
	qw "raidhub/queue-workers"
)

func main() {
	// Parse command line arguments
	var topicType = flag.String("topic", "", "Type of topic to run (player_crawl, pgcr_blocked, activity_history, etc.). If empty, starts all topics.")
	flag.Parse()

	log.SetFlags(0)

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Import packages to trigger singleton initialization
	_ = postgres.DB
	_ = rabbit.Conn

	topics := []processing.Topic{
		qw.PlayerCrawlTopic(),
		qw.PgcrBlockedTopic(),
		qw.ActivityHistoryTopic(),
		qw.CharacterFillTopic(),
		qw.ClanCrawlTopic(),
		qw.PgcrExistsTopic(),
		qw.PgcrCheatCheckTopic(),
		qw.PgcrStoreTopic(),
	}

	if *topicType == "" {
		// Start all topics
		log.Printf("Starting all topics...")
		for _, t := range topics {
			err := processing.StartTopicManager(t, processing.TopicManagerConfig{
				Context: ctx,
				Wg:      nil,
			})
			if err != nil {
				log.Printf("Failed to start topic %s: %v", t.Config.QueueName, err)
				continue
			}
			log.Printf("Started %s topic", t.Config.QueueName)
		}
		log.Printf("All topics started. Press Ctrl+C to shutdown.")
	} else {
		// Start a specific topic
		log.Printf("Starting %s Topic", *topicType)

		var topicInstance processing.Topic
		var found bool
		for _, t := range topics {
			if t.Config.QueueName == *topicType {
				topicInstance = t
				found = true
				break
			}
		}

		if !found {
			log.Fatalf("Unknown topic type: %s", *topicType)
		}

		err := processing.StartTopicManager(topicInstance, processing.TopicManagerConfig{
			Context: ctx,
		})
		if err != nil {
			log.Fatal("Failed to start topic:", err)
		}

		log.Printf("%s Topic started. Press Ctrl+C to shutdown.", *topicType)
	}

	// Keep running
	select {}
}
