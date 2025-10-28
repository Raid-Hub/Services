package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
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
	"raidhub/lib/web/discord"

	"slices"
)

var (
	gap        = flag.Bool("gap", false, "process gaps in the missed log")
	force      = flag.Bool("force", false, "force processing all PGCRs, ignoring database recency check")
	numWorkers = flag.Int("workers", 1, "number of workers to spawn")
	retries    = flag.Int("retries", 5, "number of retries for each PGCR")

	// Global worker state
	workerLatestId   int64
	workerForce      bool
	workerMaxRetries int
)

func main() {
	flag.Parse()

	// monitoring.RegisterPrometheus(9091)

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
					panic(err)
				}
			} else if !os.IsNotExist(err) {
				// Some other error checking missedLogPath
				panic(err)
			} else {
				// missedLogPath doesn't exist, nothing to process - just create empty missedLogPath and exit
				if err := os.MkdirAll(logDir, 0755); err != nil {
					panic(err)
				}
				missedLogFile, err := createFile(missedLogPath)
				if err != nil {
					panic(err)
				}
				missedLogFile.Close()
				log.Println("No missed PGCRs to process")
				return
			}
			// After moving missedLogPath to processingLogPath, we don't create a new missedLogPath yet
			// It will be created at the end after processing completes, so that any failed PGCRs
			// written during processing are preserved
		} else {
			panic(err)
		}
	}

	logFile, err := os.Open(processingLogPath)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	log.Printf("Reading missed PGCRs from: %s", processingLogPath)

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
		log.Fatalf("Gap too large, max: %d, min: %d", maxN, minN)
	}

	if *gap && minN > 0 && maxN > 0 {
		log.Printf("Found %d unique numbers in the file, min: %d, max: %d", len(uniqueNumbers), minN, maxN)
		for i := minN - 1000; i <= maxN+1000; i++ {
			uniqueNumbers[i] = true
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// postgres.DB is initialized in init()
	stmnt, err := postgres.DB.Prepare("SELECT instance_id FROM instance INNER JOIN pgcr USING (instance_id) WHERE instance_id = $1 LIMIT 1;")
	if err != nil {
		log.Fatalf("Error preparing statement: %s", err)
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
		log.Printf("Processing %d PGCRs in the gap", len(numbers))
	} else {
		log.Printf("Found %d missing PGCRs", len(numbers))
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
				panic(fmt.Sprintf("error getting latest instance ID: %s", err))
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
		log.Printf("Workers starting at %d", firstId)
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
			panic(err)
		}
		log.Println("Temporary file deleted successfully")
	}

	// Create new empty missedLogPath only if it doesn't exist
	// (failed PGCRs written during processing should be preserved)
	if _, err := os.Stat(missedLogPath); os.IsNotExist(err) {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			panic(err)
		}
		missedLogFile, err := createFile(missedLogPath)
		if err != nil {
			panic(err)
		}
		missedLogFile.Close()
	}

	gaps := findGaps(failed)
	postResults(len(numbers), len(failed), len(found), minFailed, maxFailed, gaps)
}

func worker(ch chan int64, successes chan int64, failures chan int64, wg *sync.WaitGroup) {
	defer wg.Done()

	for instanceID := range ch {
		if !workerForce && instanceID > workerLatestId {
			log.Printf("PGCR %d is newer than latestId, skipping", instanceID)
			instance_storage.WriteMissedLog(instanceID)
			failures <- instanceID
			continue
		}

		var errors = 0
		var processed = false
		for errors <= workerMaxRetries && !processed {
			result, activity, raw, err := pgcr_processing.FetchAndProcessPGCR(instanceID)

			if result == pgcr_processing.NonRaid {
				processed = true
				// NonRaid activities are successfully processed, just not raids
			} else if result == pgcr_processing.Success {
				_, committed, err := instance_storage.StorePGCR(activity, raw)
				if err != nil {
					log.Printf("Failed to store PGCR: %v", err)
					time.Sleep(3 * time.Second)
					errors++
					continue
				} else if committed {
					log.Printf("Found raid %d", instanceID)
					successes <- instanceID
					processed = true
				} else {
					log.Printf("Non-raid activity %d", instanceID)
					processed = true
					// Not a raid, successfully processed and ignored
				}
			} else if result == pgcr_processing.ExternalError || result == pgcr_processing.InternalError || result == pgcr_processing.DecodingError || result == pgcr_processing.RateLimited {
				log.Printf("Error fetching instanceId %d: %s", instanceID, err)
				time.Sleep(5 * time.Second)
				errors++
				// continue loop to retry
			} else {
				log.Printf("Could not resolve instance id %d: %s", instanceID, err)
				instance_storage.WriteMissedLog(instanceID)
				failures <- instanceID
				processed = true
			}
		}
		if errors >= workerMaxRetries {
			log.Printf("Failed to fetch instanceId %d %d+ times, skipping", instanceID, workerMaxRetries)
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

func postResults(count int, failed int, found int, minFailed int64, maxFailed int64, gaps []Gap) {
	// Discord webhook URL
	webhookURL := env.HadesWebhookURL

	// Message to be sent
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

	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Processed missing PGCRs",
			Color: 3447003, // Blue
			Fields: []discord.Field{{
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
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(webhookURL, &webhook)
	log.Println(message)
	if len(gaps) > 0 {
		log.Printf("Gaps found in the missed log:\n%s", gapsString)
	}
	if failed > 0 {
		log.Printf("Min failed on: %d, Max failed on: %d, range: %d", minFailed, maxFailed, maxFailed-minFailed+1)
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
