package pgcr_cheat_check

import (
	"context"
	"encoding/json"
	"raidhub/packages/async"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PgcrCheatCheckRequest struct {
	InstanceId int64 `json:"instanceId,string"`
}

const queueName = "pgcr_cheat_check"

func Create() async.QueueWorker {
	return async.QueueWorker{
		QueueName: queueName,
		Processer: process_request,
	}
}

func SendMessage(ch *amqp.Channel, data int64) error {
	body, err := json.Marshal(PgcrCheatCheckRequest{
		InstanceId: data,
	})
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
