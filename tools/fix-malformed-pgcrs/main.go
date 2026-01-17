package main

import (
	"bufio"
	"context"
	"flag"
	"os"
	"strconv"
	"sync"
	"time"

	"raidhub/lib/database/postgres"
	"raidhub/lib/services/instance_storage"
	"raidhub/lib/services/pgcr_processing"
	"raidhub/lib/utils/logging"
)

var logger = logging.NewLogger("fix-malformed-pgcrs")

func main() {
	// Define our custom flags in the global flag package before logging.ParseFlags()
	filePath := flag.String("file", "", "path to file containing list of malformed PGCR instance IDs (one per line)")
	numWorkers := flag.Int("workers", 1, "number of workers to spawn")
	retries := flag.Int("retries", 5, "number of retries for each PGCR")

	logging.ParseFlags()

	if *filePath == "" {
		logger.Fatal("USAGE_ERROR", nil, map[string]any{
			"message": "Usage: scripts fix-malformed-pgcrs -file <path_to_log_file> [-workers N] [-retries N]",
		})
		return
	}

	// Wait for PostgreSQL connection to be ready
	postgres.Wait()

	// Read instance IDs from file
	instanceIDs, err := readInstanceIDsFromFile(*filePath)
	if err != nil {
		logger.Fatal("FAILED_TO_READ_FILE", err, map[string]any{"file": *filePath})
		return
	}

	logger.Info("READING_MALFORMED_PGCRS", map[string]any{
		"file":  *filePath,
		"count": len(instanceIDs),
	})

	if len(instanceIDs) == 0 {
		logger.Info("NO_INSTANCE_IDS_TO_PROCESS", nil)
		return
	}

	// Process instances
	var successCount int64
	var failureCount int64
	var skippedCount int64

	ch := make(chan int64)
	successes := make(chan int64)
	failures := make(chan int64)
	skipped := make(chan int64)
	var wg sync.WaitGroup

	// Start workers
	logger.Info("WORKERS_STARTING", map[string]any{"count": *numWorkers})
	wg.Add(*numWorkers)
	for i := 0; i < *numWorkers; i++ {
		go worker(ch, successes, failures, skipped, &wg, *retries)
	}

	// Collect results
	var wg2 sync.WaitGroup
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for id := range successes {
			successCount++
			_ = id
		}
	}()

	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for id := range failures {
			failureCount++
			_ = id
		}
	}()

	wg2.Add(1)
	go func() {
		defer wg2.Done()
		for id := range skipped {
			skippedCount++
			_ = id
		}
	}()

	// Send instance IDs to workers
	for _, id := range instanceIDs {
		ch <- id
	}

	close(ch)
	wg.Wait()
	close(successes)
	close(failures)
	close(skipped)
	wg2.Wait()

	logger.Info("PROCESSING_COMPLETE", map[string]any{
		"total":   len(instanceIDs),
		"success": successCount,
		"failed":  failureCount,
		"skipped": skippedCount,
	})
}

func readInstanceIDsFromFile(filePath string) ([]int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var instanceIDs []int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		instanceID, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			logger.Warn("INVALID_INSTANCE_ID_IN_FILE", err, map[string]any{"line": line})
			continue
		}
		instanceIDs = append(instanceIDs, instanceID)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return instanceIDs, nil
}

func worker(ch chan int64, successes chan int64, failures chan int64, skipped chan int64, wg *sync.WaitGroup, maxRetries int) {
	defer wg.Done()

	for instanceID := range ch {
		var errors = 0
		var processed = false

		for errors <= maxRetries && !processed {
			// Fetch and process the PGCR
			result, instance, pgcr := pgcr_processing.FetchAndProcessPGCR(context.Background(), instanceID, 0)

			if result == pgcr_processing.NonRaid {
				// Not a raid, skip it
				logger.Debug("PGCR_NOT_A_RAID", map[string]any{logging.INSTANCE_ID: instanceID})
				skipped <- instanceID
				processed = true
			} else if result == pgcr_processing.Success {
				// Replace the PGCR (delete old, store new)
				_, committed, err := instance_storage.ReplacePGCR(instance, pgcr)
				if err != nil {
					attempt := errors + 1
					if attempt > maxRetries {
						logger.Error("FAILED_TO_REPLACE_PGCR", err, map[string]any{
							logging.INSTANCE_ID: instanceID,
							logging.ATTEMPT:     attempt,
						})
					} else {
						logger.Warn("FAILED_TO_REPLACE_PGCR_RETRYING", err, map[string]any{
							logging.INSTANCE_ID: instanceID,
							logging.ATTEMPT:     attempt,
							logging.RETRIES:     maxRetries,
						})
					}
					time.Sleep(3 * time.Second)
					errors++
					continue
				} else if committed {
					logger.Debug("REPLACED_PGCR_SUCCESSFULLY", map[string]any{logging.INSTANCE_ID: instanceID})
					successes <- instanceID
					processed = true
				} else {
					// This shouldn't happen after replacement, but handle it
					logger.Debug("PGCR_REPLACEMENT_NO_CHANGE", map[string]any{logging.INSTANCE_ID: instanceID})
					successes <- instanceID
					processed = true
				}
			} else if result == pgcr_processing.ExternalError || result == pgcr_processing.RateLimited {
				time.Sleep(5 * time.Second)
				errors++
				// continue loop to retry
			} else if result == pgcr_processing.InsufficientPrivileges {
				logger.Warn("PGCR_INSUFFICIENT_PRIVILEGES", nil, map[string]any{logging.INSTANCE_ID: instanceID})
				skipped <- instanceID
				processed = true
			} else if result == pgcr_processing.NotFound {
				logger.Warn("PGCR_NOT_FOUND", nil, map[string]any{logging.INSTANCE_ID: instanceID})
				skipped <- instanceID
				processed = true
			} else {
				logger.Warn("COULD_NOT_PROCESS_PGCR", nil, map[string]any{
					logging.INSTANCE_ID: instanceID,
					"result":           result,
				})
				failures <- instanceID
				processed = true
			}
		}

		if errors > maxRetries && !processed {
			logger.Warn("FAILED_TO_PROCESS_PGCR_AFTER_RETRIES", nil, map[string]any{
				logging.INSTANCE_ID: instanceID,
				logging.RETRIES:     maxRetries,
			})
			failures <- instanceID
		}
	}
}

