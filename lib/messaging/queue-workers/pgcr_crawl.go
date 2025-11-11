package queueworkers

import (
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/instance"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
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
		BungieSystemDeps:      []string{"Destiny2"},
	}, processPgcrCrawl)
}

// processPgcrCrawl handles PGCR crawl messages
// It fetches and processes PGCRs from the Bungie API, then stores them if they're raids
func processPgcrCrawl(worker processing.WorkerInterface, message amqp.Delivery) error {
	instanceId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		return err
	}

	worker.Debug("PGCR_CRAWL_STARTED", map[string]any{logging.INSTANCE_ID: instanceId})

	// Check if instance already exists in database
	exists, err := instance.CheckExists(instanceId)
	if err != nil {
		worker.Error("FAILED_TO_CHECK_PGCR_EXISTENCE", err, map[string]any{
			logging.INSTANCE_ID: instanceId,
		})
		return err
	}

	if exists {
		worker.Debug("INSTANCE_ALREADY_EXISTS", map[string]any{logging.INSTANCE_ID: instanceId})
		return nil // Already stored, no need to fetch again
	}

	// Fetch and process the PGCR from Bungie API
	result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(instanceId)

	switch result {
	case pgcr_processing.Success:
		// Successfully fetched and processed a raid PGCR - publish to storage queue
		storeMessage := messages.NewPGCRStoreMessage(instance, pgcr)
		if publishErr := publishing.PublishJSONMessage(worker.Context(), routing.InstanceStore, storeMessage); publishErr != nil {
			worker.Error("FAILED_TO_PUBLISH_PGCR_FOR_STORAGE", publishErr, map[string]any{
				logging.INSTANCE_ID: instanceId,
			})
			instance_storage.WriteMissedLog(instanceId)
			return publishErr
		}
		worker.Info("PGCR_CRAWL_SUCCESS", map[string]any{
			logging.INSTANCE_ID: instanceId,
			logging.STATUS:      "published_to_store",
		})
		return nil

	case pgcr_processing.NonRaid:
		// Not a raid - this is expected, just skip
		worker.Debug("PGCR_NOT_A_RAID", map[string]any{logging.INSTANCE_ID: instanceId})
		return nil

	case pgcr_processing.NotFound:
		// PGCR doesn't exist
		worker.Info("PGCR_NOT_FOUND", map[string]any{logging.INSTANCE_ID: instanceId})
		return nil

	case pgcr_processing.InsufficientPrivileges:
		// Blocked by Bungie - publish to retry queue
		if publishErr := publishing.PublishInt64Message(worker.Context(), routing.PGCRRetry, instanceId); publishErr != nil {
			worker.Error("FAILED_TO_PUBLISH_TO_RETRY_QUEUE", publishErr, map[string]any{
				logging.INSTANCE_ID: instanceId,
			})
			return publishErr
		}
		worker.Info("PGCR_PUBLISHED_TO_RETRY", map[string]any{logging.INSTANCE_ID: instanceId})
		return nil

	case pgcr_processing.SystemDisabled:
		// System disabled - log but don't retry
		worker.Warn("BUNGIE_SYSTEM_DISABLED", nil, map[string]any{logging.INSTANCE_ID: instanceId})
		return nil

	case pgcr_processing.BadFormat, pgcr_processing.ExternalError:
		// Malformed or external error - log but don't retry
		worker.Warn("PGCR_PROCESSING_ERROR", nil, map[string]any{
			logging.INSTANCE_ID: instanceId,
			"result":            result,
		})
		return nil

	default:
		worker.Error("UNKNOWN_PGCR_RESULT", nil, map[string]any{
			logging.INSTANCE_ID: instanceId,
			"result":            result,
		})
		return nil
	}
}
