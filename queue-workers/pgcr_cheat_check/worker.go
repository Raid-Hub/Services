package pgcr_cheat_check

import (
	"encoding/json"
	"log"
	"raidhub/queue-workers"
	"raidhub/shared/cheat_detection"

	amqp "github.com/rabbitmq/amqp091-go"
)

func process_request(qw *queueworkers.QueueWorker, msg amqp.Delivery) {
	var request PgcrCheatCheckRequest

	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Printf("Failed to unmarshal message: %s", err)
		return
	}

	instance, instanceResult, playerResults, isSolo, err := cheat_detection.CheckForCheats(request.InstanceId, qw.Db)
	if err != nil {
		log.Printf("Failed to process cheat_check for instance %d: %v", request.InstanceId, err)
		if err := msg.Reject(true); err != nil {
			log.Fatalf("Failed to acknowledge message: %v", err)
		}
	}

	if instanceResult.Probability > cheat_detection.Threshold {
		log.Printf("Flagging instance %d with probability %f: %s", instance.InstanceId, instanceResult.Probability, instanceResult.Explanation)
		go cheat_detection.SendFlaggedInstanceWebhook(instance, instanceResult, playerResults, isSolo)
	} else if len(playerResults) > 0 {
		for membershipId, result := range playerResults {
			if result.Probability <= cheat_detection.Threshold {
				log.Printf("Flagging player %d in instance %d with probability %f: %s", membershipId, instance.InstanceId, result.Probability, result.Explanation)
			}
		}
		go cheat_detection.SendFlaggedPlayerWebhooks(instance, playerResults)
	}
}
