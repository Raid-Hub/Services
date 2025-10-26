package queueworkers

import (
	"log"
	"math/rand"
	"raidhub/lib/domains/instance_storage"
	"raidhub/lib/domains/pgcr"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/processing"
	"raidhub/lib/messaging/routing"
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
		QueueName:             routing.PGCRBlocked,
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
		worker.Error("Failed to parse instance ID", "error", err)
		return err
	}

	worker.Info("Processing blocked PGCR", "instanceId", instanceId)

	randomVariation := retryDelayTime / 3
	i := 0
	errCount := 0

	for {
		// Try to fetch and process the PGCR
		result, activity, raw, err := pgcr.FetchAndProcessPGCR(instanceId)
		if err != nil && result != pgcr.NotFound {
			log.Println(err)
		}

		// Handle the result
		if result == pgcr.NonRaid {
			// Successfully confirmed it's not a raid - mark as processed
			markPgcrSuccess(instanceId)
			worker.Info("Confirmed non-raid", "instanceId", instanceId)
			return nil
		} else if result == pgcr.Success {
			// Successfully fetched and processed - floodgates are open!
			markPgcrSuccess(instanceId)
			worker.Info("Blocked PGCR successfully processed - floodgates opened!", "instanceId", instanceId)

			// Publish to storage queue
			storeMessage := messages.NewPGCRStoreMessage(activity, raw)
			if publishErr := routing.Publisher.PublishJSONMessage(routing.PGCRStore, storeMessage); publishErr != nil {
				worker.Error("Failed to publish PGCR for storage", "instanceId", instanceId, "error", publishErr)
				return publishErr
			}
			return nil
		} else if result == pgcr.InsufficientPrivileges {
			// Still blocked - check if floodgates have opened
			if !isUnblocked() {
				// Floodgates still closed, wait and retry
				worker.Info("Still blocked, waiting for floodgates", "instanceId", instanceId, "attempt", i+1)
				time.Sleep(60 * time.Second)
				continue
			} else {
				// Floodgates are open but this one is still blocked
				worker.Info("Still blocked despite open floodgates", "instanceId", instanceId, "attempt", errCount)
				errCount++
			}
		} else if result == pgcr.NotFound {
			worker.Info("PGCR not found", "instanceId", instanceId)
			return nil // Give up
		} else if result == pgcr.SystemDisabled {
			time.Sleep(45 * time.Second)
			continue
		} else {
			errCount++
			time.Sleep(time.Duration(5*errCount*errCount) * time.Second)
		}

		// If we've failed too many times, give up
		if errCount > 3 {
			worker.Warn("Giving up on blocked PGCR after multiple failed attempts", "instanceId", instanceId)
			instance_storage.WriteMissedLog(instanceId)
			return nil
		}

		// Exponential backoff with random jitter
		timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
		i++
		time.Sleep(timeout)
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
