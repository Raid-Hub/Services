package main

import (
	"context"
	"flag"
	"raidhub/lib/database/postgres"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/utils"
	qw "raidhub/queue-workers"
)

func main() {
	// Parse command line arguments
	var topicType = flag.String("topic", "", "Type of topic to run (player_crawl, pgcr_blocked_retry, activity_history_crawl, character_fill, clan_crawl, pgcr_crawl, instance_cheat_check, instance_store). If empty, starts all topics.")
	flag.Parse()

	// Create Hermes logger
	hermesLogger := utils.NewLogger("Hermes")

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
		qw.PgcrCrawlTopic(),
		qw.InstanceCheatCheckTopic(),
		qw.InstanceStoreTopic(),
	}

	if *topicType == "" {
		// Start all topics
		hermesLogger.Info("Starting all topics...")
		for _, t := range topics {
			tm, err := processing.StartTopicManager(t, processing.TopicManagerConfig{
				Context: ctx,
				Wg:      nil,
			})
			if err != nil {
				hermesLogger.ErrorF("Failed to start topic %s: %v", t.Config.QueueName, err)
				continue
			}
			hermesLogger.InfoF("Started %s topic", t.Config.QueueName)
			tm.InfoF("Started self-scaling worker initialWorkers=%d", tm.GetInitialWorkers())
		}
		hermesLogger.Info("All topics started. Press Ctrl+C to shutdown.")
	} else {
		// Start a specific topic
		hermesLogger.InfoF("Starting %s Topic", *topicType)

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
			hermesLogger.ErrorF("Unknown topic type: %s", *topicType)
			return
		}

		tm, err := processing.StartTopicManager(topicInstance, processing.TopicManagerConfig{
			Context: ctx,
		})
		if err != nil {
			hermesLogger.ErrorF("Failed to start topic: %v", err)
			return
		}

		hermesLogger.InfoF("%s Topic started. Press Ctrl+C to shutdown.", *topicType)
		tm.InfoF("Started self-scaling worker initialWorkers=%d", tm.GetInitialWorkers())
	}

	// Keep running
	select {}
}
