package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/publishing"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring/atlas_metrics"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/discord"
)

// Run starts the worker processing loop
func (w *AtlasWorker) Run(wg *sync.WaitGroup, ch chan int64) {
	defer wg.Done()

	randomVariation := retryDelayTime / 3

	for instanceID := range ch {
		startTime := time.Now()
		notFoundCount := 0
		errCount := 0
		i := 0

		for {
			result, activity, raw, err := pgcr_processing.FetchAndProcessPGCR(instanceID)
			if err != nil && result != pgcr_processing.NotFound {
				w.LogPGCRError(err, instanceID, i+1)
			}

			statusStr := fmt.Sprintf("%d", result)
			attemptsStr := fmt.Sprintf("%d", i+1)

			atlas_metrics.PGCRCrawlStatus.WithLabelValues(statusStr, attemptsStr).Inc()

			// Handle the result
			if result == pgcr_processing.NonRaid {
				startDate, err := time.Parse(time.RFC3339, raw.Period)
				if err != nil {
					w.Error("Failed to parse time", map[string]any{
						logging.ERROR: err.Error(),
					})
					continue
				}
				endDate := pgcr_processing.CalculateDateCompleted(startDate, raw.Entries[0])

				lag := time.Since(endDate)
				if lag >= 0 {
					atlas_metrics.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(float64(lag.Seconds()))
				}
				break
			} else if result == pgcr_processing.Success {
				// Publish to queue for async storage
				storeMessage := messages.NewPGCRStoreMessage(activity, raw)
				err := publishing.PublishJSONMessage(context.Background(), routing.InstanceStore, storeMessage)
				if err != nil {
					errCount++
					w.Warn("FAILED_TO_PUBLISH_INSTANCE_STORE_MESSAGE", map[string]any{
						logging.ERROR: err.Error(),
					})
					time.Sleep(5 * time.Second)
				} else {
					endTime := time.Now()
					workerTime := endTime.Sub(startTime)
					lag := time.Since(activity.DateCompleted)
					if lag >= 0 {
						atlas_metrics.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(float64(lag.Seconds()))
					}
					w.Info("PUBLISHED_INSTANCE", map[string]any{
						logging.INSTANCE_ID: instanceID,
						logging.ATTEMPT:     i + 1,
						"duration":          fmt.Sprintf("%dms", workerTime.Milliseconds()),
						"lag":               discord.FormatDuration(lag.Seconds()),
					})
					break
				}
			} else if result == pgcr_processing.NotFound {
				notFoundCount++
			} else if result == pgcr_processing.SystemDisabled {
				atlas_metrics.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(0)
				time.Sleep(45 * time.Second)
				continue
			} else if result == pgcr_processing.InsufficientPrivileges {
				publishing.PublishJSONMessage(context.Background(), routing.PGCRRetry, fmt.Sprintf("%d", instanceID))
				break
			} else if result == pgcr_processing.BadFormat || result == pgcr_processing.ExternalError {
				instance_storage.WriteMissedLog(instanceID)
				if errCount > 0 {
					w.offloadChannel <- instanceID
					break
				} else {
					errCount++
				}
			}

			// If we have not found the instance id after some time
			if notFoundCount > 3 || errCount > 2 {
				instance_storage.WriteMissedLog(instanceID)
				w.offloadChannel <- instanceID
				break
			}

			timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
			time.Sleep(timeout)
			i++
		}
	}
}
