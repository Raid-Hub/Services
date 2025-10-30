package main

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"raidhub/lib/monitoring"
	"raidhub/lib/utils/logging"
)

var workers = 0

func run(latestId int64, devSkip int, maxWorkersValue int) {
	consumerConfig := ConsumerConfig{
		LatestId:       latestId,
		OffloadChannel: make(chan int64),
		Skip:           devSkip,
	}

	sendStartUpAlert()

	// Start a goroutine to offload malformed or slowly resolving PGCRs
	go offloadWorker(&consumerConfig)

	// check for gaps
	go gapCheckWorker(&consumerConfig)

	periodLength := 10_000
	for {
		startTime := time.Now()

		// Ensure workers is at least minWorkers to avoid zero workers on first iteration
		if workers < minWorkers {
			workers = minWorkers
		}

		spawnWorkers(workers, periodLength, &consumerConfig)

		monitoring.ActiveWorkers.Set(float64(workers))

		metrics, err := GetMetricsForScaling(time.Since(startTime))
		if err != nil {
			logger.Error("FAILED_TO_GET_METRICS", map[string]any{
				logging.ERROR: err.Error(),
			})
			// Continue with previous worker count and period length on metrics failure
			continue
		}

		logIntervalState(metrics.P20Lag, workers, 100*metrics.Fraction404, 100*metrics.ErrorFraction)

		var newPeriodLength int
		newWorkers := 0
		if metrics.Fraction404 > 0.50 {
			// If we are getting a lot of 404s, let's do a quick probe set
			newPeriodLength = 2500
			newWorkers = 25
		} else if metrics.Fraction404 < 0.001 || metrics.P20Lag >= 600 {
			// Ensure we have at least minWorkers to avoid zero workers edge case
			effectiveWorkers := max(workers, minWorkers)
			// Clamp P20Lag to at least 20 to avoid negative scaling when close to live
			effectiveLag := math.Max(metrics.P20Lag, 20.0)
			// how much we expect to get catch up
			newPeriodLength = max(int(math.Round(math.Pow(float64(effectiveWorkers)*(math.Ceil(effectiveLag)-20.0), 0.824))), 10_000)
			// If we aren't getting 404's, just spike the workers up to ensure we catch up to live ASAP
			// Use consistent ceil operation for both period and worker calculations
			newWorkers = int(math.Ceil(float64(workers) * (1 + float64(effectiveLag-20)/100)))
		} else {
			adjf := metrics.Fraction404 - 0.025 // do not let workers go below 2.5 %
			decreaseFraction := min(math.Pow(retryDelayTime/8*math.Abs(adjf), 0.88)/100, 0.65)
			sign := adjf / math.Abs(adjf)
			// Adjust number of workers for the next period
			newWorkers = int(math.Round(float64(workers) - sign*decreaseFraction*float64(workers)))

			// Calculate the new period length based on the number of PGCRs per second
			if metrics.PGCRRate == 0 {
				newPeriodLength = 600 * newWorkers
			} else if metrics.Fraction404 >= 0.075 {
				newPeriodLength = int(math.Round(100 * metrics.PGCRRate))
			} else {
				newPeriodLength = int(math.Round(300 * metrics.PGCRRate))
			}
		}

		// Use the max workers value passed to run function
		workers = min(max(newWorkers, minWorkers), maxWorkersValue)
		periodLength = newPeriodLength
	}
}

func spawnWorkers(countWorkers int, periodLength int, consumerConfig *ConsumerConfig) {
	var wg sync.WaitGroup
	// unbuffered channel ensures ids don't sit in the buffer and are immediately passed to workers
	ids := make(chan int64)

	logWorkersStarting(countWorkers, periodLength, consumerConfig.LatestId)

	wg.Add(countWorkers)
	for i := 0; i < countWorkers; i++ {
		worker := NewAtlasWorker(i, consumerConfig.OffloadChannel)
		go worker.Run(&wg, ids)
	}

	// Pass IDs to workers
	for i := 0; i < periodLength; i++ {
		latest := atomic.AddInt64(&consumerConfig.LatestId, int64(consumerConfig.Skip)+1)
		ids <- latest
	}
	close(ids)

	wg.Wait()
}
