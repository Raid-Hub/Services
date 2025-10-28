package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/player"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PlayerCrawlTopic creates a new player crawl topic
func PlayerCrawlTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PlayerCrawl,
		MinWorkers:            5,
		MaxWorkers:            100,
		DesiredWorkers:        20,
		ContestWeekendWorkers: 50,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPlayerCrawl)
}

// processPlayerCrawl handles player crawl messages
func processPlayerCrawl(worker *processing.Worker, message amqp.Delivery) error {
	membershipIdStr := string(message.Body)
	membershipId, err := strconv.ParseInt(membershipIdStr, 10, 64)
	if err != nil {
		worker.Error("Failed to parse membership ID", "error", err)
		return err
	}

	worker.Info("Processing player request", "membershipId", membershipId)

	// Call player crawl logic
	err = player.Crawl(worker.Config.Context, membershipId)

	if err != nil {
		worker.Error("Failed to crawl player", "error", err)
		return err
	}

	worker.Info("Successfully crawled player")
	return nil
}
