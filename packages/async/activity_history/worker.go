package activity_history

import (
	"log"
	"raidhub/packages/async"
	"raidhub/packages/async/pgcr_exists"
	"raidhub/packages/bungie"
	"raidhub/packages/rabbit"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	outgoing *amqp.Channel
	once     sync.Once
)

func create_outbound_channel() {
	once.Do(func() {
		conn, err := rabbit.Init()
		if err != nil {
			log.Fatalf("Failed to create outbound channel: %s", err)
		}
		outgoing, _ = conn.Channel()
	})
}

func process_request(qw *async.QueueWorker, msg amqp.Delivery) {
	qw.Wg.Wait()
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	membershipId, err := strconv.ParseInt(string(msg.Body), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse message body: %s", err)
		return
	}

	var lastCrawled *time.Time
	err = qw.Db.QueryRow(`SELECT history_last_crawled FROM player WHERE membership_id = $1 LIMIT 1`, membershipId).Scan(&lastCrawled)
	if err != nil {
		log.Println("Failed to get last crawled time:", err)
		return
	}
	if lastCrawled != nil && time.Since(*lastCrawled) < 400*time.Hour {
		log.Printf("Skipping history call for player %d, history last crawled at %s", membershipId, lastCrawled)
	}

	profiles, err := bungie.GetLinkedProfiles(-1, membershipId, false)
	if err != nil {
		log.Printf("Failed to get linked profiles: %s", err)
		return
	}

	var membershipType int
	for _, profile := range profiles {
		if profile.MembershipId == membershipId {
			membershipType = profile.MembershipType
			break
		}
	}

	if membershipType == 0 {
		log.Printf("Failed to find membership type for %d", membershipId)
		return
	}

	stats, err := bungie.GetHistoricalStats(membershipType, membershipId)
	if err != nil {
		log.Printf("Failed to get stats: %s", err)
		return
	}

	out := make(chan int64, 2000)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for instanceId := range out {
			pgcr_exists.SendFetchMessage(outgoing, instanceId)
		}
	}()

	var success = false
	for _, character := range stats.Characters {
		err := bungie.GetActivityHistory(membershipType, membershipId, character.CharacterId, 3, out)
		if err != nil {
			log.Println(err)
			break
		}
		success = true
	}

	if success {
		log.Printf("Updating player %d history_last_crawled", membershipId)
		_, err := qw.Db.Exec(`UPDATE player SET history_last_crawled = NOW() WHERE membership_id = $1`, membershipId)
		if err != nil {
			log.Fatal(err)
		}
	}

	close(out)
	wg.Wait()
}
