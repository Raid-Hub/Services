// Stage 3 of the subscription pipeline: consume SubscriptionDeliveryMessage, SendSubscriptionDelivery
// (HTTP POST). Messages must carry WebhookURL + EmbedPreload from subscription_match.
// See lib/services/subscriptions/README.md.
package queueworkers

import (
	"encoding/json"
	"time"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// SubscriptionDeliveryTopic POSTs Discord webhooks (payloads from subscription_match only).
func SubscriptionDeliveryTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:          routing.SubscriptionDelivery,
		MinWorkers:         1,
		MaxWorkers:         20,
		DesiredWorkers:     3,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 10,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		MaxRetryCount:      100,
		// Base delay before the first republish after failure; Hermes doubles this each retry (cap 30m).
		RetryDelay: 30 * time.Second,
	}, processSubscriptionDelivery)
}

func processSubscriptionDelivery(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSON[messages.SubscriptionDeliveryMessage](worker, message.Body)
	if err != nil {
		return err
	}

	attempt := deliveryAttemptNumber(message.Headers)

	infoFields := map[string]any{
		logging.INSTANCE_ID: request.InstanceId,
		"channel_id":        request.DestinationChannelId,
	}
	if attempt > 1 {
		infoFields[logging.ATTEMPT] = attempt
	}
	worker.Info("PROCESSING_SUBSCRIPTION_DELIVERY", infoFields)

	if err := subscriptions.SendSubscriptionDelivery(worker.Context(), request); err != nil {
		worker.Warn("SUBSCRIPTION_DELIVERY_FAILED", err, map[string]any{
			logging.INSTANCE_ID: request.InstanceId,
			"channel_id":        request.DestinationChannelId,
			logging.ATTEMPT:     attempt,
			logging.QUEUE:       routing.SubscriptionDelivery,
		})
		return err
	}

	return nil
}

// deliveryAttemptNumber maps broker x-retry-count (0 = first try) to a 1-based attempt for logs.
func deliveryAttemptNumber(headers amqp.Table) int {
	v, ok := headers["x-retry-count"]
	if !ok {
		return 1
	}
	var n int64
	switch x := v.(type) {
	case int32:
		n = int64(x)
	case int64:
		n = x
	case int:
		n = int64(x)
	case uint8:
		n = int64(x)
	case uint16:
		n = int64(x)
	case uint32:
		n = int64(x)
	case uint64:
		n = int64(x)
	case float64:
		n = int64(x)
	case json.Number:
		parsed, err := x.Int64()
		if err != nil {
			return 1
		}
		n = parsed
	default:
		return 1
	}
	if n < 0 {
		return 1
	}
	return int(n) + 1
}
