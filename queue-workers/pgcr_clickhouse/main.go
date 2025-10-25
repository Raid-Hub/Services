package pgcr_clickhouse

import (
	"context"
	"encoding/json"
	"log"
	"raidhub/queue-workers"
	"raidhub/shared/database/clickhouse"
	"raidhub/shared/pgcr_types"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "pgcr_clickhouse"

func CreateClickhouseQueue() queueworkers.QueueWorker {
	client, err := clickhouse.Connect(false)
	if err != nil {
		log.Fatal("Error connecting to clickhouse", err)
	}

	ch := make(chan amqp.Delivery)
	qw := queueworkers.QueueWorker{
		QueueName: queueName,
		Processer: func(qw *queueworkers.QueueWorker, msg amqp.Delivery) {
			ch <- msg
		},
	}
	go process_queue(&client, ch)

	return qw
}

func SendToClickhouse(ch *amqp.Channel, activity *pgcr_types.ProcessedActivity) error {
	body, err := json.Marshal(activity)
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
