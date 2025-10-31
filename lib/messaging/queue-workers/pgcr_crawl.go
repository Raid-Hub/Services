package queueworkers

import (
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/instance"
	"raidhub/lib/utils/logging"

	amqp "github.com/rabbitmq/amqp091-go"
)

// PgcrCrawlTopic creates a new PGCR crawl topic
func PgcrCrawlTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PGCRCrawl,
		MinWorkers:            1,
		MaxWorkers:            20,
		DesiredWorkers:        2,
		ContestWeekendWorkers: 5,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPgcrCrawl)
}

// processPgcrCrawl handles PGCR crawl messages
func processPgcrCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	instanceId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	// Check if instance exists
	exists, err := instance.CheckExists(instanceId)
	if err == nil {
		if exists {
			worker.Info("INSTANCE_EXISTS_IN_DATABASE", map[string]any{logging.INSTANCE_ID: instanceId})
		} else {
			worker.Info("INSTANCE_DOES_NOT_EXIST_IN_DATABASE", map[string]any{logging.INSTANCE_ID: instanceId})
		}
	}

	if err != nil {
		worker.Error("FAILED_TO_CHECK_PGCR_EXISTENCE", map[string]any{logging.ERROR: err.Error()})
		return err
	}

	return nil
}
