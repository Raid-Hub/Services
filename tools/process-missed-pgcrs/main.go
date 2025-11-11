package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/env"
	"raidhub/lib/services/instance"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
	"raidhub/lib/web/discord"

	"slices"
)

var logger = logging.NewLogger("process-missed-pgcrs")

// Global worker state
var (
	workerLatestId   int64
	workerForce      bool
	workerMaxRetries int
)

// ProcessMissedPGCRs is the command function for processing missed PGCRs
func ProcessMissedPGCRs() {
	fs := flag.NewFlagSet("process-missed-pgcrs", flag.ExitOnError)
	gap := fs.Bool("gap", false, "process gaps in the missed log")
	force := fs.Bool("force", false, "force processing all PGCRs, ignoring database recency check")
	numWorkers := fs.Int("workers", 1, "number of workers to spawn")
	retries := fs.Int("retries", 5, "number of retries for each PGCR")
	fs.Parse(flag.Args())

	hadesAlerting := discord.NewDiscordAlerting(env.HadesWebhookURL, logger)

	// missedLogPath is where new missed PGCRs are accumulated
	missedLogPath := env.MissedPGCRLogFilePath
	logDir := filepath.Dir(missedLogPath)
	// processingLogPath is where we temporarily store logs to process
	processingLogPath := filepath.Join(logDir, filepath.Base(missedLogPath)+".temp")

	_, err := os.Stat(processingLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if missedLogPath exists before trying to move it
			_, err = os.Stat(missedLogPath)
			if err == nil {
				// missedLogPath exists, move it to processingLogPath for processing
				err = moveFile(missedLogPath, processingLogPath)
				if err != nil {
					logger.Fatal("ERROR_MOVING_FILE", err, map[string]any{})
				}
			} else if !os.IsNotExist(err) {
				// Some other error checking missedLogPath
				logger.Fatal("ERROR_CHECKING_MISSED_LOG_PATH", err, map[string]any{})
			} else {
				// missedLogPath doesn't exist, nothing to process - just create empty missedLogPath and exit
				if err := os.MkdirAll(logDir, 0755); err != nil {
					logger.Fatal("ERROR_CREATING_LOG_DIRECTORY", err, map[string]any{})
				}
				missedLogFile, err := createFile(missedLogPath)
				if err != nil {
					logger.Fatal("ERROR_CREATING_MISSED_LOG_FILE", err, map[string]any{})
				}
				missedLogFile.Close()
				logger.Info("NO_MISSED_PGCRS_TO_PROCESS", map[string]any{})
				return
			}
			// After moving missedLogPath to processingLogPath, we don't create a new missedLogPath yet
			// It will be created at the end after processing completes, so that any failed PGCRs
			// written during processing are preserved
		} else {
			logger.Fatal("ERROR_STAT_PROCESSING_LOG_PATH", err, map[string]any{})
		}
	}

	logFile, err := os.Open(processingLogPath)
	if err != nil {
		logger.Fatal("ERROR_OPENING_PROCESSING_LOG_FILE", err, map[string]any{})
	}
	defer logFile.Close()

	logger.Info("READING_MISSED_PGCRS", map[string]any{logging.PATH: processingLogPath})

	// Wait for PostgreSQL connection to be ready
	postgres.Wait()

	// Create a map to store unique numbers
	uniqueNumbers := make(map[int64]bool)

	// Read the file line by line
	scanner := bufio.NewScanner(logFile)
	minN := int64(0)
	maxN := int64(0)
	for scanner.Scan() {
		line := scanner.Text()
		number, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			fmt.Printf("Error parsing line %s: %v\n", line, err)
			continue
		}
		uniqueNumbers[number] = true
		if minN == 0 {
			minN = number
		} else {
			minN = min(minN, number)
		}
		maxN = max(maxN, number)
	}

	if *gap && (maxN-minN > 200_000) {
		logger.Error("GAP_TOO_LARGE", nil, map[string]any{logging.MAX: maxN, logging.MIN: minN})
	}

	if *gap && minN > 0 && maxN > 0 {
		logger.Debug("FOUND_UNIQUE_NUMBERS_IN_FILE", map[string]any{logging.COUNT: len(uniqueNumbers), logging.MIN: minN, logging.MAX: maxN})
		for i := minN - 1000; i <= maxN+1000; i++ {
			uniqueNumbers[i] = true
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Warn("DATABASE_QUERY_ERROR", err, map[string]any{})
		logger.Fatal("ERROR_SCANNING_FILE", err, map[string]any{})
	}

	// postgres.DB is initialized in init()
	stmnt, err := postgres.DB.Prepare("SELECT instance_id FROM instance INNER JOIN pgcr USING (instance_id) WHERE instance_id = $1 LIMIT 1;")
	if err != nil {
		logger.Error("ERROR_PREPARING_STATEMENT", err, map[string]any{})
	}
	defer stmnt.Close()

	var numbers []int64
	for num := range uniqueNumbers {
		var foo int64
		if !*gap {
			err := stmnt.QueryRow(num).Scan(&foo)
			if err != nil {
				numbers = append(numbers, num)
			}
		} else {
			numbers = append(numbers, num)
		}

	}

	if *gap {
		logger.Info("PROCESSING_PGCRS_IN_GAP", map[string]any{logging.COUNT: len(numbers)})
	} else {
		logger.Info("FOUND_MISSING_PGCRS", map[string]any{logging.COUNT: len(numbers)})
	}
	// Sort the numbers
	slices.Sort(numbers)

	var found []int64
	var failed []int64
	minFailed := int64(^uint64(0) >> 1) // Max int64 value
	maxFailed := int64(0)
	if len(numbers) > 0 {
		var latestId int64
		if !*force {
			var err error
			latestId, err = instance.GetLatestInstanceId(5_000)
			if err != nil {
				logger.Fatal("ERROR_GETTING_LATEST_INSTANCE_ID", err, map[string]any{})
			}
		} else {
			latestId = 0 // Set to 0 when forcing, worker will skip the comparison
		}

		// Set global worker state
		workerLatestId = latestId
		workerForce = *force
		workerMaxRetries = *retries

		firstId := numbers[0]
		ch := make(chan int64)
		successes := make(chan int64)
		failures := make(chan int64)
		var wg sync.WaitGroup

		workerCount := *numWorkers

		// Start workers
		logger.Info("WORKERS_STARTING", map[string]any{logging.FIRST_ID: firstId})
		wg.Add(workerCount)
		for i := 0; i < workerCount; i++ {
			go worker(ch, successes, failures, &wg)
		}

		var wg2 sync.WaitGroup
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for id := range successes {
				found = append(found, id)
			}
		}()

		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for id := range failures {
				failed = append(failed, id)
				minFailed = min(minFailed, id)
				maxFailed = max(maxFailed, id)
			}
		}()

		for j := 0; j < len(numbers); j++ {
			ch <- numbers[j]
		}

		close(ch)
		wg.Wait()
		close(failures)
		close(successes)
		wg2.Wait()
	}

	// Remove processingLogPath file
	if _, err := os.Stat(processingLogPath); err == nil {
		if err := os.Remove(processingLogPath); err != nil {
			logger.Fatal("ERROR_REMOVING_PROCESSING_LOG_FILE", err, map[string]any{})
		}
		logger.Debug("TEMPORARY_FILE_DELETED_SUCCESSFULLY", map[string]any{})
	}

	// Create new empty missedLogPath only if it doesn't exist
	// (failed PGCRs written during processing should be preserved)
	if _, err := os.Stat(missedLogPath); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Fatal("ERROR_CREATING_LOG_DIRECTORY", err, map[string]any{})
		}
		missedLogFile, err := createFile(missedLogPath)
		if err != nil {
			logger.Fatal("ERROR_CREATING_MISSED_LOG_FILE", err, map[string]any{})
		}
		missedLogFile.Close()
	}

	gaps := findGaps(failed)
	postResults(hadesAlerting, len(numbers), len(failed), len(found), minFailed, maxFailed, gaps)
}

