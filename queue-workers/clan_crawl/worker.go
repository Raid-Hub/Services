package clan_crawl

import (
	"database/sql"
	"encoding/json"
	"log"
	"raidhub/queue-workers"
	"raidhub/shared/bungie"
	clan_util "raidhub/shared/clan"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func process_request(qw *queueworkers.QueueWorker, msg amqp.Delivery) {
	qw.Wg.Wait()
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	var request ClanRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Printf("Failed to unmarshal message: %s", err)
		return
	}

	var lastCrawled sql.NullTime
	row := qw.Db.QueryRow(`SELECT updated_at FROM clan WHERE group_id = $1`, request.GroupId)

	err := row.Scan(&lastCrawled)
	if err != nil && err != sql.ErrNoRows {
		log.Fatalf("Error getting last crawled time for clan %d: %s", request.GroupId, err)
	}

	if err != nil || !lastCrawled.Valid || time.Since(lastCrawled.Time) > 3*time.Hour {
		res, code, err := bungie.GetGroup(request.GroupId)
		if err != nil {
			if code == 622 {
				log.Printf("Group %d not found", request.GroupId)
			} else {
				log.Printf("Error getting group %d: %s", request.GroupId, err)
			}
		}

		if res.Detail.GroupType != 1 {
			log.Printf("Group %d is not a clan, skipping", request.GroupId)
			return
		}

		clanBannerData, name, callSign, motto, err := clan_util.ParseClanDetails(&res.Detail)
		if err != nil {
			log.Fatalf("Error parsing clan details: %s", err)
		}

		_, err = qw.Db.Exec(`INSERT INTO clan (group_id, name, motto, call_sign, clan_banner_data, updated_at) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (group_id)
			DO UPDATE SET name = $2, motto = $3, call_sign = $4, clan_banner_data = $5, updated_at = $6`,
			res.Detail.GroupId, name, motto, callSign, clanBannerData, time.Now().UTC())
		if err != nil {
			log.Fatalf("Error upserting clan %d: %s", res.Detail.GroupId, err)
		}

		log.Printf("Upserted clan %d: %s", res.Detail.GroupId, name)
	} else {
		log.Printf("Skipping crawl for clan %d", request.GroupId)
	}

}
