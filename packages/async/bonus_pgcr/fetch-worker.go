package bonus_pgcr

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"raidhub/packages/async"
	"raidhub/packages/pgcr"
	"strconv"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PGCRFetchRequest struct {
	InstanceId string `json:"instanceId"`
}

func process_fetch_request(qw *async.QueueWorker, msg amqp.Delivery, client *http.Client, apiKey string) {
	qw.Wg.Wait()

	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	var request PGCRFetchRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Fatalf("Failed to unmarshal message: %s", err)
		return
	}

	log.Printf("Checking bonus pgcr %s", request.InstanceId)
	exists, err := check_if_pgcr_exists(request.InstanceId, qw.Db)
	if err != nil {
		log.Printf("Error reading database for pgcr request %s: %s", request.InstanceId, err)
	} else if exists {
		log.Printf("%s already exists", request.InstanceId)
		return
	} else {
		instanceIdInt, err := strconv.ParseInt(request.InstanceId, 10, 64)
		if err != nil {
			log.Printf("Error parsing instance_id %s: %s", request.InstanceId, err)
			return
		}

		result, activity, raw, err := pgcr.FetchAndProcessPGCR(client, instanceIdInt, apiKey)

		if err != nil {
			log.Printf("Error fetching instanceId %d: %s", instanceIdInt, err)
			pgcr.WriteMissedLog(instanceIdInt)
			return
		}

		if result == pgcr.Success {
			sendStoreMessage(outgoing, activity, raw)
		} else if result == pgcr.NonRaid {
			log.Printf("%s is not a raid", request.InstanceId)
		} else {
			log.Printf("%s returned a nil error result: %d", request.InstanceId, result)
			pgcr.WriteMissedLog(instanceIdInt)
		}
	}
}

func check_if_pgcr_exists(instanceid string, db *sql.DB) (bool, error) {
	var result bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM activity a INNER JOIN pgcr ON a.instance_id = pgcr.instance_id WHERE a.instance_id = $1 LIMIT 1)`, instanceid).Scan(&result)
	if err != nil {
		return false, err
	} else {
		return result, nil
	}
}
