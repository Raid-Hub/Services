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
	}, processActivityHistory)
}

// processActivityHistory handles activity history messages
func processActivityHistory(worker *processing.Worker, message amqp.Delivery) error {
	membershipIdStr := string(message.Body)

	// Call player activity history logic
	err := player.UpdateActivityHistory(membershipIdStr)

	if err != nil {
		worker.Error("ACTIVITY_HISTORY_PROCESSING_ERROR", map[string]any{
			logging.MEMBERSHIP_ID: membershipIdStr,
			logging.ERROR:         err.Error(),
		})
		return err
	}

	return nil
}
