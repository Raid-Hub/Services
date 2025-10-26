package queueworkers

import (
	"raidhub/lib/domains/instance"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PgcrExistsTopic creates a new PGCR exists topic
func PgcrExistsTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PGCRExists,
		MinWorkers:            1,
		MaxWorkers:            20,
		DesiredWorkers:        2,
		ContestWeekendWorkers: 5,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPgcrExists)
}

// processPgcrExists handles PGCR exists messages
func processPgcrExists(worker *processing.Worker, message amqp.Delivery) error {
	instanceIdStr := string(message.Body)
	instanceId, err := strconv.ParseInt(instanceIdStr, 10, 64)
	if err != nil {
		worker.Error("Failed to parse instance ID", "error", err)
		return err
	}

	// Check if instance exists
	exists, err := instance.CheckExists(instanceId)
	if err == nil {
		if exists {
			worker.Info("Instance exists in database", "instanceId", instanceId)
		} else {
			worker.Info("Instance does not exist in database", "instanceId", instanceId)
		}
	}

	if err != nil {
		worker.Error("Failed to check PGCR existence", "error", err)
		return err
	}

	return nil
}
