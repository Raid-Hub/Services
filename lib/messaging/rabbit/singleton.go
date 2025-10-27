package rabbit

import (
	"fmt"
	"log"
	"time"

	"raidhub/lib/env"

	amqp "github.com/rabbitmq/amqp091-go"
)

var Conn *amqp.Connection

func init() {
	// Retry connection with backoff
	maxRetries := 10
	var err error
	for i := 0; i < maxRetries; i++ {
		Conn, err = connect()
		if err == nil {
			log.Printf("RabbitMQ connected")
			return
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	log.Fatalf("Failed to connect to RabbitMQ after %d retries: %v", maxRetries, err)
}

func connect() (*amqp.Connection, error) {
	rabbitURL := fmt.Sprintf("amqp://%s:%s@localhost:%s/", env.RabbitMQUser, env.RabbitMQPassword, env.RabbitMQPort)
	return amqp.Dial(rabbitURL)
}
