package queueworkers

import (
	"encoding/json"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/utils/logging"

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
		worker.Error("FAILED_TO_UNMARSHAL_PGCR_STORE_MESSAGE", map[string]any{
			logging.ERROR: err.Error(),
		})
		return err
	}

	worker.Debug("PROCESSING_INSTANCE_STORE", map[string]any{logging.INSTANCE_ID: msg.Activity.InstanceId})

	// Store the PGCR using the orchestrator
	lag, isNew, err := instance_storage.StorePGCR(&msg.Activity, &msg.Raw)
	if err != nil {
		worker.Warn(instance_storage.FAILED_TO_STORE_INSTANCE, map[string]any{logging.INSTANCE_ID: msg.Activity.InstanceId, logging.ERROR: err.Error()})
		// Write to missed log for storage failures - if successful, ACK the message since it's tracked
		instance_storage.WriteMissedLog(msg.Activity.InstanceId)
		return nil
	}

	if isNew {
		worker.Info(instance_storage.STORED_NEW_INSTANCE, map[string]any{
			logging.INSTANCE_ID: msg.Activity.InstanceId,
			logging.LAG:         lag,
		})
	} else {
		worker.Debug(instance_storage.DUPLICATE_INSTANCE, map[string]any{
			logging.INSTANCE_ID: msg.Activity.InstanceId,
		})
	}

	return nil
}
