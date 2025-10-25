package character_fill

import (
	"context"
	"encoding/json"
	"raidhub/queue-workers"

	amqp "github.com/rabbitmq/amqp091-go"
)

type CharacterFillRequest struct {
	MembershipId int64 `json:"membershipId,string"`
	CharacterId  int64 `json:"characterId,string"`
	InstanceId   int64 `json:"instanceId,string"`
}

const queueName = "character_fill"

func Create() queueworkers.QueueWorker {
	return queueworkers.QueueWorker{
		QueueName: queueName,
		Processer: process_request,
	}
}

func SendMessage(ch *amqp.Channel, data *CharacterFillRequest) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		"",        // exchange
		queueName, // routing key (queue name)
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
