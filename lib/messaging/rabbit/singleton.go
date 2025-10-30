package rabbit

import (
	"fmt"
	"raidhub/lib/env"
	"raidhub/lib/utils/singleton"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	Conn     *amqp.Connection
	initDone <-chan struct{}
)

func init() {
	initDone = singleton.InitAsync("RABBITMQ", 10, func() error {
		rabbitURL := fmt.Sprintf("amqp://%s:%s@localhost:%s/", env.RabbitMQUser, env.RabbitMQPassword, env.RabbitMQPort)
		conn, err := amqp.Dial(rabbitURL)
		if err != nil {
			return err
		}
		Conn = conn
		return nil
	})
}

// Wait blocks until RabbitMQ initialization is complete
func Wait() {
	<-initDone
}