func worker(ch chan int64, successes chan int64, failures chan int64, wg *sync.WaitGroup) {
	defer wg.Done()

	for instanceID := range ch {
		if !workerForce && instanceID > workerLatestId {
			logger.Debug("PGCR_NEWER_THAN_LATEST_SKIPPING", map[string]any{logging.INSTANCE_ID: instanceID})
			instance_storage.WriteMissedLog(instanceID)
			failures <- instanceID
			continue
		}

		var errors = 0
		var processed = false
		for errors <= workerMaxRetries && !processed {
			result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(instanceID)

			if result == pgcr_processing.NonRaid {
				processed = true
				// NonRaid activities are successfully processed, just not raids
			} else if result == pgcr_processing.Success {
				_, committed, err := instance_storage.StorePGCR(instance, pgcr)
				if err != nil {
					attempt := errors + 1
					if attempt > workerMaxRetries {
						logger.Error(instance_storage.FAILED_TO_STORE_INSTANCE, err, map[string]any{logging.INSTANCE_ID: instanceID,
							logging.ATTEMPT: attempt})
					} else {
						logger.Warn(instance_storage.FAILED_TO_STORE_INSTANCE, err, map[string]any{logging.INSTANCE_ID: instanceID, logging.ATTEMPT: attempt, logging.RETRIES: workerMaxRetries})
					}
					time.Sleep(3 * time.Second)
					errors++
					continue
				} else if committed {
					logger.Info(instance_storage.STORED_NEW_INSTANCE, map[string]any{logging.INSTANCE_ID: instanceID})
					successes <- instanceID
					processed = true
				} else {
					logger.Debug("NON_RAID_ACTIVITY", map[string]any{logging.INSTANCE_ID: instanceID})
					processed = true
					// Not a raid, successfully processed and ignored
				}
			} else if result == pgcr_processing.ExternalError || result == pgcr_processing.RateLimited {
				time.Sleep(5 * time.Second)
				errors++
				// continue loop to retry
			} else {
				logger.Warn("COULD_NOT_RESOLVE_INSTANCE_ID", nil, map[string]any{logging.INSTANCE_ID: instanceID})
				instance_storage.WriteMissedLog(instanceID)
				failures <- instanceID
				processed = true
			}
		}
		if errors >= workerMaxRetries {
			logger.Warn("FAILED_TO_FETCH_INSTANCE_ID_MULTIPLE_TIMES_SKIPPING", nil, map[string]any{logging.INSTANCE_ID: instanceID, logging.RETRIES: workerMaxRetries})
			instance_storage.WriteMissedLog(instanceID)
			failures <- instanceID
		}
	}
}

