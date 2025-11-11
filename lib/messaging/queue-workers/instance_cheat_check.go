package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/cheat_detection"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// InstanceCheatCheckTopic creates a new instance cheat check topic
func InstanceCheatCheckTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.InstanceCheatCheck,
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
	}, processInstanceCheatCheck)
}

// processInstanceCheatCheck handles instance cheat check messages
func processInstanceCheatCheck(worker processing.WorkerInterface, message amqp.Delivery) error {
	instanceId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	// Call PGCR cheat check logic
	err = cheat_detection.CheckCheat(instanceId)
	if err != nil {
		worker.Error("Failed to check for cheat", err, map[string]any{logging.INSTANCE_ID: instanceId})
		return err
	}

	return nil
}
