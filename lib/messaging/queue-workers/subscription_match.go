// Stage 2 of the subscription pipeline: consume SubscriptionMatchMessage, run MatchEvent (rules +
// hydrate), marshal each SubscriptionDeliveryMessage, publish N to subscription_delivery.
// See lib/services/subscriptions/README.md.
package queueworkers

import (
	"time"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/subscriptions"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

func SubscriptionMatchTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:          routing.SubscriptionMatch,
		MinWorkers:         1,
		MaxWorkers:         10,
		DesiredWorkers:     2,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 10,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		MaxRetryCount:      10,
		RetryDelay:         processing.ExponentialRetryDelay(2 * time.Minute),
	}, processSubscriptionMatch)
}

func processSubscriptionMatch(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSONUnretryable[messages.SubscriptionMatchMessage](worker, message.Body)
	if err != nil {
		return err
	}

	worker.Info("PROCESSING_SUBSCRIPTION_MATCH", map[string]any{
		logging.INSTANCE_ID: request.InstanceId,
		logging.COUNT:       len(request.ParticipantData),
	})

	deliveryMessages, err := subscriptions.MatchEvent(worker.Context(), request)
	if err != nil {
		worker.Warn("SUBSCRIPTION_MATCH_FAILED", err, map[string]any{
			logging.INSTANCE_ID: request.InstanceId,
		})
		return err
	}

	if err := publishing.PublishJSONMessageBatchTx(worker.Context(), routing.SubscriptionDelivery, deliveryMessages); err != nil {
		worker.Warn("FAILED_TO_PUBLISH_SUBSCRIPTION_DELIVERY", err, map[string]any{
			logging.INSTANCE_ID: request.InstanceId,
		})
		return err
	}

	return nil
}
