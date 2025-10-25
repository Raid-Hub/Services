package pgcr_exists

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"raidhub/queue-workers"
	"raidhub/shared/bungie"
	"raidhub/packages/discord"
	"raidhub/shared/pgcr"
	"raidhub/shared/pgcr_types"
	"raidhub/shared/database/postgres"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PGCRStoreRequest struct {
	Raw      *bungie.DestinyPostGameCarnageReport `json:"raw"`
	Activity *pgcr_types.ProcessedActivity        `json:"activity"`
}

func process_store_queue(qw *queueworkers.QueueWorker, msg amqp.Delivery) {
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	var request PGCRStoreRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Println(string(msg.Body[:]))
		log.Fatalf("Failed to unmarshal pgcr store request: %s", err)
		return
	}

	latestId, err := postgres.GetLatestInstanceId(qw.Db, 1000)
	if err != nil {
		log.Printf("Failed to get latest instanceId: %s", err)
		pgcr.WriteMissedLog(request.Activity.InstanceId)
		return
	}

	offset := request.Activity.InstanceId - latestId
	if offset > 0 {
		log.Printf("InstanceId %d is too far ahead of latestId %d", request.Activity.InstanceId, latestId)
		pgcr.WriteMissedLog(request.Activity.InstanceId)
		return
	}

	if request.Activity.PlayerCount > 20 {
		// For now, don't bother with checkpoint instances and log for later
		log.Printf("Skipping PGCR %d with %d players", request.Activity.InstanceId, request.Activity.PlayerCount)
		pgcr.WriteMissedLog(request.Activity.InstanceId)
		return
	}

	_, committed, err := pgcr.StorePGCR(request.Activity, request.Raw, qw.Db, outgoing)
	if err != nil {
		log.Printf("Error storing instanceId %d: %s", request.Activity.InstanceId, err)
		pgcr.WriteMissedLog(request.Activity.InstanceId)
	} else if committed {
		msg := fmt.Sprintf("Found missing PGCR: %d", request.Activity.InstanceId)
		webhook := discord.Webhook{
			Content: &msg,
		}

		if offset < -10_000_000 {
			// If the offset is too large, we need to write a range of missed logs
			msg = fmt.Sprintf("Found missing PGCR: %d from <t:%d>", request.Activity.InstanceId, request.Activity.DateCompleted.Unix())
			for i := request.Activity.InstanceId - 10_000; i < request.Activity.InstanceId+10_000; i++ {
				pgcr.WriteMissedLog(i)
			}
		}

		discord.SendWebhook(os.Getenv("PAN_WEBHOOK_URL"), &webhook)

	} else {
		log.Printf("%d is already added", request.Activity.InstanceId)
	}
}
