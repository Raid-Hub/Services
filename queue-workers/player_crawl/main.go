package player_crawl

import (
	"context"
	"encoding/json"
	"raidhub/queue-workers"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PlayerRequest struct {
	MembershipId int64 `json:"membershipId,string"`
}

const queueName = "player_requests"

func Create() queueworkers.QueueWorker {
	return queueworkers.QueueWorker{
		QueueName: queueName,
		Processer: process_player_request,
	}
}

func SendMessage(ch *amqp.Channel, membershipId int64) error {
	body, err := json.Marshal(PlayerRequest{
		MembershipId: membershipId,
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

var (
	outgoing *amqp.Channel
	once     sync.Once
)

func CreateOutboundChannel(conn *amqp.Connection) {
	once.Do(func() {
		outgoing, _ = conn.Channel()
	})
}
