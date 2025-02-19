package bonus_pgcr

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"raidhub/packages/async"
	"raidhub/packages/bungie"
	"raidhub/packages/discord"
	"raidhub/packages/pgcr"
	"raidhub/packages/pgcr_types"

	amqp "github.com/rabbitmq/amqp091-go"
)

type PGCRStoreRequest struct {
	Raw      *bungie.DestinyPostGameCarnageReport `json:"raw"`
	Activity *pgcr_types.ProcessedActivity        `json:"activity"`
}

func process_store_queue(qw *async.QueueWorker, msg amqp.Delivery) {
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
		log.Printf("%d added to data set", request.Activity.InstanceId)
		discord.SendWebhook(os.Getenv("PAN_WEBHOOK_URL"), &webhook)

		for i := request.Activity.InstanceId - 100_000; i < request.Activity.InstanceId+100_000; i++ {
			pgcr.WriteMissedLog(i)
		}
	} else {
		log.Printf("%d is already added", request.Activity.InstanceId)
	}
}
