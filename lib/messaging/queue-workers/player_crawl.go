package queueworkers

import (
	"time"

	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"

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
		ScaleUpPercent:        0.5, // Add 50% more workers (more aggressive)
		ScaleDownPercent:      0.1,
		MinWorkersPerStep:     3,
		MaxWorkersPerStep:     25,               // Can add up to 25 workers at once (more aggressive)
		ConsecutiveChecksUp:   1,                // Scale up after just 1 check (immediate)
		ConsecutiveChecksDown: 3,                // More conservative for scale-down
		ScaleCooldown:         30 * time.Second, // Shorter cooldown for faster scaling
		BungieSystemDeps:      []string{"Destiny2", "D2Profiles", "Activities"},
		MaxRetryCount:         12, // Important for player data collection
	}, processPlayerCrawl)
}

// processPlayerCrawl handles player crawl messages
func processPlayerCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	membershipId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	// Call player crawl logic
	err = player.Crawl(worker.Context(), membershipId)

	if err != nil {
		worker.Warn("PLAYER_CRAWL_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}

	worker.Debug("PLAYER_CRAWL_COMPLETE", map[string]any{
		logging.MEMBERSHIP_ID: membershipId,
		logging.STATUS:        "success",
	})
	return nil
}
