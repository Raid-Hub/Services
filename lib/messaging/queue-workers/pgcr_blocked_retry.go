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
		QueueName:          routing.PGCRRetry,
		MinWorkers:         10,
		MaxWorkers:         500,
		DesiredWorkers:     1,
		KeepInReady:        true,
		PrefetchCount:      1,
		ScaleUpThreshold:   100,
		ScaleDownThreshold: 10,
		ScaleUpPercent:     0.2,
		ScaleDownPercent:   0.1,
		BungieSystemDeps:   []string{"Destiny2"},
		MaxRetryCount:      25, // Designed for retries, but still need a limit
	}, processPgcrBlocked)
}

// processPgcrBlocked handles PGCR blocked messages with floodgate detection
// When Bungie blocks PGCRs (InsufficientPrivileges), this worker retries them
// When enough PGCRs successfully process, it assumes floodgates are "opened"
func processPgcrBlocked(worker processing.WorkerInterface, message amqp.Delivery) error {
	instanceId, err := processing.ParseInt64(worker, message.Body)
	if err != nil {
		// temporary fallback to handle old messages
		instanceIdStr, err2 := processing.ParseJSON[string](worker, message.Body)
		if err2 != nil {
			return err2
		}
		instanceIdInt, err3 := strconv.ParseInt(instanceIdStr, 10, 64)
		if err3 != nil {
			return err3
		}
		instanceId = instanceIdInt
	}

	fields := map[string]any{
		logging.INSTANCE_ID: instanceId,
	}
	worker.Debug("PROCESSING_BLOCKED_PGCR", fields)

	randomVariation := retryDelayTime / 3
	i := 0
	errCount := 0

	var workerCancelled = fmt.Errorf("worker cancelled")

	for {
		// Check if we should stop processing due to cancellation
		select {
		case <-worker.Context().Done():
			worker.Debug("WORKER_CANCELLED_DURING_PROCESSING", fields)
			return workerCancelled
		default:
		}

		// Try to fetch and process the PGCR
		result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(worker.Context(), instanceId, 0)
		// Handle the result
		if result == pgcr_processing.NonRaid {
			// Successfully confirmed it's not a raid - mark as processed
			markPgcrSuccess(instanceId)
			worker.Info("CONFIRMED_NON_RAID", fields)
			return nil
		} else if result == pgcr_processing.Success {
			// Successfully fetched and processed - floodgates are open!
			markPgcrSuccess(instanceId)
			worker.Info("BLOCKED_PGCR_SUCCESSFULLY_PROCESSED", fields)

			// Publish to storage queue
			storeMessage := messages.NewPGCRStoreMessage(instance, pgcr)
			if publishErr := publishing.PublishJSONMessage(worker.Context(), routing.InstanceStore, storeMessage); publishErr != nil {
				worker.Error("FAILED_TO_PUBLISH_PGCR_FOR_STORAGE", publishErr, fields)
				return publishErr
			}
			return nil
		} else if result == pgcr_processing.InsufficientPrivileges {
			// Still blocked - check if floodgates have opened
			if !isUnblocked() {
				// Floodgates still closed, wait and retry
				worker.Info("STILL_BLOCKED_WAITING_FOR_FLOODGATES", fields)

				// Sleep with context cancellation check
				select {
				case <-worker.Context().Done():
					worker.Debug("WORKER_CANCELLED_DURING_WAIT", fields)
					return workerCancelled
				case <-time.After(60 * time.Second):
				}
				continue
			} else {
				// Floodgates are open but this one is still blocked
				worker.Info("STILL_BLOCKED_DESPITE_OPEN_FLOODGATES", fields)
				errCount++
			}
		} else if result == pgcr_processing.NotFound {
			worker.Info("PGCR_NOT_FOUND", map[string]any{logging.INSTANCE_ID: instanceId})
			return nil // Give up
		} else if result == pgcr_processing.SystemDisabled {
			// Sleep with context cancellation check
			select {
			case <-worker.Context().Done():
				worker.Debug("WORKER_CANCELLED_DURING_SYSTEM_DISABLED_WAIT", fields)
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
				worker.Debug("WORKER_CANCELLED_DURING_ERROR_BACKOFF", fields)
				return workerCancelled
			case <-time.After(sleepDuration):
			}
		}

		// If we've failed too many times, give up
		if errCount > 3 {
			worker.Warn("GIVING_UP_ON_BLOCKED_PGCR", nil, fields)
			instance_storage.WriteMissedLog(instanceId)
			return nil
		}

		// Exponential backoff with random jitter
		timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
		i++

		// Sleep with context cancellation check
		select {
		case <-worker.Context().Done():
			worker.Debug("WORKER_CANCELLED_DURING_BACKOFF", fields)
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
