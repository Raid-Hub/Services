package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"raidhub/packages/discord"
	"raidhub/packages/monitoring"
	"raidhub/packages/pgcr"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"

	"github.com/rabbitmq/amqp091-go"
)

const (
	numWorkers = 100
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	monitoring.RegisterPrometheus(9091)

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
	for scanner.Scan() {
		line := scanner.Text()
		number, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			fmt.Printf("Error parsing line %s: %v\n", line, err)
			continue
		}
		uniqueNumbers[number] = true
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
	var numbers []int64
	for num := range uniqueNumbers {
		var foo int64
		err := stmnt.QueryRow(num).Scan(&foo)
		if err != nil {
			log.Printf("Preparing %d", num)
			numbers = append(numbers, num)
		} else {
			log.Printf("Skipping %d", num)
		}
	}

	log.Printf("Found %d missing PGCRs", len(numbers))
	// Sort the numbers
	sort.Slice(numbers, func(i, j int) bool {
		return numbers[i] < numbers[j]
	})

	if err != nil {
		log.Fatalf("Error connecting to the database: %s", err)
	}
	defer db.Close()

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
	if len(numbers) > 0 {
		latestID := numbers[0]
		ch := make(chan int64)
		successes := make(chan int64)
		failures := make(chan int64)
		var wg sync.WaitGroup

		// Start workers
		log.Printf("Workers starting at %d", latestID)
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go worker(ch, successes, failures, db, rabbitChannel, &wg)
		}

		go func() {
			for id := range successes {
				found = append(found, id)
			}
		}()

		var wg2 sync.WaitGroup
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for id := range failures {
				failed = append(failed, id)
			}
		}()

		for j := 0; j < len(numbers); j++ {
			latestID = numbers[j]
			ch <- latestID
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

	webhook(len(numbers), len(failed), len(found))
}

func worker(ch chan int64, successes chan int64, failures chan int64, db *sql.DB, rabbitChannel *amqp091.Channel, wg *sync.WaitGroup) {
	defer wg.Done()
	securityKey := os.Getenv("BUNGIE_API_KEY")

	client := &http.Client{}

	for instanceID := range ch {
		result, activity, raw, err := pgcr.FetchAndProcessPGCR(client, instanceID, securityKey)
		if err != nil {
			log.Println(err)
		}

		if result == pgcr.NonRaid {
			log.Printf("Non raid %d", instanceID)
			continue
		} else if result == pgcr.Success {
			_, committed, err := pgcr.StorePGCR(activity, raw, db, rabbitChannel)
			if err != nil {
				log.Printf("Failed to store raid %d: %s", instanceID, err)
				writeMissedLog(instanceID)
				failures <- instanceID
			} else if committed {
				log.Printf("Found raid %d", instanceID)
				successes <- instanceID
			}
		} else {
			log.Printf("Could not resolve instance id %d: %s", instanceID, err)
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

func webhook(count int, failed int, found int) {
	// Discord webhook URL
	webhookURL := os.Getenv("HADES_WEBHOOK_URL")

	// Message to be sent
	message := fmt.Sprintf("Info: Processed %d missing PGCR(s). Failed on %d. Added %d to the dataset.", count, failed, found)

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
			}},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer:    discord.CommonFooter,
		}},
	}
	discord.SendWebhook(webhookURL, &webhook)
	log.Println(message)
}
