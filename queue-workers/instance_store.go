package queueworkers

import (
	"encoding/json"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/instance_storage"

	amqp "github.com/rabbitmq/amqp091-go"
)

// InstanceStoreTopic creates a new instance store topic
func InstanceStoreTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.InstanceStore,
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
	}, processInstanceStore)
}

// processInstanceStore handles instance store messages
func processInstanceStore(worker *processing.Worker, message amqp.Delivery) error {
	var msg messages.PGCRStoreMessage
	if err := json.Unmarshal(message.Body, &msg); err != nil {
		worker.ErrorF("Failed to unmarshal PGCR store message: %v", err)
		return err
	}

	worker.InfoF("Processing PGCR store instanceId=%d", msg.Activity.InstanceId)

	// Store the PGCR using the orchestrator
	lag, isNew, err := instance_storage.StorePGCR(&msg.Activity, &msg.Raw)
	if err != nil {
		worker.ErrorF("Failed to store PGCR instanceId=%d: %v", msg.Activity.InstanceId, err)
		return err
	}

	if isNew {
		worker.InfoF("Successfully stored new PGCR instanceId=%d lag=%v", msg.Activity.InstanceId, lag)
	} else {
		worker.InfoF("Duplicate PGCR instanceId=%d", msg.Activity.InstanceId)
	}

	return nil
}
