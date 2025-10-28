package routing

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher implements publisher.MessagePublisher using RabbitMQ
type RabbitMQPublisher struct {
	channel *amqp.Channel
}

// PublishJSONMessage publishes a JSON message to the specified queue
func (p *RabbitMQPublisher) PublishJSONMessage(queueName string, body any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return p.channel.PublishWithContext(
		context.Background(),
		"",        // default exchange
		queueName, // routing key = queue name
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         jsonBody,
			DeliveryMode: amqp.Persistent, // Make messages persistent
		},
	)
}

// PublishTextMessage publishes a text message to the specified queue
func (p *RabbitMQPublisher) PublishTextMessage(queueName string, text string) error {
	return p.channel.PublishWithContext(
		context.Background(),
		"",        // default exchange
		queueName, // routing key = queue name
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte(text),
			DeliveryMode: amqp.Persistent, // Make messages persistent
		},
	)
}
