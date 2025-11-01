package publishing

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	channel *amqp.Channel
)

// PublishJSONMessage publishes a JSON message to the specified queue
func PublishJSONMessage(ctx context.Context, queueName string, body any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return channel.PublishWithContext(
		ctx,
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
func PublishTextMessage(ctx context.Context, queueName string, text string) error {
	return channel.PublishWithContext(
		ctx,
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

// PublishInt64Message publishes an int64 message to the specified queue
func PublishInt64Message(ctx context.Context, queueName string, value int64) error {
	return channel.PublishWithContext(
		ctx,
		"",        // default exchange
		queueName, // routing key = queue name
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "text/plain",
			Body:         []byte(fmt.Sprintf("%d", value)),
			DeliveryMode: amqp.Persistent, // Make messages persistent
		},
	)
}
