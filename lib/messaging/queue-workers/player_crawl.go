package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PlayerCrawlTopic creates a new player crawl topic
func PlayerCrawlTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PlayerCrawl,
		MinWorkers:            5,
		MaxWorkers:            70,
		DesiredWorkers:        20,
		ContestWeekendWorkers: 40,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPlayerCrawl)
}

// processPlayerCrawl handles player crawl messages
func processPlayerCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	membershipIdStr := string(message.Body)
	membershipId, err := strconv.ParseInt(membershipIdStr, 10, 64)
	if err != nil {
		worker.Warn("MEMBERSHIP_ID_PARSE_ERROR", map[string]any{
			logging.ERROR: err.Error(),
		})
		return err
	}

	// Call player crawl logic
	err = player.Crawl(worker.Context(), membershipId)

	if err != nil {
		worker.Warn("PLAYER_CRAWL_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
			logging.ERROR:         err.Error(),
		})
		return err
	}

	worker.Debug("PLAYER_CRAWL_COMPLETE", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.STATUS:        "success",
	})
	return nil
}
