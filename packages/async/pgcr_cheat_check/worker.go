package pgcr_cheat_check

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"raidhub/packages/async"
	"raidhub/packages/cheat_detection"
	"raidhub/packages/discord"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/time/rate"
)

var (
	_webhookUrl string
	once        sync.Once
	webhookRl   = rate.NewLimiter(rate.Every(10*time.Second), 5)
)

func getWebhookUrl() string {
	once.Do(func() {
		_webhookUrl = os.Getenv("CHEAT_CHECK_WEBHOOK_URL")
	})
	return _webhookUrl
}

func process_request(qw *async.QueueWorker, msg amqp.Delivery) {
	var request PgcrCheatCheckRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Fatalf("Failed to unmarshal message: %s", err)
		return
	}

	instanceResult, playerResults, isSolo, err := cheat_detection.CheckForCheats(request.InstanceId, qw.Db)
	if err != nil {
		log.Printf("Failed to process cheat_check for instance %d: %v", request.InstanceId, err)
		if err := msg.Reject(true); err != nil {
			log.Fatalf("Failed to acknowledge message: %v", err)
		}
	}

	if instanceResult.Probability > cheat_detection.Threshold {
		go sendFlaggedInstanceWebhook(request.InstanceId, instanceResult, playerResults, isSolo)
	} else if len(playerResults) > 0 {
		go sendFlaggedPlayerWebhooks(request.InstanceId, playerResults)
	}

	if err := msg.Ack(false); err != nil {
		log.Fatalf("Failed to acknowledge message: %v", err)
	}
}

func sendFlaggedInstanceWebhook(instanceId int64, result cheat_detection.ResultTuple, playerResults []cheat_detection.ResultTuple, isSolo bool) {
	ctx := context.Background()
	err := webhookRl.Wait(ctx)
	if err != nil {
		log.Fatalf("Failed to wait for rate limiter: %v", err)
	}

	webhook := discord.Webhook{
		Embeds: []discord.Embed{{
			Title: "Flagged instance",
			Color: int(0xFFFF00 + (0xFF0000-0xFFFF00)*result.Probability), // Yellow to Red based on probability
			Fields: []discord.Field{
				{
					Name:  "URL",
					Value: fmt.Sprintf("https://raidhub.io/pgcr/%d", instanceId),
				},
				{
					Name:  "Cheat Probability",
					Value: fmt.Sprintf("%.3f", result.Probability),
				},
				{
					Name:  "Explanation",
					Value: result.Explanation,
				},
			},
			Timestamp: time.Now().Format(time.RFC3339),
			Footer: discord.Footer{
				Text:    fmt.Sprintf("Cheat Check %s", cheat_detection.CheatCheckVersion),
				IconURL: discord.CommonFooter.IconURL,
			},
		}},
	}

	if !isSolo {
		for _, playerResult := range playerResults {
			if playerResult.Probability > cheat_detection.Threshold {
				webhook.Embeds[0].Fields = append(webhook.Embeds[0].Fields, discord.Field{
					Name:  fmt.Sprintf("Player %d", playerResult.MembershipId),
					Value: fmt.Sprintf("%.3f - https://raidhub.io/pgcr/%d?player=%d - %s", playerResult.Probability, instanceId, playerResult.MembershipId, playerResult.Explanation),
				})
			}
		}
	}

	discord.SendWebhook(getWebhookUrl(), &webhook)
}

func sendFlaggedPlayerWebhooks(instanceId int64, playerResults []cheat_detection.ResultTuple) {
	ctx := context.Background()
	err := webhookRl.Wait(ctx)
	if err != nil {
		log.Fatalf("Failed to wait for rate limiter: %v", err)
	}

	webhook := discord.Webhook{
		Embeds: []discord.Embed{},
	}
	for _, playerResult := range playerResults {
		if playerResult.Probability > cheat_detection.Threshold {
			webhook.Embeds = append(webhook.Embeds, discord.Embed{
				Title: "Flagged player",
				Color: int(0xFFFF00 + (0xFF0000-0xFFFF00)*playerResult.Probability), // Yellow to Red based on probability
				Fields: []discord.Field{
					{
						Name:  "URL",
						Value: fmt.Sprintf("https://raidhub.io/pgcr/%d?player=%d", instanceId, playerResult.MembershipId),
					},
					{
						Name:  "Cheat Probability",
						Value: fmt.Sprintf("%.3f", playerResult.Probability),
					},
					{
						Name:  "Explanation",
						Value: playerResult.Explanation,
					},
				},
				Timestamp: time.Now().Format(time.RFC3339),
				Footer: discord.Footer{
					Text:    fmt.Sprintf("Cheat Check %s", cheat_detection.CheatCheckVersion),
					IconURL: discord.CommonFooter.IconURL,
				},
			})
			log.Printf("Flagged player %d in instance %d with probability %f: %s", playerResult.MembershipId, instanceId, playerResult.Probability, playerResult.Explanation)
		}
	}

	discord.SendWebhook(getWebhookUrl(), &webhook)
}
