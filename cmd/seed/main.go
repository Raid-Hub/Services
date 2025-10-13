package main

import (
	"log"
	"os"
	"raidhub/packages/async/activity_history"
	"raidhub/packages/async/player_crawl"
	"raidhub/packages/postgres"
	"raidhub/packages/rabbit"
	"strconv"
	"strings"
	"sync"

	_ "github.com/lib/pq"
)

func main() {
	// Parse the ids
	membershipIds := os.Args[1:]

	data, err := os.ReadFile("data.sql")
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	statements := strings.Split(string(data), ";")

	db, err := postgres.Connect()
	if err != nil {
		log.Fatalf("Failed to create postgres connection: %s", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Failed to create postgres transaction: %s", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("TRUNCATE TABLE season")
	if err != nil {
		log.Fatalf("Error truncating table: %s", err)
	}
	_, err = tx.Exec("TRUNCATE TABLE activity_definition")
	if err != nil {
		log.Fatalf("Error truncating table: %s", err)
	}
	_, err = tx.Exec("TRUNCATE TABLE version_definition")
	if err != nil {
		log.Fatalf("Error truncating table: %s", err)
	}
	_, err = tx.Exec("TRUNCATE TABLE activity_version")
	if err != nil {
		log.Fatalf("Error truncating table: %s", err)
	}

	for _, statement := range statements {
		// Trim any leading or trailing whitespace
		query := strings.TrimSpace(statement)
		if query == "" {
			continue // Skip empty statements
		}

		// Execute the SQL statement
		_, err := tx.Exec(query)
		if err != nil {
			log.Fatalf("Error executing statement: %v", err)
		}
		log.Println("Statement executed successfully:", query)
	}

	err = tx.Commit()
	if err != nil {
		log.Fatalf("Error committing transaction: %s", err)
	}

	// Connect to the RabbitMQ
	conn, err := rabbit.Init()
	if err != nil {
		log.Fatalf("Failed to create rabbit connection: %s", err)
	}
	defer rabbit.Cleanup()

	rabbitChannel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to create rabbit channel: %s", err)
	}
	defer rabbitChannel.Close()

	var wg sync.WaitGroup
	for _, id := range membershipIds {
		wg.Add(1)
		go func(membershipId string) {
			defer wg.Done()
			idInt64, err := strconv.ParseInt(membershipId, 10, 64)
			if err != nil {
				log.Fatal(err)
			}
			err = player_crawl.SendMessage(rabbitChannel, idInt64)
			if err != nil {
				log.Fatal(err)
			}
			err = activity_history.SendMessage(rabbitChannel, idInt64)
			if err != nil {
				log.Fatal(err)
			}
		}(id)
	}

	wg.Wait()

	if len(membershipIds) > 0 {
		log.Println("Queued all players for crawl and activity history. Make sure to run bin/hermes")
	}

}
