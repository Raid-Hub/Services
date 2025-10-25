package clan_crawl

import (
	"context"
	"encoding/json"
	"raidhub/queue-workers"

	amqp "github.com/rabbitmq/amqp091-go"
)

type ClanRequest struct {
	GroupId int64 `json:"groupId,string"`
}

const queueName = "clan"

func Create() queueworkers.QueueWorker {
	return queueworkers.QueueWorker{
		QueueName: queueName,
		Processer: process_request,
	}
}

func SendMessage(ch *amqp.Channel, groupId int64) error {
	body, err := json.Marshal(ClanRequest{
		GroupId: groupId,
	})
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		"",        // exchange
		queueName, // routing key (queue name)
		true,      // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
