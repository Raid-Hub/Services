package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/player"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ActivityHistoryTopic creates a new activity history topic
func ActivityHistoryTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.ActivityCrawl,
		MinWorkers:            1,
		MaxWorkers:            20,
		DesiredWorkers:        3,
		ContestWeekendWorkers: 10,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
		BungieSystemDeps:      []string{"Destiny2", "Activities", "D2Profiles"},
		MaxRetryCount:         12, // Important for activity history data
	}, processActivityHistory)
}

// processActivityHistory handles activity history messages
func processActivityHistory(worker processing.WorkerInterface, message amqp.Delivery) error {
	membershipId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	// Call player activity history logic
	err = player.UpdateActivityHistory(membershipId)

	if err != nil {
		worker.Error("ACTIVITY_HISTORY_PROCESSING_ERROR", err, map[string]any{
			logging.MEMBERSHIP_ID: membershipId,
		})
		return err
	}

	return nil
}
