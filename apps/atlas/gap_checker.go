package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"raidhub/lib/services/instance"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

func gapCheckWorker(consumerConfig *ConsumerConfig) {
	// Check for gaps in the PGCRs
	for {
		metrics, err := GetMetrics(4)
		if err != nil {
			AtlasLogger.Error("FAILED_TO_GET_METRICS_IN_GAP_CHECKER", err, nil)
			time.Sleep(5 * time.Minute)
			continue
		}

		if metrics.Fraction404 > 0.8 && metrics.Count404 > 50 {
			startTime := time.Now()
			logHigh404Rate(int(metrics.Count404), metrics.Fraction404*100)
			// spawn an additional 500 workers to process the potential gap
			spawnWorkers(500, 10_000, consumerConfig)

			metrics, err := GetMetricsForScaling(time.Since(startTime))
			if err != nil {
				AtlasLogger.Error("FAILED_TO_GET_METRICS_AFTER_GAP_SUPERCHARGE", err, nil)
				// Continue loop without evaluating metrics
				time.Sleep(5 * time.Minute)
				continue
			}

			logExitGapSupercharge(100*metrics.Fraction404, metrics.P20Lag)

			if metrics.Fraction404 > 0.99 {
				// try to find the starting point after the gap, if there is one
				minCursor := consumerConfig.LatestId
				maxCursor := consumerConfig.LatestId + 5_000_000
				foundId, err := binarySearchForBlockStart(minCursor, maxCursor)

				if err != nil {
					// Error finding block start
					AtlasLogger.Warn("GAP_BLOCK_SEARCH_FAILED", err, map[string]any{
						logging.INSTANCE_ID: consumerConfig.LatestId,
						logging.FROM:        minCursor,
						logging.TO:          maxCursor,
						logging.ACTION:      "resetting_to_latest",
					})
					latestId, completionDate, err := instance.GetLatestInstance()
					if err != nil {
						AtlasLogger.Fatal("FAILED_TO_GET_LATEST_INSTANCE", err, nil)
					}

					// reset the crawler
					currentId := consumerConfig.LatestId
					logRunawayError(100*metrics.Fraction404, currentId, latestId, completionDate)
					atomic.StoreInt64(&consumerConfig.LatestId, latestId-10_000)
				} else {
					prevId := consumerConfig.LatestId

					for id := prevId; id < foundId; id++ {
						instance_storage.WriteMissedLog(id)
					}

					// push the crawler forward
					logGapCheckBlockSkip(prevId, foundId)
					atomic.StoreInt64(&consumerConfig.LatestId, foundId)
				}
			}
		}

		time.Sleep(5 * time.Minute)
	}
}

func binarySearchForBlockStart(minCursor, maxCursor int64) (int64, error) {
	// Binary search to find the latest instanceId
	hasFound := false
	for minCursor < maxCursor {
		mid := (minCursor + maxCursor) / 2
		result, _ := pgcr_processing.FetchPGCR(context.Background(), mid, 0)
		switch result {
		case pgcr_processing.Success:
			hasFound = true
			maxCursor = mid
		case pgcr_processing.NotFound:
			if hasFound {
				minCursor = mid + 1
			} else {
				maxCursor = mid
			}
		case pgcr_processing.SystemDisabled:
			time.Sleep(60 * time.Second)
		case pgcr_processing.ExternalError, pgcr_processing.RateLimited:
			// retry the request
			time.Sleep(5 * time.Second)
		default:
			return -1, fmt.Errorf("unexpected result %d for instanceId %d while binary searching", result, mid)
		}
	}

	if hasFound {
		return maxCursor, nil
	} else {
		return -1, fmt.Errorf("no valid instanceId found in the range %d to %d", minCursor, maxCursor)
	}
}
