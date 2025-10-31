package queueworkers

import (
	"encoding/json"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/character"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// CharacterFillTopic creates a new character fill topic
func CharacterFillTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.CharacterFill,
		MinWorkers:            1,
		MaxWorkers:            15,
		DesiredWorkers:        3,
		ContestWeekendWorkers: 8,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processCharacterFill)
}

// processCharacterFill handles character fill messages
func processCharacterFill(worker processing.WorkerInterface, message amqp.Delivery) error {
	var request messages.CharacterFillMessage
	if err := json.Unmarshal(message.Body, &request); err != nil {
		worker.Error("Failed to unmarshal character fill request", map[string]any{logging.ERROR: err.Error()})
		return err
	}

	// Call character fill logic
	err := character.Fill(request.MembershipId, request.CharacterId, request.InstanceId)

	if err != nil {
		worker.Error("Failed to fill character", map[string]any{logging.ERROR: err.Error()})
		return err
	}

	return nil
}
