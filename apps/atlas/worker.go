package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"raidhub/lib/domains/instance_storage"
	"raidhub/lib/domains/pgcr"
	"raidhub/lib/messaging/messages"
	"raidhub/lib/messaging/routing"
	"raidhub/lib/monitoring"
)

func Worker(wg *sync.WaitGroup, ch chan int64, offloadChannel chan int64) {
	defer wg.Done()

	randomVariation := retryDelayTime / 3

	for instanceID := range ch {
		startTime := time.Now()
		notFoundCount := 0
		errCount := 0
		i := 0

		for {
			reqStartTime := time.Now()
			result, activity, raw, err := pgcr.FetchAndProcessPGCR(instanceID)
			if err != nil && result != pgcr.NotFound {
				log.Println(err)
			}

			statusStr := fmt.Sprintf("%d", result)
			attemptsStr := fmt.Sprintf("%d", i+1)

			monitoring.PGCRCrawlStatus.WithLabelValues(statusStr, attemptsStr).Inc()

			// Handle the result
			if result == pgcr.NonRaid {
				startDate, err := time.Parse(time.RFC3339, raw.Period)
				if err != nil {
					log.Fatal(err)
				}
				endDate := pgcr.CalculateDateCompleted(startDate, raw.Entries[0])

				lag := time.Since(endDate)
				if lag >= 0 {
					monitoring.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(float64(lag.Seconds()))
				}
				break
			} else if result == pgcr.Success {
				// Publish to queue for async storage
				storeMessage := messages.NewPGCRStoreMessage(activity, raw)
				err := routing.Publisher.PublishJSONMessage(routing.PGCRStore, storeMessage)
				if err != nil {
					errCount++
					log.Printf("Failed to publish PGCR store message: %v", err)
					time.Sleep(5 * time.Second)
				} else {
					endTime := time.Now()
					workerTime := endTime.Sub(startTime).Milliseconds()
					lag := time.Since(activity.DateCompleted)
					if lag >= 0 {
						monitoring.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(float64(lag.Seconds()))
						reqTime := endTime.Sub(reqStartTime)
						log.Printf("Added PGCR with instanceId %d (%d / %d / %d / %.0f)", instanceID, i, workerTime, reqTime.Milliseconds(), lag.Seconds())
					}
					break
				}
			} else if result == pgcr.NotFound {
				notFoundCount++
			} else if result == pgcr.SystemDisabled {
				monitoring.PGCRCrawlLag.WithLabelValues(statusStr, attemptsStr).Observe(0)
				time.Sleep(45 * time.Second)
				continue
			} else if result == pgcr.InsufficientPrivileges {
				routing.Publisher.PublishJSONMessage(routing.PGCRBlocked, fmt.Sprintf("%d", instanceID))
				break
			} else if result == pgcr.InternalError || result == pgcr.DecodingError {
				errCount++
				time.Sleep(time.Duration(5*errCount*errCount) * time.Second)
			} else if result == pgcr.RateLimited {
				errCount++
				time.Sleep(time.Duration(30) * time.Second)
			} else if result == pgcr.BadFormat || result == pgcr.ExternalError {
				instance_storage.WriteMissedLog(instanceID)
				if errCount > 0 {
					offloadChannel <- instanceID
					break
				} else {
					errCount++
				}
			}

			// If we have not found the instance id after some time
			if notFoundCount > 3 || errCount > 2 {
				instance_storage.WriteMissedLog(instanceID)
				offloadChannel <- instanceID
				break
			}

			timeout := time.Duration((retryDelayTime - randomVariation + rand.Intn(retryDelayTime*(i+1)))) * time.Millisecond
			time.Sleep(timeout)
			i++
		}
	}
}
