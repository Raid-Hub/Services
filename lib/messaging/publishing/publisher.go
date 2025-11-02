package publishing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/network"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	channel *amqp.Channel
	logger  = logging.NewLogger("PUBLISHING_SERVICE")
)

const (
	ERROR   = "error"
	SUCCESS = "success"
)

// publishWithTracking is a helper that publishes a message and tracks metrics/logs
func publishWithTracking(ctx context.Context, queueName string, publishMsg amqp.Publishing) error {
	publishMsg.DeliveryMode = amqp.Persistent

	config := network.DefaultRetryConfig()
	config.MaxAttempts = 5
	config.InitialDelay = 200 * time.Millisecond

	err := network.WithRetry(ctx, config, func() error {
		return channel.PublishWithContext(
			ctx,
			"",        // default exchange
			queueName, // routing key = queue name
			false,     // mandatory
			false,     // immediate
			publishMsg,
		)
	})

	if err != nil {
		global_metrics.PublishingOperations.WithLabelValues(queueName, ERROR).Inc()
		logger.Error("PUBLISH_FAILED", err, map[string]any{
			logging.QUEUE: queueName,
		})
	} else {
		global_metrics.PublishingOperations.WithLabelValues(queueName, SUCCESS).Inc()
	}

	return err
}

// PublishJSONMessage publishes a JSON message to the specified queue
func PublishJSONMessage(ctx context.Context, queueName string, body any) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		global_metrics.PublishingOperations.WithLabelValues(queueName, ERROR).Inc()
		logger.Error("PUBLISH_MARSHAL_FAILED", err, map[string]any{
			logging.QUEUE: queueName,
		})
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonBody,
	})
}

// PublishTextMessage publishes a text message to the specified queue
func PublishTextMessage(ctx context.Context, queueName string, text string) error {
	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(text),
	})
}

// PublishInt64Message publishes an int64 message to the specified queue
func PublishInt64Message(ctx context.Context, queueName string, value int64) error {
	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "text/plain",
		Body:        fmt.Appendf(nil, "%d", value),
	})
}
