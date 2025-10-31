package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

var offloadLogger = logging.NewLogger("ATLAS::OFFLOAD_WORKER")

// isTimeoutError checks if an error is a timeout/deadline exceeded error
func isTimeoutError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")
}

func offloadWorker(consumerConfig *ConsumerConfig) {
	for id := range consumerConfig.OffloadChannel {
		// Spawn a worker for each instanceId
		go func(instanceId int64) {
			time.Sleep(15 * time.Second)
			startTime := time.Now()
			for i := 1; i <= 6; i++ {
				result, activity, raw, err := pgcr_processing.FetchAndProcessPGCR(instanceId)

				if err != nil {
					// Check if this is a timeout error - log at Debug level instead of Warn
					if isTimeoutError(err) {
						offloadLogger.Debug("PGCR_FETCH_TIMEOUT", map[string]any{
							logging.INSTANCE_ID: instanceId,
							logging.ERROR:       err.Error(),
							logging.ATTEMPT:     i,
						})
					} else {
						offloadLogger.Warn("PGCR_FETCH_ERROR", map[string]any{
							logging.INSTANCE_ID: instanceId,
							logging.ERROR:       err.Error(),
							logging.ATTEMPT:     i,
						})
					}
				}

				if result == pgcr_processing.NonRaid {
					return
				} else if result == pgcr_processing.Success {
					// Publish to queue for async storage
					storeMessage := messages.NewPGCRStoreMessage(activity, raw)
					err := publishing.PublishJSONMessage(context.Background(), routing.InstanceStore, storeMessage)
					if err != nil {
						offloadLogger.Warn("FAILED_TO_PUBLISH_PGCR_STORE_MESSAGE", map[string]any{
							logging.INSTANCE_ID: instanceId,
							logging.ERROR:       err.Error(),
							logging.ATTEMPT:     i,
						})
					} else {
						offloadLogger.Debug("PGCR_QUEUED_FOR_STORAGE", map[string]any{
							logging.INSTANCE_ID: instanceId,
							logging.ATTEMPT:     i,
							"duration":          fmt.Sprintf("%dms", time.Since(startTime).Milliseconds()),
						})
						return
					}
				} else if result == pgcr_processing.SystemDisabled {
					i--
					time.Sleep(60 * time.Second)
					continue
				} else if result == pgcr_processing.InsufficientPrivileges {
					publishing.PublishJSONMessage(context.Background(), routing.PGCRRetry, fmt.Sprintf("%d", instanceId))
					return
				} else if result == pgcr_processing.ExternalError {
					i--
					continue
				}

				if i == 3 {
					go logMissedInstanceWarning(instanceId, startTime)
				}

				// Exponential Backoff
				time.Sleep(time.Duration(i*(2*i+rand.Intn(5*(i)))) * time.Second)
			}

			logMissedInstance(instanceId, startTime)
		}(id)
	}
}
