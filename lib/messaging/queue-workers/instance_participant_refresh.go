// Stage 1 of the subscription pipeline: consume SubscriptionEventMessage, run PrepareParticipants,
// publish SubscriptionMatchMessage. See lib/services/subscriptions/README.md.
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

func InstanceParticipantRefreshTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:          routing.InstanceParticipantRefresh,
		MinWorkers:         1,
		MaxWorkers:         10,
		DesiredWorkers:     2,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 10,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		BungieSystemDeps:   []string{"Destiny2", "D2Profiles", "Groups", "Clans"},
		MaxRetryCount:      10,
		RetryDelay:         processing.ExponentialRetryDelay(2 * time.Minute),
	}, processInstanceParticipantRefresh)
}

func processInstanceParticipantRefresh(worker processing.WorkerInterface, message amqp.Delivery) error {
	request, err := processing.ParseJSONUnretryable[messages.SubscriptionEventMessage](worker, message.Body)
	if err != nil {
		return err
	}

	worker.Info("PROCESSING_INSTANCE_PARTICIPANT_REFRESH", map[string]any{
		logging.INSTANCE_ID: request.InstanceId,
		logging.COUNT:       request.ParticipantCount,
	})

	matchMessage, err := subscriptions.PrepareParticipants(worker.Context(), request)
	if err != nil {
		worker.Warn("INSTANCE_PARTICIPANT_REFRESH_FAILED", err, map[string]any{
			logging.INSTANCE_ID: request.InstanceId,
		})
		return err
	}

	if matchMessage.InstanceId == 0 {
		return nil
	}

	if err := publishing.PublishJSONMessage(worker.Context(), routing.SubscriptionMatch, matchMessage); err != nil {
		worker.Warn("FAILED_TO_PUBLISH_SUBSCRIPTION_MATCH", err, map[string]any{
			logging.INSTANCE_ID: request.InstanceId,
		})
		return err
	}

	return nil
}
