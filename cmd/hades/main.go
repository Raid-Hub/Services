package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"raidhub/packages/discord"
	"raidhub/packages/pgcr"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"

	"slices"

	"github.com/rabbitmq/amqp091-go"
)

var (
	gap = flag.Bool("gap", false, "process gaps in the missed log")
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	flag.Parse()

	// monitoring.RegisterPrometheus(9091)

	src := filepath.Join(cwd, "logs", "missed.log")
	temp := filepath.Join(cwd, "logs", "missed.temp.log")

	_, err = os.Stat(temp)
	if err != nil {
		if os.IsNotExist(err) {
			err = moveFile(src, temp)
			if err != nil {
				panic(err)
			}

			_, err = createFile(src)
			if err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}

	file, err := os.Open(temp)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Create a map to store unique numbers
	uniqueNumbers := make(map[int64]bool)

	// Read the file line by line
	scanner := bufio.NewScanner(file)
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

	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}
	defer db.Close()

	stmnt, err := db.Prepare("SELECT instance_id FROM instance INNER JOIN pgcr USING (instance_id) WHERE instance_id = $1 LIMIT 1;")
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

	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}

	latestId, err := postgres.GetLatestInstanceId(db, 5_000)
	if err != nil {
		log.Fatalf("Error getting latest instance ID: %s", err)
	}

	conn, err := rabbit.Init()
	if err != nil {
		log.Fatalf("Error connecting to rabbit: %s", err)
	}
	defer rabbit.Cleanup()

	rabbitChannel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to create channel: %s", err)
	}
	defer rabbitChannel.Close()

	var found []int64
	var failed []int64
	minFailed := int64(^uint64(0) >> 1) // Max int64 value
	maxFailed := int64(0)
	if len(numbers) > 0 {
		firstId := numbers[0]
		ch := make(chan int64)
		successes := make(chan int64)
		failures := make(chan int64)
		var wg sync.WaitGroup

		numWorkers := len(numbers) / 10
		numWorkers = max(5, numWorkers)
		numWorkers = min(numWorkers, 200)

		// Start workers
		log.Printf("Workers starting at %d", firstId)
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go worker(ch, successes, failures, db, rabbitChannel, &wg, latestId)
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

	if err := os.Remove(temp); err != nil {
		panic(err)
	}

	log.Println("Temporary file deleted successfully")

	gaps := findGaps(failed)
	postResults(len(numbers), len(failed), len(found), minFailed, maxFailed, gaps)
}

func worker(ch chan int64, successes chan int64, failures chan int64, db *sql.DB, rabbitChannel *amqp091.Channel, wg *sync.WaitGroup, latestId int64) {
	defer wg.Done()
	securityKey := os.Getenv("BUNGIE_API_KEY")

	client := &http.Client{}

	for instanceID := range ch {
		if instanceID > latestId {
			log.Printf("PGCR %d is newer than latestId, skipping", instanceID)
			writeMissedLog(instanceID)
			failures <- instanceID
			continue
		}

		var errors = 0
		for errors < 10 {
			result, activity, raw, err := pgcr.FetchAndProcessPGCR(client, instanceID, securityKey)

			if result == pgcr.NonRaid {
				break
			} else if result == pgcr.Success {
				_, committed, err := pgcr.StorePGCR(activity, raw, db, rabbitChannel)
				if err != nil {
					log.Printf("Failed to store raid %d: %s", instanceID, err)
					time.Sleep(3 * time.Second)
					errors++
					continue
				} else if committed {
					log.Printf("Found raid %d", instanceID)
					successes <- instanceID
				}
				break
			} else if result == pgcr.ExternalError || result == pgcr.InternalError || result == pgcr.DecodingError || result == pgcr.RateLimited {
				log.Printf("Error fetching instanceId %d: %s", instanceID, err)
				time.Sleep(5 * time.Second)
				errors++
				continue
			} else {
				log.Printf("Could not resolve instance id %d: %s", instanceID, err)
				writeMissedLog(instanceID)
				failures <- instanceID
				break
			}
		}
		if errors >= 10 {
			log.Printf("Failed to fetch instanceId %d 10+ times, skipping", instanceID)
			writeMissedLog(instanceID)
			failures <- instanceID
		}
	}
}

func createFile(src string) (*os.File, error) {
	file, err := os.Create(src)
	return file, err
}

func moveFile(src, dst string) error {
	err := os.Rename(src, dst)
	return err
}

func writeMissedLog(instanceId int64) {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Open the file in append mode with write permissions
	file, err := os.OpenFile(filepath.Join(cwd, "logs", "missed.log"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	// Create a writer to append to the file
	writer := bufio.NewWriter(file)

	// Write the line you want to append
	_, err = writer.WriteString(fmt.Sprint(instanceId) + "\n")
	if err != nil {
		log.Fatalln(err)
	}

	// Flush the writer to ensure the data is written to the file
	err = writer.Flush()
	if err != nil {
		log.Fatalln(err)
	}
}

type Gap struct {
	Min   int64
	Max   int64
	Count int64
}

func postResults(count int, failed int, found int, minFailed int64, maxFailed int64, gaps []Gap) {
	// Discord webhook URL
	webhookURL := os.Getenv("HADES_WEBHOOK_URL")

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
