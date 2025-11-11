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
	initDone = singleton.InitAsync("RABBITMQ", 10, map[string]any{
		"user": env.RabbitMQUser,
		"host": env.RabbitMQHost,
		"port": env.RabbitMQPort,
	}, func() error {
		rabbitURL := fmt.Sprintf("amqp://%s:%s@%s:%s/", env.RabbitMQUser, env.RabbitMQPassword, env.RabbitMQHost, env.RabbitMQPort)
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
