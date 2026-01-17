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
		QueueName:          routing.CharacterFill,
		MinWorkers:         2,
		MaxWorkers:         15,
		DesiredWorkers:     3,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 10,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		BungieSystemDeps:   []string{"Destiny2", "D2Characters"},
		MaxRetryCount:      4, // Character data is useful but not critical
	}, processCharacterFill)
}

// processCharacterFill handles character fill messages
func processCharacterFill(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSON[messages.CharacterFillMessage](worker, message.Body)
	if err != nil {
		return err
	}
	fields := map[string]any{
		logging.MEMBERSHIP_ID: request.MembershipId,
		logging.CHARACTER_ID:  request.CharacterId,
		logging.INSTANCE_ID:   request.InstanceId,
	}
	worker.Debug("PROCESSING_CHARACTER_FILL", fields)

	// Call character fill logic
	wasFilled, err := character.Fill(worker.Context(), request.MembershipId, request.CharacterId, request.InstanceId)

	if !wasFilled && err != nil {
		worker.Warn("FAILED_TO_UPDATE_CHARACTER", err, map[string]any{
			logging.MEMBERSHIP_ID: request.MembershipId,
			logging.CHARACTER_ID:  request.CharacterId,
			logging.INSTANCE_ID:   request.InstanceId,
		})
		return err
	}

	return nil
}
