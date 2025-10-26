package queueworkers

import (
	"encoding/json"
	"raidhub/lib/domains/instance_storage"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PgcrStoreTopic creates a new PGCR store topic
func PgcrStoreTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PGCRStore,
		MinWorkers:            1,
		MaxWorkers:            50,
		DesiredWorkers:        10,
		ContestWeekendWorkers: 20,
		KeepInReady:           false,
		PrefetchCount:         1,
		ScaleUpThreshold:      1000,
		ScaleDownThreshold:    100,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPgcrStore)
}

// processPgcrStore handles PGCR store messages
func processPgcrStore(worker *processing.Worker, message amqp.Delivery) error {
	var msg messages.PGCRStoreMessage
	if err := json.Unmarshal(message.Body, &msg); err != nil {
		worker.Error("Failed to unmarshal PGCR store message", "error", err)
		return err
	}

	worker.Info("Processing PGCR store", "instanceId", msg.Activity.InstanceId)

	// Store the PGCR using the orchestrator
	lag, isNew, err := instance_storage.StorePGCR(&msg.Activity, &msg.Raw)
	if err != nil {
		worker.Error("Failed to store PGCR", "error", err)
		return err
	}

	if isNew {
		worker.Info("Successfully stored new PGCR",
			"instanceId", msg.Activity.InstanceId,
			"lag", lag)
	} else {
		worker.Info("Duplicate PGCR", "instanceId", msg.Activity.InstanceId)
	}

	return nil
}
