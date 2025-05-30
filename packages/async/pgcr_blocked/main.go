package pgcr_blocked

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"raidhub/packages/async"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	outgoing *amqp.Channel
	once     sync.Once
)

func CreateOutboundChannel(conn *amqp.Connection) *amqp.Channel {
	once.Do(func() {
		outgoing, _ = conn.Channel()
	})

	return outgoing
}

const fetchQueueName = "pgcr_blocked"

func Create() async.QueueWorker {
	client := &http.Client{}
	apiKey := os.Getenv("BUNGIE_API_KEY")

	qw := async.QueueWorker{
		QueueName: fetchQueueName,
		Processer: func(qw *async.QueueWorker, msg amqp.Delivery) {
			process_request(qw, msg, client, apiKey)
		},
	}

	return qw
}

func SendMessage(ch *amqp.Channel, instanceId int64) error {
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
