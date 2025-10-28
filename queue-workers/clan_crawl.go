package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/clan"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ClanCrawlTopic creates a new clan crawl topic
func ClanCrawlTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.ClanCrawl,
		MinWorkers:            1,
		MaxWorkers:            5,
		DesiredWorkers:        1,
		ContestWeekendWorkers: 2,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processClanCrawl)
}

// processClanCrawl handles clan crawl messages
func processClanCrawl(worker *processing.Worker, message amqp.Delivery) error {
	groupIdStr := string(message.Body)
	groupId, err := strconv.ParseInt(groupIdStr, 10, 64)
	if err != nil {
		worker.Error("Failed to parse group ID", "error", err)
		return err
	}

	// Call clan crawl logic
	err = clan.Crawl(groupId)

	if err != nil {
		worker.Error("Failed to crawl clan", "error", err)
		return err
	}

	return nil
}
