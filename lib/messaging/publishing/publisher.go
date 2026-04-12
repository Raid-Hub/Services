package publishing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"raidhub/lib/messaging/rabbit"
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

// PublishJSONBytes publishes pre-marshaled JSON with the same headers as PublishJSONMessage.
func PublishJSONBytes(ctx context.Context, queueName string, jsonBody []byte) error {
	return publishWithTracking(ctx, queueName, amqp.Publishing{
		ContentType: "application/json",
		Body:        jsonBody,
		Headers: amqp.Table{
			"x-retry-count": int32(0),
		},
	})
}

// PublishJSONBytesBatchTx publishes multiple messages in one AMQP transaction so either all are
// enqueued or none (avoids duplicate subscription_delivery notifications if a later publish fails).
func PublishJSONBytesBatchTx(ctx context.Context, queueName string, bodies [][]byte) error {
	if len(bodies) == 0 {
		return nil
	}

	err := retry.WithRetry(ctx, publishingRetryConfig, func(attempt int) error {
		ch, err := rabbit.Conn.Channel()
		if err != nil {
			return err
		}
		defer ch.Close()

		if err := ch.Tx(); err != nil {
			return err
		}
		for _, jsonBody := range bodies {
			pub := amqp.Publishing{
				ContentType:  "application/json",
				Body:         jsonBody,
				DeliveryMode: amqp.Persistent,
				Headers: amqp.Table{
					"x-retry-count": int32(0),
				},
			}
			if err := ch.PublishWithContext(ctx, "", queueName, false, false, pub); err != nil {
				_ = ch.TxRollback()
				return err
			}
		}
		if err := ch.TxCommit(); err != nil {
			_ = ch.TxRollback()
			return err
		}
		return nil
	})

	if err != nil {
		global_metrics.PublishingOperations.WithLabelValues(queueName, ERROR).Inc()
		logger.Error("PUBLISH_BATCH_TX_FAILED", err, map[string]any{
			logging.QUEUE: queueName,
		})
	} else {
		for range bodies {
			global_metrics.PublishingOperations.WithLabelValues(queueName, SUCCESS).Inc()
		}
	}

	return err
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
