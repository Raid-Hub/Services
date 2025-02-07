package pgcr_clickhouse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"raidhub/packages/pgcr_types"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	maxBatchSize = 8192
	batchTime    = 30 * time.Second
)

func process_queue(clickhouse *driver.Conn, msgs <-chan amqp.Delivery) {

	batch := make([]amqp.Delivery, 0, maxBatchSize)
	timer := time.NewTimer(batchTime)

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				// Channel is closed, process remaining messages and return
				if len(batch) > 0 {
					process(batch, clickhouse)
				}
				return
			}

			batch = append(batch, msg)

			if len(batch) >= maxBatchSize {
				chunk := batch[0:maxBatchSize]
				process(chunk, clickhouse)
				batch = batch[maxBatchSize:]
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(batchTime)
			}
		case <-timer.C:
			if len(batch) > 0 {
				process(batch, clickhouse)
				batch = batch[:0]
			}

			timer.Reset(batchTime)
		}

	}
}

type ClickhouseInstance = map[string]interface{}

func process(msgs []amqp.Delivery, c *driver.Conn) {
	var success bool
	defer func() {
		if !success {
			log.Printf("failed to send %d instances to Clickhouse, rejecting all", len(msgs))
		} else {
			log.Printf("Sent %d instances to Clickhouse", len(msgs))
		}
		for _, msg := range msgs {
			if success {
				if err := msg.Ack(false); err != nil {
					log.Printf("Failed to acknowledge messages: %v", err)
				}
			} else {
				if err := msg.Reject(true); err != nil {
					log.Printf("Failed to acknowledge messages: %v", err)
				}
			}
		}

	}()
	var instances []ClickhouseInstance
	for _, msg := range msgs {
		var request pgcr_types.ProcessedActivity
		if err := json.Unmarshal(msg.Body, &request); err != nil {
			log.Println("Failed to unmarshal activity:", err)
			return
		}
		instances = append(instances, *parse(request))
	}

	err := insertProcessedInstances(*c, instances)
	if err != nil {
		log.Fatalf("Failed to insert instances: %s", err)
	}

	success = true
}

func parse(request pgcr_types.ProcessedActivity) *ClickhouseInstance {
	instance := ClickhouseInstance{
		"instance_id":    request.InstanceId,
		"hash":           request.Hash,
		"completed":      request.Completed,
		"player_count":   request.PlayerCount,
		"fresh":          2,
		"flawless":       2,
		"date_started":   request.DateStarted,
		"date_completed": request.DateCompleted,
		"platform_type":  uint16(request.MembershipType),
		"duration":       request.DurationSeconds,
		"score":          request.Score,
	}
	if request.Fresh != nil {
		if *request.Fresh {
			instance["fresh"] = 1
		} else {
			instance["fresh"] = 0
		}
	}
	if request.Flawless != nil {
		if *request.Flawless {
			instance["flawless"] = 1
		} else {
			instance["flawless"] = 0
		}
	}

	players := make([]map[string]interface{}, len(request.Players))

	for i, player := range request.Players {
		instancePlayer := map[string]interface{}{
			"membership_id":       player.Player.MembershipId,
			"completed":           player.Finished,
			"time_played_seconds": player.TimePlayedSeconds,
			"sherpas":             player.Sherpas,
			"is_first_clear":      player.IsFirstClear,
		}
		characters := make([]map[string]interface{}, len(player.Characters))

		for j, character := range player.Characters {
			instanceCharacter := map[string]interface{}{
				"character_id":        character.CharacterId,
				"class_hash":          0,
				"emblem_hash":         0,
				"completed":           character.Completed,
				"score":               character.Score,
				"kills":               character.Kills,
				"assists":             character.Assists,
				"deaths":              character.Deaths,
				"precision_kills":     character.PrecisionKills,
				"super_kills":         character.SuperKills,
				"grenade_kills":       character.GrenadeKills,
				"melee_kills":         character.MeleeKills,
				"time_played_seconds": character.TimePlayedSeconds,
				"start_seconds":       character.StartSeconds,
			}
			if character.ClassHash != nil {
				instanceCharacter["class_hash"] = *character.ClassHash
			}
			if character.EmblemHash != nil {
				instanceCharacter["emblem_hash"] = *character.EmblemHash
			}
			weapons := make([]map[string]interface{}, len(character.Weapons))

			for k, weapon := range character.Weapons {
				instanceCharacterWeapon := map[string]interface{}{
					"weapon_hash":     weapon.WeaponHash,
					"kills":           weapon.Kills,
					"precision_kills": weapon.PrecisionKills,
				}
				weapons[k] = instanceCharacterWeapon
			}
			instanceCharacter["weapons"] = weapons
			characters[j] = instanceCharacter
		}
		instancePlayer["characters"] = characters
		players[i] = instancePlayer
	}
	instance["players"] = players
	return &instance
}

func insertProcessedInstances(conn driver.Conn, instances []ClickhouseInstance) error {
	ctx := context.Background()
	batch, err := conn.PrepareBatch(ctx, "INSERT INTO instance SETTINGS async_insert=1, wait_for_async_insert=1")
	if err != nil {
		return fmt.Errorf("error preparing batch for instances: %s", err)
	}

	for _, instance := range instances {
		err = batch.Append(
			instance["instance_id"],
			instance["hash"],
			instance["completed"],
			instance["player_count"],
			instance["fresh"],
			instance["flawless"],
			instance["date_started"],
			instance["date_completed"],
			instance["platform_type"],
			instance["duration"],
			instance["score"],
			instance["players"])
		if err != nil {
			return err
		}
	}

	return batch.Send()
}
