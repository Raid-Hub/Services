package activity_history

import (
	"context"
	"fmt"
	"raidhub/queue-workers"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "activity_history"

func Create() queueworkers.QueueWorker {
	create_outbound_channel()
	return queueworkers.QueueWorker{
		QueueName: queueName,
		Processer: process_request,
	}
}

func SendMessage(ch *amqp.Channel, membershipId int64) error {
	body := fmt.Appendf(nil, "%d", membershipId)

	return ch.PublishWithContext(
		context.Background(),
		"",        // exchange
		queueName, // routing key (queue name)
		true,      // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
}
