package pgcr_exists

import (
	"database/sql"
	"log"
	"net/http"
	"raidhub/packages/async"
	"raidhub/packages/pgcr"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

func process_fetch_request(qw *async.QueueWorker, msg amqp.Delivery, client *http.Client, apiKey string) {
	qw.Wg.Wait()

	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	instanceId, err := strconv.ParseInt(string(msg.Body), 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse message body: %s", err)
		return
	}

	exists, err := check_if_pgcr_exists(instanceId, qw.Db)
	if err != nil {
		log.Printf("Error reading database for pgcr request %d: %s", instanceId, err)
	} else if exists {
		return
	} else {
		result, activity, raw, err := pgcr.FetchAndProcessPGCR(client, instanceId, apiKey)

		if err != nil {
			log.Printf("Error fetching instanceId %d: %s", instanceId, err)
			pgcr.WriteMissedLog(instanceId)
			return
		}

		if result == pgcr.Success {
			sendStoreMessage(outgoing, activity, raw)
		} else if result == pgcr.NonRaid {
			log.Printf("%d is not a raid", instanceId)
		} else {
			log.Printf("%d returned a nil error result: %d", instanceId, result)
			pgcr.WriteMissedLog(instanceId)
		}
	}
}

func check_if_pgcr_exists(instanceid int64, db *sql.DB) (bool, error) {
	var result bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM instance a INNER JOIN pgcr ON a.instance_id = pgcr.instance_id WHERE a.instance_id = $1 LIMIT 1)`, instanceid).Scan(&result)
	if err != nil {
		return false, err
	} else {
		return result, nil
	}
}
