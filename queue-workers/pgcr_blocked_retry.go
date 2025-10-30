package queueworkers

import (
	"fmt"
	"math/rand"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	retryDelayTime = 10000 // milliseconds
)

var (
	pgcrSuccess sync.Map // Tracks successfully processed PGCRs to detect floodgates
)

// PgcrBlockedTopic creates a new PGCR blocked topic
func PgcrBlockedTopic() processing.Topic {
	return processing.NewTopic(processing.TopicConfig{
		QueueName:             routing.PGCRRetry,
		MinWorkers:            10,
		MaxWorkers:            500,
		DesiredWorkers:        50,
		ContestWeekendWorkers: 200,
		KeepInReady:           true,
		PrefetchCount:         1,
		ScaleUpThreshold:      100,
		ScaleDownThreshold:    10,
		ScaleUpPercent:        0.2,
		ScaleDownPercent:      0.1,
	}, processPgcrBlocked)
}

// processPgcrBlocked handles PGCR blocked messages with floodgate detection
// When Bungie blocks PGCRs (InsufficientPrivileges), this worker retries them
// When enough PGCRs successfully process, it assumes floodgates are "opened"
func processPgcrBlocked(worker *processing.Worker, message amqp.Delivery) error {
	instanceId, err := strconv.ParseInt(string(message.Body), 10, 64)
	if err != nil {
		worker.Error("FAILED_TO_PARSE_INSTANCE_ID", map[string]any{logging.ERROR: err.Error()})
		return err
	}

	worker.Info("PROCESSING_BLOCKED_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})

	randomVariation := retryDelayTime / 3
	i := 0
	errCount := 0

	var workerCancelled = fmt.Errorf("worker cancelled")

	for {
		// Check if we should stop processing due to cancellation
		select {
		case <-worker.Context().Done():
			worker.Debug("WORKER_CANCELLED_DURING_PROCESSING", map[string]any{logging.INSTANCE_ID: instanceId})
			return workerCancelled
		default:
		}

		// Try to fetch and process the PGCR
		result, activity, raw, err := pgcr_processing.FetchAndProcessPGCR(instanceId)
		if err != nil && result != pgcr_processing.NotFound {
			worker.Error("ERROR_FETCHING_PGCR", map[string]any{logging.INSTANCE_ID: instanceId, logging.ERROR: err.Error()})
		}

		// Handle the result
		if result == pgcr_processing.NonRaid {
			// Successfully confirmed it's not a raid - mark as processed
			markPgcrSuccess(instanceId)
			worker.Info("CONFIRMED_NON_RAID", map[string]any{logging.INSTANCE_ID: instanceId})
			return nil
		} else if result == pgcr_processing.Success {
			// Successfully fetched and processed - floodgates are open!
			markPgcrSuccess(instanceId)
			worker.Info("BLOCKED_PGCR_SUCCESSFULLY_PROCESSED", map[string]any{logging.INSTANCE_ID: instanceId})

			// Publish to storage queue
			storeMessage := messages.NewPGCRStoreMessage(activity, raw)
			if publishErr := publishing.PublishJSONMessage(worker.Context(), routing.InstanceStore, storeMessage); publishErr != nil {
				worker.Error("FAILED_TO_PUBLISH_PGCR_FOR_STORAGE", map[string]any{logging.INSTANCE_ID: instanceId, logging.ERROR: publishErr.Error()})
				return publishErr
			}
			return nil
		} else if result == pgcr_processing.InsufficientPrivileges {
			// Still blocked - check if floodgates have opened
			if !isUnblocked() {
				// Floodgates still closed, wait and retry
				worker.Info("STILL_BLOCKED_WAITING_FOR_FLOODGATES", map[string]any{logging.INSTANCE_ID: instanceId, logging.ATTEMPT: i + 1})

				// Sleep with context cancellation check
				select {
				case <-worker.Context().Done():
					worker.Debug("WORKER_CANCELLED_DURING_WAIT", map[string]any{logging.INSTANCE_ID: instanceId})
					return workerCancelled
				case <-time.After(60 * time.Second):
				}
				continue
			} else {
				// Floodgates are open but this one is still blocked
				worker.Info("STILL_BLOCKED_DESPITE_OPEN_FLOODGATES", map[string]any{logging.INSTANCE_ID: instanceId, logging.ATTEMPT: errCount})
				errCount++
			}
		} else if result == pgcr_processing.NotFound {
			worker.Info("PGCR_NOT_FOUND", map[string]any{logging.INSTANCE_ID: instanceId})
			return nil // Give up
		} else if result == pgcr_processing.SystemDisabled {
			// Sleep with context cancellation check
			select {
			case <-worker.Context().Done():
				worker.Debug("WORKER_CANCELLED_DURING_SYSTEM_DISABLED_WAIT", map[string]any{logging.INSTANCE_ID: instanceId})
				return workerCancelled
			case <-time.After(45 * time.Second):
			}
			continue
		} else {
			errCount++
			// Sleep with context cancellation check
			sleepDuration := time.Duration(5*errCount*errCount) * time.Second
			select {
			case <-worker.Context().Done():
				worker.Debug("WORKER_CANCELLED_DURING_ERROR_BACKOFF", map[string]any{logging.INSTANCE_ID: instanceId})
				return workerCancelled
			case <-time.After(sleepDuration):
			}
		}

		// If we've failed too many times, give up
		if errCount > 3 {
			worker.Warn("GIVING_UP_ON_BLOCKED_PGCR", map[string]any{logging.INSTANCE_ID: instanceId})
			instance_storage.WriteMissedLog(instanceId)
			return nil
		}

		// Exponential backoff with random jitter
		timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
		i++

		// Sleep with context cancellation check
		select {
		case <-worker.Context().Done():
			worker.Debug("WORKER_CANCELLED_DURING_BACKOFF", map[string]any{logging.INSTANCE_ID: instanceId})
			return workerCancelled
		case <-time.After(timeout):
		}
	}
}

// markPgcrSuccess records that a PGCR was successfully processed (floodgates opened)
func markPgcrSuccess(instanceId int64) {
	pgcrSuccess.Store(instanceId, time.Now())

	// Remove from cache after 3 minutes
	go func() {
		time.Sleep(3 * time.Minute)
		pgcrSuccess.Delete(instanceId)
	}()
}

// isUnblocked checks if the floodgates are open by counting recent successful PGCRs
func isUnblocked() bool {
	countUnblocked := 0

	// Count successful PGCRs in the last 3 minutes
	pgcrSuccess.Range(func(_, _ any) bool {
		countUnblocked++
		// Early exit if we have enough evidence
		return countUnblocked <= 3
	})

	// If we've seen 3+ successful PGCRs recently, floodgates are open
	return countUnblocked > 3
}
