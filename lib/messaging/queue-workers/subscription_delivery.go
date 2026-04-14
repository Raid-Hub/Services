// Stage 3 of the subscription pipeline: consume SubscriptionDeliveryMessage, SendSubscriptionDelivery
// (HTTP POST only — URLs, EmbedPreload, and dto.Instance are filled in stage 2 match_preload).
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

// subscriptionDeliveryRetryDelay is the delay before the redelivered message is consumed when
// republishing with x-retry-count=newRetryCount. Counts 1–10: 5s; 11–15: 12s→60s; 16–17: 30m.
func subscriptionDeliveryRetryDelay(newRetryCount int) time.Duration {
	switch {
	case newRetryCount <= 10:
		return 5 * time.Second
	case newRetryCount <= 15:
		step := newRetryCount - 11 // 0..4 → 12s, 24s, 36s, 48s, 60s
		return time.Duration(12+step*12) * time.Second
	default:
		return 30 * time.Minute
	}
}

// SubscriptionDeliveryTopic POSTs outbound URLs (Discord webhooks or HTTPS JSON callbacks).
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
		// 17 failed attempts max: 10×5s delay, then 5 steps ramping to 60s, then 2×30m, then drop.
		MaxRetryCount: 17,
		RetryDelay:    subscriptionDeliveryRetryDelay,
	}, processSubscriptionDelivery)
}

// subscriptionDeliveryMaxRetries must match SubscriptionDeliveryTopic MaxRetryCount (final failure recording).
const subscriptionDeliveryMaxRetries = 17

func processSubscriptionDelivery(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSONUnretryable[messages.SubscriptionDeliveryMessage](worker, message.Body)
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

	retryCount := subscriptionDeliveryRetryCount(message.Headers)
	err = subscriptions.SendSubscriptionDelivery(worker.Context(), request)
	subscriptions.RecordDestinationDeliveryOutcome(worker.Context(), request.DestinationChannelId, err, retryCount, subscriptionDeliveryMaxRetries)
	if err != nil {
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

// subscriptionDeliveryRetryCount matches Hermes worker getRetryCount (x-retry-count, 0 = first try).
func subscriptionDeliveryRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}
	v, ok := headers["x-retry-count"]
	if !ok {
		return 0
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
	default:
		return 0
	}
	if n < 0 {
		return 0
	}
	return int(n)
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
