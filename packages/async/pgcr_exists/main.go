package pgcr_exists

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"raidhub/packages/async"
	"raidhub/packages/bungie"
	"raidhub/packages/pgcr_types"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	outgoing *amqp.Channel
	once     sync.Once
)

func CreateOutboundChannel(conn *amqp.Connection) {
	once.Do(func() {
		outgoing, _ = conn.Channel()
	})
}

const fetchQueueName = "pgcr_exists"

func CreateFetchWorker() async.QueueWorker {
	client := &http.Client{}
	apiKey := os.Getenv("BUNGIE_API_KEY")

	qw := async.QueueWorker{
		QueueName: fetchQueueName,
		Processer: func(qw *async.QueueWorker, msg amqp.Delivery) {
			process_fetch_request(qw, msg, client, apiKey)
		},
	}

	return qw
}

func SendFetchMessage(ch *amqp.Channel, instanceId int64) error {
	body := fmt.Appendf(nil, "%d", instanceId)

	return ch.PublishWithContext(
		context.Background(),
		"",             // exchange
		fetchQueueName, // routing key (queue name)
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
}

const storeQueueName = "pgcr_store"

func CreateStoreWorker() async.QueueWorker {
	qw := async.QueueWorker{
		QueueName: storeQueueName,
		Processer: process_store_queue,
	}
	return qw
}

func sendStoreMessage(ch *amqp.Channel, activity *pgcr_types.ProcessedActivity, raw *bungie.DestinyPostGameCarnageReport) error {
	body, err := json.Marshal(PGCRStoreRequest{
		Activity: activity,
		Raw:      raw,
	})
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		"",             // exchange
		storeQueueName, // routing key (queue name)
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
