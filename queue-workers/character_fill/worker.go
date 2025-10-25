package character_fill

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"raidhub/queue-workers"
	"raidhub/shared/bungie"

	amqp "github.com/rabbitmq/amqp091-go"
)

func process_request(qw *queueworkers.QueueWorker, msg amqp.Delivery) {
	qw.Wg.Wait()
	defer func() {
		if err := msg.Ack(false); err != nil {
			log.Printf("Failed to acknowledge message: %v", err)
		}
	}()

	var request CharacterFillRequest
	if err := json.Unmarshal(msg.Body, &request); err != nil {
		log.Fatalf("Failed to unmarshal message: %s", err)
		return
	}

	var currentClassHash sql.NullInt64
	err := qw.Db.QueryRow("SELECT class_hash FROM instance_character WHERE membership_id = $1 AND character_id = $2 AND instance_id = $3 LIMIT 1", request.MembershipId, request.CharacterId, request.InstanceId).Scan(&currentClassHash)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	if currentClassHash.Valid {
		return
	}

	var classHash uint32 = 0
	err = qw.Db.QueryRow("SELECT class_hash FROM instance_character WHERE membership_id = $1 AND character_id = $2 AND class_hash IS NOT NULL LIMIT 1", request.MembershipId, request.CharacterId).Scan(&classHash)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal("Failed to select class hash", err)
	} else if classHash == 0 {
		var membershipTypeValue sql.NullInt32
		err = qw.Db.QueryRow("SELECT membership_type FROM player WHERE membership_id = $1", request.MembershipId).Scan(&membershipTypeValue)
		if err != nil {
			log.Fatal(err)
		}

		var membershipType int32
		if membershipTypeValue.Valid && membershipTypeValue.Int32 > 0 {
			log.Printf("character %d, membership type identified: %d", request.CharacterId, membershipTypeValue.Int32)
			if membershipTypeValue.Int32 == 4 {
				return
			}
			membershipType = membershipTypeValue.Int32
		} else {
			log.Println("character membership type not found")
			profiles, err := bungie.GetLinkedProfiles(-1, request.MembershipId, false)
			if err != nil {
				log.Printf("Failed to get linked profile: %s", err)
				return
			}
			for _, p := range profiles {
				if p.MembershipId == request.MembershipId {
					membershipType = int32(p.MembershipType)
					break
				}
			}
		}
		char, err := bungie.GetCharacter(membershipType, request.MembershipId, request.CharacterId)
		if err != nil {
			log.Printf("Failed to get character: %s", err)
			return
		}

		if char.Character == nil {
			log.Printf("Failed to get character: %s", fmt.Errorf("no character component in the response"))
			return
		}

		classHash = char.Character.Data.ClassHash
	}

	_, err = qw.Db.Exec(`UPDATE instance_character
		SET class_hash = $1
		WHERE membership_id = $2 AND character_id = $3 AND instance_id = $4`,
		classHash, request.MembershipId, request.CharacterId, request.InstanceId)
	if err != nil {
		log.Fatal("Failed to update instance_character", err)
	}

	log.Printf("Updated instance,character: %d,%d", request.InstanceId, request.CharacterId)
}
