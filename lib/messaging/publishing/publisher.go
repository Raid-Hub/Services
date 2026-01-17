package publishing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"raidhub/lib/monitoring/global_metrics"
	"raidhub/lib/utils/logging"
	"raidhub/lib/utils/network"
	"raidhub/lib/utils/retry"

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

var publishingRetryConfig = retry.RetryConfig{
	MaxAttempts:  5,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	Multiplier:   1.25,
	Jitter:       0.05,
	OnRetry:      nil,
	ShouldRetry: func(err error) bool {
		netErr := network.CategorizeNetworkError(err)
		switch netErr.Type {
		case network.ErrorTypeTimeout, network.ErrorTypeConnection:
			return true
		default:
			return false
		}
	},
}

// publishWithTracking is a helper that publishes a message and tracks metrics/logs
func publishWithTracking(ctx context.Context, queueName string, publishMsg amqp.Publishing) error {
	publishMsg.DeliveryMode = amqp.Persistent

	err := retry.WithRetry(ctx, publishingRetryConfig, func(attempt int) error {
		return channel.PublishWithContext(ctx, "", queueName, false, false, publishMsg)
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
		Headers: amqp.Table{
			"x-retry-count": int32(0),
		},
	})
}

// PublishTextMessage publishes a text message to the specified queue
func PublishTextMessage(ctx context.Context, queueName string, text string) error {
	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(text),
		Headers: amqp.Table{
			"x-retry-count": int32(0),
		},
	})
}

// PublishInt64Message publishes an int64 message to the specified queue
func PublishInt64Message(ctx context.Context, queueName string, value int64) error {
	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "text/plain",
		Body:        fmt.Appendf(nil, "%d", value),
		Headers: amqp.Table{
			"x-retry-count": int32(0),
		},
	})
}
