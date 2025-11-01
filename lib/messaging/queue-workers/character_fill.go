package queueworkers

import (
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
		BungieSystemDeps:      []string{"Destiny2", "D2Characters"},
	}, processCharacterFill)
}

// processCharacterFill handles character fill messages
func processCharacterFill(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSON[messages.CharacterFillMessage](worker, message.Body)
	if err != nil {
		return err
	}

	// Call character fill logic
	err = character.Fill(request.MembershipId, request.CharacterId, request.InstanceId)

	if err != nil {
		worker.Error("FAILED_TO_UPDATE_CHARACTER", map[string]any{logging.ERROR: err.Error()})
		return err
	}

	return nil
}
