package queueworkers

import (
	"raidhub/lib/domains/cheat_detection"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PgcrCheatCheckTopic creates a new PGCR cheat check topic
func PgcrCheatCheckTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PGCRCheatCheck,
		MinWorkers:            1,
		MaxWorkers:            10,
		DesiredWorkers:        2,
		ContestWeekendWorkers: 5,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPgcrCheatCheck)
}

// processPgcrCheatCheck handles PGCR cheat check messages
func processPgcrCheatCheck(worker *processing.Worker, message amqp.Delivery) error {
	instanceIdStr := string(message.Body)
	instanceId, err := strconv.ParseInt(instanceIdStr, 10, 64)
	if err != nil {
		worker.Error("Failed to parse instance ID", "error", err)
		return err
	}

	// Call PGCR cheat check logic
	err = cheat_detection.CheckCheat(instanceId)

	if err != nil {
		worker.Error("Failed to check for cheat", "error", err)
		return err
	}

	return nil
}