func createFile(filePath string) (*os.File, error) {
	file, err := os.Create(filePath)
	return file, err
}

func moveFile(srcPath, dstPath string) error {
	err := os.Rename(srcPath, dstPath)
	return err
}

type Gap struct {
	Min   int64
	Max   int64
	Count int64
}

func postResults(hadesAlerting *discord.DiscordAlerting, count int, failed int, found int, minFailed int64, maxFailed int64, gaps []Gap) {
	message := fmt.Sprintf("Info: Processed %d missing PGCR(s). Failed on %d. Added %d to the dataset.", count, failed, found)

	gapsStrWebhook := "None"
	gapsString := ""
	if failed > 0 {
		gapsStrWebhook = ""
		for i, gap := range gaps {
			if gap.Count == 1 {
				gapsString += fmt.Sprintf("%d\n", gap.Min)
			} else {
				gapsString += fmt.Sprintf("%d - %d (%d)\n", gap.Min, gap.Max, gap.Count)
			}
			if i < 15 {
				if gap.Count == 1 {
					gapsStrWebhook += fmt.Sprintf("%d\n", gap.Min)
				} else {
					gapsStrWebhook += fmt.Sprintf("%d - %d (%d)\n", gap.Min, gap.Max, gap.Count)
				}
			} else if i == 15 {
				gapsStrWebhook += "...and more"
			}
		}
	}

	fields := []discord.Field{{
		Name:  "Processed",
		Value: fmt.Sprintf("%d", count),
	}, {
		Name:  "Failed On",
		Value: fmt.Sprintf("%d", failed),
	}, {
		Name:  "Added to Dataset",
		Value: fmt.Sprintf("%d", found),
	}, {
		Name:  "Still Missing",
		Value: gapsStrWebhook,
	}}

	hadesAlerting.SendInfo("Processed missing PGCRs", fields, "PROCESSED_MISSING_PGCRS", map[string]any{
		"message": message,
		"count":   count,
		"failed":  failed,
		"found":   found,
		"minFailed": minFailed,
		"maxFailed": maxFailed,
		"range": maxFailed - minFailed + 1,
		"gaps": gaps,
	})

	if len(gaps) > 0 {
		logger.Info("GAPS_FOUND_IN_MISSED_LOG", map[string]any{logging.GAPS: gapsString})
	}
	if failed > 0 {
		logger.Info("FAILED_RANGE_SUMMARY", map[string]any{
			logging.MIN_FAILED: minFailed,
			logging.MAX_FAILED: maxFailed,
			logging.RANGE:      maxFailed - minFailed + 1,
		})
	}
}

func findGaps(failed []int64) []Gap {
	if len(failed) == 0 {
		return []Gap{}
	}
	// sort the failed slice and find contiguous ranges
	slices.Sort(failed)
	var gaps []Gap
	start := failed[0]
	end := failed[0]
	for i := 1; i < len(failed); i++ {
		if failed[i] == end+1 {
			end = failed[i]
		} else {
			gaps = append(gaps, Gap{
				Min:   start,
				Max:   end,
				Count: end - start + 1,
			})

			start = failed[i]
			end = failed[i]
		}
	}

	gaps = append(gaps, Gap{
		Min:   start,
		Max:   end,
		Count: end - start + 1,
	})

	return gaps
}

func main() {
	logging.ParseFlags()

	flushSentry, recoverSentry := logger.InitSentry()
	defer flushSentry()
	defer recoverSentry()

	ProcessMissedPGCRs()
}
