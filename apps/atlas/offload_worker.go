package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

var offloadLogger = logging.NewLogger("atlas::offloadWorker")

func offloadWorker(consumerConfig *ConsumerConfig) {
	for id := range consumerConfig.OffloadChannel {
		// Spawn a worker for each instanceId
		go func(instanceId int64) {
			time.Sleep(15 * time.Second)
			startTime := time.Now()
			for i := 1; i <= 6; i++ {
				result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(instanceId)

				if result == pgcr_processing.NonRaid {
					return
				} else if result == pgcr_processing.Success {
					// Publish to queue for async storage
					storeMessage := messages.NewPGCRStoreMessage(instance, pgcr)
					err := publishing.PublishJSONMessage(context.Background(), routing.InstanceStore, storeMessage)
					if err != nil {
						offloadLogger.Warn("FAILED_TO_PUBLISH_PGCR_STORE_MESSAGE", err, map[string]any{
							logging.INSTANCE_ID: instanceId,
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
