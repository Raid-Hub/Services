package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"raidhub/lib/domains/pgcr"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring"
)

func offloadWorker(ch chan int64) {
	for id := range ch {
		// Spawn a worker for each instanceId
		go func(instanceId int64) {
			log.Printf("Offloading instanceId %d", instanceId)
			time.Sleep(15 * time.Second)
			startTime := time.Now()
			for i := 1; i <= 6; i++ {
				result, activity, raw, err := pgcr.FetchAndProcessPGCR(instanceId)

				statusStr := fmt.Sprintf("%d", result)
				attemptsStr := fmt.Sprintf("%d", -i)
				monitoring.PGCRCrawlStatus.WithLabelValues(statusStr, attemptsStr).Inc()

				if err != nil {
					log.Printf("[Offload Worker] Error fetching instanceId %d: %s", instanceId, err)
				}

				if result == pgcr.NonRaid {
					log.Printf("[Offload Worker] Found non-raid raid with instanceId %d", instanceId)
					return
				} else if result == pgcr.Success {
					// Publish to queue for async storage
					storeMessage := messages.NewPGCRStoreMessage(activity, raw)
					err := routing.Publisher.PublishJSONMessage(routing.PGCRStore, storeMessage)
					if err != nil {
						log.Printf("[Offload Worker] Failed to publish PGCR store message: %v", err)
					} else {
						endTime := time.Now()
						lag := time.Since(activity.DateCompleted)
						log.Printf("[Offload Worker] Queued PGCR with instanceId %d (%d, %.0f, %.0f)", instanceId, i, endTime.Sub(startTime).Seconds(), lag.Seconds())
						return
					}
				} else if result == pgcr.SystemDisabled {
					i--
					time.Sleep(60 * time.Second)
					continue
				} else if result == pgcr.InsufficientPrivileges {
					routing.Publisher.PublishJSONMessage(routing.PGCRBlocked, fmt.Sprintf("%d", instanceId))
					return
				} else if result == pgcr.ExternalError {
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
