package routing

import (
	"raidhub/lib/messaging/rabbit"
)

var (
	Publisher *RabbitMQPublisher
)

func init() {
	ch, err := rabbit.Conn.Channel()
	if err != nil {
		panic("Failed to create publisher channel: " + err.Error())
	}
	Publisher = &RabbitMQPublisher{
		channel: ch,
	}
}
