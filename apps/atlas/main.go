package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"

	"raidhub/lib/domains/instance"
	"raidhub/lib/domains/instance_storage"
	"raidhub/lib/domains/pgcr"
	"raidhub/lib/messaging/rabbit"
	"raidhub/lib/monitoring"
)

var (
	numWorkers       = flag.Int("workers", 25, "number of workers to spawn at the start")
	buffer           = flag.Int64("buffer", 10_000, "number of ids to start behind last added")
	targetInstanceId = flag.Int64("target", -1, "specific instance id to start at (optional)")
	workers          = 0
)

func main() {
	log.SetFlags(0) // Disable timestamps
	flag.Parse()
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	workers = *numWorkers
	if *buffer < 0 || workers <= 0 || workers > maxWorkers {
		log.Fatalln("Invalid flags")
	}

	// postgres.DB is initialized in init()
	var instanceId int64
	var err error
	if *targetInstanceId == -1 {
		instanceId, err = instance.GetLatestInstanceId(*buffer)
		if err != nil {
			log.Fatalf("Error getting latest instance id: %s", err)
		}
	} else {
		instanceId = *targetInstanceId - *buffer
	}

	monitoring.RegisterAtlas(8080)
	run(instanceId)

}

func run(latestId int64) {
	defer func() {
		if r := recover(); r != nil {
			handlePanic(r)
		}
	}()

	// rabbit.Conn is initialized in init()
	rabbitChannel, err := rabbit.Conn.Channel()
	if err != nil {
		log.Fatalf("Failed to create channel: %s", err)
	}
	defer rabbitChannel.Close()

	consumerConfig := ConsumerConfig{
		LatestId:       latestId,
		OffloadChannel: make(chan int64),
		RabbitChannel:  rabbitChannel,
	}

	sendStartUpAlert()

	// Start a goroutine to offload malformed or slowly resolving PGCRs
	go offloadWorker(consumerConfig.OffloadChannel)

	// check for gaps
	go gapCheckWorker(&consumerConfig)

	periodLength := 10_000
	for {
		startTime := time.Now()
		spawnWorkers(workers, periodLength, &consumerConfig)

		monitoring.ActiveWorkers.Set(float64(workers))

		medianLag, err := getMedianLag(min(4, int(time.Since(startTime).Minutes())))
		if err != nil {
			log.Fatal(err)
		}
		fractionNotFound, err := get404Fraction(min(4, int(time.Since(startTime).Minutes())))
		if err != nil {
			log.Fatal(err)
		}
		errFraction, err := getErrorFraction(min(4, int(time.Since(startTime).Minutes())))
		if err != nil {
			log.Fatal(err)
		}

		logIntervalState(medianLag, workers, 100*fractionNotFound, 100*errFraction)

		var newPeriodLength int
		newWorkers := 0
		if fractionNotFound > 0.50 {
			// If we are getting a lot of 404s, let's do a quick probe set
			newPeriodLength = 2500
			newWorkers = 25
		} else if fractionNotFound < 0.001 || medianLag >= 600 {
			// how much we expect to get catch up
			newPeriodLength = max(int(math.Round(math.Pow(float64(workers)*(math.Ceil(medianLag)-20.0), 0.824))), 10_000)
			// If we aren't getting 404's, just spike the workers up to ensure we catch up to live ASAP
			newWorkers = int(math.Ceil(float64(workers) * (1 + float64(medianLag-20)/100)))
		} else {
			adjf := fractionNotFound - 0.025 // do not let workers go below 2.5 %
			decreaseFraction := min(math.Pow(retryDelayTime/8*math.Abs(adjf), 0.88)/100, 0.65)
			sign := adjf / math.Abs(adjf)
			// Adjust number of workers for the next period
			newWorkers = int(math.Round(float64(workers) - sign*decreaseFraction*float64(workers)))

			pgcrRate, err := getPgcrsPerSecond(int(time.Since(startTime).Minutes()))
			// Calculate the new period length based on the number of PGCRs per second
			if err != nil {
				log.Fatal(err)
			} else if pgcrRate == 0 {
				newPeriodLength = 600 * newWorkers
			} else if fractionNotFound >= 0.075 {
				newPeriodLength = int(math.Round(100 * pgcrRate))
			} else {
				newPeriodLength = int(math.Round(300 * pgcrRate))
			}
		}

		workers = min(max(newWorkers, minWorkers), maxWorkers)
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
		go Worker(&wg, ids, consumerConfig.OffloadChannel)
	}

	// Pass IDs to workers
	for i := 0; i < periodLength; i++ {
		latest := atomic.AddInt64(&consumerConfig.LatestId, 1)
		ids <- latest
	}
	close(ids)

	wg.Wait()
}

func gapCheckWorker(consumerConfig *ConsumerConfig) {

	// Check for gaps in the PGCRs
	for {
		count, err := get404Rate(4)
		if err != nil {
			log.Fatal(err)
		}
		frac, err := get404Fraction(4)
		if err != nil {
			log.Fatal(err)
		}
		if frac > 0.8 && count > 50 {
			startTime := time.Now()
			logHigh404Rate(int(count), frac*100)
			// spawn an additional 500 workers to process the potential gap
			spawnWorkers(500, 10_000, consumerConfig)

			medianLag, err := getMedianLag(min(5, int(time.Since(startTime).Minutes())))
			if err != nil {
				log.Fatal(err)
			}

			fractionNotFound, err := get404Fraction(min(5, int(time.Since(startTime).Minutes())))
			if err != nil {
				log.Fatal(err)
			}

			logExitGapSupercharge(100*fractionNotFound, medianLag)

			if fractionNotFound > 0.99 {
				// try to find the starting point after the gap, if there is one
				minCursor := consumerConfig.LatestId
				maxCursor := consumerConfig.LatestId + 5_000_000
				foundId, err := binarySearchForBlockStart(minCursor, maxCursor)

				if err != nil {
					log.Println("Error finding block start:", err)
					latestId, completionDate, err := instance.GetLatestInstance()
					if err != nil {
						log.Fatal(err)
					}

					// reset the crawler
					currentId := consumerConfig.LatestId
					logRunawayError(100*fractionNotFound, currentId, latestId, completionDate)
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
		log.Println("Gap Mode Block Search: Searching between", minCursor, "and", maxCursor)
		mid := (minCursor + maxCursor) / 2
		result, _, _, _ := pgcr.FetchAndProcessPGCR(mid)
		switch result {
		case pgcr.Success, pgcr.NonRaid:
			hasFound = true
			maxCursor = mid
		case pgcr.NotFound:
			if hasFound {
				minCursor = mid + 1
			} else {
				maxCursor = mid
			}
		case pgcr.SystemDisabled:
			time.Sleep(60 * time.Second)
		case pgcr.InternalError, pgcr.DecodingError, pgcr.ExternalError, pgcr.RateLimited:
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
