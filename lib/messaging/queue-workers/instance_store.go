package queueworkers

import (
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
		MaxRetryCount:         15, // Critical - storing data, but DB errors might be permanent
	}, processInstanceStore)
}

// processInstanceStore handles instance store messages
func processInstanceStore(worker processing.WorkerInterface, message amqp.Delivery) error {
	msg, err := processing.ParseJSON[messages.PGCRStoreMessage](worker, message.Body)
	if err != nil {
		return err
	}
	worker.Debug("PROCESSING_INSTANCE_STORE", map[string]any{logging.INSTANCE_ID: msg.Instance.InstanceId})

	// Store the PGCR using the orchestrator
	_, _, err = instance_storage.StorePGCR(&msg.Instance, &msg.PGCR)
	if err != nil {
		instance_storage.WriteMissedLog(msg.Instance.InstanceId)
		return nil
	}
	worker.Debug("INSTANCE_STORE_MESSAGE_PROCESSED", map[string]any{logging.INSTANCE_ID: msg.Instance.InstanceId})
	return nil
}
