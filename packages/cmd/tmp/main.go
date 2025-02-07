package main

import (
	"log"
	"raidhub/packages/cheat_detection"
	"raidhub/packages/postgres"
	"sync"
)

func main() {
	db, err := postgres.Connect()
	if err != nil {
		log.Fatal("Error connecting to postgres", err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT instance_id FROM instance WHERE completed ORDER BY instance_id DESC")
	if err != nil {
		log.Fatalf("Error getting instance_ids: %s", err)
	}
	defer rows.Close()

	log.Println("Starting cheat check")

	workers := 50
	var wg sync.WaitGroup
	ch := make(chan int64, workers*1024)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for instanceId := range ch {
				_, _, _, err := cheat_detection.CheckForCheats(instanceId, db)
				if err != nil {
					log.Printf("Failed to process cheat_check for instance %d: %v", instanceId, err)
					ch <- instanceId
				}
			}
		}()
	}
	for rows.Next() {
		var instanceId int64
		err = rows.Scan(&instanceId)
		if err != nil {
			log.Fatalf("Error getting instance_id: %s", err)
		}
		ch <- instanceId
	}
	close(ch)
	wg.Wait()

}
