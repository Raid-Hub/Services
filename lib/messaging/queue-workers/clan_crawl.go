package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/clan"
	"raidhub/lib/utils/logging"

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
		BungieSystemDeps:      []string{"Groups", "Clans", "Destiny2"},
		MaxRetryCount:         8, // Less critical than player/instance data
	}, processClanCrawl)
}

// processClanCrawl handles clan crawl messages
func processClanCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	groupId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	// Call clan crawl logic
	err = clan.Crawl(groupId)

	if err != nil {
		worker.Error("CLAN_CRAWL_FAILED", err, map[string]any{
			logging.GROUP_ID: groupId,
		})
		return err
	}

	return nil
}
